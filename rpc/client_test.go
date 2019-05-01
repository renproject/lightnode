package rpc_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/rpc"

	"github.com/republicprotocol/darknode-go/processor"
	"github.com/republicprotocol/darknode-go/server/jsonrpc"
	"github.com/sirupsen/logrus"
)

var _ = Describe("RPC client", func() {
	// Construct a mock Darknode server.
	initServer := func() *httptest.Server {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		}))
		return server
	}

	// Construct a mock Darknode server that always returns errors.
	initErrorServer := func() *httptest.Server {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		return server
	}

	Context("when we receive an InvokeRPC message", func() {
		It("should get a response from the server for a SendMessageRequest", func() {
			// Intialise Darknode.
			done := make(chan struct{})
			defer close(done)
			server := initServer()
			defer server.Close()

			// Initialise the client task.
			logger := logrus.New()
			client := NewClient(logger, 32, 8, time.Second)
			go client.Run(done)
			responder := make(chan jsonrpc.Response)

			// Send a request to the task.
			for i := 0; i < 32; i++ {
				client.IO().InputWriter() <- InvokeRPC{
					Request: jsonrpc.SendMessageRequest{
						Responder: responder,
					},
					Addresses: []string{server.URL},
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
			// Intialise Darknode.
			done := make(chan struct{})
			defer close(done)
			server := initServer()
			defer server.Close()

			// Initialise the client task.
			logger := logrus.New()
			client := NewClient(logger, 32, 8, time.Second)
			go client.Run(done)
			responder := make(chan jsonrpc.Response)

			// Send a request to the task.
			for i := 0; i < 32; i++ {
				client.IO().InputWriter() <- InvokeRPC{
					Request: jsonrpc.ReceiveMessageRequest{
						Responder: responder,
					},
					Addresses: []string{server.URL},
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

	Context("when we receive an InvokeRPC message with a single target", func() {
		It("should respond with the result returned by the majority of the Darknodes", func() {
			// Intialise 8 Darknodes (2 malicious).
			done := make(chan struct{})
			defer close(done)
			servers := make([]*httptest.Server, 8)
			serverAddrs := make([]string, 8)
			for i := 0; i < 8; i++ {
				if i < 6 {
					servers[i] = initServer()
					defer servers[i].Close()
				} else {
					servers[i] = initErrorServer()
				}
				serverAddrs[i] = servers[i].URL
			}

			// Initialise the client task.
			logger := logrus.New()
			client := NewClient(logger, 32, 8, 5*time.Second)
			go client.Run(done)

			// Send a request to the task.
			for i := 0; i < 8; i++ {
				responder := make(chan jsonrpc.Response)
				client.IO().InputWriter() <- InvokeRPC{
					Request: jsonrpc.SendMessageRequest{
						Responder: responder,
					},
					Addresses: serverAddrs,
				}

				// Expect to receive a response from the responder channel.
				select {
				case response := <-responder:
					resp, ok := response.(jsonrpc.SendMessageResponse)
					Expect(ok).To(BeTrue())
					Expect(resp.Ok).To(BeTrue())
				case <-time.After(5 * time.Second):
					Fail("timeout")
				}
			}
		})
	})
})
