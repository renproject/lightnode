package db

import (
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/renproject/darknode/abi"
)

// DB is a consistent interface across all sql databases.
type DB interface {
	CreateTxTable() error
	InsertTx(tx abi.Tx) error
	GetTx(hash abi.B32) (abi.Tx, error)
	DeleteTx(hash abi.B32) error
}

type sqlDB struct {
	db *sql.DB
}

// NewSqlDB creates a new DB instance.
func NewSqlDB(db *sql.DB) DB {
	return &sqlDB{
		db: db,
	}
}

// CreateTxTable creates the tx table if not exists. Multiple calls of this function will only create the table once.
func (db *sqlDB) CreateTxTable() error {
	script := `CREATE TABLE IF NOT EXISTS tx (
    hash                 CHAR(64) NOT NULL PRIMARY KEY,
    contract             VARCHAR(255),
    p_hash               CHAR(64),
    amount               INT,
    token                CHAR(40),
    toAddr               CHAR(40),
    n                    CHAR(64),
	utxo_tx_hash         CHAR(64),
    utxo_vout            INT,
	utxo_script_pub_key  VARCHAR(255),
    utxo_amount          INT,
    utxo_g_hash          CHAR(64));`
	_, err := db.db.Exec(script)
	return err
}

// InsertTx inserts the RenVM transaction into the db.
func (db *sqlDB) InsertTx(tx abi.Tx) error {
	phash, ok := tx.Args.Get("phash").Value.(abi.B32)
	if !ok {
		return fmt.Errorf("unexpected type for phash, expected abi.B32, got %v", tx.Args.Get("phash").Value.Type())
	}
	amount, ok := tx.Args.Get("amount").Value.(abi.U64)
	if !ok {
		return fmt.Errorf("unexpected type for amount, expected abi.U64, got %v", tx.Args.Get("amount").Value.Type())
	}
	token, ok := tx.Args.Get("token").Value.(abi.B20)
	if !ok {
		return fmt.Errorf("unexpected type for token, expected abi.B20, got %v", tx.Args.Get("token").Value.Type())
	}
	to, ok := tx.Args.Get("to").Value.(abi.B20)
	if !ok {
		return fmt.Errorf("unexpected type for to, expected abi.B20, got %v", tx.Args.Get("to").Value.Type())
	}
	n, ok := tx.Args.Get("n").Value.(abi.B32)
	if !ok {
		return fmt.Errorf("unexpected type for n, expected abi.B32, got %v", tx.Args.Get("n").Value.Type())
	}
	utxo, ok := tx.Args.Get("utxo").Value.(abi.ExtBtcCompatUTXO)
	if !ok {
		return fmt.Errorf("unexpected type for utxo, expected abi.ExtTypeBtcCompatUTXO, got %v", tx.Args.Get("utxo").Value.Type())
	}

	script := `INSERT INTO Tx (hash, contract, p_hash, amount, token, toAddr, n, utxo_tx_hash, utxo_vout, utxo_script_pub_key, utxo_amount, utxo_g_hash)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) ON CONFLICT DO NOTHING;`
	_, err := db.db.Exec(script, hex.EncodeToString(tx.Hash[:]), tx.To, hex.EncodeToString(phash[:]), int(amount),
		hex.EncodeToString(token[:]), hex.EncodeToString(to[:]), hex.EncodeToString(n[:]), hex.EncodeToString(utxo.TxHash[:]),
		int(utxo.VOut), utxo.ScriptPubKey.String(), int(utxo.Amount), hex.EncodeToString(utxo.GHash[:]))
	return err
}

// GetTx queries the db and returns the tx with given hash.
func (db *sqlDB) GetTx(txHash abi.B32) (abi.Tx, error) {
	var hash, contract, phash, token, to, n, utxoHash, utxoScriptPubKey, utxoGhash string
	var amount, utxoVout, utxoAmount int
	err := db.db.QueryRow("SELECT hash, contract, p_hash, amount, token, toAddr, n, utxo_tx_hash, utxo_vout, utxo_script_pub_key, utxo_amount, utxo_g_hash FROM Tx WHERE hash=$1", hex.EncodeToString(txHash[:])).Scan(
		&hash, &contract, &phash, &amount, &token, &to, &n, &utxoHash, &utxoVout, &utxoScriptPubKey, &utxoAmount, &utxoGhash)
	if err != nil {
		return abi.Tx{}, nil
	}
	tx := abi.Tx{
		Hash: txHash,
		To:   abi.Addr(contract),
	}
	phashArg, err := decodeB32("phash", phash)
	if err != nil {
		return abi.Tx{}, err
	}
	amountArg := decodeU64("amount", amount)
	tokenArg, err := decodeB20("token", token)
	if err != nil {
		return abi.Tx{}, err
	}
	toArg, err := decodeB20("to", to)
	if err != nil {
		return abi.Tx{}, err
	}
	nArg, err := decodeB32("n", n)
	if err != nil {
		return abi.Tx{}, err
	}

	// Parse the utxo details
	utxoHashArg, err := decodeB32("utxo", utxoHash)
	if err != nil {
		return abi.Tx{}, err
	}
	scriptPubKey, err := hex.DecodeString(utxoScriptPubKey)
	if err != nil {
		return abi.Tx{}, err
	}
	ghashArg, err := decodeB32("ghash", utxoGhash)
	if err != nil {
		return abi.Tx{}, err
	}

	utxoArg := abi.Arg{
		Name: "utxo",
		Type: abi.ExtTypeBtcCompatUTXO,
		Value: abi.ExtBtcCompatUTXO{
			TxHash:       utxoHashArg.Value.(abi.B32),
			VOut:         abi.U32(utxoVout),
			ScriptPubKey: scriptPubKey,
			Amount:       abi.U64(utxoAmount),
			GHash:        ghashArg.Value.(abi.B32),
		},
	}

	tx.Args.Append(phashArg, amountArg, tokenArg, toArg, nArg, utxoArg)

	return tx, nil

}

// DeleteTx with given hash from the db.
func (db *sqlDB) DeleteTx(hash abi.B32) error {
	_, err := db.db.Exec("DELETE FROM Tx where hash=$1;", hex.EncodeToString(hash[:]))
	return err
}

// decodeU64 decodes the value into a RenVM U64 Argument
func decodeU64(name string, value int) abi.Arg {
	return abi.Arg{
		Name:  name,
		Type:  abi.TypeU64,
		Value: abi.U64(value),
	}
}

// decodeB32 decodes the value into a RenVM B32 Argument
func decodeB32(name, value string) (abi.Arg, error) {
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return abi.Arg{}, err
	}
	var val abi.B32
	copy(val[:], decoded)
	return abi.Arg{
		Name:  name,
		Type:  abi.TypeB32,
		Value: val,
	}, nil
}

// decodeB20 decodes the value into a RenVM B20 Argument
func decodeB20(name, value string) (abi.Arg, error) {
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return abi.Arg{}, err
	}
	var val abi.B20
	copy(val[:], decoded)
	return abi.Arg{
		Name:  name,
		Type:  abi.TypeB20,
		Value: val,
	}, nil
}
