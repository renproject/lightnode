package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"

	"github.com/renproject/darknode/addr"
)

const TableNameAddresses = "addresses"

// MultiAddrStore stores darknode MultiAddresses.
type MultiAddrStore interface {

	// Insert the MultiAddress if it does not exist, otherwise update it.
	Insert(addr addr.MultiAddress) error

	// Insert a list of MultiAddress in batch
	InsertAddresses(addrs addr.MultiAddresses) error

	// Address returns the MultiAddress of the given id.
	Address(id string) (addr.MultiAddress, error)

	// Addresses returns all stored MultiAddresses.
	Addresses() (addr.MultiAddresses, error)

	// BootstrapAddresses return MultiAddresses for all bootstrap nodes.
	BootstrapAddresses() addr.MultiAddresses

	// RandomAddresses returns random number of MultiAddresses from the store.
	RandomAddresses(n int, isBootstrap bool) (addr.MultiAddresses, error)

	// When cycling through all addresses, this returns the next `n` available
	// addresses. It starts again from beginning when reaching the end.
	CycleThroughAddresses(n int) (addr.MultiAddresses, error)

	// Size returns the total number of MultiAddresses stored.
	Size() (int, error)

	// Deletes the MultiAddress with given id from the store.
	Delete(id string) error
}

type store struct {
	lastIndex      int
	bootstrapAddrs addr.MultiAddresses
	db             *sql.DB
}

func New(db *sql.DB, bootstrapAddrs addr.MultiAddresses) (MultiAddrStore, error) {

	// Create the addresses table if not exist.
	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %v (
    id                   SERIAL,
    renID                VARCHAR PRIMARY KEY NOT NULL,
	address              VARCHAR
);`, TableNameAddresses)
	_, err := db.Exec(query)
	if err != nil {
		return nil, err
	}

	s := &store{0, bootstrapAddrs, db}
	for _, addr := range bootstrapAddrs {
		_, err := s.Address(addr.ID().String())
		if err != nil {
			if err == sql.ErrNoRows {
				if err := s.Insert(addr); err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		}
	}
	return s, nil
}

func (s *store) Insert(addr addr.MultiAddress) error {
	data, err := addr.MarshalJSON()
	if err != nil {
		return err
	}
	query := `INSERT INTO addresses (renID, address) VALUES ($1, $2) ON CONFLICT (renID) DO UPDATE SET address=$2;`
	_, err = s.db.Exec(query, addr.ID().String(), string(data))
	return err
}

func (s *store) InsertAddresses(addrs addr.MultiAddresses) error {
	// According to https://stackoverflow.com/questions/12486436/how-do-i-batch-sql-statements-with-package-database-sql/25192138#25192138
	values := make([]string, len(addrs))
	for i, addr := range addrs {
		data, err := addr.MarshalJSON()
		if err != nil {
			return err
		}
		values[i] = fmt.Sprintf("('%v', '%v')", addr.ID().String(), string(data))
	}

	query := fmt.Sprintf(`INSERT INTO addresses (renID, address) VALUES %v ON CONFLICT (renID) DO UPDATE SET address=excluded.address;`, strings.Join(values, ","))
	_, err := s.db.Exec(query)
	return err
}

func (s *store) Address(id string) (addr.MultiAddress, error) {
	script := "SELECT address FROM addresses WHERE renID = $1"
	row := s.db.QueryRow(script, id)
	var data string
	if err := row.Scan(&data); err != nil {
		return addr.MultiAddress{}, err
	}
	var address addr.MultiAddress
	err := json.Unmarshal([]byte(data), &address)
	return address, err
}

func (s *store) Addresses() (addr.MultiAddresses, error) {
	addrs := make(addr.MultiAddresses, 0)
	query := `SELECT address FROM addresses ORDER BY id;`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var address addr.MultiAddress
		if err := json.Unmarshal([]byte(data), &address); err != nil {
			return nil, err
		}
		addrs = append(addrs, address)
	}

	return addrs, rows.Err()
}

func (s *store) BootstrapAddresses() addr.MultiAddresses {
	return s.bootstrapAddrs
}

func (s *store) RandomAddresses(n int, isBootstrap bool) (addr.MultiAddresses, error) {
	if isBootstrap {
		indexes := rand.Perm(len(s.bootstrapAddrs))
		if n > len(s.bootstrapAddrs) {
			n = len(s.bootstrapAddrs)
		}
		addrs := make(addr.MultiAddresses, 0, n)

		for _, index := range indexes {
			addrs = append(addrs, s.bootstrapAddrs[index])
		}
		return addrs, nil
	} else {
		addrs, err := s.Addresses()
		if err != nil {
			return nil, err
		}

		rand.Shuffle(len(addrs), func(i, j int) {
			addrs[i], addrs[j] = addrs[j], addrs[i]
		})

		if len(addrs) < n {
			return addrs, nil
		}
		return addrs[:n], nil
	}
}

func (s *store) CycleThroughAddresses(n int) (addr.MultiAddresses, error) {
	if n == 0 {
		return nil, nil
	}
	addrs := make(addr.MultiAddresses, 0, n)
	query := `SELECT id, address FROM addresses where id > $1 ORDER BY id LIMIT $2;`
	rows, err := s.db.Query(query, s.lastIndex, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var index int
	for rows.Next() {
		var data string
		if err := rows.Scan(&index, &data); err != nil {
			return nil, err
		}
		var address addr.MultiAddress
		err := json.Unmarshal([]byte(data), &address)
		if err != nil {
			return nil, err
		}
		addrs = append(addrs, address)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(addrs) == n {
		s.lastIndex = index
		return addrs, nil
	}

	diff := n - len(addrs)
	query = `SELECT id, address FROM addresses where id <= $1 ORDER BY id LIMIT $2;`
	rows, err = s.db.Query(query, s.lastIndex, diff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var data string
		if err := rows.Scan(&index, &data); err != nil {
			return nil, err
		}
		var address addr.MultiAddress
		err := json.Unmarshal([]byte(data), &address)
		if err != nil {
			return nil, err
		}
		addrs = append(addrs, address)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.lastIndex = index
	return addrs, nil
}

func (s *store) Size() (int, error) {
	query := `SELECT COUNT(*) FROM addresses;`
	var count int
	err := s.db.QueryRow(query).Scan(&count)
	return count, err
}

func (s *store) Delete(id string) error {
	query := `DELETE FROM addresses WHERE renID=$1;`
	_, err := s.db.Exec(query, id)
	return err
}
