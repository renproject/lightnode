package testutils

import (
	"database/sql"
	"errors"
	"fmt"
)

// CheckTableExistenceSqlite checks if table with given exists in a sqlite db.
func CheckTableExistenceSqlite(db *sql.DB, name string) error {
	script := fmt.Sprintf("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='%v';", name)
	var num int
	if err := db.QueryRow(script).Scan(&num); err != nil {
		return err
	}
	if num != 1 {
		return errors.New("no such table")
	}
	return nil
}

// CheckTableExistencePostgres checks if table with given exists in a postgres db.
func CheckTableExistencePostgres(db *sql.DB, name string) error {
	script := fmt.Sprintf(`SELECT EXISTS (
	SELECT 1
	FROM   pg_tables
	WHERE  schemaname = 'public'
	AND    tablename = '%v'
	);`, name)
	var exist bool
	if err := db.QueryRow(script).Scan(&exist); err != nil {
		return err
	}
	if !exist {
		return errors.New("no such table")
	}
	return nil
}

func NumOfDataEntries(db *sql.DB) (int, error) {
	script := "SELECT count(*) FROM tx"
	var num int
	err := db.QueryRow(script).Scan(&num)
	return num, err
}
