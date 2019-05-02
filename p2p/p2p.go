// Package p2p defines the p2p tau.task. It maintains the peer-to-peer network of all the darknodes. The task tries to
// ping all bootstrap nodes after receiving a `Tick`message. It updates the multi-address store with the results it gets
// back. The results will be cached for a certain amount of time.

package p2p

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/renproject/lightnode/rpc"
	jrpc "github.com/renproject/lightnode/rpc/jsonrpc"
	"github.com/renproject/lightnode/store"
	"github.com/republicprotocol/co-go"
	"github.com/republicprotocol/darknode-go/server/jsonrpc"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/republicprotocol/tau"
	"github.com/sirupsen/logrus"
)

// P2P handles the peer-to-peer network of nodes.
type P2P struct {
	timeout        time.Duration
	bootstrapAddrs []peer.MultiAddr
	logger         logrus.FieldLogger
	store          store.KVStore
}

// New returns a `tau.Task`.
func New(logger logrus.FieldLogger, cap int, timeout time.Duration, store store.KVStore, bootstrapAddrs []peer.MultiAddr) tau.Task {
	p2p := &P2P{
		timeout:        timeout,
		logger:         logger,
		store:          store,
		bootstrapAddrs: bootstrapAddrs,
	}
	return tau.New(tau.NewIO(cap), p2p)
}

// Reduce implements the `tau.Task` interface.
func (p2p *P2P) Reduce(message tau.Message) tau.Message {
	switch message := message.(type) {
	case Tick:
		return p2p.handleTick(message)
	case rpc.QueryMessage:
		return p2p.handleQuery(message)
	default:
		p2p.logger.Panicf("unexpected message type %T", message)
	}
	return nil
}

// handleTick will query all the bootstrap nodes. If we got response from the bootstrap node, we update its
// multi-address in the store, otherwise we delete it from the store. We also add all the multi-address returned by the
// bootstrap node to the store.
func (p2p *P2P) handleTick(message tau.Message) tau.Message {
	request := jsonrpc.JSONRequest{
		JSONRPC: "2.0",
		Method:  jsonrpc.MethodQueryPeers,
		ID:      rand.Int31(),
	}

	go func() {
		co.ParForAll(p2p.bootstrapAddrs, func(i int) {
			// Get the net.Addr of the bootstrap node. We assume the jsonrpcPort = grpcPort + 1
			multi := p2p.bootstrapAddrs[i]
			client := jrpc.NewClient(p2p.timeout)
			addr := multi.ResolveTCPAddr().(*net.TCPAddr)
			addr.Port += 1

			// Send the JSON-RPC request.
			response, err := client.Call(fmt.Sprintf("http://%v", addr.String()), request)
			if err != nil {
				p2p.logger.Warnf("cannot connect to node %v: %v", multi.Addr().String(), err)
				if err := p2p.store.Delete(multi.Addr().String()); err != nil {
					p2p.logger.Errorf("cannot delete multi-address from store: %v", err)
				}
				return
			}
			if err := p2p.store.Write(multi.Addr().String(), multi); err != nil {
				p2p.logger.Errorf("cannot add multi-address to store: %v", err)
				return
			}
			if response.Error != nil {
				p2p.logger.Warnf("received error in response: code = %v, message = %v, data = %v", response.Error.Code, response.Error.Message, string(response.Error.Data))
				return
			}

			// Parse the response and add the multi-addresses returned by the bootstrap node.
			var result jsonrpc.QueryPeersResponse
			if err := json.Unmarshal(response.Result, &result); err != nil {
				p2p.logger.Errorf("invalid QueryPeersResponse from node %v: %v", multi.Addr().String(), err)
				return
			}
			for _, node := range result.Peers {
				multiAddr, err := peer.NewMultiAddr(node, 0, [65]byte{})
				if err != nil {
					p2p.logger.Errorf("invalid QueryPeersResponse from node %v: %v", multi.Addr().String(), err)
					return
				}
				if err := p2p.store.Write(multiAddr.Addr().String(), multiAddr); err != nil {
					p2p.logger.Errorf("cannot add multi-address to store: %v", err)
					return
				}
			}
		})
		p2p.logger.Infof("connecting to %v darknodes", p2p.store.Entries())
	}()

	return nil
}

// handleQuery will retrieve the data from the store and write the response to the query to the responder channel.
func (p2p *P2P) handleQuery(message rpc.QueryMessage) tau.Message {
	var response jsonrpc.Response
	var responder chan<- jsonrpc.Response
	switch request := message.Request.(type) {
	// Return 5 random addresses from the multi-address store.
	case jsonrpc.QueryPeersRequest:
		iter := p2p.store.Iterator()
		addresses := p2p.randomPeers(iter, 5)
		response = jsonrpc.QueryPeersResponse{
			Peers: addresses,
		}
		responder = request.Responder
	// Return the number of entries in the store.
	case jsonrpc.QueryNumPeersRequest:
		entries := p2p.store.Entries()
		response = jsonrpc.QueryNumPeersResponse{
			NumPeers: entries,
		}
		responder = request.Responder
	default:
		p2p.logger.Panicf("unknown query request type: %T", request)
	}

	// Defensive check
	if responder == nil {
		p2p.logger.Error("responder channel is nil which should not happen")
	} else {
		responder <- response
	}

	return nil
}

// randomPeers returns at most 5 random multi-addresses from the store.
func (p2p *P2P) randomPeers(iter store.KVStoreIterator, num int) []string {
	// Get all the darknodes stored in the KVStore.
	addresses := make([]string, 0, p2p.store.Entries())
	for iter.Next() {
		var value peer.MultiAddr
		_, err := iter.KV(&value)
		if err != nil {
			p2p.logger.Errorf("cannot read multi-address using iterator: %v", err)
			continue
		}
		addresses = append(addresses, value.Value())
	}

	// Shuffle the list
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(addresses), func(i, j int) {
		addresses[i], addresses[j] = addresses[j], addresses[i]
	})

	// Return at most 5 darknode addresses.
	length := 5
	if len(addresses) < 5 {
		length = len(addresses)
	}

	return addresses[:length]
}

// Tick message is periodically generated by the parent task. The p2p task will try to update the cached multi-store
// while receiving this message.
type Tick struct {
}

// IsMessage implements the `tau.Message` interface
func (message Tick) IsMessage() {
}
