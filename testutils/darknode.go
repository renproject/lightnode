package testutils

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/republicprotocol/darknode-go"
	"github.com/republicprotocol/darknode-go/rpc/jsonrpc"
)

type MockDarknode struct {
	config darknode.Config
}

func NewMockDarknode(config darknode.Config) *MockDarknode {
	return &MockDarknode{
		config: config,
	}
}

func (node *MockDarknode) Run(done <-chan struct{}) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request jsonrpc.JSONRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			node.writeError(w, err)
			return
		}

		response := jsonrpc.JSONResponse{
			JSONRPC: "2.0",
			ID:      request.ID,
		}

		switch request.Method {
		case jsonrpc.MethodSendMessage:
			response.Result = json.RawMessage([]byte(`{"messageID":"messageID","ok":true}`))
		case jsonrpc.MethodReceiveMessage:
			response.Result = json.RawMessage([]byte(`{"values":[{"type":"private","value":"0"}]}`))
		case jsonrpc.MethodQueryPeers:
			response.Result = json.RawMessage([]byte(`{"peers": null}`))
		case jsonrpc.MethodQueryNumPeers:
			response.Result = json.RawMessage([]byte(`{"numPeers": 0}`))
		case jsonrpc.MethodQueryStats:
			response.Result = json.RawMessage([]byte(`{"version": "1.0", "address": "abc", "cpus": null, "ram": 1, "disk": 8, "location": "New York"}`))
		default:
			panic(fmt.Sprintf("unknown method %s", request.Method))
		}

		time.Sleep(100 * time.Millisecond)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Fatal(err)
		}
	})

	address := fmt.Sprintf("0.0.0.0:%v", node.config.JSONRPCPort)
	server := &http.Server{Addr: address, Handler: handler}

	go func() {
		<-done
		server.Close()
	}()

	server.ListenAndServe()
}

func (node *MockDarknode) writeError(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(err.Error()))
}
