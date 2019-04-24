package jsonrpc_test

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/republicprotocol/lightnode/rpc/jsonrpc"

	"github.com/republicprotocol/darknode-go/server/jsonrpc"
)

var _ = Describe("JSON-RPC Client", func() {

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

	badServer := func() *httptest.Server {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))

		return server
	}

	// Construct a valid `jsonrpc.SendMessageRequest` .
	newSendMessageRequest := func() jsonrpc.JSONRequest {
		params, err := json.Marshal(jsonrpc.SendMessageRequest{})
		Expect(err).ToNot(HaveOccurred())
		return jsonrpc.JSONRequest{
			JSONRPC: "2.0",
			ID:      rand.Int31(),
			Method:  jsonrpc.MethodSendMessage,
			Params:  params,
		}
	}

	// Construct a valid `jsonrpc.ReceiveMessageRequest`.
	newReceiveMessageRequest := func() jsonrpc.JSONRequest {
		params, err := json.Marshal(jsonrpc.ReceiveMessageRequest{})
		Expect(err).ToNot(HaveOccurred())
		return jsonrpc.JSONRequest{
			JSONRPC: "2.0",
			ID:      rand.Int31(),
			Method:  jsonrpc.MethodReceiveMessage,
			Params:  params,
		}
	}

	// Construct a bad jsonrpc request.
	badRequest := func() jsonrpc.JSONRequest {
		params := json.RawMessage("bad request")
		return jsonrpc.JSONRequest{
			JSONRPC: "2.0",
			ID:      rand.Int31(),
			Method:  jsonrpc.MethodSendMessage,
			Params:  params,
		}
	}

	Context("when sending valid requests", func() {
		It("should reach the server and get a response back when it's a SendMessageRequest", func() {
			// Init the testing server
			server := initServer()
			defer server.Close()

			// Send the request
			request := newSendMessageRequest()
			client := NewClient(time.Second)
			response, err := client.Call(server.URL, request)
			Expect(err).ToNot(HaveOccurred())

			// Validate the response
			Expect(response.JSONRPC).To(Equal("2.0"))
			Expect(int32(response.ID.(float64))).Should(Equal(request.ID))
			Expect(response.Error).To(BeNil())

			log.Print(string(response.Result))
			var resp jsonrpc.SendMessageResponse
			Expect(json.Unmarshal(response.Result, &resp)).NotTo(HaveOccurred())
			Expect(resp.Ok).To(BeTrue())
			Expect(resp.MessageID).To(Equal("messageID"))
		})

		It("should reach the server and get a response back when it's a ReceiveMessageRequest", func() {
			// Init the testing server
			server := initServer()
			defer server.Close()

			// Send the request
			request := newReceiveMessageRequest()
			client := NewClient(time.Second)
			response, err := client.Call(server.URL, request)
			Expect(err).ToNot(HaveOccurred())

			// Validate the response
			Expect(response.JSONRPC).To(Equal("2.0"))
			Expect(int32(response.ID.(float64))).Should(Equal(request.ID))
			Expect(response.Error).To(BeNil())
			var resp jsonrpc.ReceiveMessageResponse
			Expect(json.Unmarshal(response.Result, &resp)).NotTo(HaveOccurred())
			Expect(len(resp.Result)).To(Equal(1))
			Expect(resp.Result[0].Index).To(Equal(0))
			Expect(resp.Result[0].Private).To(BeFalse())
		})
	})

	Context("when server doesn't response in time", func() {
		It("should timeout and returns an error", func() {
			// Init the testing server
			server := initServer()
			defer server.Close()

			// Send the request
			request := newSendMessageRequest()
			client := NewClient(10 * time.Millisecond)
			_, err := client.Call(server.URL, request)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when sending a bad request", func() {
		It("should return an error before calling the server", func() {
			server := initServer()
			defer server.Close()

			request := badRequest()
			client := NewClient(time.Second)
			_, err := client.Call(server.URL, request)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when server is out of order", func() {
		It("should return an error if the server is offline", func() {
			request := newSendMessageRequest()
			client := NewClient(time.Second)
			_, err := client.Call("0.0.0.0:8888", request)
			Expect(err).To(HaveOccurred())
		})

		It("should return an error if we get a status code other than 200", func() {
			server := badServer()
			defer server.Close()

			request := newSendMessageRequest()
			client := NewClient(time.Second)
			_, err := client.Call(server.URL, request)
			Expect(err).To(HaveOccurred())
		})
	})
})
