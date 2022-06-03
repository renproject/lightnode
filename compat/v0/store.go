package v0

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v7"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/id"
	"github.com/renproject/lightnode/db"
)

// ErrNotFound wraps redis errors to hide implementation details
var ErrNotFound = errors.New("compatstore : not found")

// CompatStore aims to abstract compat persistence mappings
type CompatStore interface {

	// PersistTxMappings maps the v0 tx to the v1 tx for future querying.
	PersistTxMappings(v0tx Tx, v1tx tx.Tx) error

	// GetV1TxFromTx gets the v1 transaction from v0 transaction
	GetV1TxFromTx(tx Tx) (tx.Tx, error)

	// GetV1HashFromHash gets the v1 transaction hash from v0 transaction hash.
	GetV1HashFromHash(hash B32) (id.Hash, error)
}

type Store struct {
	db     db.DB         // db which stores all v1 transactions
	client redis.Cmdable // client interacting the mapping between v0 and v1 txs
	expiry time.Duration // expiry of the mapping entry, should be same as the db prune time.
}

func NewCompatStore(db db.DB, client redis.Cmdable, expiry time.Duration) Store {
	return Store{
		db:     db,
		client: client,
		expiry: expiry,
	}
}

func (store Store) PersistTxMappings(v0tx Tx, v1tx tx.Tx) error {
	// persist v0 hash for later query-lookup
	err := store.client.Set(v0tx.Hash.String(), v1tx.Hash.String(), store.expiry).Err()
	if err != nil {
		return err
	}

	// We assume both v0 and v1 txs are valid.
	if IsShiftIn(v0tx.To) {
		// For mints, we also maps the utxo+vout to v1 hash for future lookup
		// as we don't have the v0 hash at submission
		utxo := v0tx.In.Get("utxo").Value.(ExtBtcCompatUTXO)
		utxoKey := utxoLookupString(utxo)
		return store.client.Set(utxoKey, v1tx.Hash.String(), store.expiry).Err()
	} else {
		// For burns, we also maps the ref to v1 hash for future look up
		// as we don't have the v0 hash at submission
		selector := tx.Selector(fmt.Sprintf("%s/fromEthereum", v0tx.To[0:3]))
		ref := v0tx.In.Get("ref").Value.(U64)
		refKey := refLookupString(selector, ref)
		return store.client.Set(refKey, v1tx.Hash.String(), store.expiry).Err()
	}
}

func (store Store) GetV1HashFromHash(v0hash B32) (id.Hash, error) {
	hashS, err := store.client.Get(v0hash.String()).Result()
	if err != nil {
		if err == redis.Nil {
			err = ErrNotFound
		}
		return id.Hash{}, err
	}

	return store.decodeHashString(hashS)
}

func (store Store) GetV1TxFromTx(transaction Tx) (tx.Tx, error) {
	// We don't trust the tx hash from the input, instead we query the utxo/ref
	// for mints/burns which is a primary key for mints/burns
	var hash id.Hash
	var err error
	if IsShiftIn(transaction.To) {
		utxo := transaction.In.Get("utxo").Value.(ExtBtcCompatUTXO)
		hash, err = store.getV1TxHashFromUTXO(utxo)
	} else {
		selector := tx.Selector(fmt.Sprintf("%s/fromEthereum", transaction.To[0:3]))
		ref := transaction.In.Get("ref").Value.(U64)
		hash, err = store.getV1TxHashFromRef(selector, ref)
	}
	if err != nil {
		return tx.Tx{}, err
	}

	// Fetch the tx details from the db
	v1Tx, err := store.db.Tx(hash)
	if err == sql.ErrNoRows {
		err = ErrNotFound
	}
	return v1Tx, err
}

func (store Store) getV1TxHashFromUTXO(utxo ExtBtcCompatUTXO) (id.Hash, error) {
	hashS, err := store.client.Get(utxoLookupString(utxo)).Result()
	if err != nil {
		if err == redis.Nil {
			err = ErrNotFound
		}
		return id.Hash{}, err
	}
	return store.decodeHashString(hashS)
}

func (store Store) getV1TxHashFromRef(selector tx.Selector, ref U64) (id.Hash, error) {
	key := refLookupString(selector, ref)
	hashS, err := store.client.Get(key).Result()
	if err != nil {
		if err == redis.Nil {
			err = ErrNotFound
		}
		return id.Hash{}, err
	}
	return store.decodeHashString(hashS)
}

func (store Store) decodeHashString(s string) (id.Hash, error) {
	hash := id.Hash{}
	hashBytes, err1 := base64.RawURLEncoding.DecodeString(s)
	if err1 != nil {
		hashBytes2, err2 := base64.StdEncoding.DecodeString(s)
		if err2 != nil {
			err := fmt.Errorf("invalid hash encoding ( %v ) persisted: not base64URL %v not base64 %v", s, err1, err2)
			return hash, err
		}
		hashBytes = hashBytes2
	}

	copy(hash[:], hashBytes)
	return hash, nil
}

func utxoLookupString(utxo ExtBtcCompatUTXO) string {
	txid := utxo.TxHash.String()
	txindex := utxo.VOut.Int.String()
	return txid + "_" + txindex
}

func refLookupString(selector tx.Selector, ref U64) string {
	return fmt.Sprintf("%v_%v", selector.String(), ref.Int.String())
}
