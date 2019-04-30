package p2p_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/p2p"

	"github.com/renproject/lightnode/rpc"
	"github.com/renproject/lightnode/testutils"
	"github.com/republicprotocol/darknode-go/server/jsonrpc"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/sirupsen/logrus"
)

var _ = Describe("RPC client", func() {
	// Construct a mock Darknode server.
	initServer := func(address string) *http.Server {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var request jsonrpc.JSONRequest
			Expect(json.NewDecoder(r.Body).Decode(&request)).To(Succeed())

			response := jsonrpc.JSONResponse{
				JSONRPC: "2.0",
				ID:      request.ID,
			}

			peers := make([]peer.MultiAddr, 3)
			for i := range peers {
				multi, err := peer.NewMultiAddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/500%d/ren/8MKXcuQAjR2eEq8bsSHDPkYEmqmjt%s", i, string('A'-1+i)), 1, [65]byte{})
				Expect(err).ToNot(HaveOccurred())
				peers[i] = multi
			}
			peersBytes, err := json.Marshal(peers)
			Expect(err).ToNot(HaveOccurred())

			switch request.Method {
			case jsonrpc.MethodQueryPeers:
				response.Result = json.RawMessage(peersBytes)
			default:
				panic("unknown message type")
			}

			time.Sleep(100 * time.Millisecond)
			Expect(json.NewEncoder(w).Encode(response)).To(Succeed())
		})
		server := &http.Server{Addr: address, Handler: handler}

		go func() {
			defer GinkgoRecover()
			Expect(server.ListenAndServe()).To(Succeed())
		}()

		return server
	}

	Context("when the task receives a Tick message", func() {
		It("should update the multi-address store", func() {
			// Intialise Darknode.
			done := make(chan struct{})
			defer close(done)
			server := initServer("0.0.0.0:5000")
			defer server.Close()
			multi, err := testutils.ServerMultiAddress(server)
			Expect(err).ToNot(HaveOccurred())
			store, err := testutils.InitStore(multi)
			Expect(err).ToNot(HaveOccurred())

			bootstrapAddrs := make([]peer.MultiAddr, 2)
			for i := range bootstrapAddrs {
				bootstrapServer := initServer(fmt.Sprintf("0.0.0.0:500%d", i+1))
				defer bootstrapServer.Close()

				addr, err := testutils.ServerMultiAddress(bootstrapServer)
				Expect(err).ToNot(HaveOccurred())
				bootstrapAddrs[i] = addr
			}

			// Initialise the P2P task.
			logger := logrus.New()
			p2p := New(logger, 128, time.Second, store, bootstrapAddrs)
			go func() {
				defer GinkgoRecover()
				p2p.Run(done)
			}()

			// Send a request to the task.
			p2p.IO().InputWriter() <- Tick{}

			time.Sleep(1 * time.Second)

			responder := make(chan jsonrpc.Response, 1)
			p2p.IO().InputWriter() <- rpc.QueryPeersMessage{
				Request: jsonrpc.QueryPeersRequest{
					Responder: responder,
				},
			}

			// Expect to receive a response from the responder channel.
			select {
			case response := <-responder:
				resp, ok := response.(jsonrpc.QueryPeersResponse)
				Expect(ok).To(BeTrue())
				fmt.Println(resp)
			}
		})
	})
})
