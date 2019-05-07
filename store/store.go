package store

import (
	"errors"
)

// ErrKeyNotFound is returned when there is no value associated with the key.
var ErrKeyNotFound = errors.New("key not found")

// KVStore is a generic key-value store. The key must be of type string, though there are no restrictions on the type
// of the value.
type KVStore interface {
	// Read the value associated with the given key. This function returns ErrKeyNotFound if the key cannot be found. I
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
