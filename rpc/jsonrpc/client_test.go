package jsonrpc_test

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/rpc/jsonrpc"
	"github.com/sirupsen/logrus"

	"github.com/republicprotocol/darknode-go/abi"
	"github.com/republicprotocol/darknode-go/rpc/jsonrpc"
)

var _ = Describe("JSON-RPC Client", func() {
	// Construct a mock Darknode server.
	initServer := func() *httptest.Server {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var request jsonrpc.JSONRequest
			Expect(json.NewDecoder(r.Body).Decode(&request)).To(Succeed())

			response, err := constructResponse(request)
			Expect(err).ToNot(HaveOccurred())

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

	// Construct a valid `jsonrpc.SendMessageRequest`.
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

	// Construct an invalid `jsonrpc.SendMessageRequest`.
	newInvalidSendMessageRequest := func() jsonrpc.JSONRequest {
		params := json.RawMessage("bad request")
		return jsonrpc.JSONRequest{
			JSONRPC: "2.0",
			ID:      rand.Int31(),
			Method:  jsonrpc.MethodSendMessage,
			Params:  params,
		}
	}

	Context("when sending a SendMessageRequest", func() {
		It("should get a valid response", func() {
			// Initialise Darknode.
			server := initServer()
			defer server.Close()

			// Send request.
			logger := logrus.New()
			request := newSendMessageRequest()
			client := NewClient(logger, time.Second)
			jsonResponse, err := client.Call(server.URL, request)
			Expect(err).ToNot(HaveOccurred())

			// Validate response.
			Expect(jsonResponse.JSONRPC).To(Equal("2.0"))
			Expect(int32(jsonResponse.ID.(float64))).Should(Equal(request.ID))
			Expect(jsonResponse.Error).To(BeNil())

			var response jsonrpc.SendMessageResponse
			Expect(json.Unmarshal(jsonResponse.Result, &response)).NotTo(HaveOccurred())
			Expect(response.Ok).To(BeTrue())
			Expect(response.MessageID).To(Equal("messageID"))
		})
	})

	Context("when sending a ReceiveMessageRequest", func() {
		It("should get a valid response", func() {
			// Initialise Darknode.
			server := initServer()
			defer server.Close()

			// Send request.
			logger := logrus.New()
			request := newReceiveMessageRequest()
			client := NewClient(logger, time.Second)
			jsonResponse, err := client.Call(server.URL, request)
			Expect(err).ToNot(HaveOccurred())

			// Validate response.
			Expect(jsonResponse.JSONRPC).To(Equal("2.0"))
			Expect(int32(jsonResponse.ID.(float64))).Should(Equal(request.ID))
			Expect(jsonResponse.Error).To(BeNil())

			var response jsonrpc.ReceiveMessageResponse
			Expect(json.Unmarshal(jsonResponse.Result, &response)).NotTo(HaveOccurred())
			Expect(len(response.Result)).To(Equal(1))
		})
	})

	Context("when sending an invalid request", func() {
		It("should return an error before calling the server", func() {
			// Initialise Darknode.
			server := initServer()
			defer server.Close()

			// Send request.
			logger := logrus.New()
			request := newInvalidSendMessageRequest()
			client := NewClient(logger, time.Second)
			_, err := client.Call(server.URL, request)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when the server is not working", func() {
		It("should timeout if we do not receive a response", func() {
			// Initialise Darknode.
			server := initServer()
			defer server.Close()

			// Send request.
			logger := logrus.New()
			request := newSendMessageRequest()
			client := NewClient(logger, 10*time.Millisecond)
			_, err := client.Call(server.URL, request)
			Expect(err).To(HaveOccurred())
		})

		It("should return an error if the server is offline", func() {
			// Send request.
			logger := logrus.New()
			request := newSendMessageRequest()
			client := NewClient(logger, time.Second)
			_, err := client.Call("http://0.0.0.0:8888", request)
			Expect(err).To(HaveOccurred())
		})

		It("should return an error if we get a status code other than 200", func() {
			// Initialise server.
			server := initErrorServer()
			defer server.Close()

			// Send request.
			logger := logrus.New()
			request := newSendMessageRequest()
			client := NewClient(logger, time.Second)
			_, err := client.Call(server.URL, request)
			Expect(err).To(HaveOccurred())
		})
	})
})

func constructResponse(req jsonrpc.JSONRequest) (jsonrpc.JSONResponse, error) {
	resp := jsonrpc.JSONResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	switch req.Method {
	case jsonrpc.MethodSendMessage:
		result := jsonrpc.SendMessageResponse{
			MessageID: "messageID",
			Ok:        true,
		}
		resultBytes, err := json.Marshal(result)
		if err != nil {
			return jsonrpc.JSONResponse{}, err
		}
		resp.Result = json.RawMessage(resultBytes)
	case jsonrpc.MethodReceiveMessage:
		args := abi.ArgsJSON{
			abi.ArgJSON{
				Type:  "public",
				Value: []byte{},
			},
		}
		result := jsonrpc.ReceiveMessageResponse{
			Result: args,
		}
		resultBytes, err := json.Marshal(result)
		if err != nil {
			return jsonrpc.JSONResponse{}, err
		}
		resp.Result = json.RawMessage(resultBytes)
	default:
		panic("unknown message type")
	}

	return resp, nil
}
