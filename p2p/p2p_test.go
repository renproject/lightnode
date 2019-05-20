package p2p_test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/p2p"

	"github.com/renproject/lightnode/rpc"
	"github.com/renproject/lightnode/testutils"
	"github.com/republicprotocol/darknode-go/health"
	"github.com/republicprotocol/darknode-go/rpc/jsonrpc"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/republicprotocol/renp2p-go/foundation/addr"
	"github.com/republicprotocol/store"
	"github.com/republicprotocol/tau"
	"github.com/sirupsen/logrus"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var _ = Describe("RPC client", func() {
	// Construct a mock Darknode server.
	initServer := func(address string, numPeers int) *http.Server {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var request jsonrpc.JSONRequest
			Expect(json.NewDecoder(r.Body).Decode(&request)).To(Succeed())

			response := jsonrpc.JSONResponse{
				JSONRPC: "2.0",
				ID:      request.ID,
			}

			switch request.Method {
			case jsonrpc.MethodQueryPeers:
				// Construct 5 random peers for the response message.
				peers := make([]string, numPeers)
				for i := range peers {
					peers[i] = fmt.Sprintf("/ip4/0.0.0.0/tcp/888%d/ren/8MKXcuQAjR2eEq8bsSHDPkYEmqmjt%s", i, string('A'+i))
				}

				peersResp := jsonrpc.QueryPeersResponse{
					Peers: peers,
				}
				peersRespBytes, err := json.Marshal(peersResp)
				Expect(err).ToNot(HaveOccurred())

				response.Result = json.RawMessage(peersRespBytes)
			case jsonrpc.MethodQueryStats:
				statsResp := jsonrpc.QueryStatsResponse{
					Info: health.Info{
						RAM:       1,
						HardDrive: 1,
						Location:  "Canberra",
						Version:   "1",
					},
				}
				statsRespBytes, err := json.Marshal(statsResp)
				Expect(err).ToNot(HaveOccurred())

				response.Result = json.RawMessage(statsRespBytes)
			default:
				panic("unknown message type")
			}

			time.Sleep(100 * time.Millisecond)
			Expect(json.NewEncoder(w).Encode(response)).To(Succeed())
		})
		server := &http.Server{Addr: address, Handler: handler}

		go server.ListenAndServe()

		return server
	}

	// Construct mock Darknode servers and initialise P2P task.
	initTask := func(done chan struct{}, numPeers, numBootstrapAddresses int) (tau.Task, []*http.Server, peer.MultiAddrs) {
		servers := make([]*http.Server, numBootstrapAddresses+1)

		// Initialise Darknode.
		server := initServer("0.0.0.0:8000", numPeers)
		servers[0] = server

		multi, err := testutils.ServerMultiAddress(server)
		Expect(err).ToNot(HaveOccurred())
		multiStore, err := testutils.InitStore(multi)
		Expect(err).ToNot(HaveOccurred())

		// Initialise Bootstrap nodes.
		bootstrapAddrs := make(peer.MultiAddrs, numBootstrapAddresses)
		for i := range bootstrapAddrs {
			bootstrapServer := initServer(fmt.Sprintf("0.0.0.0:800%d", i+1), numPeers)
			servers[i+1] = bootstrapServer

			multiAddr, err := testutils.ServerMultiAddress(bootstrapServer)
			Expect(err).ToNot(HaveOccurred())
			bootstrapAddrs[i] = multiAddr
		}

		// Initialise the P2P task.
		logger := logrus.New()
		store := NewProxy(multiStore, store.NewIterableCache(0))
		health := health.NewHealthCheck("1.0", addr.New(""))
		p2p := New(logger, 128, 5, time.Second, 5*time.Minute, store, health, bootstrapAddrs)
		go p2p.Run(done)

		return p2p, servers, bootstrapAddrs
	}

	Context("when sending a query peers message", func() {
		It("should return the correct number of peers", func() {
			numPeers := 5
			numBootstrapAddresses := 2
			done := make(chan struct{})
			defer close(done)

			p2p, servers, _ := initTask(done, numPeers, numBootstrapAddresses)
			for _, server := range servers {
				defer server.Close()
			}

			// Wait for the P2P task to query the Bootstrap nodes and update its store.
			time.Sleep(1 * time.Second)

			// Send a QueryPeers message to the task.
			responder := make(chan jsonrpc.Response, 1)
			p2p.IO().InputWriter() <- rpc.QueryMessage{
				Request: jsonrpc.QueryPeersRequest{
					Responder: responder,
				},
			}

			// Expect to receive a response from the responder channel.
			select {
			case response := <-responder:
				resp, ok := response.(jsonrpc.QueryPeersResponse)
				Expect(ok).To(BeTrue())
				Expect(len(resp.Peers)).To(Equal(5))
			}
		})

		It("should return no peers if the P2P task has not finished querying the Bootstrap nodes", func() {
			numPeers := 5
			numBootstrapAddresses := 2
			done := make(chan struct{})
			defer close(done)

			p2p, servers, _ := initTask(done, numPeers, numBootstrapAddresses)
			for _, server := range servers {
				defer server.Close()
			}

			// Send a QueryPeers message to the task.
			responder := make(chan jsonrpc.Response, 1)
			p2p.IO().InputWriter() <- rpc.QueryMessage{
				Request: jsonrpc.QueryPeersRequest{
					Responder: responder,
				},
			}

			// Expect to receive a response from the responder channel.
			select {
			case response := <-responder:
				resp, ok := response.(jsonrpc.QueryPeersResponse)
				Expect(ok).To(BeTrue())

				// Note: the store for a server contains its own multi-address.
				Expect(len(resp.Peers)).To(Equal(1))
			}
		})
	})

	Context("when sending a query num peers message", func() {
		It("should return the correct number of peers", func() {
			numPeers := 5
			numBootstrapAddresses := 2
			done := make(chan struct{})
			defer close(done)

			p2p, servers, _ := initTask(done, numPeers, numBootstrapAddresses)
			for _, server := range servers {
				defer server.Close()
			}

			// Wait for the P2P task to query the Bootstrap nodes and update its store.
			time.Sleep(1 * time.Second)

			// Send a QueryPeers message to the task.
			responder := make(chan jsonrpc.Response, 1)
			p2p.IO().InputWriter() <- rpc.QueryMessage{
				Request: jsonrpc.QueryNumPeersRequest{
					Responder: responder,
				},
			}

			// Expect to receive a response from the responder channel.
			select {
			case response := <-responder:
				resp, ok := response.(jsonrpc.QueryNumPeersResponse)
				Expect(ok).To(BeTrue())
				Expect(resp.NumPeers).To(Equal(8))
			}
		})
	})

	Context("when sending a query stats message", func() {
		It("should return the stats for a darknode", func() {
			numPeers := 5
			numBootstrapAddresses := 2
			done := make(chan struct{})
			defer close(done)

			p2p, servers, bootstrapAddrs := initTask(done, numPeers, numBootstrapAddresses)
			for _, server := range servers {
				defer server.Close()
			}

			// Wait for the P2P task to query the Bootstrap nodes and update its store.
			time.Sleep(1 * time.Second)

			// Send a QueryPeers message to the task.
			responder := make(chan jsonrpc.Response, 1)
			p2p.IO().InputWriter() <- rpc.QueryMessage{
				Request: jsonrpc.QueryStatsRequest{
					DarknodeID: bootstrapAddrs[0].Addr().String(),
					Responder:  responder,
				},
			}

			// Expect to receive a response from the responder channel.
			select {
			case response := <-responder:
				resp, ok := response.(jsonrpc.QueryStatsResponse)
				Expect(ok).To(BeTrue())
				Expect(resp.Info.Version).To(Equal("1"))
				Expect(resp.Info.RAM).To(Equal(1))
				Expect(resp.Info.HardDrive).To(Equal(1))
				Expect(resp.Info.Location).To(Equal("Canberra"))
			}
		})

		It("should return the stats for the lightnode", func() {
			numPeers := 5
			numBootstrapAddresses := 2
			done := make(chan struct{})
			defer close(done)

			p2p, servers, _ := initTask(done, numPeers, numBootstrapAddresses)
			for _, server := range servers {
				defer server.Close()
			}

			// Wait for the P2P task to query the Bootstrap nodes and update its store.
			time.Sleep(1 * time.Second)

			// Send a QueryPeers message to the task.
			responder := make(chan jsonrpc.Response, 1)
			p2p.IO().InputWriter() <- rpc.QueryMessage{
				Request: jsonrpc.QueryStatsRequest{
					Responder: responder,
				},
			}

			// Expect to receive a response from the responder channel.
			select {
			case response := <-responder:
				resp, ok := response.(jsonrpc.QueryStatsResponse)
				Expect(ok).To(BeTrue())
				Expect(resp.Error).Should(BeNil())
				Expect(resp.Info.RAM).Should(BeNumerically(">", 0))
				Expect(resp.Info.HardDrive).Should(BeNumerically(">", 0))
				Expect(len(resp.Info.CPUs)).Should(BeNumerically(">", 0))
			}
		})
	})

	Context("when bootstrap darknodes are offline", func() {
		It("should be deleted from the store after the query fails", func() {
			done := make(chan struct{})
			defer close(done)

			// Generate random number of bootstraps nodes
			bootstraps := make([]peer.MultiAddr, rand.Intn(10))
			for i := range bootstraps {
				var err error
				bootstraps[i], err = testutils.RandomMultiAddress()
				Expect(err).NotTo(HaveOccurred())
			}

			// Initialise the P2P task.
			logger := logrus.New()
			multiStore, err := testutils.InitStore()
			Expect(err).ToNot(HaveOccurred())
			statsStore := store.NewIterableCache(0)
			store := NewProxy(multiStore, statsStore)
			health := health.NewHealthCheck("1.0", addr.New(""))
			p2p := New(logger, 128, 5, time.Second, 5*time.Minute, store, health, bootstraps)

			go p2p.Run(done)

			// Expect all bootstrap nodes been removes from both the mutli-store and the stats store.
			time.Sleep(3 * time.Second)
			multis, err := store.MultiAddrs()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(multis)).Should(BeZero())
			iter := statsStore.Iterator()
			Expect(iter.Next()).Should(BeFalse())
		})
	})
})
