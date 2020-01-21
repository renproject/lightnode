package testutils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"log"
	"net"
	"net/http/httptest"
	"strings"

	"github.com/renproject/darknode/addr"
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
	Server *httptest.Server
	Me     addr.MultiAddress
	// Peers   addr.MultiAddresses
}

func NewMockDarknode(server *httptest.Server) *MockDarknode {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	if server.URL == "" {
		panic("cannot parse an unstarted server to darknode")
	}
	host, port, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		panic(err)
	}
	multiStr := fmt.Sprintf("/ip4/%v/tcp/%v/ren/%v", host, port, addr.FromPublicKey(key.PublicKey))
	multi, err := addr.NewMultiAddressFromString(multiStr)
	if err != nil {
		panic(err)
	}
	return &MockDarknode{
		Server: server,
		Me:     multi,
	}
}

func (dn *MockDarknode) Start() {
	defer log.Printf("darknode listening on %v...", dn.Server.URL)

	// Server has already been started
	if dn.Server.URL != "" {
		return
	}
	dn.Server.Start()
}

func (dn *MockDarknode) Close() {
	dn.Server.Close()
}
