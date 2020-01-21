package db

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/renproject/darknode/abi"
)

// DB abstract all database interactions.
type DB struct {
	db *sql.DB
}

// New creates a new DB instance.
func New(db *sql.DB) DB {
	return DB{
		db: db,
	}
}

// CreateTxTable creates the tx table if not exists. Multiple calls of this
// function will only create the table once and will not cause any error
func (db DB) CreateTxTable() error {
	script := `CREATE TABLE IF NOT EXISTS tx (
    hash                 CHAR(64) NOT NULL PRIMARY KEY,
    status               INT,
    created_time         INT, 
    contract             VARCHAR(255),
    phash                CHAR(64),
    token                CHAR(40),
    toAddr               CHAR(40),
    n                    CHAR(64),
    amount               INT,
	ghash                CHAR(64),
	nhash                CHAR(64),
	sighash              CHAR(64),
	utxo_tx_hash         CHAR(64),
    utxo_vout            INT 
);`
	_, err := db.db.Exec(script)
	return err
}

// InsertTx inserts the RenVM tx into the db.
func (db DB) InsertTx(tx abi.Tx) error {
	phash, ok := tx.In.Get("phash").Value.(abi.B32)
	if !ok {
		return fmt.Errorf("unexpected type for phash, expected abi.B32, got %v", tx.In.Get("phash").Value.Type())
	}
	amount, ok := tx.In.Get("amount").Value.(abi.U256)
	if !ok {
		return fmt.Errorf("unexpected type for amount, expected abi.U64, got %v", tx.In.Get("amount").Value.Type())
	}
	token, ok := tx.In.Get("token").Value.(abi.ExtEthCompatAddress)
	if !ok {
		return fmt.Errorf("unexpected type for token, expected abi.B20, got %v", tx.In.Get("token").Value.Type())
	}
	to, ok := tx.In.Get("to").Value.(abi.ExtEthCompatAddress)
	if !ok {
		return fmt.Errorf("unexpected type for to, expected abi.B20, got %v", tx.In.Get("to").Value.Type())
	}
	n, ok := tx.In.Get("n").Value.(abi.B32)
	if !ok {
		return fmt.Errorf("unexpected type for n, expected abi.B32, got %v", tx.In.Get("n").Value.Type())
	}
	utxo, ok := tx.In.Get("utxo").Value.(abi.ExtBtcCompatUTXO)
	if !ok {
		return fmt.Errorf("unexpected type for utxo, expected abi.ExtTypeBtcCompatUTXO, got %v", tx.In.Get("utxo").Value.Type())
	}
	ghash, ok := tx.Autogen.Get("ghash").Value.(abi.B32)
	if !ok {
		return fmt.Errorf("unexpected type for ghash, expected abi.B32, got %v", tx.In.Get("ghash").Value.Type())
	}
	nhash, ok := tx.Autogen.Get("nhash").Value.(abi.B32)
	if !ok {
		return fmt.Errorf("unexpected type for nhash, expected abi.B32, got %v", tx.In.Get("nhash").Value.Type())
	}
	sighash, ok := tx.Autogen.Get("sighash").Value.(abi.B32)
	if !ok {
		return fmt.Errorf("unexpected type for sighash, expected abi.B32, got %v", tx.In.Get("sighash").Value.Type())
	}

	script := `INSERT INTO Tx (hash, status, created_time, contract, phash, token, toAddr, n, amount, ghash, nhash, sighash, utxo_tx_hash, utxo_vout)
VALUES ($1, 1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) ON CONFLICT DO NOTHING;`
	_, err := db.db.Exec(script,
		hex.EncodeToString(tx.Hash[:]),
		time.Now().Unix(),
		tx.To,
		hex.EncodeToString(phash[:]),
		hex.EncodeToString(token[:]),
		hex.EncodeToString(to[:]),
		hex.EncodeToString(n[:]),
		amount.Int.Int64(),
		hex.EncodeToString(ghash[:]),
		hex.EncodeToString(nhash[:]),
		hex.EncodeToString(sighash[:]),
		hex.EncodeToString(utxo.TxHash[:]),
		utxo.VOut.Int.Int64(),
	)
	return err
}

// Tx queries the db and returns the tx with given hash.
func (db DB) Tx(txHash abi.B32) (abi.Tx, error) {
	var contract, phash, token, to, n, ghash, nhash, sighash, utxoHash string
	var amount, utxoVout int
	err := db.db.QueryRow("SELECT contract, phash, token, toAddr, n, amount, ghash, nhash, sighash, utxo_tx_hash, utxo_vout FROM Tx WHERE hash=$1", hex.EncodeToString(txHash[:])).Scan(
		&contract, &phash, &token, &to, &n, &amount, &ghash, &nhash, &sighash, &utxoHash, &utxoVout)
	if err != nil {
		return abi.Tx{}, err
	}
	return constructTx(txHash, contract, phash, token, to, n, ghash, nhash, sighash, utxoHash, amount, utxoVout)
}

// PendingTxs returns all txs from the db which are still pending and not expired.
func (db DB) PendingTxs() (abi.Txs, error) {
	rows, err := db.db.Query(`SELECT hash, contract, phash, token, toAddr, n, amount, ghash, nhash, sighash, utxo_tx_hash, utxo_vout FROM Tx 
		WHERE status = 1 AND $1 - created_time < 86400`, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	txs := make(abi.Txs, 0, 128)
	for rows.Next() {
		var hash, contract, phash, token, to, n, ghash, nhash, sighash, utxoHash string
		var amount, utxoVout int
		err = rows.Scan(&hash, &contract, &phash, &token, &to, &n, &amount, &ghash, &nhash, &sighash, &utxoHash, &utxoVout)
		if err != nil {
			return nil, err
		}

		txHash, err := stringToB32(hash)
		if err != nil {
			return nil, err
		}
		tx, err := constructTx(txHash, contract, phash, token, to, n, ghash, nhash, sighash, utxoHash, amount, utxoVout)
		if err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}
	return txs, rows.Err()
}

// Expired returns if the tx with given hash has expired.
func (db DB) Expired(hash abi.B32) (bool, error) {
	var count int
	err := db.db.QueryRow(`SELECT count(*) FROM tx 
			WHERE hash=$1 AND $2 - created_time < 86400;`,
		hex.EncodeToString(hash[:]), time.Now().Unix()).Scan(&count)
	return count != 1, err
}

// ConfirmTx updates the tx status to 2 (means confirmed).
func (db DB) ConfirmTx(hash abi.B32) error {
	_, err := db.db.Exec("UPDATE tx SET status = 2 WHERE hash=$1;", hex.EncodeToString(hash[:]))
	return err
}

// DeleteTx with given hash from the db.
func (db DB) DeleteTx(hash abi.B32) error {
	_, err := db.db.Exec("DELETE FROM tx where hash=$1;", hex.EncodeToString(hash[:]))
	return err
}

// constructTx takes data queried from db and reconstruct the tx from them.
func constructTx(hash abi.B32, contract, phash, token, to, n, ghash, nhash, sighash, utxoHash string, amount, utxoVout int) (abi.Tx, error) {
	tx := abi.Tx{
		Hash: hash,
		To:   abi.Address(contract),
	}
	phashArg, err := decodeB32("phash", phash)
	if err != nil {
		return abi.Tx{}, err
	}
	tokenArg, err := decodeEthAddress("token", token)
	if err != nil {
		return abi.Tx{}, err
	}
	toArg, err := decodeEthAddress("to", to)
	if err != nil {
		return abi.Tx{}, err
	}
	nArg, err := decodeB32("n", n)
	if err != nil {
		return abi.Tx{}, err
	}
	amountArg := abi.Arg{
		Name:  "amount",
		Type:  abi.TypeU256,
		Value: abi.U256{Int: big.NewInt(int64(amount))},
	}
	ghashArg, err := decodeB32("ghash", ghash)
	if err != nil {
		return abi.Tx{}, err
	}
	nhashArg, err := decodeB32("nhash", nhash)
	if err != nil {
		return abi.Tx{}, err
	}
	sighashArg, err := decodeB32("sighash", sighash)
	if err != nil {
		return abi.Tx{}, err
	}

	// Parse the utxo details
	utxoHashArg, err := decodeB32("utxo", utxoHash)
	if err != nil {
		return abi.Tx{}, err
	}
	utxoArg := abi.Arg{
		Name: "utxo",
		Type: abi.ExtTypeBtcCompatUTXO,
		Value: abi.ExtBtcCompatUTXO{
			TxHash: utxoHashArg.Value.(abi.B32),
			VOut:   abi.U32{Int: big.NewInt(int64(utxoVout))},
		},
	}
	tx.In.Append(phashArg, tokenArg, toArg, nArg, utxoArg, amountArg)
	tx.Autogen.Append(ghashArg, nhashArg, sighashArg)

	return tx, nil
}

// decodeB32 decodes the value into a RenVM B32 Argument
func decodeB32(name, value string) (abi.Arg, error) {
	val, err := stringToB32(value)
	if err != nil {
		return abi.Arg{}, err
	}
	return abi.Arg{
		Name:  name,
		Type:  abi.TypeB32,
		Value: val,
	}, nil
}

// stringToB32 decoding the hex string to a `abi.B32` object
func stringToB32(str string) (abi.B32, error) {
	decoded, err := hex.DecodeString(str)
	if err != nil {
		return abi.B32{}, err
	}
	var val abi.B32
	copy(val[:], decoded)
	return val, nil
}

// decodeEthAddress decodes the value into a RenVM ExtTypeEthCompatAddress Argument
func decodeEthAddress(name, value string) (abi.Arg, error) {
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return abi.Arg{}, err
	}
	var val abi.ExtEthCompatAddress
	copy(val[:], decoded)
	return abi.Arg{
		Name:  name,
		Type:  abi.ExtTypeEthCompatAddress,
		Value: val,
	}, nil
}
