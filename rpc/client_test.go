package rpc_test

import (
	"encoding/json"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/rpc"

	"github.com/renproject/lightnode/store"
	"github.com/renproject/lightnode/testutils"
	"github.com/republicprotocol/darknode-go/processor"
	"github.com/republicprotocol/darknode-go/rpc/jsonrpc"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/republicprotocol/renp2p-go/foundation/addr"
	"github.com/sirupsen/logrus"
)

var _ = Describe("RPC client", func() {
	initServer := func() *http.Server {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var request jsonrpc.JSONRequest
			Expect(json.NewDecoder(r.Body).Decode(&request)).To(Succeed())

			response := jsonrpc.JSONResponse{
				JSONRPC: "2.0",
				ID:      request.ID,
			}

			switch request.Method {
			case jsonrpc.MethodSendMessage:
				response.Result = json.RawMessage([]byte(`{"messageID":"messageID","ok":true}`))
			case jsonrpc.MethodReceiveMessage:
				response.Result = json.RawMessage([]byte(`{"values":[{"type":"private","value":"0"}]}`))
			default:
				panic("unknown message type")
			}

			time.Sleep(100 * time.Millisecond)
			Expect(json.NewEncoder(w).Encode(response)).To(Succeed())
		})
		server := &http.Server{Addr: "0.0.0.0:18515", Handler: handler}

		go func() {
			defer GinkgoRecover()

			Expect(func() {
				Expect(server.ListenAndServe()).To(Equal(http.ErrServerClosed))
			}).NotTo(Panic())
		}()

		return server
	}

	// Construct a test server that always returns an incorrectly formatted response.
	initErrorServer := func() *http.Server {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := json.NewEncoder(w).Encode(jsonrpc.JSONResponse{
				JSONRPC: "2.0",
				Result:  []byte(""),
			})
			Expect(err).NotTo(HaveOccurred())
		})
		server := &http.Server{Addr: "0.0.0.0:18516", Handler: handler}

		go func() {
			defer GinkgoRecover()

			Expect(func() {
				Expect(server.ListenAndServe()).To(Equal(http.ErrServerClosed))
			}).NotTo(Panic())
		}()

		return server
	}

	initTimeoutServer := func() *http.Server {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(time.Minute)
		})
		server := &http.Server{Addr: "0.0.0.0:18517", Handler: handler}

		go func() {
			defer GinkgoRecover()

			Expect(func() {
				Expect(server.ListenAndServe()).To(Equal(http.ErrServerClosed))
			}).NotTo(Panic())
		}()

		return server
	}

	Context("when we receive an InvokeRPC message", func() {
		It("should get a response from the server for a SendMessageRequest", func() {
			// Initialise darknodes.
			done := make(chan struct{})
			defer close(done)
			server := initServer()
			defer server.Close()
			multi, err := testutils.ServerMultiAddress(server)
			Expect(err).ToNot(HaveOccurred())
			multiStore, err := testutils.InitStore(multi)
			Expect(err).ToNot(HaveOccurred())
			store := store.NewProxy(multiStore, store.NewCache(0), store.NewCache(0))

			// Initialise the client task.
			logger := logrus.New()
			client := NewClient(logger, store, 32, 8, time.Second)
			go client.Run(done)
			responder := make(chan jsonrpc.Response, 1)

			// Send a request to the task.
			for i := 0; i < 32; i++ {
				client.IO().InputWriter() <- InvokeRPC{
					Request: jsonrpc.SendMessageRequest{
						Responder: responder,
					},
					Addresses: []addr.Addr{multi.Addr()},
				}

				// Expect to receive a response from the responder channel.
				select {
				case response := <-responder:
					resp, ok := response.(jsonrpc.SendMessageResponse)
					Expect(ok).To(BeTrue())
					Expect(resp.Ok).To(BeTrue())
				case <-time.After(time.Second):
					Fail("timeout")
				}
			}
		})

		It("should get a response from the server for a ReceiveMessageRequest", func() {
			// Initialise darknodes.
			done := make(chan struct{})
			defer close(done)
			server := initServer()
			defer server.Close()
			multi, err := testutils.ServerMultiAddress(server)
			Expect(err).ToNot(HaveOccurred())
			multiStore, err := testutils.InitStore(multi)
			Expect(err).ToNot(HaveOccurred())
			store := store.NewProxy(multiStore, store.NewCache(0), store.NewCache(0))

			// Initialise the client task.
			logger := logrus.New()
			client := NewClient(logger, store, 32, 8, time.Second)
			go client.Run(done)
			responder := make(chan jsonrpc.Response, 1)

			// Send a request to the task.
			for i := 0; i < 32; i++ {
				client.IO().InputWriter() <- InvokeRPC{
					Request: jsonrpc.ReceiveMessageRequest{
						Responder: responder,
					},
					Addresses: []addr.Addr{multi.Addr()},
				}

				// Expect to receive a response from the responder channel.
				select {
				case response := <-responder:
					resp, ok := response.(jsonrpc.ReceiveMessageResponse)
					Expect(ok).To(BeTrue())

					var params []processor.Param
					Expect(json.Unmarshal(resp.Result, &params)).To(Succeed())
					Expect(len(params)).To(Equal(1))
				case <-time.After(time.Second):
					Fail("timeout")
				}
			}
		})
	})

	Context("when the darknode is offline", func() {
		It("should return proper error ", func() {
			// Initialise Darknode.
			done := make(chan struct{})
			defer close(done)
			multi, err := peer.NewMultiAddr("/ip4/0.0.0.0/tcp/18515/ren/8MKXcuQAjR2eEq8bsSHDPkYEmqmjtj", 1, [65]byte{})
			Expect(err).NotTo(HaveOccurred())
			multiStore, err := testutils.InitStore(multi)
			Expect(err).ToNot(HaveOccurred())
			store := store.NewProxy(multiStore, store.NewCache(0), store.NewCache(0))

			// Initialise the client task.
			logger := logrus.New()
			client := NewClient(logger, store, 32, 8, time.Second)
			go client.Run(done)
			responder := make(chan jsonrpc.Response, 1)

			// Send a request to the task.
			client.IO().InputWriter() <- InvokeRPC{
				Request: jsonrpc.SendMessageRequest{
					Responder: responder,
				},
				Addresses: []addr.Addr{multi.Addr()},
			}

			// Expect to receive a response from the responder channel.
			select {
			case response := <-responder:
				resp, ok := response.(jsonrpc.SendMessageResponse)
				Expect(ok).To(BeTrue())
				Expect(resp.Ok).To(BeFalse())
				Expect(resp.Error).NotTo(BeNil())
			}
		})
	})

	Context("when the darknode gives a bad response", func() {
		It("should return proper error when it's a sendMessage request", func() {
			// Initialise Darknode.
			done := make(chan struct{})
			defer close(done)
			server := initErrorServer()
			defer server.Close()
			multi, err := testutils.ServerMultiAddress(server)
			Expect(err).ToNot(HaveOccurred())
			multiStore, err := testutils.InitStore(multi)
			Expect(err).ToNot(HaveOccurred())
			store := store.NewProxy(multiStore, store.NewCache(0), store.NewCache(0))

			// Initialise the client task.
			logger := logrus.New()
			client := NewClient(logger, store, 32, 8, time.Second)
			go client.Run(done)
			responder := make(chan jsonrpc.Response, 1)

			// Send a request to the task.
			client.IO().InputWriter() <- InvokeRPC{
				Request: jsonrpc.SendMessageRequest{
					Responder: responder,
				},
				Addresses: []addr.Addr{multi.Addr()},
			}

			// Expect to receive a response from the responder channel.
			select {
			case response := <-responder:
				resp, ok := response.(jsonrpc.SendMessageResponse)
				Expect(ok).To(BeTrue())
				Expect(resp.Ok).To(BeFalse())
				Expect(resp.Error).NotTo(BeNil())
			case <-time.After(time.Second):
				Fail("timeout")
			}
		})

		It("should return proper error when it's a receiveMessage request", func() {
			// Initialise Darknode.
			done := make(chan struct{})
			defer close(done)
			server := initErrorServer()
			defer server.Close()
			multi, err := testutils.ServerMultiAddress(server)
			Expect(err).ToNot(HaveOccurred())
			multiStore, err := testutils.InitStore(multi)
			Expect(err).ToNot(HaveOccurred())
			store := store.NewProxy(multiStore, store.NewCache(0), store.NewCache(0))

			// Initialise the client task.
			logger := logrus.New()
			client := NewClient(logger, store, 32, 8, time.Second)
			go client.Run(done)
			responder := make(chan jsonrpc.Response, 1)

			// Send a request to the task.
			client.IO().InputWriter() <- InvokeRPC{
				Request: jsonrpc.ReceiveMessageRequest{
					Responder: responder,
				},
				Addresses: []addr.Addr{multi.Addr()},
			}

			// Expect to receive a response from the responder channel.
			select {
			case response := <-responder:
				resp, ok := response.(jsonrpc.ReceiveMessageResponse)
				Expect(ok).To(BeTrue())
				Expect(resp.Error).NotTo(BeNil())
			case <-time.After(time.Second):
				Fail("timeout")
			}
		})
	})

	Context("when the darknode takes too long to respond", func() {
		It("should not block other send message requests", func() {
			// Initialise Darknodes.
			done := make(chan struct{})
			defer close(done)
			server := initServer()
			defer server.Close()
			badServer := initTimeoutServer()
			defer badServer.Close()

			// Construct the store for the client.
			multi, err := testutils.ServerMultiAddress(server)
			Expect(err).ToNot(HaveOccurred())
			badMulti, err := testutils.ServerMultiAddress(badServer)
			Expect(err).ToNot(HaveOccurred())
			multiStore, err := testutils.InitStore(multi, badMulti)
			Expect(err).ToNot(HaveOccurred())
			store := store.NewProxy(multiStore, store.NewCache(0), store.NewCache(0))

			// Initialise the client task.
			logger := logrus.New()
			client := NewClient(logger, store, 4, 2, time.Minute)
			go client.Run(done)

			// Send a request to the task.
			for i := 0; i < 2; i++ {
				responder := make(chan jsonrpc.Response, 1)
				address := badMulti.Addr()
				if i%2 != 0 {
					address = multi.Addr()
				}

				client.IO().InputWriter() <- InvokeRPC{
					Request: jsonrpc.SendMessageRequest{
						Responder: responder,
					},
					Addresses: []addr.Addr{address},
				}

				// Since the client task has two workers, we should get a response from the good server.
				if i%2 != 0 {
					select {
					case response := <-responder:
						resp, ok := response.(jsonrpc.SendMessageResponse)
						Expect(ok).To(BeTrue())
						Expect(resp.Ok).To(BeTrue())
					case <-time.After(time.Second):
						Fail("timeout")
					}
				}
			}
		})
	})

	Context("client should cache the result of receiveMessage result", func() {
		It("should return the caches result within a certain amount of period", func() {
			// Initialise darknodes.
			done := make(chan struct{})
			defer close(done)
			server := initServer()
			multi, err := testutils.ServerMultiAddress(server)
			Expect(err).ToNot(HaveOccurred())
			multiStore, err := testutils.InitStore(multi)
			Expect(err).ToNot(HaveOccurred())
			store := store.NewProxy(multiStore, store.NewCache(0), store.NewCache(0))

			// Initialise the client task.
			logger := logrus.New()
			client := NewClient(logger, store, 32, 8, time.Second)
			go client.Run(done)
			responder := make(chan jsonrpc.Response, 1)

			// Send a couple requests to the task.
			for i := 0; i < 32; i++ {
				client.IO().InputWriter() <- InvokeRPC{
					Request: jsonrpc.ReceiveMessageRequest{
						Responder: responder,
					},
					Addresses: []addr.Addr{multi.Addr()},
				}

				// Expect to receive a cached response from the responder channel.
				select {
				case response := <-responder:
					resp, ok := response.(jsonrpc.ReceiveMessageResponse)
					Expect(ok).To(BeTrue())

					var params []processor.Param
					Expect(json.Unmarshal(resp.Result, &params)).To(Succeed())
					Expect(len(params)).To(Equal(1))
				case <-time.After(time.Second):
					Fail("timeout")
				}

				// Close the server after receiving the first result
				if i == 0 {
					server.Close()
				}
			}
		})
	})
})
