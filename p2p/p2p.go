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

func New(logger logrus.FieldLogger, cap int, timeout time.Duration, store store.KVStore, bootstrapAddrs []peer.MultiAddr) tau.Task {
	p2p := &P2P{
		timeout:        timeout,
		logger:         logger,
		store:          store,
		bootstrapAddrs: bootstrapAddrs,
	}
	return tau.New(tau.NewIO(cap), p2p)
}

func (p2p *P2P) Reduce(message tau.Message) tau.Message {
	switch message := message.(type) {
	case Tick:
		return p2p.handleTick(message)
	case rpc.QueryPeersMessage:
		return p2p.handleQuery(message)
	default:
		panic(fmt.Errorf("unexpected message type %T", message))
	}
}

func (p2p *P2P) handleTick(message tau.Message) tau.Message {
	// TODO: Fix the version and ID
	request := jsonrpc.JSONRequest{
		JSONRPC: "2.0",
		Version: "0.1",
		Method:  jsonrpc.MethodQueryPeers,
		ID:      rand.Int31(),
	}

	go func() {
		co.ParForAll(p2p.bootstrapAddrs, func(i int) {
			multi := p2p.bootstrapAddrs[i]
			client := jrpc.NewClient(p2p.timeout)
			addr := multi.ResolveTCPAddr().(*net.TCPAddr)
			// addr.Port = 18515
			response, err := client.Call(fmt.Sprintf("http://%v", addr.String()), request)
			if err != nil {
				p2p.logger.Errorf("cannot connect to node %v: %v", multi.Addr().String(), err)
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
				// TODO: Handle error
			}

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

func (p2p *P2P) handleQuery(message rpc.QueryPeersMessage) tau.Message {
	var response jsonrpc.Response
	var responder chan<- jsonrpc.Response
	switch request := message.Request.(type) {
	case jsonrpc.QueryPeersRequest:
		iter := p2p.store.Iterator()
		addresses := make([]string, 0, p2p.store.Entries())
		for iter.Next() {
			var value peer.MultiAddr
			_, err := iter.KV(&value)
			if err != nil {
				p2p.logger.Errorf("cannot read multi-address using iterator: %v", err)
			}
			addresses = append(addresses, value.Value())
		}

		indices := rand.Perm(len(addresses))

		// Return at most 5 addresses from the list.
		var randAddresses []string
		if len(addresses) > 5 {
			randAddresses = make([]string, 5)
		} else {
			randAddresses = make([]string, len(addresses))
		}
		for i := range randAddresses {
			randAddresses[i] = addresses[indices[i]]
		}
		response = jsonrpc.QueryPeersResponse{
			Peers: randAddresses,
		}
		responder = request.Responder
	case jsonrpc.QueryNumPeersRequest:
		entries := p2p.store.Entries()
		response = jsonrpc.QueryNumPeersResponse{
			NumPeers: entries,
		}
		responder = request.Responder
	default:
		panic("unknown query request type") // TODO: Should this be a panic?
	}

	select {
	case responder <- response:
		return nil
	case <-time.After(time.Second):
		p2p.logger.Debug("failed to write response to responder channel")
		return nil
	}
}

type Tick struct {
}

func (message Tick) IsMessage() {
}
