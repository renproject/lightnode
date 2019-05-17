package p2p

import (
	"errors"

	"github.com/republicprotocol/darknode-go/rpc/jsonrpc"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/republicprotocol/renp2p-go/foundation/addr"
	"github.com/republicprotocol/store"
)

// ErrInvalidDarknodeAddress is returned when we do not have health information for a particular Darknode in the store.
var ErrInvalidDarknodeAddress = errors.New("invalid darknode address")

type Proxy struct {
	multiStore peer.MultiAddrStore
	statsStore store.IterableStore
}

// NewProxy returns a new Proxy.
func NewProxy(multiStore peer.MultiAddrStore, statsStore store.IterableStore) Proxy {
	return Proxy{
		multiStore: multiStore,
		statsStore: statsStore,
	}
}

func (proxy Proxy) InsertMultiAddr(multiAddr peer.MultiAddr) error {
	return proxy.multiStore.InsertMultiAddr(multiAddr)
}

func (proxy Proxy) MultiAddr(addr addr.Addr) (peer.MultiAddr, error) {
	return proxy.multiStore.MultiAddr(addr)
}

func (proxy Proxy) MultiAddrs() (peer.MultiAddrs, error) {
	return proxy.multiStore.MultiAddrs()
}

func (proxy Proxy) DeleteMultiAddr(addr addr.Addr) error {
	return proxy.multiStore.DeleteMultiAddr(addr)
}

// InsertStats implements the `Proxy` interface.
func (proxy Proxy) InsertStats(darknodeAddr addr.Addr, value jsonrpc.QueryStatsResponse) error {
	return proxy.statsStore.Write(darknodeAddr.String(), value)
}

// Stats implements the `Proxy` interface.
func (proxy Proxy) Stats(darknodeAddr addr.Addr) jsonrpc.QueryStatsResponse {
	var value jsonrpc.QueryStatsResponse
	if err := proxy.statsStore.Read(darknodeAddr.String(), &value); err != nil {
		value.Error = ErrInvalidDarknodeAddress
	}
	return value
}

// DeleteStats implements the `Proxy` interface.
func (proxy Proxy) DeleteStats(darknodeAddr addr.Addr) error {
	return proxy.statsStore.Delete(darknodeAddr.String())
}
