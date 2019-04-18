package store

import (
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// ErrDataExpired is returned when the data is expired.
var ErrDataExpired = errors.New("data expired")

// cache is an in-memory implementation of the KVStore. It is safe for concurrent read and write. The data stored will have
// a valid duration. After the data expired, it will returns ErrDataExpired error to alert user to update the data.
type cache struct {
	mu          *sync.RWMutex
	data        map[string][]byte
	lastSeen    map[string]int64
	expiredTime int64
}

// NewCache returns a new cached KVStore
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

	lastSeen, ok := cache.lastSeen[key]
	if !ok {
		return ErrKeyNotFound
	}
	if cache.expiredTime > 0 && (time.Now().Unix()-lastSeen) > cache.expiredTime {
		return ErrDataExpired
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

	// TODO: It may be better to use gob in this case for performance
	val, err := json.Marshal(value)
	if err != nil {
		return err
	}
	cache.data[key] = val
	cache.lastSeen[key] = time.Now().Unix()
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
