package db

import (
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/renproject/darknode/abi"
)

type TxStatus int8

const (
	TxStatusNil TxStatus = iota
	TxStatusConfirming
	TxStatusConfirmed
	// TxStatusDispatched
	TxStatusSubmitted
)

type Scannable interface {
	Scan(dest ...interface{}) error
}

// DB is an storage adapter building on top of a sql database. It is the place
// where all txs details are stored. We can query details of a particular txs or
// update its status.
type DB interface {

	// Initialize the database. Init should be called once the db object is created.
	Init() error

	// InsertTx inserts the tx into the database.
	InsertTx(tx abi.Tx) error

	// Tx gets the details of the tx with given txHash. It returns an `sql.ErrNoRows`
	// if tx cannot be found.
	Tx(hash abi.B32, transformed bool) (abi.Tx, error)

	// ShiftIns returns shiftIn txs with given status and are not expired.
	ShiftIns(status TxStatus, expiry time.Duration, contract string) (abi.Txs, error)

	// PendingTxs returns all pending txs in the database which are not expired.
	PendingTxs(expiry time.Duration) ([]abi.Tx, error)

	// UnsubmittedTxs returns the txs with payload which have reached enough
	// confirmations and been sent to darknodes.
	UnsubmittedTxs(expiry time.Duration) ([]abi.B32, error)

	// TxStatus returns the current status of the tx with given has.
	TxStatus(hash abi.B32) (TxStatus, error)

	// UpdateStatus updates the status of given tx. Please noted the status cannot
	// be updated to an previous status.
	UpdateStatus(hash abi.B32, status TxStatus) error

	// Prune deletes tx which are expired.
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

// Init creates the tables for storing txs if it does not exist. Multiple calls
// of this function will only create the tables once and not return an error.
// TODO: Decide approach for versioning database tables.
func (db database) Init() error {

	// Create the shift_in table if not exist.
	shiftIn := `CREATE TABLE IF NOT EXISTS shift_in (
    hash                 CHAR(64) NOT NULL PRIMARY KEY,
    status               INT,
    created_time         INT,
    contract             VARCHAR(255),
    p                    VARCHAR,
    token                CHAR(40),
    toAddr               CHAR(40),
    n                    CHAR(64),
    utxo_hash            CHAR(64),
    utxo_vout            INT
);`
	_, err := db.db.Exec(shiftIn)
	if err != nil {
		return err
	}

	// Create the shift_in_autogen table if not exist.
	shiftInAutogen := `CREATE TABLE IF NOT EXISTS shift_in_autogen (
    hash                 CHAR(64) NOT NULL PRIMARY KEY,
    ghash                CHAR(64),
	nhash                CHAR(64),
	sighash              CHAR(64),
	phash                CHAR(64),
	amount               VARCHAR,
	utxo                 VARCHAR
);`
	_, err = db.db.Exec(shiftInAutogen)
	if err != nil {
		return err
	}

	// Create the shift_out table if not exist.
	shiftOut := `CREATE TABLE IF NOT EXISTS shift_out (
    hash                 CHAR(64) NOT NULL PRIMARY KEY,
    status               INT,
    created_time         INT,
    contract             VARCHAR(255), 
    ref                  VARCHAR, 
    toAddr               VARCHAR(255),
    amount               VARCHAR
);`
	_, err = db.db.Exec(shiftOut)
	return err
}

// InsertTx implements the `DB` interface.
func (db database) InsertTx(tx abi.Tx) error {
	if abi.IsShiftIn(tx.To) {
		if err := db.insertShiftIn(tx); err != nil {
			return err
		}
		return db.insertShiftInAutogen(tx)
	} else {
		return db.insertShiftOut(tx)
	}
}

// Tx implements the `DB` interface.
func (db database) Tx(hash abi.B32, transformed bool) (abi.Tx, error) {
	tx, err := db.shiftIn(hash, transformed)
	if err == sql.ErrNoRows {
		return db.shiftOut(hash, transformed)
	}
	return tx, err
}

func (db database) PendingTxs(expiry time.Duration) ([]abi.Tx, error) {
	txs := make(abi.Txs, 0, 128)
	shiftIns, err := db.db.Query(`SELECT hash, contract, p, token, toAddr, n, utxo_hash, utxo_vout FROM shift_in 
	WHERE status = $1 AND $2 - created_time < $3;`, TxStatusConfirming, time.Now().Unix(), int64(expiry.Seconds()))
	if err != nil {
		return nil, err
	}
	// Loop through rows and convert them to txs.
	for shiftIns.Next() {
		tx, err := rowToShiftIn(shiftIns)
		if err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}
	if shiftIns.Err() != nil {
		return nil, err
	}

	// Get pending shiftOuts txs from the db.
	shiftOuts, err := db.db.Query(`SELECT hash, contract, ref, toAddr, amount FROM shift_out 
		WHERE status = $1 AND $2 - created_time < $3`, TxStatusConfirming, time.Now().Unix(), int64(expiry.Seconds()))
	if err != nil {
		return nil, err
	}
	defer shiftOuts.Close()

	for shiftOuts.Next() {
		tx, err := rowToShiftOut(shiftOuts, false)
		if err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}
	return txs, shiftOuts.Err()
}

// TxsWithStatus implements the `DB` interface.
func (db database) ShiftIns(status TxStatus, expiry time.Duration, contract string) (abi.Txs, error) {
	txs := make(abi.Txs, 0, 128)

	// Check if a particular contract address if provided.
	contractCons := ""
	if contract != "" {
		contractCons = fmt.Sprintf("AND toAddr = '%v'", contract)
	}
	script := fmt.Sprintf(`SELECT hash, contract, p, token, toAddr, n, utxo_hash, utxo_vout FROM shift_in 
    WHERE status = $1 AND $2 - created_time < %v %v;`, int64(expiry.Seconds()), contractCons)

	// Get pending shiftIn txs from db.
	shiftIns, err := db.db.Query(script, status, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	defer shiftIns.Close()

	// Loop through rows and convert them to txs.
	for shiftIns.Next() {
		tx, err := rowToShiftIn(shiftIns)
		if err != nil {
			return nil, err
		}
		tx, err = db.autogen(tx)
		if err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}

	return txs, shiftIns.Err()
}

// UnsubmittedTxs implements the `DB` interface.
func (db database) UnsubmittedTxs(expiry time.Duration) ([]abi.B32, error) {
	hashes := make([]abi.B32, 0)

	// Get txs which haven't been submitted
	shiftIns, err := db.db.Query(`SELECT hash FROM shift_in 
		WHERE status = $1 AND $2 - created_time < $3 AND LENGTH(p)>0;`, TxStatusConfirmed, time.Now().Unix(), int64(expiry.Seconds()))
	if err != nil {
		return nil, err
	}
	defer shiftIns.Close()

	for shiftIns.Next() {
		var hash string
		if err := shiftIns.Scan(&hash); err != nil {
			return nil, err
		}
		txHash, err := stringToB32(hash)
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, txHash)
	}
	return hashes, shiftIns.Err()
}

// TxStatus implements the `DB` interface.
func (db database) TxStatus(hash abi.B32) (TxStatus, error) {
	var status int
	err := db.db.QueryRow(`SELECT status FROM shift_in WHERE hash = $1;`,
		hex.EncodeToString(hash[:])).Scan(&status)
	if err == sql.ErrNoRows {
		err = db.db.QueryRow(`SELECT status FROM shift_out WHERE hash = $1;`,
			hex.EncodeToString(hash[:])).Scan(&status)
	}
	return TxStatus(status), err
}

// UpdateStatus implements the `DB` interface.
func (db database) UpdateStatus(hash abi.B32, status TxStatus) error {
	_, err := db.db.Exec("UPDATE shift_in SET status = $1 WHERE hash = $2 AND status < $1;", status, hex.EncodeToString(hash[:]))
	if err != nil {
		return err
	}
	_, err = db.db.Exec("UPDATE shift_out SET status = $1 WHERE hash = $2 AND status < $1;", status, hex.EncodeToString(hash[:]))
	return err
}

// Prune deletes txs which have expired based on the given expiry.
func (db database) Prune(expiry time.Duration) error {
	_, err := db.db.Exec("DELETE FROM shift_in WHERE $1 - created_time > $2;", time.Now().Unix(), int(expiry.Seconds()))
	if err != nil {
		return err
	}

	_, err = db.db.Exec("DELETE FROM shift_out WHERE $1 - created_time > $2;", time.Now().Unix(), int(expiry.Seconds()))
	return err
}

// Inserts the original request received from user into the shift_in table.
func (db database) insertShiftIn(tx abi.Tx) error {
	p := tx.In.Get("p")
	if p.IsNil() {
		return errors.New("invalid tx, missing parameter p")
	}
	pVal, err := json.Marshal(p.Value)
	if err != nil {
		return err
	}
	token, ok := tx.In.Get("token").Value.(abi.ExtEthCompatAddress)
	if !ok {
		return fmt.Errorf("unexpected type for token, expected abi.ExtEthCompatAddress, got %v", tx.In.Get("token").Value.Type())
	}
	to, ok := tx.In.Get("to").Value.(abi.ExtEthCompatAddress)
	if !ok {
		return fmt.Errorf("unexpected type for to, expected abi.ExtEthCompatAddress, got %v", tx.In.Get("to").Value.Type())
	}
	n, ok := tx.In.Get("n").Value.(abi.B32)
	if !ok {
		return fmt.Errorf("unexpected type for n, expected abi.B32, got %v", tx.In.Get("n").Value.Type())
	}
	utxo, ok := tx.In.Get("utxo").Value.(abi.ExtBtcCompatUTXO)
	if !ok {
		return fmt.Errorf("unexpected type for utxo, expected abi.ExtTypeBtcCompatUTXO, got %v", tx.In.Get("utxo").Value.Type())
	}

	script := `INSERT INTO shift_in (hash, status, created_time, contract, p, token, toAddr, n, utxo_hash, utxo_vout)
VALUES ($1, 1, $2, $3, $4, $5, $6, $7, $8, $9);`
	_, err = db.db.Exec(script,
		hex.EncodeToString(tx.Hash[:]),
		time.Now().Unix(),
		tx.To,
		hex.EncodeToString(pVal),
		hex.EncodeToString(token[:]),
		hex.EncodeToString(to[:]),
		hex.EncodeToString(n[:]),
		hex.EncodeToString(utxo.TxHash[:]),
		utxo.VOut.Int.Int64(),
	)

	return err
}

// Inserts extra fields generated by lightnode into shift_in_autogen table.
func (db database) insertShiftInAutogen(tx abi.Tx) error {
	ghash, ok := tx.Autogen.Get("ghash").Value.(abi.B32)
	if !ok {
		return fmt.Errorf("unexpected type for ghash, expected abi.B32, got %v", tx.Autogen.Get("ghash").Value.Type())
	}
	nhash, ok := tx.Autogen.Get("nhash").Value.(abi.B32)
	if !ok {
		return fmt.Errorf("unexpected type for nhash, expected abi.B32, got %v", tx.Autogen.Get("nhash").Value.Type())
	}
	sighash, ok := tx.Autogen.Get("sighash").Value.(abi.B32)
	if !ok {
		return fmt.Errorf("unexpected type for sighash, expected abi.B32, got %v", tx.Autogen.Get("sighash").Value.Type())
	}
	phash, ok := tx.Autogen.Get("phash").Value.(abi.B32)
	if !ok {
		return fmt.Errorf("unexpected nil value for phash argument in tx = %v", tx.Hash.String())
	}
	amount, ok := tx.Autogen.Get("amount").Value.(abi.U256)
	if !ok {
		return fmt.Errorf("unexpected type for amount, expected abi.U256, got %v", tx.In.Get("amount").Value.Type())
	}
	utxo, ok := tx.Autogen.Get("utxo").Value.(abi.ExtBtcCompatUTXO)
	if !ok {
		return fmt.Errorf("unexpected type for utxo, expected abi.ExtTypeBtcCompatUTXO, got %v", tx.In.Get("utxo").Value.Type())
	}
	utxoBytes, err := utxo.MarshalBinary()
	if err != nil {
		panic(err)
	}

	script := `INSERT INTO shift_in_autogen (hash, ghash, nhash, sighash, phash, amount, utxo)
VALUES ($1, $2, $3, $4, $5, $6, $7);`
	_, err = db.db.Exec(script,
		hex.EncodeToString(tx.Hash[:]),
		hex.EncodeToString(ghash[:]),
		hex.EncodeToString(nhash[:]),
		hex.EncodeToString(sighash[:]),
		hex.EncodeToString(phash[:]),
		amount.Int.String(),
		hex.EncodeToString(utxoBytes),
	)

	return err
}

// InsertShiftOut stores a shift out tx to the database.
func (db database) insertShiftOut(tx abi.Tx) error {
	ref, ok := tx.In.Get("ref").Value.(abi.U64)
	if !ok {
		return fmt.Errorf("unexpected type for ref, expected abi.U64, got %v", tx.In.Get("ref").Value.Type())
	}
	to, ok := tx.In.Get("to").Value.(abi.B)
	if !ok {
		return fmt.Errorf("unexpected type for to, expected abi.B, got %v", tx.In.Get("to").Value.Type())
	}
	amount, ok := tx.In.Get("amount").Value.(abi.U256)
	if !ok {
		return fmt.Errorf("unexpected type for amount, expected abi.U256, got %v", tx.In.Get("amount").Value.Type())
	}

	script := `INSERT INTO shift_out (hash, status, created_time, contract, ref, toAddr, amount) 
VALUES ($1, 1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING;`
	_, err := db.db.Exec(script,
		hex.EncodeToString(tx.Hash[:]),
		time.Now().Unix(),
		tx.To,
		ref.Int.String(),
		hex.EncodeToString(to),
		amount.Int.String(),
	)
	return err
}

// ShiftIn returns the shift in tx with the given hash.
func (db database) shiftIn(txHash abi.B32, transformed bool) (abi.Tx, error) {
	script := "SELECT hash, contract, p, token, toAddr, n, utxo_hash, utxo_vout FROM shift_in WHERE hash = $1"
	row := db.db.QueryRow(script, hex.EncodeToString(txHash[:]))
	tx, err := rowToShiftIn(row)
	if err != nil {
		return abi.Tx{}, err
	}
	if transformed {
		return db.autogen(tx)
	}
	return tx, nil
}

// ShiftOut returns the shift out tx with the given hash.
func (db database) shiftOut(txHash abi.B32, transformed bool) (abi.Tx, error) {
	script := "SELECT hash, contract, ref, toAddr, amount FROM shift_out WHERE hash = $1"
	row := db.db.QueryRow(script, hex.EncodeToString(txHash[:]))
	return rowToShiftOut(row, transformed)
}

// scan data from the returned row and parse it into a abi.Tx.
func rowToShiftIn(row Scannable) (abi.Tx, error) {
	var hashStr, p, contract, token, to, n, utxoHash string
	var utxoVout int

	err := row.Scan(&hashStr, &contract, &p, &token, &to, &n, &utxoHash, &utxoVout)
	if err != nil {
		return abi.Tx{}, err
	}

	tx := abi.Tx{}
	hash, err := stringToB32(hashStr)
	if err != nil {
		return abi.Tx{}, err
	}
	tx.Hash = hash
	tx.To = abi.Address(contract)

	// Decode all the parameters
	pArg, err := decodePayload(p)
	if err != nil {
		return abi.Tx{}, err
	}
	tx.In.Set(pArg)

	tokenArg, err := decodeEthAddress("token", token)
	if err != nil {
		return abi.Tx{}, err
	}
	tx.In.Set(tokenArg)

	toArg, err := decodeEthAddress("to", to)
	if err != nil {
		return abi.Tx{}, err
	}
	tx.In.Set(toArg)

	nArg, err := decodeB32("n", n)
	if err != nil {
		return abi.Tx{}, err
	}
	tx.In.Set(nArg)

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
	tx.In.Set(utxoArg)
	return tx, nil
}

// Get extra data generated by the lightnode and add to the given tx.
func (db database) autogen(tx abi.Tx) (abi.Tx, error) {
	var phash, ghash, nhash, sighash, amountStr, utxoStr string

	script := "SELECT phash, ghash, nhash, sighash, amount, utxo FROM shift_in_autogen WHERE hash = $1"
	err := db.db.QueryRow(script, hex.EncodeToString(tx.Hash[:])).Scan(
		&phash, &ghash, &nhash, &sighash, &amountStr, &utxoStr)
	if err != nil {
		return abi.Tx{}, err
	}
	// Decode other inputs
	phashArg, err := decodeB32("phash", phash)
	if err != nil {
		return abi.Tx{}, err
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
	amount, ok := big.NewInt(0).SetString(amountStr, 10)
	if !ok {
		return abi.Tx{}, fmt.Errorf("fail to parse big number [%v]", amountStr)
	}
	amountArg := abi.Arg{
		Name:  "amount",
		Type:  abi.TypeU256,
		Value: abi.U256{Int: amount},
	}
	utxoBytes, err := hex.DecodeString(utxoStr)
	if err != nil {
		return abi.Tx{}, err
	}
	var utxo abi.ExtBtcCompatUTXO
	if err := utxo.UnmarshalBinary(utxoBytes); err != nil {
		return abi.Tx{}, err
	}
	utxoArg := abi.Arg{
		Name:  "utxo",
		Type:  abi.ExtTypeBtcCompatUTXO,
		Value: utxo,
	}
	tx.Autogen.Set(phashArg)
	tx.Autogen.Set(ghashArg)
	tx.Autogen.Set(nhashArg)
	tx.Autogen.Set(sighashArg)
	tx.Autogen.Set(amountArg)
	tx.Autogen.Set(utxoArg)
	return tx, nil
}

func rowToShiftOut(row Scannable, transformed bool) (abi.Tx, error) {
	var hashStr, contract, to, amountStr, refStr string
	err := row.Scan(&hashStr, &contract, &refStr, &to, &amountStr)
	if err != nil {
		return abi.Tx{}, err
	}

	hash, err := stringToB32(hashStr)
	if err != nil {
		return abi.Tx{}, err
	}
	tx := abi.Tx{
		Hash: hash,
		To:   abi.Address(contract),
	}

	ref, ok := big.NewInt(0).SetString(refStr, 10)
	if !ok {
		return abi.Tx{}, fmt.Errorf("fail to parse big number [%v]", refStr)
	}
	refArg := abi.Arg{
		Name:  "ref",
		Type:  abi.TypeU64,
		Value: abi.U64{Int: ref},
	}
	tx.In.Set(refArg)

	if transformed {
		toBytes, err := hex.DecodeString(to)
		if err != nil {
			return abi.Tx{}, err
		}
		toArg := abi.Arg{
			Name:  "to",
			Type:  abi.TypeB,
			Value: abi.B(toBytes),
		}
		tx.In.Set(toArg)

		amount, ok := big.NewInt(0).SetString(amountStr, 10)
		if !ok {
			return abi.Tx{}, fmt.Errorf("fail to parse big number [%v]", amountStr)
		}

		amountArg := abi.Arg{
			Name:  "amount",
			Type:  abi.TypeU256,
			Value: abi.U256{Int: amount},
		}
		tx.In.Set(amountArg)
	}

	return tx, nil
}

func decodePayload(p string) (abi.Arg, error) {
	var pVal abi.ExtEthCompatPayload
	data, err := hex.DecodeString(p)
	if err != nil {
		return abi.Arg{}, err
	}
	if err := json.Unmarshal(data, &pVal); err != nil {
		return abi.Arg{}, err
	}
	return abi.Arg{
		Name:  "p",
		Type:  abi.ExtTypeEthCompatPayload,
		Value: pVal,
	}, nil
}

// decodeB32 decodes the value into a RenVM B32 argument.
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

// stringToB32 decoding the hex string into a RenVM B32 object.
func stringToB32(str string) (abi.B32, error) {
	decoded, err := hex.DecodeString(str)
	if err != nil {
		return abi.B32{}, err
	}
	var val abi.B32
	copy(val[:], decoded)
	return val, nil
}

// decodeEthAddress decodes the value into a RenVM ExtTypeEthCompatAddress
// argument.
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
