package db

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"math/big"
	"time"

	"github.com/renproject/darknode/engine"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/id"
	"github.com/renproject/pack"
)

type TxStatus uint8

const (
	TxStatusNil TxStatus = iota
	TxStatusConfirming
	TxStatusConfirmed
	TxStatusUnconfirmed
)

type GatewayStatus uint8

const (
	GatewayStatusNil GatewayStatus = iota
	GatewayStatusEmpty
	GatewayStatusUsed
)

type Scannable interface {
	Scan(dest ...interface{}) error
}

// DB is a storage adapter (built on top of a SQL database) that stores all
// transaction details.
type DB interface {
	// Init the database. This should be called once the database object is
	// created.
	Init() error

	// InsertTx inserts the transaction into the database.
	InsertTx(tx tx.Tx) error

	// Tx gets the details of the transaction with the given hash. It returns an
	// `sql.ErrNoRows` if the transaction cannot be found.
	Tx(hash id.Hash) (tx.Tx, error)

	// Txs returns transactions with the given pagination options.
	Txs(offset, limit int, latest bool) ([]tx.Tx, error)

	// TxsByTxid returns transactions with the given blockchain transaction id.
	TxsByTxid(id pack.Bytes) ([]tx.Tx, error)

	// TxsByStatus returns txs with given status in the db
	TxsByStatus(status TxStatus, within, beyond time.Duration) ([]tx.Tx, error)

	// TxStatus returns the current status of the transaction with the given
	// hash.
	TxStatus(hash id.Hash) (TxStatus, error)

	// UpdateStatus updates the status of the given transaction. The status
	// cannot be updated to a previous status.
	UpdateStatus(hash id.Hash, status TxStatus) error

	// Prune deletes transactions which have expired.
	Prune(expiry time.Duration) error

	// InsertGateway inserts the gateway into the database.
	InsertGateway(address string, tx tx.Tx) error

	// Gateway gets the details of the gateway with the given gateway address. It returns an
	// `sql.ErrNoRows` if the gateway cannot be found.
	Gateway(address string) (tx.Tx, error)
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
// exist. The tables will only be created the first time this function is called
// and any future calls will not return an error.
func (db database) Init() error {
	script := `CREATE TABLE IF NOT EXISTS txs (
		hash               VARCHAR NOT NULL PRIMARY KEY,
		status             SMALLINT,
		created_time       BIGINT,
		selector           VARCHAR(255),
		txid               VARCHAR,
		txindex            BIGINT,
		amount             VARCHAR(100),
		payload            VARCHAR,
		phash              VARCHAR,
		to_address         VARCHAR,
		nonce              VARCHAR,
		nhash              VARCHAR,
		gpubkey            VARCHAR,
		ghash              VARCHAR,
		version            VARCHAR
	);
CREATE TABLE IF NOT EXISTS gateways (
		gateway_address    VARCHAR NOT NULL PRIMARY KEY,
		status             SMALLINT,
		created_time       BIGINT,
		selector           VARCHAR(255),
		payload            VARCHAR,
		phash              VARCHAR,
		to_address         VARCHAR,
		nonce              VARCHAR,
		nhash              VARCHAR,
		gpubkey            VARCHAR,
		ghash              VARCHAR,
		version            VARCHAR
);
`
	_, err := db.db.Exec(script)
	return err
}

// InsertTx implements the DB interface.
func (db database) InsertTx(tx tx.Tx) error {
	txid, ok := tx.Input.Get("txid").(pack.Bytes)
	if !ok {
		return fmt.Errorf("unexpected type for txid: expected pack.Bytes, got %v", tx.Input.Get("txid").Type())
	}
	txindex, ok := tx.Input.Get("txindex").(pack.U32)
	if !ok {
		return fmt.Errorf("unexpected type for txindex: expected pack.U32, got %v", tx.Input.Get("txindex").Type())
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

	script := `INSERT INTO txs (hash, status, created_time, selector, txid, txindex, amount, payload, phash, to_address, nonce, nhash, gpubkey, ghash, version) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15);`
	_, err := db.db.Exec(script,
		tx.Hash.String(),
		TxStatusConfirming,
		time.Now().Unix(),
		tx.Selector.String(),
		txid.String(),
		txindex.String(),
		amount.String(),
		payload.String(),
		phash.String(),
		to.String(),
		nonce.String(),
		nhash.String(),
		gpubkey.String(),
		ghash.String(),
		tx.Version.String(),
	)

	return err
}

// Tx implements the DB interface.
func (db database) Tx(txHash id.Hash) (tx.Tx, error) {
	script := "SELECT hash, selector, txid, txindex, amount, payload, phash, to_address, nonce, nhash, gpubkey, ghash, version FROM txs WHERE hash = $1"
	row := db.db.QueryRow(script, txHash.String())
	err := row.Err()
	if err != nil {
		return tx.Tx{}, err
	}
	return rowToTx(row)
}

// Txs implements the DB interface.
func (db database) Txs(offset, limit int, latest bool) ([]tx.Tx, error) {
	txs := make([]tx.Tx, 0, limit)
	order := "ASC"
	if latest {
		order = "DESC"
	}
	// We cant make a prepared statement with variable order directives,
	// so we need to generate the query manually
	queryString := fmt.Sprintf(`SELECT hash, selector, txid, txindex, amount, payload, phash, to_address, nonce, nhash, gpubkey, ghash, version FROM txs ORDER BY created_time %s LIMIT $1 OFFSET $2;`, order)

	rows, err := db.db.Query(queryString, limit, offset)
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

// TxsById implements the DB interface.
func (db database) TxsByTxid(txid pack.Bytes) ([]tx.Tx, error) {
	txs := make([]tx.Tx, 0)
	rows, err := db.db.Query(`SELECT hash, selector, txid, txindex, amount, payload, phash, to_address, nonce, nhash, gpubkey, ghash, version FROM txs WHERE txid = $1;`, txid.String())
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

// TxsByStatus implements the DB interface.
func (db database) TxsByStatus(status TxStatus, before, after time.Duration) ([]tx.Tx, error) {
	txs := make([]tx.Tx, 0, 128)

	now := time.Now()
	script := fmt.Sprintf(`SELECT hash, selector, txid, txindex, amount, payload, phash, to_address, nonce, nhash, gpubkey, ghash, version FROM txs
		WHERE status = %v `, status)
	if before != 0 {
		script += fmt.Sprintf("AND %v - created_time < %v", now.Unix(), int64(before.Seconds()))
	}
	if after != 0 {
		script += fmt.Sprintf("AND %v - created_time > %v", now.Unix(), int64(after.Seconds()))
	}
	script += ";"

	// Get pending transactions from the database.
	rows, err := db.db.Query(script)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		transaction, err := rowToTx(rows)
		if err != nil {
			return nil, err
		}
		txs = append(txs, transaction)
	}
	return txs, rows.Err()
}

// TxStatus implements the DB interface.
func (db database) TxStatus(txHash id.Hash) (TxStatus, error) {
	var status int
	err := db.db.QueryRow(`SELECT status FROM txs WHERE hash = $1;`, txHash.String()).Scan(&status)
	if err != nil {
		return TxStatusNil, err
	}
	return TxStatus(status), err
}

// UpdateStatus implements the DB interface.
func (db database) UpdateStatus(txHash id.Hash, status TxStatus) error {
	r, err := db.db.Exec("UPDATE txs SET status = $1 WHERE hash = $2 AND status < $1;", status, txHash.String())
	updated, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if updated != 1 {
		return fmt.Errorf("failed to update tx %s status correctly - updated %v txs", txHash, updated)
	}
	return err
}

// Prune deletes txs which have expired based on the given expiry.
func (db database) Prune(expiry time.Duration) error {
	_, err := db.db.Exec("DELETE FROM txs WHERE $1 - created_time > $2;", time.Now().Unix(), int(expiry.Seconds()))
	if err != nil {
		return err
	}

	_, err = db.db.Exec("DELETE FROM gateways WHERE $1 - created_time > $2;", time.Now().Unix(), int(expiry.Seconds()))
	return err
}

// A gateway is a partial Tx that does not have deposits
// We store it in order to be able to re-create the parameters needed to finish a mint
func (db database) InsertGateway(address string, tx tx.Tx) error {
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

	script := `INSERT INTO gateways
(gateway_address, status, created_time, selector, payload, phash, to_address, nonce, nhash, gpubkey, ghash, version)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);`
	_, err := db.db.Exec(script,
		address,
		GatewayStatusEmpty,
		time.Now().Unix(),
		tx.Selector.String(),
		payload.String(),
		phash.String(),
		to.String(),
		nonce.String(),
		nhash.String(),
		gpubkey.String(),
		ghash.String(),
		tx.Version.String(),
	)

	return err
}

// Returns the gateway information for a given address
func (db database) Gateway(address string) (tx.Tx, error) {
	script := "SELECT gateway_address, selector, payload, phash, to_address, nonce, nhash, gpubkey, ghash, version FROM gateways WHERE gateway_address = $1"
	row := db.db.QueryRow(script, address)
	err := row.Err()
	if err != nil {
		return tx.Tx{}, err
	}
	return rowToGateway(row)
}

func rowToGateway(row Scannable) (tx.Tx, error) {
	var gatewayAddress, selector, payloadStr, phashStr, toStr, nonceStr, nhashStr, gpubkeyStr, ghashStr, version string
	if err := row.Scan(&gatewayAddress, &selector, &payloadStr, &phashStr, &toStr, &nonceStr, &nhashStr, &gpubkeyStr, &ghashStr, &version); err != nil {
		return tx.Tx{}, err
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
		engine.LockMintBurnReleaseInput{
			Payload: payload,
			Phash:   phash,
			To:      pack.String(toStr),
			Nonce:   nonce,
			Nhash:   nhash,
			Gpubkey: gpubkey,
			Ghash:   ghash,
		},
	)
	if err != nil {
		return tx.Tx{}, err
	}

	return tx.Tx{
		Selector: tx.Selector(selector),
		Input:    pack.Typed(input.(pack.Struct)),
	}, err
}

func rowToTx(row Scannable) (tx.Tx, error) {
	var hash, selector, txidStr, amountStr, payloadStr, phashStr, toStr, nonceStr, nhashStr, gpubkeyStr, ghashStr, version string
	var txindex int
	if err := row.Scan(&hash, &selector, &txidStr, &txindex, &amountStr, &payloadStr, &phashStr, &toStr, &nonceStr, &nhashStr, &gpubkeyStr, &ghashStr, &version); err != nil {
		return tx.Tx{}, err
	}

	txID, err := decodeBytes(txidStr)
	if err != nil {
		return tx.Tx{}, fmt.Errorf("decoding txid %v: %v", txidStr, err)
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
		engine.LockMintBurnReleaseInput{
			Txid:    txID,
			Txindex: pack.U32(txindex),
			Amount:  amount,
			Payload: payload,
			Phash:   phash,
			To:      pack.String(toStr),
			Nonce:   nonce,
			Nhash:   nhash,
			Gpubkey: gpubkey,
			Ghash:   ghash,
		},
	)
	if err != nil {
		return tx.Tx{}, err
	}

	if version == tx.Version0.String() {
		// we have to construct the tx manually because tx.NewTx
		// only produces v1 txes
		transaction := tx.Tx{
			Version:  tx.Version(version),
			Selector: tx.Selector(selector),
			Input:    pack.Typed(input.(pack.Struct)),
		}
		hash32, err := decodeBytes32(hash)
		transaction.Hash = [32]byte(hash32)

		return transaction, err
	} else {
		return tx.NewTx(tx.Selector(selector), pack.Typed(input.(pack.Struct)))
	}
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
