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
	"github.com/renproject/lightnode/store"
	"github.com/renproject/lightnode/testutils"
	"github.com/republicprotocol/darknode-go/server/jsonrpc"
	"github.com/republicprotocol/renp2p-go/core/peer"
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

			// Construct 5 random peers for the response message.
			peers := make([]string, numPeers)
			for i := range peers {
				peers[i] = fmt.Sprintf("/ip4/0.0.0.0/tcp/888%d/ren/8MKXcuQAjR2eEq8bsSHDPkYEmqmjt%s", i, string('A'+i))
			}

			resp := jsonrpc.QueryPeersResponse{
				Peers: peers,
			}
			respBytes, err := json.Marshal(resp)
			Expect(err).ToNot(HaveOccurred())

			switch request.Method {
			case jsonrpc.MethodQueryPeers:
				response.Result = json.RawMessage(respBytes)
			default:
				panic("unknown message type")
			}

			time.Sleep(100 * time.Millisecond)
			Expect(json.NewEncoder(w).Encode(response)).To(Succeed())
		})
		server := &http.Server{Addr: address, Handler: handler}

		go func() {
			defer GinkgoRecover()
			Expect(func() {
				server.ListenAndServe()
			}).NotTo(Panic())
		}()

		return server
	}

	// Construct mock Darknode servers and initialise P2P task.
	initTask := func(done chan struct{}, numPeers, numBootstrapAddresses int) (tau.Task, store.KVStore, []*http.Server) {
		servers := make([]*http.Server, numBootstrapAddresses+1)

		// Intialise Darknode.
		server := initServer("0.0.0.0:8000", numPeers)
		servers[0] = server

		multi, err := testutils.ServerMultiAddress(server)
		Expect(err).ToNot(HaveOccurred())
		store, err := testutils.InitStore(multi)
		Expect(err).ToNot(HaveOccurred())

		// Intialise Bootstrap nodes.
		bootstrapAddrs := make([]peer.MultiAddr, numBootstrapAddresses)
		for i := range bootstrapAddrs {
			bootstrapServer := initServer(fmt.Sprintf("0.0.0.0:800%d", i+1), numPeers)
			servers[i+1] = bootstrapServer

			multiAddr, err := testutils.ServerMultiAddress(bootstrapServer)
			Expect(err).ToNot(HaveOccurred())
			bootstrapAddrs[i] = multiAddr
		}

		// Initialise the P2P task.
		logger := logrus.New()
		p2p := New(logger, 128, time.Second, store, bootstrapAddrs, 5*time.Minute, 5)
		go func() {
			defer GinkgoRecover()
			p2p.Run(done)
		}()

		return p2p, store, servers
	}

	Context("when sending a query peers message", func() {
		It("should return the correct number of peers", func() {
			numPeers := 5
			numBootstrapAddresses := 2
			done := make(chan struct{})
			defer close(done)

			p2p, _, servers := initTask(done, numPeers, numBootstrapAddresses)
			for _, server := range servers {
				defer server.Close()
			}

			// Wait for it to P2P task to query the Bootstrap nodes and update its store.
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

			p2p, _, servers := initTask(done, numPeers, numBootstrapAddresses)
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
})
