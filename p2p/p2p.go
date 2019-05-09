// Package p2p defines the P2P task, which maintains the network information for the Darknodes. The task pings the
// Bootstrap nodes on a regular interval and subsequently updates the multi-address store and the health store.
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
	"github.com/republicprotocol/darknode-go/health"
	"github.com/republicprotocol/darknode-go/rpc/jsonrpc"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/republicprotocol/renp2p-go/foundation/addr"
	"github.com/republicprotocol/tau"
	"github.com/sirupsen/logrus"
)

// P2P handles the peer-to-peer network of nodes.
type P2P struct {
	peerCount      int
	bootstrapAddrs []peer.MultiAddr
	logger         logrus.FieldLogger
	store          store.Proxy
	health         health.HealthCheck
	timeout        time.Duration
	pollRate       time.Duration
}

// New returns a new P2P task. `pollRate` is the amount of time to sleep after each round of Darknode queries.
// `peerCount` is the number of multi-addresses that should be returned when querying for peers.
func New(logger logrus.FieldLogger, cap int, timeout time.Duration, store store.Proxy, health health.HealthCheck, bootstrapAddrs []peer.MultiAddr, pollRate time.Duration, peerCount int) tau.Task {
	p2p := &P2P{
		peerCount:      peerCount,
		bootstrapAddrs: bootstrapAddrs,
		logger:         logger,
		store:          store,
		health:         health,
		timeout:        timeout,
		pollRate:       pollRate,
	}

	// Start background polling service.
	go p2p.run()

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
// to the responder channel in the request.
func (p2p *P2P) handleQuery(request jsonrpc.Request) {
	switch req := request.(type) {
	case jsonrpc.QueryPeersRequest:
		req.Responder <- p2p.handleQueryPeers(req)
	case jsonrpc.QueryNumPeersRequest:
		req.Responder <- p2p.handleQueryNumPeers(req)
	case jsonrpc.QueryStatsRequest:
		req.Responder <- p2p.handleQueryStats(req)
	default:
		p2p.logger.Panicf("unexpected message type %T", request)
	}
}

// run starts a background routine querying the Bootstrap nodes for their peers and health information. Upon receiving
// responses, we update the stats store with the health information and the multi-address store with the address of the
// node we queried, as well as any nodes it returns. If we do not receive a response, we remove it from the store if it
// previously existed. After the querying is complete, this service sleeps for `pollRate` amount of time before querying
// the nodes again.
func (p2p *P2P) run() {
	ticker := time.NewTicker(p2p.pollRate)
	defer ticker.Stop()

	for range ticker.C {
		go p2p.updateMultiAddress()
		go p2p.updateStats()
	}
}

// updateMultiAddress queries the Bootstrap nodes for their peers.
func (p2p *P2P) updateMultiAddress() {
	peersRequest := jsonrpc.JSONRequest{
		JSONRPC: "2.0",
		Method:  jsonrpc.MethodQueryPeers,
		ID:      rand.Int31(),
	}

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
	})
}

// updateStats pings the Bootstrap nodes to update their heath stats.
func (p2p *P2P) updateStats() {
	// TODO: We only update the health stats for the darknodes for now. In the future, we might need to change this to
	//  loop through the multiAddress store to get stats for all the peers we know.
	co.ParForAll(p2p.bootstrapAddrs, func(i int) {
		// Construct the JSON-RPC request
		multi := p2p.bootstrapAddrs[i]
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

		// Send request to the node to retrieve its health.
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

	// Return at most p2p.peerCount addresses.
	if len(addresses) < p2p.peerCount {
		return addresses
	}
	return addresses[:p2p.peerCount]
}

// handleQueryNumPeers retrieves the number of multi-addresses in the store.
func (p2p *P2P) handleQueryNumPeers(request jsonrpc.QueryNumPeersRequest) jsonrpc.Response {
	return jsonrpc.QueryNumPeersResponse{
		NumPeers: p2p.store.MultiAddressEntries(),
	}
}

// handleQueryStats retrieves the stats for the given Darknode address from the store.
func (p2p *P2P) handleQueryStats(request jsonrpc.QueryStatsRequest) jsonrpc.Response {
	// If no Darknode ID is provided, return the stats for the Lightnode.
	if request.DarknodeID == "" {
		response := jsonrpc.QueryStatsResponse{
			Version: p2p.health.Version(),
		}
		cpus, err := p2p.health.CPUs()
		if err != nil {
			response.Error = err
			return response
		}
		ram, err := p2p.health.RAM()
		if err != nil {
			response.Error = err
			return response
		}
		disk, err := p2p.health.HardDrive()
		if err != nil {
			response.Error = err
			return response
		}
		location, err := p2p.health.Location()
		if err != nil {
			response.Error = err
			return response
		}

		response.CPUs = cpus
		response.RAM = ram
		response.Disk = disk
		response.Location = location
		return response
	}
	return p2p.store.Stats(addr.New(request.DarknodeID))
}
