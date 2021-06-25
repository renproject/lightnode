package v1

import (
	"encoding/base64"
	"fmt"

	"github.com/go-redis/redis/v7"
	"github.com/renproject/darknode/engine"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/id"
	"github.com/renproject/pack"
)

type GpubkeyCompatStore interface {
	RemoveGpubkey(tx.Tx) (tx.Tx, error)
	UpdatedHash(id.Hash) (id.Hash, error)
}

type Store struct {
	client redis.Cmdable
}

func NewCompatStore(client redis.Cmdable) *Store {
	return &Store{
		client: client,
	}
}

func (store *Store) RemoveGpubkey(transaction tx.Tx) (tx.Tx, error) {
	var input engine.LockMintBurnReleaseInput
	if err := pack.Decode(&input, transaction.Input); err != nil {
		return tx.Tx{}, err
	}
	input.Gpubkey = pack.Bytes{}
	inputEncoded, err := pack.Encode(input)
	if err != nil {
		return tx.Tx{}, err
	}
	newTx, err := tx.NewTx(transaction.Selector, pack.Typed(inputEncoded.(pack.Struct)))
	if err != nil {
		return tx.Tx{}, err
	}
	err = store.client.Set(transaction.Hash.String(), newTx.Hash.String(), 0).Err()
	return newTx, err
}

func (store *Store) UpdatedHash(hash id.Hash) (id.Hash, error) {
	hashStr, err := store.client.Get(hash.String()).Result()
	if err != nil {
		return id.Hash{}, err
	}

	newHash := id.Hash{}
	newHashBytes, err := base64.RawURLEncoding.DecodeString(hashStr)
	if err != nil {
		return hash, fmt.Errorf("bad compat hash %v for %v: %v", hashStr, hash, err)
	}

	copy(newHash[:], newHashBytes)
	return newHash, nil
}
