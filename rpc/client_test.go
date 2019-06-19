package rpc_test

import (
	"encoding/json"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/rpc"

	"github.com/renproject/lightnode/testutils"
	"github.com/republicprotocol/darknode-go/rpc/jsonrpc"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/republicprotocol/renp2p-go/foundation/addr"
	"github.com/republicprotocol/tau"
	"github.com/sirupsen/logrus"
)

var _ = Describe("RPC client", func() {

	initServer := func(addr string, handler http.Handler) (*http.Server, peer.MultiAddr, peer.MultiAddrStore) {
		server := &http.Server{Addr: addr, Handler: handler}
		go server.ListenAndServe()

		multi, err := testutils.ServerMultiAddress(server)
		Expect(err).NotTo(HaveOccurred())
		multiStore, err := testutils.InitStore(multi)
		Expect(err).NotTo(HaveOccurred())

		return server, multi, multiStore
	}

	// normalHandler will returns a successfule response.
	normalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		Expect(json.NewEncoder(w).Encode(response)).To(Succeed())
	})

	// errHandler will always response with an error.
	errHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request jsonrpc.JSONRequest
		Expect(json.NewDecoder(r.Body).Decode(&request)).To(Succeed())

		response := jsonrpc.JSONResponse{
			JSONRPC: "2.0",
			ID:      request.ID,
		}

		switch request.Method {
		case jsonrpc.MethodSendMessage:
			response.Error = &jsonrpc.JSONError{
				Code:    jsonrpc.ErrorCodeInvalidRequest,
				Message: "bad request",
				Data:    nil,
			}
		case jsonrpc.MethodReceiveMessage:
			response.Error = &jsonrpc.JSONError{
				Code:    jsonrpc.ErrorCodeInvalidRequest,
				Message: "bad request",
				Data:    nil,
			}
		default:
			panic("unknown message type")
		}
		Expect(json.NewEncoder(w).Encode(response)).To(Succeed())
	})

	// badFormatHandler always returns an incorrectly formatted response.
	badFormatHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewEncoder(w).Encode(jsonrpc.JSONResponse{
			JSONRPC: "2.0",
			Result:  []byte(""),
		})
		Expect(err).NotTo(HaveOccurred())
	})

	// timeoutHandler will hang there for a long time.
	timeoutHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Minute)
	})

	sendMessage := func(client tau.Task, address addr.Addr) chan jsonrpc.Response {
		responder := make(chan jsonrpc.Response, 1)
		client.IO().InputWriter() <- InvokeRPC{
			Request: jsonrpc.SendMessageRequest{
				Responder: responder,
			},
			Addresses: []addr.Addr{address},
		}

		return responder
	}

	receiveMessage := func(client tau.Task, address addr.Addr) chan jsonrpc.Response {
		responder := make(chan jsonrpc.Response, 1)
		client.IO().InputWriter() <- InvokeRPC{
			Request: jsonrpc.ReceiveMessageRequest{
				Responder: responder,
			},
			Addresses: []addr.Addr{address},
		}

		return responder
	}

	Context("when we receive an InvokeRPC message", func() {
		It("should get a response from the server for a SendMessageRequest", func() {
			// Initialise darknodes.
			done := make(chan struct{})
			defer close(done)
			server, multi, multiStore := initServer("0.0.0.0:18515", normalHandler)
			defer server.Close()

			// Initialise the client task.
			client := NewClient(logrus.New(), multiStore, 32, 8, time.Second, 5*time.Minute)
			go client.Run(done)

			// Send a request to the task.
			for i := 0; i < 32; i++ {
				responder := sendMessage(client, multi.Addr())
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
			server, multi, multiStore := initServer("0.0.0.0:18515", normalHandler)
			defer server.Close()

			// Initialise the client task.
			client := NewClient(logrus.New(), multiStore, 32, 8, time.Second, 5*time.Minute)
			go client.Run(done)

			// Send a request to the task.
			for i := 0; i < 32; i++ {
				responder := receiveMessage(client, multi.Addr())
				// Expect to receive a response from the responder channel.
				select {
				case response := <-responder:
					resp, ok := response.(jsonrpc.ReceiveMessageResponse)
					Expect(ok).To(BeTrue())

					if resp.Error == nil {
						Expect(len(resp.Result)).To(Equal(1))
					} else {
						Expect(resp.Error).Should(Equal(jsonrpc.ErrResultNotAvailable))
					}
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

			// Initialise the client task.
			client := NewClient(logrus.New(), multiStore, 32, 8, time.Second, 5*time.Minute)
			go client.Run(done)

			// Expect to receive a response from the responder channel.
			responder := sendMessage(client, multi.Addr())
			select {
			case <-time.After(2 * time.Second):
				Fail("timeout")
			case response := <-responder:
				resp, ok := response.(jsonrpc.SendMessageResponse)
				Expect(ok).To(BeTrue())
				Expect(resp.Ok).To(BeFalse())
				Expect(resp.Error).NotTo(BeNil())
			}

			// Expect to receive a response from the responder channel.
			responder = receiveMessage(client, multi.Addr())
			select {
			case <-time.After(2 * time.Second):
				Fail("timeout")
			case response := <-responder:
				resp, ok := response.(jsonrpc.ReceiveMessageResponse)
				Expect(ok).To(BeTrue())
				Expect(resp.Error).NotTo(BeNil())
			}
		})
	})

	Context("when the darknode gives back an error ", func() {
		It("should return a proper error when the server gives back an error", func() {
			// Initialise darknodes.
			done := make(chan struct{})
			defer close(done)
			server, multi, multiStore := initServer("0.0.0.0:18515", errHandler)
			defer server.Close()

			// Initialise the client task.
			client := NewClient(logrus.New(), multiStore, 32, 8, time.Second, 5*time.Minute)
			go client.Run(done)

			// Expect to receive a response from the responder channel.
			responder := sendMessage(client, multi.Addr())
			select {
			case <-time.After(time.Second):
				Fail("timeout")
			case response := <-responder:
				resp, ok := response.(jsonrpc.SendMessageResponse)
				Expect(ok).To(BeTrue())
				Expect(resp.Ok).To(BeFalse())
				Expect(resp.Error).Should(Equal(ErrNotEnoughResultsReturned))
			}

			// Expect to receive a response from the responder channel.
			responder = receiveMessage(client, multi.Addr())
			select {
			case <-time.After(time.Second):
				Fail("timeout")
			case response := <-responder:
				resp, ok := response.(jsonrpc.ReceiveMessageResponse)
				Expect(ok).To(BeTrue())
				Expect(resp.Error).Should(Equal(jsonrpc.ErrResultNotAvailable))
			}
		})

		It("should return an proper error when the server returns random results.", func() {
			// Initialise darknodes.
			done := make(chan struct{})
			defer close(done)
			server, multi, multiStore := initServer("0.0.0.0:18515", badFormatHandler)
			defer server.Close()

			// Initialise the client task.
			client := NewClient(logrus.New(), multiStore, 32, 8, time.Second, 5*time.Minute)
			go client.Run(done)

			// Expect to receive a response from the responder channel.
			responder := sendMessage(client, multi.Addr())
			select {
			case response := <-responder:
				resp, ok := response.(jsonrpc.SendMessageResponse)
				Expect(ok).To(BeTrue())
				Expect(resp.Ok).To(BeFalse())
				Expect(resp.Error).NotTo(BeNil())
			case <-time.After(time.Second):
				Fail("timeout")
			}

			// Expect to receive a response from the responder channel.
			responder = receiveMessage(client, multi.Addr())
			select {
			case <-time.After(time.Second):
				Fail("timeout")
			case response := <-responder:
				resp, ok := response.(jsonrpc.ReceiveMessageResponse)
				Expect(ok).To(BeTrue())
				Expect(resp.Error).Should(Equal(jsonrpc.ErrResultNotAvailable))
			}
		})
	})

	Context("when the darknode takes too long to respond", func() {
		It("should not block other send message requests", func() {
			// Initialise darknodes.
			done := make(chan struct{})
			defer close(done)
			server, multi, multiStore := initServer("0.0.0.0:18515", normalHandler)
			defer server.Close()
			badServer, badMulti, _ := initServer("http://0.0.0.0:18516", timeoutHandler)
			defer badServer.Close()
			Expect(multiStore.InsertMultiAddr(badMulti)).NotTo(HaveOccurred())

			// Initialise the client task.
			client := NewClient(logrus.New(), multiStore, 32, 8, time.Second, 5*time.Minute)
			go client.Run(done)

			// Send some requests to the task.
			for i := 0; i < 2; i++ {
				address := badMulti.Addr()
				if i%2 != 0 {
					address = multi.Addr()
				}
				responder := sendMessage(client, address)

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
		It("should cache the successful result within a certain amount of period", func() {
			// Initialise darknodes.
			done := make(chan struct{})
			defer close(done)
			server, multi, multiStore := initServer("0.0.0.0:18515", normalHandler)
			defer server.Close()

			// Initialise the client task.
			client := NewClient(logrus.New(), multiStore, 32, 8, time.Second, 5*time.Minute)
			go client.Run(done)

			// It should cache successful result.
			for i := 0; i < 32; i++ {
				responder := receiveMessage(client, multi.Addr())

				// Expect to receive a cached response in the responder channel.
				select {
				case response := <-responder:
					resp, ok := response.(jsonrpc.ReceiveMessageResponse)
					Expect(ok).To(BeTrue())

					Expect(len(resp.Result)).To(Equal(1))
				case <-time.After(time.Second):
					Fail("timeout")
				}

				// Close the server after receiving the first result.
				if i == 0 {
					server.Close()
				}
			}
		})

		It("should cache the unsuccessful result within a certain amount of period", func() {
			// Initialise darknodes.
			done := make(chan struct{})
			defer close(done)
			server, multi, multiStore := initServer("0.0.0.0:18515", errHandler)
			defer server.Close()

			// Initialise the client task.
			client := NewClient(logrus.New(), multiStore, 32, 8, time.Second, 5*time.Minute)
			go client.Run(done)

			// It should cache successful result.
			for i := 0; i < 32; i++ {
				responder := receiveMessage(client, multi.Addr())

				// Expect to receive a cached response in the responder channel.
				select {
				case response := <-responder:
					resp, ok := response.(jsonrpc.ReceiveMessageResponse)
					Expect(ok).To(BeTrue())

					Expect(resp.Error).Should(Equal(jsonrpc.ErrResultNotAvailable))
				case <-time.After(time.Second):
					Fail("timeout")
				}

				// Close the server after receiving the first result.
				if i == 0 {
					server.Close()
				}
			}
		})
	})
})
