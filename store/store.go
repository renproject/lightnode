package store

import (
	"errors"
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
func (multiStore *MultiAddrStore) Delete(addr addr.MultiAddress) error {
	return multiStore.store.Delete(addr.String())
}

// Size returns the number of entries in the store.
func (multiStore *MultiAddrStore) Size() (int, error) {
	return multiStore.store.Size()
}

// AddrsAll returns all of the multi addressses that are currently in the
// store.
func (multiStore *MultiAddrStore) AddrsAll() (addr.MultiAddresses, error) {
	addrs := addr.MultiAddresses{}
	for iter := multiStore.store.Iterator(); iter.Next(); {
		str, err := iter.Key()
		if err != nil {
			return nil, err
		}
		address, err := addr.NewMultiAddressFromString(str)
		if err != nil {
			return nil, err
		}
		addrs = append(addrs, address)
	}
	return addrs, nil
}

// AddrsFirst returns the first multi addressses in the store.
func (multiStore *MultiAddrStore) AddrsFirst() (addr.MultiAddresses, error) {
	for iter := multiStore.store.Iterator(); iter.Next(); {
		str, err := iter.Key()
		if err != nil {
			return nil, err
		}
		address, err := addr.NewMultiAddressFromString(str)
		if err != nil {
			return nil, err
		}
		return addr.MultiAddresses{address}, nil
	}
	return nil, errors.New("no multi address in store")
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
