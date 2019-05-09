package store

import (
	"errors"

	"github.com/republicprotocol/darknode-go/server/jsonrpc"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/republicprotocol/renp2p-go/foundation/addr"
)

// ErrKeyNotFound is returned when there is no value associated with the key.
var ErrKeyNotFound = errors.New("key not found")

// Proxy handles the Read/Write interactions for the multi-address and stat stores. This abstracts any interaction
// with the KVStore objects.
type Proxy interface {
	// InsertMultiAddress inserts the multi-address for a Darknode mapped to its address.
	InsertMultiAddress(darknodeAddr addr.Addr, value peer.MultiAddr) error

	// InsertStats inserts the stats for a Darknode mapped to its address.
	InsertStats(darknodeAddr addr.Addr, value jsonrpc.QueryStatsResponse) error

	// DeleteMultiAddress deletes the multi-address for a Darknode using its address.
	DeleteMultiAddress(darknodeAddr addr.Addr) error

	// DeleteStats deletes the stats for a Darknode using its address.
	DeleteStats(darknodeAddr addr.Addr) error

	// MultiAddress retrieves a Darknode multi-address using its address.
	MultiAddress(darknodeAddr addr.Addr) (peer.MultiAddr, error)

	// MultiAddresses retrieves all the multi-addresses in the store.
	MultiAddresses() []string

	// MultiAddressEntries retrieves the number of multi-addresses in the store.
	MultiAddressEntries() int

	// Stats retrieves Darknode stats using its address.
	Stats(darknodeAddr addr.Addr) jsonrpc.QueryStatsResponse
}

// KVStore is a generic key-value store. The key must be of type string, though there are no restrictions on the type
// of the value.
type KVStore interface {
	// Read the value associated with the given key. This function returns ErrKeyNotFound if the key cannot be found.
	Read(key string, value interface{}) error

	// Write writes the key-value into the store.
	Write(key string, value interface{}) error

	// Delete the entry with the given key from the store. It is safe to use this function to delete a key which is not
	// in the store.
	Delete(key string) error

	// Entries returns the number of data entries in the store.
	Entries() int

	// Iterator returns a KVStoreIterator which can be used to iterate though the data in the store at the time the
	// function is been called.
	Iterator() KVStoreIterator
}

// KVStoreIterator is used to iterate through the data in the store.
type KVStoreIterator interface {

	// Next tells if we reach the end of iterator.
	Next() bool

	// KV returns the key, value of current pointer.
	KV(value interface{}) (string, error)
}
