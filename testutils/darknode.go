package testutils

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/renproject/darknode/addr"
	"github.com/renproject/darknode/jsonrpc"
)

// InitDarknodes initialize given number of darknodes on specified port numbers.
// It start running all of them within the given context time.
// func InitDarknodes(ctx context.Context, num, portStarting, portStep int) []MockDarknode{
// 	darknodes := make([]MockDarknode, num)
// 	for i := range darknodes{
// 		port := portStarting + i * portStep
// 		darknodes[i] = NewMockDarknode(fmt.Sprintf(port),
// 	}
// }

// MockDarknode
type MockDarknode struct {
	Port int
	Me   addr.MultiAddress
	// Peers   addr.MultiAddresses
	Request chan jsonrpc.Request
}

func NewMockDarknode(port int, requests chan jsonrpc.Request) *MockDarknode {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	multiStr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%v/ren/%v", port, addr.FromPublicKey(key.PublicKey))
	multi, err := addr.NewMultiAddressFromString(multiStr)
	if err != nil {
		panic(err)
	}
	return &MockDarknode{
		Port:    port,
		Me:      multi,
		Request: requests,
	}
}

func (dn *MockDarknode) Run(ctx context.Context) {
	addr := fmt.Sprintf(":%v", dn.Port+1)
	log.Printf("darknode listening on 0.0.0.0:%v", dn.Port+1)
	server := http.Server{Addr: addr, Handler: dn}
	go func() {
		<-ctx.Done()
		server.Close()
	}()
	server.ListenAndServe()
}

func (dn *MockDarknode) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rawMessage := json.RawMessage{}
	if err := json.NewDecoder(r.Body).Decode(&rawMessage); err != nil {
		panic("[mock darknode] could not decode JSON request")
	}

	var req jsonrpc.Request
	if err := json.Unmarshal(rawMessage, &req); err != nil {
		panic("[mock darknode] could not parse JSON request")
	}

	// Write a nil-result response to the lightnode
	dn.Request <- req
	response := jsonrpc.Response{
		Version: "2.0",
		ID:      req.ID,
		Result:  nil,
		Error:   nil,
	}
	data, err := json.Marshal(response)
	if err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
