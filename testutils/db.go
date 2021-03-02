package testutils

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"

	"github.com/renproject/darknode/abi"
	"github.com/renproject/darknode/addr"
	"github.com/renproject/kv"
	"github.com/renproject/kv/db"
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
func UpdateTxCreatedTime(db *sql.DB, name string, hash abi.B32, createdTime int64) error {
	txHash := hex.EncodeToString(hash[:])
	script := fmt.Sprintf("UPDATE %v set created_time = %v where hash = $1;", name, createdTime)
	_, err := db.Exec(script, txHash)
	return err
}

// MultiAddrStore is a store of `addr.MultiAddress`es.
type MultiAddrStore struct {
	store          db.Table
	bootstrapAddrs addr.MultiAddresses
}

// New constructs a new `MultiAddrStore`.
func NewStore(bootstrapAddrs addr.MultiAddresses) *MultiAddrStore {
	store := kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses")
	for _, addr := range bootstrapAddrs {
		if err := store.Insert(addr.ID().String(), addr.String()); err != nil {
			panic(fmt.Sprintf("[MultiAddrStore] cannot initialize the store with bootstrap nodes addresses"))
		}
	}
	return &MultiAddrStore{
		store:          store,
		bootstrapAddrs: bootstrapAddrs,
	}
}

func (multiStore *MultiAddrStore) Insert(addresses addr.MultiAddresses) error {
	for _, addr := range addresses {
		if err := multiStore.store.Insert(addr.ID().String(), addr.String()); err != nil {
			return err
		}
	}
	return nil
}

func (multiStore *MultiAddrStore) RandomAddresses(n int) addr.MultiAddresses {
	addrs := addr.MultiAddresses{}
	iter := multiStore.store.Iterator()
	defer iter.Close()
	for iter.Next() {
		id, err := iter.Key()
		if err != nil {
			panic(err)
		}
		address, err := multiStore.Address(id)
		if err != nil {
			panic(err)
		}
		addrs = append(addrs, address)
	}
	rand.Shuffle(len(addrs), func(i, j int) {
		addrs[i], addrs[j] = addrs[j], addrs[i]
	})

	if len(addrs) < n {
		return addrs
	}
	return addrs[:n]
}

func (multiStore *MultiAddrStore) RandomBootstraps(n int) addr.MultiAddresses {
	indexes := rand.Perm(len(multiStore.bootstrapAddrs))
	if n > len(multiStore.bootstrapAddrs) {
		n = len(multiStore.bootstrapAddrs)
	}
	addrs := make(addr.MultiAddresses, 0, n)

	for _, index := range indexes {
		addrs = append(addrs, multiStore.bootstrapAddrs[index])
	}

	return addrs
}

func (multiStore *MultiAddrStore) Delete(ids []string) error {
	for _, id := range ids {
		if err := multiStore.store.Delete(id); err != nil {
			return err
		}
	}
	return nil
}

func (multiStore *MultiAddrStore) Size() int {
	size, err := multiStore.store.Size()
	if err != nil {
		panic(err)
	}
	return size
}

func (multiStore *MultiAddrStore) Address(id string) (addr.MultiAddress, error) {
	var multiAddrString string
	if err := multiStore.store.Get(id, &multiAddrString); err != nil {
		return addr.MultiAddress{}, err
	}
	return addr.NewMultiAddressFromString(multiAddrString)
}

func (multiStore *MultiAddrStore) BootstrapAddresses() addr.MultiAddresses {
	return multiStore.bootstrapAddrs
}
