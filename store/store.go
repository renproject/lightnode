package store

import (
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"

	"github.com/renproject/darknode/addr"
)

const (
	TableNameAddresses = "addresses"

	RenPort = "18515"
)

var ErrNotFound = errors.New("multiAddress not found")

// MultiAddrStore stores darknode MultiAddresses.
type MultiAddrStore interface {

	// Insert a list of MultiAddress in batch
	Insert(addresses addr.MultiAddresses) error

	// Address returns the MultiAddress of the given id.
	Address(id string) (addr.MultiAddress, error)

	// RandomAddresses returns random number of MultiAddresses from the store.
	// The returned MultiAddresses will not contain bootstrap nodes.
	RandomAddresses(n int) addr.MultiAddresses

	// BootstrapAddresses return MultiAddresses for all bootstrap nodes.
	BootstrapAddresses() addr.MultiAddresses

	// RandomBootstraps returns random number of bootstrap MultiAddresses from the store
	// It will return all the bootstrap addresses when `n` is negative.
	RandomBootstraps(n int) addr.MultiAddresses

	// Deletes the MultiAddress with given id from the store.
	Delete(ids []string) error

	// Size returns the total number of MultiAddresses stored.
	Size() int
}

func New(db *sql.DB, bootstrapAddrs addr.MultiAddresses) (MultiAddrStore, error) {
	// Create the addresses table if not exist.
	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %v (
    id                   VARCHAR PRIMARY KEY NOT NULL,
	ip                   VARCHAR
);`, TableNameAddresses)
	_, err := db.Exec(query)
	if err != nil {
		return nil, err
	}

	s := &store{
		mu:         new(sync.RWMutex),
		bootstraps: bootstrapAddrs,
		addresses:  map[string]addr.MultiAddress{},
		db:         db,
	}

	// Insert bootstrap address to the table if not exist
	if err := s.Insert(bootstrapAddrs); err != nil {
		return nil, err
	}

	// Load all addresses from db to cache
	script := fmt.Sprintf("SELECT * FROM %v", TableNameAddresses)
	rows, err := s.db.Query(script)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, ip string
		if err := rows.Scan(&id, &ip); err != nil {
			return nil, err
		}
		ipAddr, err := net.ResolveTCPAddr("tcp", ip)
		if err != nil {
			return nil, err
		}
		multiStr := fmt.Sprintf("/ip4/%v/tcp/%v/ren/%v", ipAddr.IP.String(), ipAddr.Port, id)
		address, err := addr.NewMultiAddressFromString(multiStr)
		if err != nil {
			return nil, err
		}
		s.addresses[address.ID().String()] = address
	}
	if rows.Err() != nil {
		return nil, err
	}
	return s, nil
}

type store struct {
	mu         *sync.RWMutex
	bootstraps addr.MultiAddresses
	addresses  map[string]addr.MultiAddress
	db         *sql.DB
}

func (s *store) BootstrapAddresses() addr.MultiAddresses {
	s.mu.RLock()
	defer s.mu.RUnlock()

	addrs := make([]addr.MultiAddress, len(s.bootstraps))
	for i, addr := range s.bootstraps {
		addrs[i] = addr
	}

	return addrs
}

func (s *store) Insert(addresses addr.MultiAddresses) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	updates := make([]addr.MultiAddress, 0)
	for _, addr := range addresses {
		current, ok := s.addresses[addr.ID().String()]
		if ok && current.String() == addr.String() {
			continue
		}
		s.addresses[addr.ID().String()] = addr
		updates = append(updates, addr)
	}

	if len(updates) == 0 {
		return nil
	}

	// According to https://stackoverflow.com/questions/12486436/how-do-i-batch-sql-statements-with-package-database-sql/25192138#25192138
	values := make([]string, len(updates))
	for i, addr := range updates {
		values[i] = fmt.Sprintf("('%v', '%v')", addr.ID().String(), fmt.Sprintf("%v:%v", addr.IP4(), addr.Port()))
	}

	query := fmt.Sprintf(`INSERT INTO addresses (id, ip) VALUES %v ON CONFLICT (id) DO UPDATE SET ip=excluded.ip;`, strings.Join(values, ","))
	_, err := s.db.Exec(query)
	return err
}

func (s *store) Address(id string) (addr.MultiAddress, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	address, ok := s.addresses[id]
	if !ok {
		return addr.MultiAddress{}, ErrNotFound
	}
	return address, nil
}

func (s *store) RandomAddresses(n int) addr.MultiAddresses {
	s.mu.RLock()
	defer s.mu.RUnlock()

	addrs := make([]addr.MultiAddress, 0, n)
	for _, value := range s.addresses {
		addrs = append(addrs, value)
		if len(addrs) == n {
			return addrs
		}
	}
	return addrs
}

func (s *store) RandomBootstraps(n int) addr.MultiAddresses {
	s.mu.RLock()
	defer s.mu.RUnlock()

	indexes := rand.Perm(len(s.bootstraps))
	if n > len(s.bootstraps) {
		n = len(s.bootstraps)
	}
	addrs := make(addr.MultiAddresses, 0, n)

	for _, index := range indexes {
		addrs = append(addrs, s.bootstraps[index])
		if len(addrs) == n {
			return addrs
		}
	}
	return addrs
}

func (s *store) Delete(ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove from cache
	for _, id := range ids {
		delete(s.addresses, id)
	}

	// Remove from the underlying database
	values := make([]string, len(ids))
	for i, id := range ids {
		values[i] = fmt.Sprintf("'%v'", id)
	}
	script := fmt.Sprintf("DELETE FROM %v WHERE id IN (%v)", TableNameAddresses, strings.Join(values, ","))
	_, err := s.db.Exec(script)
	return err
}

func (s *store) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.addresses)
}
