package store

import (
	"math/rand"

	"github.com/renproject/darknode/addr"
	"github.com/renproject/kv/db"
)

// MultiAddrStore is a store of `addr.MultiAddress`es.
type MultiAddrStore struct {
	store db.Iterable
}

// New constructs a new `MultiAddrStore`.
func New(store db.Iterable) MultiAddrStore {
	return MultiAddrStore{store}
}

// Insert puts the given multi address into the store.
func (multiStore *MultiAddrStore) Insert(addr addr.MultiAddress) error {
	// TODO: What is a better key/value pair to store?
	return multiStore.store.Insert(addr.String(), []byte(addr.String()))
}

// Delete removes the given multi address from the store.
func (multiStore *MultiAddrStore) Delete(addr addr.MultiAddress) {
	// NOTE: The `Delete` function always returns a nil error, so we ignore it.
	_ = multiStore.store.Delete(addr.String())
}

// Size returns the number of entries in the store.
func (multiStore *MultiAddrStore) Size() int {
	size, err := multiStore.store.Size()
	if err != nil {
		// TODO: Is it ok to panic here?
		panic("[store] could not get size of store")
	}
	return size
}

// AddrsAll returns all of the multi addressses that are currently in the
// store.
func (multiStore *MultiAddrStore) AddrsAll() addr.MultiAddresses {
	addrs := addr.MultiAddresses{}
	for iter := multiStore.store.Iterator(); iter.Next(); {
		str, err := iter.Key()
		if err != nil {
			panic("iterator invariant violated")
		}
		address, err := addr.NewMultiAddressFromString(str)
		if err != nil {
			panic("incorrectly stored multi address")
		}
		addrs = append(addrs, address)
	}

	return addrs
}

// AddrsRandom returns a random number of addresses from the store.
func (multiStore *MultiAddrStore) AddrsRandom(n int) addr.MultiAddresses {
	addrs := multiStore.AddrsAll()

	rand.Shuffle(len(addrs), func(i, j int) {
		addrs[i], addrs[j] = addrs[j], addrs[i]
	})

	if len(addrs) < n {
		return addrs
	}
	return addrs[:n]
}
