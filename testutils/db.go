package testutils

import (
	"database/sql"
	"errors"
	"fmt"
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

// DropTable with given name from the db instance.
func DropTable(db *sql.DB, name string) error {
	script := fmt.Sprintf("DROP TABLE %v", name)
	_, err := db.Exec(script)
	return err
}

// NumOfDataEntries returns the number of data entries in the queried table.
func NumOfDataEntries(db *sql.DB, name string) (int, error) {
	script := fmt.Sprintf("SELECT count(*) FROM %v", name)
	var num int
	err := db.QueryRow(script).Scan(&num)
	return num, err
}
