package store

import (
	"math/rand"

	"github.com/renproject/darknode/addr"
	"github.com/renproject/kv/db"
)

// MultiAddrStore is a store of `addr.MultiAddress`es.
type MultiAddrStore struct {
	store db.Table
}

// New constructs a new `MultiAddrStore`.
func New(store db.Table) MultiAddrStore {
	return MultiAddrStore{
		store: store,
	}
}

// Get retrieves a multi-address from the store.
func (multiStore *MultiAddrStore) Get(id string) (addr.MultiAddress, error) {
	var multiAddrString string
	if err := multiStore.store.Get(id, &multiAddrString); err != nil {
		return addr.MultiAddress{}, err
	}
	return addr.NewMultiAddressFromString(multiAddrString)
}

// Insert puts the given multi-address into the store.
func (multiStore *MultiAddrStore) Insert(addr addr.MultiAddress) error {
	return multiStore.store.Insert(addr.ID().ToBase58(), addr.String())
}

// Delete removes the given multi-address from the store.
func (multiStore *MultiAddrStore) Delete(addr addr.MultiAddress) error {
	return multiStore.store.Delete(addr.ID().ToBase58())
}

// Size returns the number of entries in the store.
func (multiStore *MultiAddrStore) Size() (int, error) {
	return multiStore.store.Size()
}

// AddrsAll returns all of the multi addressses that are currently in the
// store.
func (multiStore *MultiAddrStore) AddrsAll() (addr.MultiAddresses, error) {
	addrs := addr.MultiAddresses{}
	iter := multiStore.store.Iterator()
	defer iter.Close()
	for iter.Next() {
		id, err := iter.Key()
		if err != nil {
			return nil, err
		}
		address, err := multiStore.Get(id)
		if err != nil {
			return nil, err
		}
		addrs = append(addrs, address)
	}
	return addrs, nil
}

// AddrsRandom returns a random number of addresses from the store.
func (multiStore *MultiAddrStore) AddrsRandom(n int) (addr.MultiAddresses, error) {
	addrs, err := multiStore.AddrsAll()
	if err != nil {
		return nil, err
	}
	rand.Shuffle(len(addrs), func(i, j int) {
		addrs[i], addrs[j] = addrs[j], addrs[i]
	})

	if len(addrs) < n {
		return addrs, nil
	}
	return addrs[:n], nil
}
