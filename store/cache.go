package store

import (
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// ErrDataExpired is returned when the data is expired.
var ErrDataExpired = errors.New("data expired")

// ErrNoMoreItems is returned when no more items left in the iterator.
var ErrNoMoreItems = errors.New("no more items in iterator")

// cache is an in-memory implementation of the KVStore. It is safe for concurrent read and write. The data stored will
// have a valid duration. After the data expired, it will returns ErrDataExpired error to alert user to update the data.
type cache struct {
	mu          *sync.RWMutex
	data        map[string][]byte
	lastSeen    map[string]int64
	expiredTime int64
}

// NewCache returns a new cached KVStore. If the expiredTime is less than or equal to 0, the data will not have
// expiration time.
func NewCache(expiredTime int64) KVStore {
	return cache{
		mu:          new(sync.RWMutex),
		data:        map[string][]byte{},
		lastSeen:    map[string]int64{},
		expiredTime: expiredTime,
	}
}

// Read implements the `KVStore` interface.
func (cache cache) Read(key string, value interface{}) error {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	// Check if the value is expired.
	if cache.expiredTime > 0 {
		lastSeen, ok := cache.lastSeen[key]
		if !ok {
			return ErrKeyNotFound
		}
		if (time.Now().Unix() - lastSeen) > cache.expiredTime {
			return ErrDataExpired
		}
	}

	val, ok := cache.data[key]
	if !ok {
		return ErrKeyNotFound
	}

	return json.Unmarshal(val, value)
}

// Write implements the `KVStore` interface.
func (cache cache) Write(key string, value interface{}) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	val, err := json.Marshal(value)
	if err != nil {
		return err
	}
	cache.data[key] = val
	if cache.expiredTime > 0 {
		cache.lastSeen[key] = time.Now().Unix()
	}

	return nil
}

// Delete implements the `KVStore` interface.
func (cache cache) Delete(key string) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	delete(cache.data, key)
	delete(cache.lastSeen, key)
	return nil
}

// Entries implements the `KVStore` interface.
func (cache cache) Entries() int {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	return len(cache.data)
}

func (cache cache) Iterator() KVStoreIterator {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	return newCacheIterator(cache.data)
}

type cacheIterator struct {
	index  int
	keys   []string
	values [][]byte
}

func newCacheIterator(data map[string][]byte) *cacheIterator {
	iter := &cacheIterator{
		index:  -1,
		keys:   make([]string, len(data)),
		values: make([][]byte, len(data)),
	}
	index := 0
	for key, value := range data {
		iter.keys[index] = key
		iter.values[index] = value
		index++
	}

	return iter
}

func (iter *cacheIterator) Next() bool {
	iter.index++
	return iter.index < len(iter.keys)
}

func (iter *cacheIterator) KV(value interface{}) (string, error) {
	if iter.index >= len(iter.keys) {
		return "", ErrNoMoreItems
	}

	if err := json.Unmarshal(iter.values[iter.index], &value); err != nil {
		return "", err
	}

	return iter.keys[iter.index], nil
}
