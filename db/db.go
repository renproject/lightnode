package db

import (
	"database/sql"
	"encoding/base64"
	"encoding/hex"
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
// TODO: Decide approach for versioning database tables.
func (db database) Init() error {
	// Create the lock-and-mint table if it does not exist.
	lockAndMint := `CREATE TABLE IF NOT EXISTS lock_and_mint (
		hash         CHAR(43) NOT NULL PRIMARY KEY,
		status       INT,
		created_time INT,
		selector     VARCHAR(255),
		utxo_hash    VARCHAR,
		utxo_index   INT,
		payload      VARCHAR,
		phash        CHAR(43),
		token        VARCHAR,
		toAddr       VARCHAR,
		nonce        CHAR(43),
		nhash        CHAR(43),
		gpubkey      VARCHAR,
		ghash        CHAR(43)
	);`
	_, err := db.db.Exec(lockAndMint)
	if err != nil {
		return err
	}

	// Create the burn-and-release table if not exist.
	burnAndRelease := `CREATE TABLE IF NOT EXISTS burn_and_release (
		hash         CHAR(43) NOT NULL PRIMARY KEY,
		status       INT,
		created_time INT,
		selector     VARCHAR(255),
		amount       VARCHAR(100),
		toAddr       VARCHAR,
		nonce        CHAR(43)
	);`
	_, err = db.db.Exec(burnAndRelease)

	// Create the burn-and-mint table if not exist.
	burnAndMint := `CREATE TABLE IF NOT EXISTS burn_and_mint (
		hash         CHAR(43) NOT NULL PRIMARY KEY,
		status       INT,
		created_time INT,
		selector     VARCHAR(255), 
		amount       VARCHAR(100),
		payload      VARCHAR,
		phash        CHAR(43),
		token        VARCHAR,
		toAddr       VARCHAR,
		nonce        CHAR(43),
		nhash        CHAR(43),
		ghash        CHAR(43)
	);`
	_, err = db.db.Exec(burnAndMint)
	return err
}

// InsertTx implements the DB interface.
func (db database) InsertTx(tx tx.Tx) error {
	switch {
	case tx.Selector.IsLockAndMint():
		return db.insertLockAndMintTx(tx)
	case tx.Selector.IsBurnAndRelease():
		return db.insertBurnAndReleaseTx(tx)
	case tx.Selector.IsBurnAndMint():
		return db.insertBurnAndMintTx(tx)
	default:
		return fmt.Errorf("unexpected tx selector %v", tx.Selector.String())
	}
}

// Tx implements the DB interface.
func (db database) Tx(hash pack.Bytes32) (tx.Tx, error) {
	tx, err := db.lockAndMintTx(hash)
	if err == sql.ErrNoRows {
		tx, err = db.burnAndReleaseTx(hash)
		if err == sql.ErrNoRows {
			return db.burnAndMintTx(hash)
		}
	}
	return tx, err
}

// Txs implements the DB interface.
func (db database) Txs(offset, limit int) ([]tx.Tx, error) {
	txs := make([]tx.Tx, 0, limit)
	rows, err := db.db.Query(`SELECT hash, selector, utxo_hash, utxo_index, payload, phash, token, toAddr, nonce, nhash, gpubkey, ghash, amount FROM (
		SELECT hash, created_time, selector, utxo_hash, utxo_index, payload, phash, token, toAddr, nonce, nhash, gpubkey, ghash, '' AS amount FROM lock_and_mint UNION
		SELECT hash, created_time, selector, utxo_hash AS '', utxo_index AS '', payload AS '', phash AS '', token AS '', toAddr, nonce, nhash AS '', gpubkey AS '', ghash AS '', amount FROM burn_and_release UNION
		SELECT hash, created_time, selector, utxo_hash AS '', utxo_index AS '', payload, phash, token, toAddr, nonce, nhash, gpubkey AS '', ghash, amount FROM burn_and_mint
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

	// Get pending lock-and-mint transactions from the database.
	rows, err := db.db.Query(`SELECT hash, selector, utxo_hash, utxo_index, payload, phash, token, toAddr, nonce, nhash, gpubkey, ghash FROM lock_and_mint
		WHERE status = $1 AND $2 - created_time < $3;`, TxStatusConfirming, time.Now().Unix(), int64(expiry.Seconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Loop through rows and convert them to txs.
	for rows.Next() {
		tx, err := rowToLockAndMintTx(rows)
		if err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}
	if rows.Err() != nil {
		return nil, err
	}

	// Get pending burn-and-release transactions from the database.
	rows, err = db.db.Query(`SELECT hash, selector, amount, toAddr, nonce FROM burn_and_release
		WHERE status = $1 AND $2 - created_time < $3`, TxStatusConfirming, time.Now().Unix(), int64(expiry.Seconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		tx, err := rowToBurnAndReleaseTx(rows)
		if err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}

	// Get pending burn-and-mint transactions from the database.
	rows, err = db.db.Query(`SELECT hash, selector, amount, payload, phash, token, toAddr, nonce, nhash, ghash FROM burn_and_mint
		WHERE status = $1 AND $2 - created_time < $3`, TxStatusConfirming, time.Now().Unix(), int64(expiry.Seconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		tx, err := rowToBurnAndMintTx(rows)
		if err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}
	return txs, rows.Err()
}

// TxStatus implements the DB interface.
func (db database) TxStatus(hash pack.Bytes32) (TxStatus, error) {
	var status int
	err := db.db.QueryRow(`SELECT status FROM lock_and_mint WHERE hash = $1;`, hex.EncodeToString(hash[:])).Scan(&status)
	if err == sql.ErrNoRows {
		err = db.db.QueryRow(`SELECT status FROM burn_and_release WHERE hash = $1;`, hex.EncodeToString(hash[:])).Scan(&status)
		if err == sql.ErrNoRows {
			err = db.db.QueryRow(`SELECT status FROM burn_and_mint WHERE hash = $1;`, hex.EncodeToString(hash[:])).Scan(&status)
		}
	}
	return TxStatus(status), err
}

// UpdateStatus implements the DB interface.
func (db database) UpdateStatus(hash pack.Bytes32, status TxStatus) error {
	_, err := db.db.Exec("UPDATE lock_and_mint SET status = $1 WHERE hash = $2 AND status < $1;", status, hex.EncodeToString(hash[:]))
	if err != nil {
		return err
	}
	_, err = db.db.Exec("UPDATE burn_and_release SET status = $1 WHERE hash = $2 AND status < $1;", status, hex.EncodeToString(hash[:]))
	if err != nil {
		return err
	}
	_, err = db.db.Exec("UPDATE burn_and_mint SET status = $1 WHERE hash = $2 AND status < $1;", status, hex.EncodeToString(hash[:]))
	return err
}

// Prune deletes txs which have expired based on the given expiry.
func (db database) Prune(expiry time.Duration) error {
	_, err := db.db.Exec("DELETE FROM lock_and_mint WHERE $1 - created_time > $2;", time.Now().Unix(), int(expiry.Seconds()))
	if err != nil {
		return err
	}
	_, err = db.db.Exec("DELETE FROM burn_and_release WHERE $1 - created_time > $2;", time.Now().Unix(), int(expiry.Seconds()))
	if err != nil {
		return err
	}
	_, err = db.db.Exec("DELETE FROM burn_and_mint WHERE $1 - created_time > $2;", time.Now().Unix(), int(expiry.Seconds()))
	return err
}

func (db database) insertLockAndMintTx(tx tx.Tx) error {
	output, ok := tx.Input.Get("output").(pack.Struct)
	if !ok {
		return fmt.Errorf("unexpected type for output: expected pack.Struct, got %v", tx.Input.Get("output").Type())
	}
	outpoint, ok := tx.Input.Get("outpoint").(pack.Struct)
	if !ok {
		return fmt.Errorf("unexpected type for outpoint: expected pack.Struct, got %v", output.Get("outpoint").Type())
	}
	hash, ok := outpoint.Get("hash").(pack.Bytes32)
	if !ok {
		return fmt.Errorf("unexpected type for hash: expected pack.Bytes32, got %v", outpoint.Get("hash").Type())
	}
	index, ok := outpoint.Get("index").(pack.U32)
	if !ok {
		return fmt.Errorf("unexpected type for index: expected pack.U32, got %v", outpoint.Get("index").Type())
	}
	payload, ok := tx.Input.Get("payload").(pack.Bytes)
	if !ok {
		return fmt.Errorf("unexpected type for payload: expected pack.Bytes, got %v", tx.Input.Get("payload").Type())
	}
	phash, ok := tx.Input.Get("phash").(pack.Bytes32)
	if !ok {
		return fmt.Errorf("unexpected type for phash: expected pack.Bytes32, got %v", tx.Input.Get("phash").Type())
	}
	token, ok := tx.Input.Get("token").(pack.String)
	if !ok {
		return fmt.Errorf("unexpected type for token: expected pack.String, got %v", tx.Input.Get("token").Type())
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

	script := `INSERT INTO lock_and_mint (hash, status, created_time, selector, utxo_hash, utxo_index, payload, phash, token, toAddr, nonce, nhash, gpubkey, ghash) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14);`
	_, err := db.db.Exec(script,
		tx.Hash.String(),
		TxStatusConfirming,
		time.Now().Unix(),
		tx.Selector.String(),
		hash.String(),
		index,
		payload.String(),
		phash.String(),
		token.String(),
		to.String(),
		nonce.String(),
		nhash.String(),
		gpubkey.String(),
		ghash.String(),
	)

	return err
}

func (db database) insertBurnAndReleaseTx(tx tx.Tx) error {
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

	script := `INSERT INTO burn_and_release (hash, status, created_time, selector, amount, toAddr, nonce) VALUES ($1, $2, $3, $4, $5, $6, $7);`
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

func (db database) insertBurnAndMintTx(tx tx.Tx) error {
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
	token, ok := tx.Input.Get("token").(pack.String)
	if !ok {
		return fmt.Errorf("unexpected type for token: expected pack.String, got %v", tx.Input.Get("token").Type())
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
	ghash, ok := tx.Input.Get("ghash").(pack.Bytes32)
	if !ok {
		return fmt.Errorf("unexpected type for ghash: expected pack.Bytes32, got %v", tx.Input.Get("ghash").Type())
	}

	script := `INSERT INTO burn_and_mint (hash, status, created_time, selector, amount, payload, phash, token, toAddr, nonce, nhash, ghash) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);`
	_, err := db.db.Exec(script,
		tx.Hash.String(),
		TxStatusConfirming,
		time.Now().Unix(),
		tx.Selector.String(),
		amount.String(),
		payload.String(),
		phash.String(),
		token.String(),
		to.String(),
		nonce.String(),
		nhash.String(),
		ghash.String(),
	)

	return err
}

func (db database) lockAndMintTx(txHash pack.Bytes32) (tx.Tx, error) {
	script := "SELECT hash, selector, utxo_hash, utxo_index, payload, phash, token, toAddr, nonce, nhash, gpubkey, ghash FROM lock_and_mint WHERE hash = $1"
	row := db.db.QueryRow(script, hex.EncodeToString(txHash[:]))
	return rowToLockAndMintTx(row)
}

func (db database) burnAndReleaseTx(txHash pack.Bytes32) (tx.Tx, error) {
	script := "SELECT hash, selector, amount, toAddr, nonce FROM burn_and_release WHERE hash = $1"
	row := db.db.QueryRow(script, hex.EncodeToString(txHash[:]))
	return rowToBurnAndReleaseTx(row)
}

func (db database) burnAndMintTx(txHash pack.Bytes32) (tx.Tx, error) {
	script := "SELECT hash, selector, amount, payload, phash, token, toAddr, nonce, nhash, ghash FROM burn_and_mint WHERE hash = $1"
	row := db.db.QueryRow(script, hex.EncodeToString(txHash[:]))
	return rowToBurnAndMintTx(row)
}

func rowToTx(row Scannable) (tx.Tx, error) {
	var hash, selector, utxoHash, payload, phash, token, to, nonce, nhash, gpubkey, ghash, amount string
	var utxoIndex int
	if err := row.Scan(&hash, &selector, &utxoHash, &utxoIndex, &payload, &phash, &token, &to, &nonce, &nhash, &gpubkey, &ghash, &amount); err != nil {
		return tx.Tx{}, err
	}

	if amount == "" {
		return lockAndMintTx(hash, selector, utxoHash, payload, phash, token, to, nonce, nhash, gpubkey, ghash, utxoIndex)
	}
	if payload == "" {
		return burnAndReleaseTx(hash, selector, amount, to, nonce)
	}
	return burnAndMintTx(hash, selector, amount, payload, phash, token, to, nonce, nhash, ghash)
}

func rowToLockAndMintTx(row Scannable) (tx.Tx, error) {
	var hash, selector, utxoHash, payload, phash, token, to, nonce, nhash, gpubkey, ghash string
	var utxoIndex int
	if err := row.Scan(&hash, &selector, &utxoHash, &utxoIndex, &payload, &phash, &token, &to, &nonce, &nhash, &gpubkey, &ghash); err != nil {
		return tx.Tx{}, err
	}

	return lockAndMintTx(hash, selector, utxoHash, payload, phash, token, to, nonce, nhash, gpubkey, ghash, utxoIndex)
}

func lockAndMintTx(hashStr, selector, utxoHashStr, payloadStr, phashStr, token, to, nonceStr, nhashStr, gpubkeyStr, ghashStr string, utxoIndex int) (tx.Tx, error) {
	utxoHash, err := base64.RawStdEncoding.DecodeString(utxoHashStr)
	if err != nil {
		return tx.Tx{}, err
	}
	payload, err := base64.RawStdEncoding.DecodeString(payloadStr)
	if err != nil {
		return tx.Tx{}, err
	}
	phash, err := decodeBytes32(phashStr)
	if err != nil {
		return tx.Tx{}, err
	}
	nonce, err := decodeBytes32(nonceStr)
	if err != nil {
		return tx.Tx{}, err
	}
	nhash, err := decodeBytes32(nhashStr)
	if err != nil {
		return tx.Tx{}, err
	}
	gpubkey, err := base64.RawStdEncoding.DecodeString(gpubkeyStr)
	if err != nil {
		return tx.Tx{}, err
	}
	ghash, err := decodeBytes32(ghashStr)
	if err != nil {
		return tx.Tx{}, err
	}
	input, err := pack.Encode(
		txengine.UTXOLockAndMintInput{
			Output: utxo.Output{
				Outpoint: utxo.Outpoint{
					Hash:  utxoHash,
					Index: pack.NewU32(uint32(utxoIndex)),
				},
			},
			Payload: payload,
			Phash:   phash,
			Token:   pack.String(token),
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

func rowToBurnAndReleaseTx(row Scannable) (tx.Tx, error) {
	var hash, selector, amount, to, nonce string
	if err := row.Scan(&hash, &selector, &amount, &to, &nonce); err != nil {
		return tx.Tx{}, err
	}
	return burnAndReleaseTx(hash, selector, amount, to, nonce)
}

func burnAndReleaseTx(hash, selector, amountStr, to, nonceStr string) (tx.Tx, error) {
	amount, ok := new(big.Int).SetString(amountStr, 10)
	if !ok {
		return tx.Tx{}, fmt.Errorf("invalid amount %v", amount)
	}
	nonce, err := decodeBytes32(nonceStr)
	if err != nil {
		return tx.Tx{}, err
	}
	input, err := pack.Encode(
		txengine.UTXOBurnAndReleaseInput{
			Amount: pack.NewU256FromInt(amount),
			To:     pack.String(to),
			Nonce:  nonce,
		},
	)
	if err != nil {
		return tx.Tx{}, err
	}
	return tx.NewTx(tx.Selector(selector), pack.Typed(input.(pack.Struct)))
}

func rowToBurnAndMintTx(row Scannable) (tx.Tx, error) {
	var hash, selector, amount, payload, phash, token, to, nonce, nhash, ghash string
	if err := row.Scan(&hash, &selector, &amount, &payload, &phash, &token, &to, &nonce, &nhash, &ghash); err != nil {
		return tx.Tx{}, err
	}
	return burnAndMintTx(hash, selector, amount, payload, phash, token, to, nonce, nhash, ghash)
}

func burnAndMintTx(hashStr, selector, amountStr, payloadStr, phashStr, token, to, nonceStr, nhashStr, ghashStr string) (tx.Tx, error) {
	amount, ok := new(big.Int).SetString(amountStr, 10)
	if !ok {
		return tx.Tx{}, fmt.Errorf("invalid amount %v", amount)
	}
	payload, err := base64.RawStdEncoding.DecodeString(payloadStr)
	if err != nil {
		return tx.Tx{}, err
	}
	phash, err := decodeBytes32(phashStr)
	if err != nil {
		return tx.Tx{}, err
	}
	nonce, err := decodeBytes32(nonceStr)
	if err != nil {
		return tx.Tx{}, err
	}
	nhash, err := decodeBytes32(nhashStr)
	if err != nil {
		return tx.Tx{}, err
	}
	ghash, err := decodeBytes32(ghashStr)
	if err != nil {
		return tx.Tx{}, err
	}
	input, err := pack.Encode(
		txengine.AccountBurnAndMintInput{
			Amount:  pack.NewU256FromInt(amount),
			Payload: payload,
			Phash:   phash,
			Token:   pack.String(token),
			To:      pack.String(to),
			Nonce:   nonce,
			Nhash:   nhash,
			Ghash:   ghash,
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
	b, err := base64.RawStdEncoding.DecodeString(str)
	if err != nil {
		return pack.Bytes32{}, err
	}
	copy(res[:], b)
	return res, nil
}
