package testutils

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/renproject/pack"
)

// CheckTableExistence checks the underlying `db` object if there exists a table
// with given name.
func CheckTableExistence(dbName, tableName string, db *sql.DB) error {
	switch dbName {
	case "sqlite3":
		script := fmt.Sprintf("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='%v';", tableName)
		var num int
		if err := db.QueryRow(script).Scan(&num); err != nil {
			return err
		}
		if num != 1 {
			return errors.New("no such table")
		}
	case "postgres":
		script := fmt.Sprintf(`SELECT EXISTS (
	SELECT 1
	FROM   pg_tables
	WHERE  schemaname = 'public'
	AND    tablename = '%v'
	);`, tableName)
		var exist bool
		if err := db.QueryRow(script).Scan(&exist); err != nil {
			return err
		}
		if !exist {
			return errors.New("no such table")
		}
	default:
		panic("unknown sql db")
	}
	return nil
}

// NumOfDataEntries returns the number of data entries in the queried table.
func NumOfDataEntries(db *sql.DB, name string) (int, error) {
	script := fmt.Sprintf("SELECT count(*) FROM %v;", name)
	var num int
	err := db.QueryRow(script).Scan(&num)
	return num, err
}

// UpdateTxCreatedTime of given tx hash.
func UpdateTxCreatedTime(db *sql.DB, name string, txHash pack.Bytes32, createdTime int64) error {
	script := fmt.Sprintf("UPDATE %v set created_time = %v where hash = $1;", name, createdTime)
	_, err := db.Exec(script, txHash.String())
	return err
}
