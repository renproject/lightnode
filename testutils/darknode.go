package testutils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"net"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/renproject/aw/wire"
	"github.com/renproject/id"
	"github.com/renproject/lightnode/store"
)

type MockDarknode struct {
	Server *httptest.Server
	Me     wire.Address
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
	addr := wire.NewUnsignedAddress(wire.TCP, fmt.Sprintf("%v:%v", host, port), uint64(time.Now().Unix()))
	if err := store.Insert(addr); err != nil {
		panic(err)
	}
	if err := addr.Sign((*id.PrivKey)(key)); err != nil {
		panic(err)
	}
	return &MockDarknode{
		Server: server,
		Me:     addr,
		Store:  store,
	}
}

func (dn *MockDarknode) Close() {
	dn.Server.Close()
}
