// Package p2p defines the P2P task, which maintains the network information for the Darknodes. The task pings the
// Bootstrap nodes upon receiving a `Tick` message and subsequently updates the multi-address store.
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

// New returns a new P2P task.
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

// handleTick queries the Bootstrap nodes for their peers. Upon receiving a response, we update the store with the
// multi-address of the node we queried, as well as any nodes it returns. If we do not receive a response, we remove it
// from the store if it previously existed.
func (p2p *P2P) handleTick(message tau.Message) tau.Message {
	request := jsonrpc.JSONRequest{
		JSONRPC: "2.0",
		Method:  jsonrpc.MethodQueryPeers,
		ID:      rand.Int31(),
	}

	go func() {
		co.ParForAll(p2p.bootstrapAddrs, func(i int) {
			// Get the net.Addr of the Bootstrap node. We make the assumption that the JSON-RPC port is equivalent to
			// the gRPC port + 1.
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

			// Parse the response and write any multi-addresses returned by the node to the store.
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

// handleQuery retrieves multi-addresses from the store and writes the response to the responder channel in the original
// request.
func (p2p *P2P) handleQuery(message rpc.QueryMessage) tau.Message {
	var response jsonrpc.Response
	var responder chan<- jsonrpc.Response
	switch request := message.Request.(type) {
	case jsonrpc.QueryPeersRequest:
		// Return at most 5 random addresses from the multi-address store.
		iter := p2p.store.Iterator()
		addresses := p2p.randomPeers(iter, 5)
		response = jsonrpc.QueryPeersResponse{
			Peers: addresses,
		}
		responder = request.Responder
	case jsonrpc.QueryNumPeersRequest:
		// Return the number of entries in the store.
		entries := p2p.store.Entries()
		response = jsonrpc.QueryNumPeersResponse{
			NumPeers: entries,
		}
		responder = request.Responder
	default:
		p2p.logger.Panicf("unknown query request type: %T", request)
	}

	if responder == nil {
		// This is a defensive check.
		p2p.logger.Error("query responder channel is nil")
	} else {
		responder <- response
	}

	return nil
}

// randomPeers returns at most 5 random multi-addresses from the store.
func (p2p *P2P) randomPeers(iter store.KVStoreIterator, num int) []string {
	// Retrieve all the Darknode multi-addresses in the store.
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

	// Shuffle the list.
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(addresses), func(i, j int) {
		addresses[i], addresses[j] = addresses[j], addresses[i]
	})

	// Return at most 5 addresses.
	length := 5
	if len(addresses) < 5 {
		length = len(addresses)
	}

	return addresses[:length]
}

// Tick message is periodically generated by the parent task. The P2P task updates the multi-addresses in the store upon
// receiving this message.
type Tick struct {
}

// IsMessage implements the `tau.Message` interface.
func (message Tick) IsMessage() {
}
