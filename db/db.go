package db

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"math/big"
	"time"

	"github.com/renproject/darknode/tx"
	"github.com/renproject/darknode/txengine"
	"github.com/renproject/multichain/api/utxo"
	"github.com/renproject/pack"
)

type TxStatus int8

const (
	TxStatusNil TxStatus = iota
	TxStatusConfirming
	TxStatusConfirmed
	TxStatusSubmitted
)

type Scannable interface {
	Scan(dest ...interface{}) error
}

// DB is a storage adapter (built on top of a SQL database) that stores all
// transaction details.
type DB interface {
	// Initialise the database. Init should be called once the database object
	// is created.
	Init() error

	// InsertTx inserts the transaction into the database.
	InsertTx(tx tx.Tx) error

	// Tx gets the details of the transaction with the given hash. It returns an
	// `sql.ErrNoRows` if the transaction cannot be found.
	Tx(hash pack.Bytes32) (tx.Tx, error)

	// Txs returns transactions with the given pagination options.
	Txs(offset, limit int) ([]tx.Tx, error)

	// PendingTxs returns all pending transactions in the database which are not
	// expired.
	PendingTxs(expiry time.Duration) ([]tx.Tx, error)

	// TxStatus returns the current status of the transaction with the given
	// hash.
	TxStatus(hash pack.Bytes32) (TxStatus, error)

	// UpdateStatus updates the status of the given transaction. The status
	// cannot be updated to a previous status.
	UpdateStatus(hash pack.Bytes32, status TxStatus) error

	// Prune deletes transactions which have expired.
	Prune(expiry time.Duration) error
}

type database struct {
	db *sql.DB
}

// New creates a new DB instance.
func New(db *sql.DB) DB {
	return database{
		db: db,
	}
}

// Init creates the tables for storing transactions if they do not already
// exist. The tables will only be created the first time this funciton is called
// and any future calls will not return an error.
func (db database) Init() error {
	// Create the lock-and-mint (UTXO -> account) table if it does not exist.
	lockUTXOMintAccount := `CREATE TABLE IF NOT EXISTS lock_utxo_mint_account (
		hash          VARCHAR NOT NULL PRIMARY KEY,
		status        SMALLINT,
		created_time  BIGINT,
		selector      VARCHAR(255),
		utxo_hash     VARCHAR,
		utxo_index    BIGINT,
		value         VARCHAR(100),
		pubkey_script VARCHAR,
		payload       VARCHAR,
		phash         VARCHAR,
		toAddr        VARCHAR,
		nonce         VARCHAR,
		nhash         VARCHAR,
		gpubkey       VARCHAR,
		ghash         VARCHAR
	);`
	_, err := db.db.Exec(lockUTXOMintAccount)
	if err != nil {
		return err
	}

	// Create the lock-and-mint (account -> account) table if it does not exist.
	lockAccountMintAccount := `CREATE TABLE IF NOT EXISTS lock_account_mint_account (
		hash          VARCHAR NOT NULL PRIMARY KEY,
		status        SMALLINT,
		created_time  BIGINT,
		selector      VARCHAR(255),
		tx_id         VARCHAR,
		amount        VARCHAR(100),
		payload       VARCHAR,
		phash         VARCHAR,
		toAddr        VARCHAR,
		nonce         VARCHAR,
		nhash         VARCHAR,
		gpubkey       VARCHAR
	);`
	_, err = db.db.Exec(lockAccountMintAccount)
	if err != nil {
		return err
	}

	// Create the burn-and-release (account -> UTXO) table if not exist.
	burnAccountReleaseUTXO := `CREATE TABLE IF NOT EXISTS burn_account_release_utxo (
		hash         VARCHAR NOT NULL PRIMARY KEY,
		status       SMALLINT,
		created_time BIGINT,
		selector     VARCHAR(255),
		amount       VARCHAR(100),
		toAddr       VARCHAR,
		nonce        VARCHAR
	);`
	_, err = db.db.Exec(burnAccountReleaseUTXO)
	if err != nil {
		return err
	}

	// Create the burn-and-release (account -> account) table if not exist.
	burnAccountReleaseAccount := `CREATE TABLE IF NOT EXISTS burn_account_release_account (
		hash         VARCHAR NOT NULL PRIMARY KEY,
		status       SMALLINT,
		created_time BIGINT,
		selector     VARCHAR(255),
		amount       VARCHAR(100),
		toAddr       VARCHAR,
		nonce        VARCHAR
	);`
	_, err = db.db.Exec(burnAccountReleaseAccount)
	return err
}

// InsertTx implements the DB interface.
func (db database) InsertTx(tx tx.Tx) error {
	switch {
	case tx.Selector.IsLockAndMint():
		lockChain, ok := tx.Selector.LockChain()
		if !ok {
			return fmt.Errorf("invalid selector %v", tx.Selector)
		}
		if lockChain.IsUTXOBased() {
			return db.insertLockUTXOMintAccountTx(tx)
		}
		return db.insertLockAccountMintAccountTx(tx)
	case tx.Selector.IsBurnAndRelease():
		releaseChain, ok := tx.Selector.ReleaseChain()
		if !ok {
			return fmt.Errorf("invalid selector %v", tx.Selector)
		}
		if releaseChain.IsUTXOBased() {
			return db.insertBurnAccountReleaseUTXOTx(tx)
		}
		return db.insertBurnAccountReleaseAccountTx(tx)
	default:
		return fmt.Errorf("unexpected tx selector %v", tx.Selector.String())
	}
}

// Tx implements the DB interface.
func (db database) Tx(hash pack.Bytes32) (tx.Tx, error) {
	transaction, err := db.lockUTXOMintAccountTx(hash)
	if err != sql.ErrNoRows {
		return transaction, err
	}
	transaction, err = db.lockAccountMintAccountTx(hash)
	if err != sql.ErrNoRows {
		return transaction, err
	}
	transaction, err = db.burnAccountReleaseUTXOTx(hash)
	if err != sql.ErrNoRows {
		return transaction, err
	}
	transaction, err = db.burnAccountReleaseAccountTx(hash)
	if err != sql.ErrNoRows {
		return transaction, err
	}
	return tx.Tx{}, err
}

// Txs implements the DB interface.
func (db database) Txs(offset, limit int) ([]tx.Tx, error) {
	txs := make([]tx.Tx, 0, limit)
	rows, err := db.db.Query(`SELECT tableName, hash, selector, tx_id, amount, utxo_hash, utxo_index, value, pubkey_script, payload, phash, toAddr, nonce, nhash, gpubkey, ghash FROM (
		SELECT 'lock_utxo_mint_account' AS tableName, hash, created_time, selector, '' AS tx_id, '' AS amount, utxo_hash, utxo_index, value, pubkey_script, payload, phash, toAddr, nonce, nhash, gpubkey, ghash FROM lock_utxo_mint_account UNION
		SELECT 'lock_account_mint_account' AS tableName, hash, created_time, selector, tx_id, amount, '' AS utxo_hash, '' AS utxo_index, '' AS value, '' AS pubkey_script, payload, phash, toAddr, nonce, nhash, gpubkey, ghash FROM lock_account_mint_account UNION
		SELECT 'burn_account_release_utxo' AS tableName, hash, created_time, selector, '' AS tx_id, amount, '' AS utxo_hash, '' AS utxo_index, '' AS value, '' AS pubkey_script, '' AS payload, '' AS phash, toAddr, nonce, '' AS nhash, '' AS gpubkey, '' AS ghash FROM burn_account_release_utxo UNION
		SELECT 'burn_account_release_account' AS tableName, hash, created_time, selector, '' AS tx_id, amount, '' AS utxo_hash, '' AS utxo_index, '' AS value, '' AS pubkey_script, '' AS payload, '' AS phash, toAddr, nonce, '' AS nhash, '' AS gpubkey, '' AS ghash FROM burn_account_release_account
	) AS shifts ORDER BY created_time ASC LIMIT $1 OFFSET $2;`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Loop through rows and convert them to transactions.
	for rows.Next() {
		tx, err := rowToTx(rows)
		if err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}
	return txs, rows.Err()
}

// PendingTxs implements the DB interface.
func (db database) PendingTxs(expiry time.Duration) ([]tx.Tx, error) {
	txs := make([]tx.Tx, 0, 128)

	// Get pending lock-and-mint (UTXO -> account) transactions from the database.
	rows, err := db.db.Query(`SELECT hash, selector, utxo_hash, utxo_index, value, pubkey_script, payload, phash, toAddr, nonce, nhash, gpubkey, ghash FROM lock_utxo_mint_account
		WHERE status = $1 AND $2 - created_time < $3;`, TxStatusConfirming, time.Now().Unix(), int64(expiry.Seconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		transaction, err := rowToLockUTXOMintAccountTx(rows)
		if err != nil {
			return nil, err
		}
		txs = append(txs, transaction)
	}
	if rows.Err() != nil {
		return nil, err
	}

	// Get pending lock-and-mint (account -> account) transactions from the database.
	rows, err = db.db.Query(`SELECT hash, selector, tx_id, amount, payload, phash, toAddr, nonce, nhash, gpubkey FROM lock_account_mint_account
		WHERE status = $1 AND $2 - created_time < $3;`, TxStatusConfirming, time.Now().Unix(), int64(expiry.Seconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		transaction, err := rowToLockAccountMintAccountTx(rows)
		if err != nil {
			return nil, err
		}
		txs = append(txs, transaction)
	}
	if rows.Err() != nil {
		return nil, err
	}

	// Get pending burn-and-release (account -> UTXO) transactions from the database.
	rows, err = db.db.Query(`SELECT hash, selector, amount, toAddr, nonce FROM burn_account_release_utxo
	WHERE status = $1 AND $2 - created_time < $3`, TxStatusConfirming, time.Now().Unix(), int64(expiry.Seconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		transaction, err := rowToBurnAccountReleaseUTXOTx(rows)
		if err != nil {
			return nil, err
		}
		txs = append(txs, transaction)
	}

	// Get pending burn-and-release (account -> account) transactions from the database.
	rows, err = db.db.Query(`SELECT hash, selector, amount, toAddr, nonce FROM burn_account_release_account
	WHERE status = $1 AND $2 - created_time < $3`, TxStatusConfirming, time.Now().Unix(), int64(expiry.Seconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		transaction, err := rowToBurnAccountReleaseAccountTx(rows)
		if err != nil {
			return nil, err
		}
		txs = append(txs, transaction)
	}
	return txs, rows.Err()
}

// TxStatus implements the DB interface.
func (db database) TxStatus(txHash pack.Bytes32) (TxStatus, error) {
	var status int
	err := db.db.QueryRow(`SELECT status FROM lock_utxo_mint_account WHERE hash = $1;`, txHash.String()).Scan(&status)
	if err != sql.ErrNoRows {
		return TxStatus(status), err
	}
	err = db.db.QueryRow(`SELECT status FROM lock_account_mint_account WHERE hash = $1;`, txHash.String()).Scan(&status)
	if err != sql.ErrNoRows {
		return TxStatus(status), err
	}
	err = db.db.QueryRow(`SELECT status FROM burn_account_release_utxo WHERE hash = $1;`, txHash.String()).Scan(&status)
	if err != sql.ErrNoRows {
		return TxStatus(status), err
	}
	err = db.db.QueryRow(`SELECT status FROM burn_account_release_account WHERE hash = $1;`, txHash.String()).Scan(&status)
	if err != sql.ErrNoRows {
		return TxStatus(status), err
	}
	return TxStatusNil, err
}

// UpdateStatus implements the DB interface.
func (db database) UpdateStatus(txHash pack.Bytes32, status TxStatus) error {
	_, err := db.db.Exec("UPDATE lock_utxo_mint_account SET status = $1 WHERE hash = $2 AND status < $1;", status, txHash.String())
	if err != nil {
		return err
	}
	_, err = db.db.Exec("UPDATE lock_account_mint_account SET status = $1 WHERE hash = $2 AND status < $1;", status, txHash.String())
	if err != nil {
		return err
	}
	_, err = db.db.Exec("UPDATE burn_account_release_utxo SET status = $1 WHERE hash = $2 AND status < $1;", status, txHash.String())
	if err != nil {
		return err
	}
	_, err = db.db.Exec("UPDATE burn_account_release_account SET status = $1 WHERE hash = $2 AND status < $1;", status, txHash.String())
	return err
}

func checkCount(rows *sql.Rows) (count int) {
	for rows.Next() {
		err := rows.Scan(&count)
		if err != nil {
			panic(err)
		}
	}
	return count
}

// Prune deletes txs which have expired based on the given expiry.
func (db database) Prune(expiry time.Duration) error {
	_, err := db.db.Exec("DELETE FROM lock_utxo_mint_account WHERE $1 - created_time > $2;", time.Now().Unix(), int(expiry.Seconds()))
	if err != nil {
		return err
	}
	_, err = db.db.Exec("DELETE FROM lock_account_mint_account WHERE $1 - created_time > $2;", time.Now().Unix(), int(expiry.Seconds()))
	if err != nil {
		return err
	}
	_, err = db.db.Exec("DELETE FROM burn_account_release_utxo WHERE $1 - created_time > $2;", time.Now().Unix(), int(expiry.Seconds()))
	if err != nil {
		return err
	}
	_, err = db.db.Exec("DELETE FROM burn_account_release_account WHERE $1 - created_time > $2;", time.Now().Unix(), int(expiry.Seconds()))
	return err
}

func (db database) insertLockUTXOMintAccountTx(tx tx.Tx) error {
	output, ok := tx.Input.Get("output").(pack.Struct)
	if !ok {
		return fmt.Errorf("unexpected type for output: expected pack.Struct, got %v", tx.Input.Get("output").Type())
	}
	outpoint, ok := output.Get("outpoint").(pack.Struct)
	if !ok {
		return fmt.Errorf("unexpected type for outpoint: expected pack.Struct, got %v", output.Get("outpoint").Type())
	}
	hash, ok := outpoint.Get("hash").(pack.Bytes)
	if !ok {
		return fmt.Errorf("unexpected type for hash: expected pack.Bytes, got %v", outpoint.Get("hash").Type())
	}
	index, ok := outpoint.Get("index").(pack.U32)
	if !ok {
		return fmt.Errorf("unexpected type for index: expected pack.U32, got %v", outpoint.Get("index").Type())
	}
	value, ok := output.Get("value").(pack.U256)
	if !ok {
		return fmt.Errorf("unexpected type for value: expected pack.U256, got %v", output.Get("value").Type())
	}
	pubKeyScript, ok := output.Get("pubKeyScript").(pack.Bytes)
	if !ok {
		return fmt.Errorf("unexpected type for pubKeyScript: expected pack.Bytes, got %v", output.Get("pubKeyScript").Type())
	}
	payload, ok := tx.Input.Get("payload").(pack.Bytes)
	if !ok {
		return fmt.Errorf("unexpected type for payload: expected pack.Bytes, got %v", tx.Input.Get("payload").Type())
	}
	phash, ok := tx.Input.Get("phash").(pack.Bytes32)
	if !ok {
		return fmt.Errorf("unexpected type for phash: expected pack.Bytes32, got %v", tx.Input.Get("phash").Type())
	}
	to, ok := tx.Input.Get("to").(pack.String)
	if !ok {
		return fmt.Errorf("unexpected type for to: expected pack.String, got %v", tx.Input.Get("to").Type())
	}
	nonce, ok := tx.Input.Get("nonce").(pack.Bytes32)
	if !ok {
		return fmt.Errorf("unexpected type for nonce: expected pack.Bytes32, got %v", tx.Input.Get("nonce").Type())
	}
	nhash, ok := tx.Input.Get("nhash").(pack.Bytes32)
	if !ok {
		return fmt.Errorf("unexpected type for nhash: expected pack.Bytes32, got %v", tx.Input.Get("nhash").Type())
	}
	gpubkey, ok := tx.Input.Get("gpubkey").(pack.Bytes)
	if !ok {
		return fmt.Errorf("unexpected type for gpubkey: expected pack.Bytes, got %v", tx.Input.Get("gpubkey").Type())
	}
	ghash, ok := tx.Input.Get("ghash").(pack.Bytes32)
	if !ok {
		return fmt.Errorf("unexpected type for ghash: expected pack.Bytes32, got %v", tx.Input.Get("ghash").Type())
	}

	script := `INSERT INTO lock_utxo_mint_account (hash, status, created_time, selector, utxo_hash, utxo_index, value, pubkey_script, payload, phash, toAddr, nonce, nhash, gpubkey, ghash) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15);`
	_, err := db.db.Exec(script,
		tx.Hash.String(),
		TxStatusConfirming,
		time.Now().Unix(),
		tx.Selector.String(),
		hash.String(),
		index,
		value.String(),
		pubKeyScript.String(),
		payload.String(),
		phash.String(),
		to.String(),
		nonce.String(),
		nhash.String(),
		gpubkey.String(),
		ghash.String(),
	)

	return err
}

func (db database) insertLockAccountMintAccountTx(tx tx.Tx) error {
	txID, ok := tx.Input.Get("txid").(pack.Bytes)
	if !ok {
		return fmt.Errorf("unexpected type for txid: expected pack.Bytes, got %v", tx.Input.Get("txid").Type())
	}
	amount, ok := tx.Input.Get("amount").(pack.U256)
	if !ok {
		return fmt.Errorf("unexpected type for amount: expected pack.U256, got %v", tx.Input.Get("amount").Type())
	}
	payload, ok := tx.Input.Get("payload").(pack.Bytes)
	if !ok {
		return fmt.Errorf("unexpected type for payload: expected pack.Bytes, got %v", tx.Input.Get("payload").Type())
	}
	phash, ok := tx.Input.Get("phash").(pack.Bytes32)
	if !ok {
		return fmt.Errorf("unexpected type for phash: expected pack.Bytes32, got %v", tx.Input.Get("phash").Type())
	}
	to, ok := tx.Input.Get("to").(pack.String)
	if !ok {
		return fmt.Errorf("unexpected type for to: expected pack.String, got %v", tx.Input.Get("to").Type())
	}
	nonce, ok := tx.Input.Get("nonce").(pack.U256)
	if !ok {
		return fmt.Errorf("unexpected type for nonce: expected pack.U256, got %v", tx.Input.Get("nonce").Type())
	}
	nhash, ok := tx.Input.Get("nhash").(pack.Bytes32)
	if !ok {
		return fmt.Errorf("unexpected type for nhash: expected pack.Bytes32, got %v", tx.Input.Get("nhash").Type())
	}
	gpubkey, ok := tx.Input.Get("gpubkey").(pack.Bytes)
	if !ok {
		return fmt.Errorf("unexpected type for gpubkey: expected pack.Bytes, got %v", tx.Input.Get("gpubkey").Type())
	}

	script := `INSERT INTO lock_account_mint_account (hash, status, created_time, selector, tx_id, amount, payload, phash, toAddr, nonce, nhash, gpubkey) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);`
	_, err := db.db.Exec(script,
		tx.Hash.String(),
		TxStatusConfirming,
		time.Now().Unix(),
		tx.Selector.String(),
		txID.String(),
		amount.String(),
		payload.String(),
		phash.String(),
		to.String(),
		nonce.String(),
		nhash.String(),
		gpubkey.String(),
	)

	return err
}

func (db database) insertBurnAccountReleaseUTXOTx(tx tx.Tx) error {
	amount, ok := tx.Input.Get("amount").(pack.U256)
	if !ok {
		return fmt.Errorf("unexpected type for amount: expected pack.U256, got %v", tx.Input.Get("amount").Type())
	}
	to, ok := tx.Input.Get("to").(pack.String)
	if !ok {
		return fmt.Errorf("unexpected type for to: expected pack.String, got %v", tx.Input.Get("to").Type())
	}
	nonce, ok := tx.Input.Get("nonce").(pack.Bytes32)
	if !ok {
		return fmt.Errorf("unexpected type for nonce: expected pack.Bytes32, got %v", tx.Input.Get("nonce").Type())
	}

	script := `INSERT INTO burn_account_release_utxo (hash, status, created_time, selector, amount, toAddr, nonce) VALUES ($1, $2, $3, $4, $5, $6, $7);`
	_, err := db.db.Exec(script,
		tx.Hash.String(),
		TxStatusConfirming,
		time.Now().Unix(),
		tx.Selector.String(),
		amount.String(),
		to.String(),
		nonce.String(),
	)

	return err
}

func (db database) insertBurnAccountReleaseAccountTx(tx tx.Tx) error {
	amount, ok := tx.Input.Get("amount").(pack.U256)
	if !ok {
		return fmt.Errorf("unexpected type for amount: expected pack.U256, got %v", tx.Input.Get("amount").Type())
	}
	to, ok := tx.Input.Get("to").(pack.String)
	if !ok {
		return fmt.Errorf("unexpected type for to: expected pack.String, got %v", tx.Input.Get("to").Type())
	}
	nonce, ok := tx.Input.Get("nonce").(pack.Bytes32)
	if !ok {
		return fmt.Errorf("unexpected type for nonce: expected pack.Bytes32, got %v", tx.Input.Get("nonce").Type())
	}

	script := `INSERT INTO burn_account_release_account (hash, status, created_time, selector, amount, toAddr, nonce) VALUES ($1, $2, $3, $4, $5, $6, $7);`
	_, err := db.db.Exec(script,
		tx.Hash.String(),
		TxStatusConfirming,
		time.Now().Unix(),
		tx.Selector.String(),
		amount.String(),
		to.String(),
		nonce.String(),
	)

	return err
}

func (db database) lockUTXOMintAccountTx(txHash pack.Bytes32) (tx.Tx, error) {
	script := "SELECT hash, selector, utxo_hash, utxo_index, value, pubkey_script, payload, phash, toAddr, nonce, nhash, gpubkey, ghash FROM lock_utxo_mint_account WHERE hash = $1"
	row := db.db.QueryRow(script, txHash.String())
	return rowToLockUTXOMintAccountTx(row)
}

func (db database) lockAccountMintAccountTx(txHash pack.Bytes32) (tx.Tx, error) {
	script := "SELECT hash, selector, tx_id, amount, payload, phash, toAddr, nonce, nhash, gpubkey FROM lock_account_mint_account WHERE hash = $1"
	row := db.db.QueryRow(script, txHash.String())
	return rowToLockAccountMintAccountTx(row)
}

func (db database) burnAccountReleaseUTXOTx(txHash pack.Bytes32) (tx.Tx, error) {
	script := "SELECT hash, selector, amount, toAddr, nonce FROM burn_account_release_utxo WHERE hash = $1"
	row := db.db.QueryRow(script, txHash.String())
	return rowToBurnAccountReleaseUTXOTx(row)
}

func (db database) burnAccountReleaseAccountTx(txHash pack.Bytes32) (tx.Tx, error) {
	script := "SELECT hash, selector, amount, toAddr, nonce FROM burn_account_release_account WHERE hash = $1"
	row := db.db.QueryRow(script, txHash.String())
	return rowToBurnAccountReleaseAccountTx(row)
}

func rowToTx(row Scannable) (tx.Tx, error) {
	var tableName, hash, selector, txID, amount, utxoHash, value, pubKeyScript, payload, phash, to, nonce, nhash, gpubkey, ghash string
	var utxoIndex int
	if err := row.Scan(&tableName, &hash, &selector, &txID, &amount, &utxoHash, &utxoIndex, &value, &pubKeyScript, &payload, &phash, &to, &nonce, &nhash, &gpubkey, &ghash); err != nil {
		return tx.Tx{}, err
	}

	switch tableName {
	case "lock_utxo_mint_account":
		return lockUTXOMintAccountTx(hash, selector, utxoHash, utxoIndex, value, pubKeyScript, payload, phash, to, nonce, nhash, gpubkey, ghash)
	case "lock_account_mint_account":
		return lockAccountMintAccountTx(hash, selector, txID, amount, payload, phash, to, nonce, nhash, gpubkey)
	case "burn_account_release_utxo":
		return burnAccountReleaseUTXOTx(hash, selector, amount, to, nonce)
	case "burn_account_release_account":
		return burnAccountReleaseAccountTx(hash, selector, amount, to, nonce)
	}

	return tx.Tx{}, fmt.Errorf("invalid table name %v", tableName)
}

func rowToLockUTXOMintAccountTx(row Scannable) (tx.Tx, error) {
	var hash, selector, utxoHash, value, pubKeyScript, payload, phash, to, nonce, nhash, gpubkey, ghash string
	var utxoIndex int
	if err := row.Scan(&hash, &selector, &utxoHash, &utxoIndex, &value, &pubKeyScript, &payload, &phash, &to, &nonce, &nhash, &gpubkey, &ghash); err != nil {
		return tx.Tx{}, err
	}

	return lockUTXOMintAccountTx(hash, selector, utxoHash, utxoIndex, value, pubKeyScript, payload, phash, to, nonce, nhash, gpubkey, ghash)
}

func lockUTXOMintAccountTx(hashStr, selector, utxoHashStr string, utxoIndex int, valueStr, pubKeyScriptStr, payloadStr, phashStr, to, nonceStr, nhashStr, gpubkeyStr, ghashStr string) (tx.Tx, error) {
	utxoHash, err := decodeBytes(utxoHashStr)
	if err != nil {
		return tx.Tx{}, fmt.Errorf("decoding utxo hash %v: %v", utxoHashStr, err)
	}
	value, err := decodeU256(valueStr)
	if err != nil {
		return tx.Tx{}, fmt.Errorf("decoding value %v: %v", valueStr, err)
	}
	pubKeyScript, err := decodeBytes(pubKeyScriptStr)
	if err != nil {
		return tx.Tx{}, fmt.Errorf("decoding pubkey script %v: %v", pubKeyScriptStr, err)
	}
	payload, err := decodeBytes(payloadStr)
	if err != nil {
		return tx.Tx{}, fmt.Errorf("decoding payload %v: %v", payloadStr, err)
	}
	phash, err := decodeBytes32(phashStr)
	if err != nil {
		return tx.Tx{}, fmt.Errorf("decoding phash %v: %v", phashStr, err)
	}
	nonce, err := decodeBytes32(nonceStr)
	if err != nil {
		return tx.Tx{}, fmt.Errorf("decoding nonce %v: %v", nonceStr, err)
	}
	nhash, err := decodeBytes32(nhashStr)
	if err != nil {
		return tx.Tx{}, fmt.Errorf("decoding nhash %v: %v", nhashStr, err)
	}
	gpubkey, err := decodeBytes(gpubkeyStr)
	if err != nil {
		return tx.Tx{}, fmt.Errorf("decoding gpubkey %v: %v", gpubkeyStr, err)
	}
	ghash, err := decodeBytes32(ghashStr)
	if err != nil {
		return tx.Tx{}, fmt.Errorf("decoding ghash %v: %v", ghashStr, err)
	}
	input, err := pack.Encode(
		txengine.InputLockOnUTXOAndMintOnAccount{
			Output: utxo.Output{
				Outpoint: utxo.Outpoint{
					Hash:  utxoHash,
					Index: pack.NewU32(uint32(utxoIndex)),
				},
				Value:        value,
				PubKeyScript: pubKeyScript,
			},
			Payload: payload,
			Phash:   phash,
			To:      pack.String(to),
			Nonce:   nonce,
			Nhash:   nhash,
			Gpubkey: gpubkey,
			Ghash:   ghash,
		},
	)
	if err != nil {
		return tx.Tx{}, err
	}
	return tx.NewTx(tx.Selector(selector), pack.Typed(input.(pack.Struct)))
}

func rowToLockAccountMintAccountTx(row Scannable) (tx.Tx, error) {
	var hash, selector, txID, amount, payload, phash, to, nonce, nhash, gpubkey string
	if err := row.Scan(&hash, &selector, &txID, &amount, &payload, &phash, &to, &nonce, &nhash, &gpubkey); err != nil {
		return tx.Tx{}, err
	}

	return lockAccountMintAccountTx(hash, selector, txID, amount, payload, phash, to, nonce, nhash, gpubkey)
}

func lockAccountMintAccountTx(hashStr, selector, txIDStr, amountStr, payloadStr, phashStr, to, nonceStr, nhashStr, gpubkeyStr string) (tx.Tx, error) {
	txID, err := decodeBytes(txIDStr)
	if err != nil {
		return tx.Tx{}, fmt.Errorf("decoding tx ID %v: %v", txIDStr, err)
	}
	amount, err := decodeU256(amountStr)
	if err != nil {
		return tx.Tx{}, fmt.Errorf("decoding amount %v: %v", amount, err)
	}
	payload, err := decodeBytes(payloadStr)
	if err != nil {
		return tx.Tx{}, fmt.Errorf("decoding payload %v: %v", payloadStr, err)
	}
	phash, err := decodeBytes32(phashStr)
	if err != nil {
		return tx.Tx{}, fmt.Errorf("decoding phash %v: %v", phashStr, err)
	}
	nonce, err := decodeU256(nonceStr)
	if err != nil {
		return tx.Tx{}, fmt.Errorf("decoding nonce %v: %v", nonceStr, err)
	}
	nhash, err := decodeBytes32(nhashStr)
	if err != nil {
		return tx.Tx{}, fmt.Errorf("decoding nhash %v: %v", nhashStr, err)
	}
	gpubkey, err := decodeBytes(gpubkeyStr)
	if err != nil {
		return tx.Tx{}, fmt.Errorf("decoding gpubkey %v: %v", gpubkeyStr, err)
	}
	input, err := pack.Encode(
		txengine.InputLockOnAccountAndMintOnAccount{
			Txid:    txID,
			Amount:  amount,
			Payload: payload,
			Phash:   phash,
			To:      pack.String(to),
			Nonce:   nonce,
			Nhash:   nhash,
			Gpubkey: gpubkey,
		},
	)
	if err != nil {
		return tx.Tx{}, err
	}
	return tx.NewTx(tx.Selector(selector), pack.Typed(input.(pack.Struct)))
}

func rowToBurnAccountReleaseUTXOTx(row Scannable) (tx.Tx, error) {
	var hash, selector, amount, to, nonce string
	if err := row.Scan(&hash, &selector, &amount, &to, &nonce); err != nil {
		return tx.Tx{}, err
	}
	return burnAccountReleaseUTXOTx(hash, selector, amount, to, nonce)
}

func burnAccountReleaseUTXOTx(hash, selector, amountStr, to, nonceStr string) (tx.Tx, error) {
	amount, err := decodeU256(amountStr)
	if err != nil {
		return tx.Tx{}, fmt.Errorf("decoding amount %v: %v", amountStr, err)
	}
	nonce, err := decodeBytes32(nonceStr)
	if err != nil {
		return tx.Tx{}, fmt.Errorf("decoding nonce %v: %v", nonceStr, err)
	}
	input, err := pack.Encode(
		txengine.InputBurnOnAccountAndReleaseOnUTXO{
			Amount: amount,
			To:     pack.String(to),
			Nonce:  nonce,
		},
	)
	if err != nil {
		return tx.Tx{}, err
	}
	return tx.NewTx(tx.Selector(selector), pack.Typed(input.(pack.Struct)))
}

func rowToBurnAccountReleaseAccountTx(row Scannable) (tx.Tx, error) {
	var hash, selector, amount, to, nonce string
	if err := row.Scan(&hash, &selector, &amount, &to, &nonce); err != nil {
		return tx.Tx{}, err
	}
	return burnAccountReleaseAccountTx(hash, selector, amount, to, nonce)
}

func burnAccountReleaseAccountTx(hash, selector, amountStr, to, nonceStr string) (tx.Tx, error) {
	amount, err := decodeU256(amountStr)
	if err != nil {
		return tx.Tx{}, fmt.Errorf("decoding amount %v: %v", amountStr, err)
	}
	nonce, err := decodeBytes32(nonceStr)
	if err != nil {
		return tx.Tx{}, fmt.Errorf("decoding nonce %v: %v", nonceStr, err)
	}
	input, err := pack.Encode(
		txengine.InputBurnOnAccountAndReleaseOnAccount{
			Amount: amount,
			To:     pack.String(to),
			Nonce:  nonce,
		},
	)
	if err != nil {
		return tx.Tx{}, err
	}
	return tx.NewTx(tx.Selector(selector), pack.Typed(input.(pack.Struct)))
}

func decodeStruct(name, value string) (pack.Struct, error) {
	val, err := decodeBytes32(value)
	if err != nil {
		return pack.Struct{}, err
	}
	return pack.NewStruct(name, val), nil
}

func decodeBytes32(str string) (pack.Bytes32, error) {
	var res pack.Bytes32
	b, err := decodeBytes(str)
	if err != nil {
		return pack.Bytes32{}, err
	}
	copy(res[:], b)
	return res, nil
}

func decodeBytes(str string) (pack.Bytes, error) {
	res, err := base64.RawURLEncoding.DecodeString(str)
	if err != nil {
		return pack.Bytes{}, err
	}
	return res, nil
}

func decodeU256(str string) (pack.U256, error) {
	amount, ok := new(big.Int).SetString(str, 10)
	if !ok {
		return pack.U256{}, fmt.Errorf("invalid string")
	}
	return pack.NewU256FromInt(amount), nil
}
