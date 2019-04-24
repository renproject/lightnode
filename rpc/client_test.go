package rpc_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/republicprotocol/lightnode/rpc"

	"github.com/republicprotocol/darknode-go/server/jsonrpc"
	"github.com/sirupsen/logrus"
)

var _ = Describe("RPC client", func() {

	initServer := func() *httptest.Server {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req jsonrpc.JSONRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			Expect(err).NotTo(HaveOccurred())

			resp := jsonrpc.JSONResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
			}

			switch req.Method {
			case jsonrpc.MethodSendMessage:
				resp.Result = json.RawMessage([]byte(`{"messageID":"messageID","ok":true}`))
			case jsonrpc.MethodReceiveMessage:
				resp.Result = json.RawMessage([]byte(`{"result":[{"private":false,"value":"0"}]}`))
			default:
				panic("unknown message type")
			}

			time.Sleep(100 * time.Millisecond)
			err = json.NewEncoder(w).Encode(resp)
			Expect(err).NotTo(HaveOccurred())
		}))

		return server
	}

	Context("when receive a InvokeRPC message", func() {
		It("should get a response from the server if it's a SendMessage request", func() {
			// init the server
			done := make(chan struct{})
			defer close(done)
			server := initServer()
			defer server.Close()

			// init the client task
			logger := logrus.New()
			client := NewClient(logger, 32, 3, time.Second)
			go client.Run(done)
			responder := make(chan jsonrpc.Response)

			// send a message to the task which contains a SendMessageRequest
			for i := 0; i < 32; i++ {
				client.IO().InputWriter() <- InvokeRPC{
					Request: jsonrpc.SendMessageRequest{
						Responder: responder,
					},
					Addresses: []string{server.URL},
				}

				// expect to receive a response from the responder channel
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

		It("should get a response from the server if it's a ReceiveMessage request", func() {
			// init the server
			done := make(chan struct{})
			defer close(done)
			server := initServer()
			defer server.Close()

			// init the client task
			logger := logrus.New()
			client := NewClient(logger, 32, 3, time.Second)
			go client.Run(done)
			responder := make(chan jsonrpc.Response)

			// send a message to the task which contains a SendMessageRequest
			for i := 0; i < 32; i++ {
				client.IO().InputWriter() <- InvokeRPC{
					Request: jsonrpc.ReceiveMessageRequest{
						Responder: responder,
					},
					Addresses: []string{server.URL},
				}

				// expect to receive a response from the responder channel
				select {
				case response := <-responder:
					resp, ok := response.(jsonrpc.ReceiveMessageResponse)
					Expect(ok).To(BeTrue())
					Expect(len(resp.Result)).To(Equal(1))
					Expect(resp.Result[0].Index).Should(Equal(0))
				case <-time.After(time.Second):
					Fail("timeout")
				}
			}
		})
	})
})
