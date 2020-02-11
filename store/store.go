package store

import (
	"fmt"
	"math/rand"

	"github.com/renproject/darknode/addr"
	"github.com/renproject/kv/db"
)

// MultiAddrStore is a store of `addr.MultiAddress`es.
type MultiAddrStore struct {
	store          db.Table
	bootstrapAddrs addr.MultiAddresses
}

// New constructs a new `MultiAddrStore`.
func New(store db.Table, bootstrapAddrs addr.MultiAddresses) MultiAddrStore {
	for _, addr := range bootstrapAddrs {
		if err := store.Insert(addr.ID().String(), addr.String()); err != nil {
			panic(fmt.Sprintf("[MultiAddrStore] cannot initialize the store with bootstrap nodes addresses"))
		}
	}
	return MultiAddrStore{
		store:          store,
		bootstrapAddrs: bootstrapAddrs,
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
	return multiStore.store.Insert(addr.ID().String(), addr.String())
}

// Delete removes the given multi-address from the store.
func (multiStore *MultiAddrStore) Delete(addr addr.MultiAddress) error {
	return multiStore.store.Delete(addr.ID().String())
}

// Size returns the number of entries in the store.
func (multiStore *MultiAddrStore) Size() (int, error) {
	return multiStore.store.Size()
}

// AddrsAll returns all of the multi-addresses in the store.
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

// BootstrapAll returns the multi-addresses of all of the Bootstrap nodes.
func (multiStore *MultiAddrStore) BootstrapAll() (addr.MultiAddresses, error) {
	return multiStore.RandomBootstrapAddrs(len(multiStore.bootstrapAddrs))
}

// RandomBootstrapAddrs returns a random number of Bootstrap multi-addresses in
// the store.
func (multiStore *MultiAddrStore) RandomBootstrapAddrs(n int) (addr.MultiAddresses, error) {
	indexes := rand.Perm(len(multiStore.bootstrapAddrs))
	if n > len(multiStore.bootstrapAddrs) {
		n = len(multiStore.bootstrapAddrs)
	}
	addrs := make(addr.MultiAddresses, 0, n)

	for _, index := range indexes {
		addrs = append(addrs, multiStore.bootstrapAddrs[index])
	}

	return addrs, nil
}

// RandomAddrs returns a random number of multi-addresses in the store.
func (multiStore *MultiAddrStore) RandomAddrs(n int) (addr.MultiAddresses, error) {
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
