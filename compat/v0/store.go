package v0

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/go-redis/redis/v7"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/pack"
)

// The compat store aims to abstract compat persistence mappings
type CompatStore interface {
	GetV1TxFromTx(tx Tx) (tx.Tx, error)
	GetV1TxFromUTXO(utxo ExtBtcCompatUTXO) (tx.Tx, error)
	GetV1TxFromHash(hash B32) (tx.Tx, error)
	GetV1HashFromHash(hash B32) (pack.Bytes32, error)
	// Because amount is not provided in v0, and we can't use
	// bindings to re-fetch the amount once the utxo has been spent
	GetAmountFromUTXO(utxo ExtBtcCompatUTXO) (int64, error)
	// store required details to restore tx at later date
	PersistTxMappings(v0tx Tx, v1tx tx.Tx) error
}

type Store struct {
	db     db.DB
	client redis.Cmdable
}

// Wrap redis errors to hide implementation details
const ErrNotFound = CompatError("compatstore: not found")

type CompatError string

func (e CompatError) Error() string { return string(e) }

func NewCompatStore(db db.DB, client redis.Cmdable) Store {
	return Store{
		db:     db,
		client: client,
	}
}

func (store Store) decodeHashString(s string) (pack.Bytes32, error) {
	hash := pack.Bytes32{}
	hashBytes, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		err = fmt.Errorf("invalid hash encoding persisted: %v", err)
		return hash, err
	}

	copy(hash[:], hashBytes)
	return hash, nil
}

// Check redis for existing hash-hash mapping
func (store Store) GetV1HashFromHash(v0hash B32) (pack.Bytes32, error) {
	hashS, err := store.client.Get(v0hash.String()).Result()
	if err != nil {
		if err == redis.Nil {
			err = ErrNotFound
		}
		return pack.Bytes32{}, err
	}

	return store.decodeHashString(hashS)
}

// Get tx using v0 hash
func (store Store) GetV1TxFromHash(v0hash B32) (tx.Tx, error) {
	tx := tx.Tx{}
	hash, err := store.GetV1HashFromHash(v0hash)
	if err != nil {
		return tx, err
	}
	return store.db.Tx(hash)
}

func (store Store) getTxHashFromUTXO(utxo ExtBtcCompatUTXO) (pack.Bytes32, error) {
	hashS, err := store.client.Get(utxoLookupString(utxo)).Result()
	if err != nil {
		if err == redis.Nil {
			err = ErrNotFound
		}
		return pack.Bytes32{}, err
	}
	return store.decodeHashString(hashS)
}

func utxoLookupString(utxo ExtBtcCompatUTXO) string {
	txid := utxo.TxHash.String()
	txindex := utxo.VOut.Int.String()
	return txid + "_" + txindex
}

func (store Store) GetAmountFromUTXO(utxo ExtBtcCompatUTXO) (int64, error) {
	amount, err := store.client.Get("amount_" + utxoLookupString(utxo)).Int64()
	if err == redis.Nil {
		err = ErrNotFound
	}
	return amount, err
}

func (store Store) GetV1TxFromUTXO(utxo ExtBtcCompatUTXO) (tx.Tx, error) {
	hash, err := store.getTxHashFromUTXO(utxo)
	if err != nil {
		return tx.Tx{}, err
	}
	return store.db.Tx(hash)
}

func (store Store) PersistTxMappings(v0tx Tx, v1tx tx.Tx) error {
	// persist v0 hash for later query-lookup
	err := store.client.Set(v0tx.Hash.String(), v1tx.Hash.String(), 0).Err()
	if err != nil {
		return err
	}

	expiry := time.Duration(time.Hour * 24 * 7)

	utxo := v0tx.In.Get("utxo").Value.(ExtBtcCompatUTXO)
	amount := v1tx.Input.Get("amount").(pack.U256)
	utxokey := utxoLookupString(utxo)

	// persist amount so that we don't need to re-fetch it
	err = store.client.Set("amount_"+utxokey, amount.Int().Int64(), time.Duration(time.Hour*24*7)).Err()
	if err != nil {
		return err
	}

	// Also allow for lookup by btc utxo; as we don't have the v0 hash at submission
	// Expire these because it's only useful during submission, not querying
	return store.client.Set(utxokey, v1tx.Hash.String(), expiry).Err()
}

func (store Store) GetV1TxFromTx(tx Tx) (tx.Tx, error) {
	// if the v0 TX has a non empty hash, use that for lookup
	if (tx.Hash != B32{}) {
		return store.GetV1TxFromHash(tx.Hash)
	}
	utxo := tx.In.Get("utxo").Value.(ExtBtcCompatUTXO)
	return store.GetV1TxFromUTXO(utxo)
}
