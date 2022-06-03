package resolver

import (
	"errors"
	"time"

	"github.com/go-redis/redis/v7"
	"github.com/renproject/id"
)

// ErrNotFound wraps redis errors to hide implementation details
var ErrNotFound = errors.New("compatstore : not found")

// CompatStore aims to abstract compat persistence mappings
type CompatStore interface {

	// TxHashMapping maps the hash of a compat tx to the hash of the standard tx.
	TxHashMapping(compatHash, standardHash id.Hash) error

	// GetStandardHash gets the standard tx hash from compat tx hash
	GetStandardHash(compatHash id.Hash) (id.Hash, error)
}

type Store struct {
	client redis.Cmdable
	expiry time.Duration
}

func NewCompatStore(client redis.Cmdable, expiry time.Duration) Store {
	return Store{
		client: client,
		expiry: expiry,
	}
}

func (store Store) TxHashMapping(compatHash, standardHash id.Hash) error {
	return store.client.Set(compatHash.String(), standardHash.String(), store.expiry).Err()
}

func (store Store) GetStandardHash(compatHash id.Hash) (id.Hash, error) {
	hashStr, err := store.client.Get(compatHash.String()).Result()
	if err != nil {
		if err == redis.Nil {
			err = ErrNotFound
		}
		return id.Hash{}, err
	}
	var hash id.Hash
	copy(hash[:], hashStr)
	return hash, nil
}
