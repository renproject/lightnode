package p2p

import (
	"errors"

	"github.com/renproject/kv"
	"github.com/republicprotocol/darknode-go/rpc/jsonrpc"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/republicprotocol/renp2p-go/foundation/addr"
)

// ErrInvalidDarknodeAddress is returned when we do not have health information for a particular Darknode in the store.
var ErrInvalidDarknodeAddress = errors.New("invalid darknode address")

// Proxy handles the Read/Write interactions for the multi-address and stat stores. This abstracts any interaction with
// the KVStore objects.
type Proxy struct {
	multiStore peer.MultiAddrStore
	statsStore kv.Iterable
}

// NewProxy returns a new Proxy.
func NewProxy(multiStore peer.MultiAddrStore, statsStore kv.Iterable) Proxy {
	return Proxy{
		multiStore: multiStore,
		statsStore: statsStore,
	}
}

// InsertMultiAddr inserts the multi-address for a Darknode mapped to its address.
func (proxy Proxy) InsertMultiAddr(multiAddr peer.MultiAddr) error {
	return proxy.multiStore.InsertMultiAddr(multiAddr)
}

// MultiAddr retrieves a Darknode multi-address using its address.
func (proxy Proxy) MultiAddr(addr addr.Addr) (peer.MultiAddr, error) {
	return proxy.multiStore.MultiAddr(addr)
}

// MultiAddrs retrieves all the multi-addresses in the store.
func (proxy Proxy) MultiAddrs() (peer.MultiAddrs, error) {
	return proxy.multiStore.MultiAddrs()
}

// DeleteMultiAddr deletes the multi-address for a Darknode using its address.
func (proxy Proxy) DeleteMultiAddr(addr addr.Addr) error {
	return proxy.multiStore.DeleteMultiAddr(addr)
}

// InsertStats inserts the stats for a Darknode mapped to its address.
func (proxy Proxy) InsertStats(darknodeAddr addr.Addr, value jsonrpc.QueryStatsResponse) error {
	return proxy.statsStore.Insert(darknodeAddr.String(), value)
}

// Stats retrieves Darknode stats using its address.
func (proxy Proxy) Stats(darknodeAddr addr.Addr) jsonrpc.QueryStatsResponse {
	var value jsonrpc.QueryStatsResponse
	if err := proxy.statsStore.Get(darknodeAddr.String(), &value); err != nil {
		value.Error = ErrInvalidDarknodeAddress
	}
	return value
}

// DeleteStats deletes the stats for a Darknode using its address.
func (proxy Proxy) DeleteStats(darknodeAddr addr.Addr) error {
	return proxy.statsStore.Delete(darknodeAddr.String())
}
