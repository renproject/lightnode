package store

import (
	"errors"

	"github.com/republicprotocol/darknode-go/server/jsonrpc"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/republicprotocol/renp2p-go/foundation/addr"
)

// ErrInvalidDarknodeAddress is returned when we do not have health information for a particular Darknode in the store.
var ErrInvalidDarknodeAddress = errors.New("invalid darknode address")

type proxy struct {
	multiStore KVStore
	statsStore KVStore
}

// NewProxy returns a new Proxy.
func NewProxy(multiStore, statsStore KVStore) Proxy {
	return proxy{
		multiStore: multiStore,
		statsStore: statsStore,
	}
}

// InsertMultiAddress implements the `store.Proxy` interface.
func (proxy proxy) InsertMultiAddress(darknodeAddr addr.Addr, value peer.MultiAddr) error {
	return proxy.multiStore.Write(darknodeAddr.String(), value)
}

// InsertStats implements the `store.Proxy` interface.
func (proxy proxy) InsertStats(darknodeAddr addr.Addr, value jsonrpc.QueryStatsResponse) error {
	return proxy.statsStore.Write(darknodeAddr.String(), value)
}

// DeleteMultiAddress implements the `store.Proxy` interface.
func (proxy proxy) DeleteMultiAddress(darknodeAddr addr.Addr) error {
	return proxy.multiStore.Delete(darknodeAddr.String())
}

// DeleteStats implements the `store.Proxy` interface.
func (proxy proxy) DeleteStats(darknodeAddr addr.Addr) error {
	return proxy.statsStore.Delete(darknodeAddr.String())
}

// MultiAddress implements the `store.Proxy` interface.
func (proxy proxy) MultiAddress(darknodeAddr addr.Addr) (peer.MultiAddr, error) {
	var value peer.MultiAddr
	if err := proxy.multiStore.Read(darknodeAddr.String(), &value); err != nil {
		return peer.MultiAddr{}, err
	}
	return value, nil
}

// MultiAddresses implements the `store.Proxy` interface.
func (proxy proxy) MultiAddresses() []string {
	iter := proxy.multiStore.Iterator()
	multiAddrs := make([]string, 0, proxy.multiStore.Entries())
	for iter.Next() {
		var value peer.MultiAddr
		_, err := iter.KV(&value)
		if err != nil {
			continue
		}
		multiAddrs = append(multiAddrs, value.Value())
	}
	return multiAddrs
}

// MultiAddressEntries implements the `store.Proxy` interface.
func (proxy proxy) MultiAddressEntries() int {
	return proxy.multiStore.Entries()
}

// Stats implements the `store.Proxy` interface.
func (proxy proxy) Stats(darknodeAddr addr.Addr) jsonrpc.QueryStatsResponse {
	var value jsonrpc.QueryStatsResponse
	if err := proxy.statsStore.Read(darknodeAddr.String(), &value); err != nil {
		value.Error = ErrInvalidDarknodeAddress
	}
	return value
}
