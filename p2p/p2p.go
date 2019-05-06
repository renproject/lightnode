// Package p2p defines the P2P task, which maintains the network information for the Darknodes. The task pings the
// Bootstrap nodes upon receiving a `Tick` message and subsequently updates the multi-address store.
package p2p

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"time"

	jrpc "github.com/renproject/lightnode/rpc/jsonrpc"
	"github.com/renproject/lightnode/store"
	"github.com/republicprotocol/co-go"
	"github.com/republicprotocol/darknode-go/server/jsonrpc"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/republicprotocol/tau"
	"github.com/sirupsen/logrus"
)

var (
	// ErrInvalidDarknodeID is returned when we do not have health information for a particular Darknode in the store.
	ErrInvalidDarknodeID = errors.New("invalid darknode ID")
)

// P2P handles the peer-to-peer network of nodes.
type P2P struct {
	timeout        time.Duration
	bootstrapAddrs []peer.MultiAddr
	logger         logrus.FieldLogger
	multiStore     store.KVStore
	statsStore     store.KVStore
}

// New returns a new P2P task.
func New(logger logrus.FieldLogger, cap int, timeout time.Duration, multiStore, statsStore store.KVStore, bootstrapAddrs []peer.MultiAddr) tau.Task {
	p2p := &P2P{
		timeout:        timeout,
		logger:         logger,
		multiStore:     multiStore,
		statsStore:     statsStore,
		bootstrapAddrs: bootstrapAddrs,
	}
	return tau.New(tau.NewIO(cap), p2p)
}

// Reduce implements the `tau.Task` interface.
func (p2p *P2P) Reduce(message tau.Message) tau.Message {
	switch message := message.(type) {
	case Tick:
		return p2p.handleTick(message)
	case InvokeQuery:
		return p2p.invoke(message)
	default:
		p2p.logger.Panicf("unexpected message type %T", message)
	}
	return nil
}

// invoke retrieves the result for the query by delegating the request to a helper function and writes the result to the
// responder channel in the request. If the queue is full, the message will be dropped.
func (p2p *P2P) invoke(message InvokeQuery) tau.Message {
	var response jsonrpc.Response
	var responder chan<- jsonrpc.Response
	switch request := message.Request.(type) {
	case jsonrpc.QueryPeersRequest:
		response = p2p.handleQueryPeers(request)
		responder = request.Responder
	case jsonrpc.QueryNumPeersRequest:
		response = p2p.handleQueryNumPeers(request)
		responder = request.Responder
	case jsonrpc.QueryStatsRequest:
		response = p2p.handleQueryStats(request)
		responder = request.Responder
	default:
		p2p.logger.Panicf("unexpected message type %T", request)
	}

	responder <- response
	return nil
}

// handleTick queries the Bootstrap nodes for their peers and health information. Upon receiving responses, we update
// the stats store with the health information and the multi-address store with the address of the node we queried, as
// well as any nodes it returns. If we do not receive a response, we remove it from the store if it previously existed.
func (p2p *P2P) handleTick(message tau.Message) tau.Message {
	peersRequest := jsonrpc.JSONRequest{
		JSONRPC: "2.0",
		Method:  jsonrpc.MethodQueryPeers,
		ID:      rand.Int31(),
	}

	go func() {
		co.ParForAll(p2p.bootstrapAddrs, func(i int) {
			multi := p2p.bootstrapAddrs[i]

			// Send request to the node to retrieve its peers.
			peersResponse := p2p.sendRequest(peersRequest, multi)
			if peersResponse == nil {
				return
			}
			var peersResult jsonrpc.QueryPeersResponse
			if err := json.Unmarshal(peersResponse, &peersResult); err != nil {
				p2p.logger.Errorf("invalid QueryPeersResponse from node %v: %v", multi.Addr().String(), err)
				return
			}

			// Parse the response and write any multi-addresses returned by the node to the store.
			for _, node := range peersResult.Peers {
				multiAddr, err := peer.NewMultiAddr(node, 0, [65]byte{})
				if err != nil {
					p2p.logger.Errorf("invalid QueryPeersResponse from node %v: %v", multi.Addr().String(), err)
					return
				}
				if err := p2p.multiStore.Write(multiAddr.Addr().String(), multiAddr); err != nil {
					p2p.logger.Errorf("cannot add multi-address to store: %v", err)
					return
				}
			}

			// Send request to the node to retrieve its health.
			statsParams := jsonrpc.QueryStatsRequest{
				DarknodeID: multi.Addr().String(),
			}
			statsParamsBytes, err := json.Marshal(statsParams)
			if err != nil {
				p2p.logger.Errorf("cannot marshal stats params: %v", err)
				return
			}
			statsRequest := jsonrpc.JSONRequest{
				JSONRPC: "2.0",
				Method:  jsonrpc.MethodQueryStats,
				Params:  statsParamsBytes,
				ID:      rand.Int31(),
			}
			statsResponse := p2p.sendRequest(statsRequest, multi)
			if statsResponse == nil {
				return
			}
			var statsResult jsonrpc.QueryStatsResponse
			if err := json.Unmarshal(statsResponse, &statsResult); err != nil {
				p2p.logger.Errorf("invalid QueryStatsResponse from node %v: %v", multi.Addr().String(), err)
				return
			}

			// Parse the response and write the node's stats to the store.
			if err := p2p.statsStore.Write(multi.Addr().String(), statsResult); err != nil {
				p2p.logger.Errorf("cannot add stats to store: %v", err)
				return
			}
		})
		p2p.logger.Infof("querying %v darknodes", p2p.multiStore.Entries())
	}()

	return nil
}

// sendRequest sends a JSON-RPC request to the given multi-address and returns the result.
func (p2p *P2P) sendRequest(request jsonrpc.JSONRequest, multi peer.MultiAddr) json.RawMessage {
	// Get the net.Addr of the Bootstrap node. We make the assumption that the JSON-RPC port is equivalent to the gRPC
	// port + 1.
	client := jrpc.NewClient(p2p.timeout)
	addr := multi.ResolveTCPAddr().(*net.TCPAddr)
	addr.Port += 1

	// Send the JSON-RPC request.
	response, err := client.Call(fmt.Sprintf("http://%v", addr.String()), request)
	if err != nil {
		p2p.logger.Warnf("cannot connect to node %v: %v", multi.Addr().String(), err)
		if err := p2p.multiStore.Delete(multi.Addr().String()); err != nil {
			p2p.logger.Errorf("cannot delete multi-address from store: %v", err)
		}
		return nil
	}
	if err := p2p.multiStore.Write(multi.Addr().String(), multi); err != nil {
		p2p.logger.Errorf("cannot add multi-address to store: %v", err)
		return nil
	}
	if response.Error != nil {
		p2p.logger.Warnf("received error in response: code = %v, message = %v, data = %v", response.Error.Code, response.Error.Message, string(response.Error.Data))
		return nil
	}

	return response.Result
}

// handleQueryPeers retrieves at most 5 random multi-addresses from the store.
func (p2p *P2P) handleQueryPeers(request jsonrpc.QueryPeersRequest) jsonrpc.Response {
	iter := p2p.multiStore.Iterator()
	addresses := p2p.randomPeers(iter, 5)
	response := jsonrpc.QueryPeersResponse{
		Peers: addresses,
	}
	return response
}

func (p2p *P2P) randomPeers(iter store.KVStoreIterator, num int) []string {
	// Retrieve all the Darknode multi-addresses in the store.
	addresses := make([]string, 0, p2p.multiStore.Entries())
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

// handleQueryNumPeers retrieves the number of multi-addresses in the store.
func (p2p *P2P) handleQueryNumPeers(request jsonrpc.QueryNumPeersRequest) jsonrpc.Response {
	// Return the number of entries in the store.
	entries := p2p.multiStore.Entries()
	response := jsonrpc.QueryNumPeersResponse{
		NumPeers: entries,
	}
	return response
}

// handleQueryStats retrieves the stats for the given Darknode address from the store.
func (p2p *P2P) handleQueryStats(request jsonrpc.QueryStatsRequest) jsonrpc.Response {
	var response jsonrpc.QueryStatsResponse

	// Return the number of entries in the store.
	if err := p2p.statsStore.Read(request.DarknodeID, &response); err != nil {
		response.Error = ErrInvalidDarknodeID
	}
	return response
}

// Tick message is periodically generated by the parent task. The P2P task updates the multi-addresses in the store upon
// receiving this message.
type Tick struct {
}

// IsMessage implements the `tau.Message` interface.
func (message Tick) IsMessage() {
}

// InvokeRPC is tau.Message contains a `jsonrpc.Request`.
type InvokeQuery struct {
	Request jsonrpc.Request
}

// IsMessage implements the `tau.Message` interface.
func (InvokeQuery) IsMessage() {
}
