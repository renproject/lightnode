package store

import (
	"fmt"
	"math/rand"

	"github.com/renproject/aw/wire"
	"github.com/renproject/kv/db"
)

// MultiAddrStore is a store of `wire.Address`es.
type MultiAddrStore struct {
	store          db.Table
	bootstrapAddrs []wire.Address
}

// New constructs a new `MultiAddrStore`.
func New(store db.Table, bootstrapAddrs []wire.Address) MultiAddrStore {
	multiStore := MultiAddrStore{
		store:          store,
		bootstrapAddrs: bootstrapAddrs,
	}

	for _, addr := range bootstrapAddrs {
		if err := multiStore.Insert(addr); err != nil {
			panic(fmt.Sprintf("[MultiAddrStore] cannot initialize the store with bootstrap nodes addresses"))
		}
	}
	return multiStore
}

// Get retrieves a multi-address from the store.
func (multiStore *MultiAddrStore) Get(id string) (wire.Address, error) {
	var addrString string
	if err := multiStore.store.Get(id, &addrString); err != nil {
		return wire.Address{}, err
	}
	return wire.DecodeString(addrString)
}

// Insert puts the given multi-address into the store.
func (multiStore *MultiAddrStore) Insert(addr wire.Address) error {
	signatory, err := addr.Signatory()
	if err != nil {
		return err
	}

	return multiStore.store.Insert(signatory.String(), addr.String())
}

// Delete removes the given multi-address from the store.
func (multiStore *MultiAddrStore) Delete(addr wire.Address) error {
	signatory, err := addr.Signatory()
	if err != nil {
		return err
	}
	return multiStore.store.Delete(signatory.String())
}

// Size returns the number of entries in the store.
func (multiStore *MultiAddrStore) Size() (int, error) {
	return multiStore.store.Size()
}

// AddrsAll returns all of the multi-addresses in the store.
func (multiStore *MultiAddrStore) AddrsAll() ([]wire.Address, error) {
	addrs := []wire.Address{}
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
func (multiStore *MultiAddrStore) BootstrapAll() ([]wire.Address, error) {
	return multiStore.bootstrapAddrs, nil
}

// RandomBootstrapAddrs returns a random number of Bootstrap multi-addresses in
// the store.
func (multiStore *MultiAddrStore) RandomBootstrapAddrs(n int) ([]wire.Address, error) {
	indexes := rand.Perm(len(multiStore.bootstrapAddrs))
	if n > len(multiStore.bootstrapAddrs) {
		n = len(multiStore.bootstrapAddrs)
	}
	addrs := make([]wire.Address, 0, n)

	for _, index := range indexes {
		addrs = append(addrs, multiStore.bootstrapAddrs[index])
	}

	return addrs, nil
}

// RandomAddrs returns a random number of multi-addresses in the store.
func (multiStore *MultiAddrStore) RandomAddrs(n int) ([]wire.Address, error) {
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
