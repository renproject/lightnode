package store

import (
	"errors"
)

// ErrKeyNotFound is returned when there is no value associated with the key.
var ErrKeyNotFound = errors.New("key not found")

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

	// Entries returns the number of entries in the store.
	Entries() int
}
