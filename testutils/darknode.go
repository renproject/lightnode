package testutils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"net"
	"net/http/httptest"
	"strconv"
	"strings"

	"github.com/renproject/darknode/addr"
	"github.com/renproject/lightnode/store"
)

type MockDarknode struct {
	Server *httptest.Server
	Me     addr.MultiAddress
	Store  store.MultiAddrStore
}

func NewMockDarknode(server *httptest.Server, store store.MultiAddrStore) *MockDarknode {
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
	portInt, err := strconv.Atoi(port)
	if err != nil {
		panic(err)
	}
	multiStr := fmt.Sprintf("/ip4/%v/tcp/%v/ren/%v", host, portInt-1, addr.FromPublicKey(key.PublicKey))
	multi, err := addr.NewMultiAddressFromString(multiStr)
	if err != nil {
		panic(err)
	}
	if err := store.Insert(multi); err != nil {
		panic(err)
	}
	return &MockDarknode{
		Server: server,
		Me:     multi,
		Store:  store,
	}
}

func (dn *MockDarknode) Close() {
	dn.Server.Close()
}
