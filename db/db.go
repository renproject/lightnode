package db

import (
	"database/sql"
	"encoding/base64"

	"github.com/renproject/darknode/abi"
)

type sqlDB struct {
	db *sql.DB
}

// DB interface for persistent storage in lightnode.
type DB interface {
	CreateGatewayTable() error
	DropGatewayTable() error

	InsertGateway(utxo abi.ExtBtcCompatUTXO) error
	DeleteGateway(gHash abi.B32) error
	SelectGateways() (abi.ExtBtcCompatUTXOs, error)
}

// NewSQLDB using the given sql driver.
func NewSQLDB(db *sql.DB) DB {
	return &sqlDB{
		db: db,
	}
}

// CreateGatewayTable creates a table for storing the gateway utxos, it uses gateway hash as the primary key
// and this table should be pruned frequently to remove spent gateway utxos.
func (db *sqlDB) CreateGatewayTable() error {
	_, err := db.db.Exec("create table gateways (ghash text not null primary key, utxo text);")
	return err
}

// DropGatewayTable can be used to delete the gateways table.
func (db *sqlDB) DropGatewayTable() error {
	_, err := db.db.Exec("drop table gateways;")
	return err
}

// InsertGateway into the gateways table, can be used to store a new gateway utxo or update a old one.
func (db *sqlDB) InsertGateway(utxo abi.ExtBtcCompatUTXO) error {
	utxoBytes, err := utxo.MarshalBinary()
	if err != nil {
		return err
	}
	_, err = db.db.Exec(`insert into gateways(ghash, utxo) values($1, $2) ON CONFLICT (ghash) DO UPDATE SET utxo = $2;`,
		utxo.GHash.String(), base64.StdEncoding.EncodeToString(utxoBytes))
	return err
}

// DeleteGateway from the gateways table, can be used to delete a gateway utxo after it is spent.
func (db *sqlDB) DeleteGateway(gHash abi.B32) error {
	_, err := db.db.Exec("delete from gateways where ghash=$1;", gHash.String())
	return err
}

// SelectGateways gets all the gateway utxos from the table, these utxos can be seeded into darknodes to recover
// lost transactions.
func (db *sqlDB) SelectGateways() (abi.ExtBtcCompatUTXOs, error) {
	rows, err := db.db.Query("select utxo from gateways;")
	if err != nil {
		return nil, err
	}

	var utxos abi.ExtBtcCompatUTXOs
	for rows.Next() {
		var utxoString string
		if err := rows.Scan(&utxoString); err != nil {
			return nil, err
		}
		utxoBytes, err := base64.StdEncoding.DecodeString(utxoString)
		if err != nil {
			return nil, err
		}
		utxo := abi.ExtBtcCompatUTXO{}
		if err := utxo.UnmarshalBinary(utxoBytes); err != nil {
			return nil, err
		}
		utxos = append(utxos, utxo)
	}

	return utxos, nil
}
