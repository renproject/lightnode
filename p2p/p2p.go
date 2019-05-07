// Package p2p defines the P2P task, which maintains the network information for the Darknodes. The task pings the
// Bootstrap nodes on a regular interval and subsequently updates the multi-address store.
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
	"github.com/republicprotocol/renp2p-go/foundation/addr"
	"github.com/republicprotocol/tau"
	"github.com/sirupsen/logrus"
)

// P2P handles the peer-to-peer network of nodes.
type P2P struct {
	timeout        time.Duration
	bootstrapAddrs []peer.MultiAddr
	logger         logrus.FieldLogger
	store          store.Proxy
	pollRate       time.Duration
	multiAddrCount int
}

// New returns a new P2P task.
func New(logger logrus.FieldLogger, cap int, timeout time.Duration, store store.Proxy, bootstrapAddrs []peer.MultiAddr, pollRate time.Duration, multiAddrCount int) tau.Task {
	p2p := &P2P{
		timeout:        timeout,
		logger:         logger,
		store:          store,
		bootstrapAddrs: bootstrapAddrs,
		pollRate:       pollRate,
		multiAddrCount: multiAddrCount,
	}

	// Start background polling service.
	p2p.Run()

	return tau.New(tau.NewIO(cap), p2p)
}

// Reduce implements the `tau.Task` interface.
func (p2p *P2P) Reduce(message tau.Message) tau.Message {
	switch request := message.(type) {
	case rpc.QueryMessage:
		p2p.handleQuery(request.Request)
	default:
		p2p.logger.Panicf("unexpected message type %T", message)
	}
	return nil
}

// handleQuery retrieves the result for the query by delegating the request to a helper function and writes the result
// to the responder channel in the request. If the queue is full, the message will be dropped.
func (p2p *P2P) handleQuery(request jsonrpc.Request) {
	var response jsonrpc.Response
	var responder chan<- jsonrpc.Response

	switch req := request.(type) {
	case jsonrpc.QueryPeersRequest:
		response = p2p.handleQueryPeers(req)
		responder = req.Responder
	case jsonrpc.QueryNumPeersRequest:
		response = p2p.handleQueryNumPeers(req)
		responder = req.Responder
	case jsonrpc.QueryStatsRequest:
		response = p2p.handleQueryStats(req)
		responder = req.Responder
	default:
		p2p.logger.Panicf("unexpected message type %T", request)
	}

	responder <- response
}

// Run starts a background routine querying the Bootstrap nodes for their peers and health information. Upon receiving
// responses, we update the stats store with the health information and the multi-address store with the address of the
// node we queried, as well as any nodes it returns. If we do not receive a response, we remove it from the store if it
// previously existed. After the querying is complete, this service waits for `pollRate` seconds before querying the
// nodes again.
func (p2p *P2P) Run() {
	peersRequest := jsonrpc.JSONRequest{
		JSONRPC: "2.0",
		Method:  jsonrpc.MethodQueryPeers,
		ID:      rand.Int31(),
	}

	go func() {
		for {
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
					if err := p2p.store.InsertMultiAddress(multiAddr.Addr(), multiAddr); err != nil {
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
				if err := p2p.store.InsertStats(multi.Addr(), statsResult); err != nil {
					p2p.logger.Errorf("cannot add stats to store: %v", err)
					return
				}
			})

			p2p.logger.Infof("querying %v darknodes", p2p.store.MultiAddressEntries())

			// Sleep before `pollRate` seconds.
			time.Sleep(p2p.pollRate)
		}
	}()
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
		if err := p2p.store.DeleteMultiAddress(multi.Addr()); err != nil {
			p2p.logger.Errorf("cannot delete multi-address from store: %v", err)
		}
		return nil
	}
	if err := p2p.store.InsertMultiAddress(multi.Addr(), multi); err != nil {
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
	addresses := p2p.randomPeers()
	response := jsonrpc.QueryPeersResponse{
		Peers: addresses,
	}
	return response
}

func (p2p *P2P) randomPeers() []string {
	// Retrieve all the Darknode multi-addresses in the store.
	addresses := p2p.store.MultiAddresses()

	// Shuffle the list.
	rand.Shuffle(len(addresses), func(i, j int) {
		addresses[i], addresses[j] = addresses[j], addresses[i]
	})

	// Return at most 5 addresses.
	length := p2p.multiAddrCount
	if len(addresses) < p2p.multiAddrCount {
		length = len(addresses)
	}

	return addresses[:length]
}

// handleQueryNumPeers retrieves the number of multi-addresses in the store.
func (p2p *P2P) handleQueryNumPeers(request jsonrpc.QueryNumPeersRequest) jsonrpc.Response {
	// Return the number of entries in the store.
	entries := p2p.store.MultiAddressEntries()
	response := jsonrpc.QueryNumPeersResponse{
		NumPeers: entries,
	}
	return response
}

// handleQueryStats retrieves the stats for the given Darknode address from the store.
func (p2p *P2P) handleQueryStats(request jsonrpc.QueryStatsRequest) jsonrpc.Response {
	if request.DarknodeID == "" {
		// TODO: Return Lightnode stats.
	}
	return p2p.store.Stats(addr.New(request.DarknodeID))
}
