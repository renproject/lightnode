package store_test

import (
	"database/sql"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/store"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	"github.com/renproject/darknode/addr"
	"github.com/renproject/darknode/testutil"
)

const (
	Sqlite   = "sqlite3"
	Postgres = "postgres"
)

var _ = Describe("MultiAddrStore", func() {

	// TODO : Enable sqlite
	testDBs := []string{ /*Sqlite, */ Postgres}

	init := func(name string) *sql.DB {
		var source string
		switch name {
		case Sqlite:
			source = "./test.db"
		case Postgres:
			source = "postgresql://postgres:postgres@localhost:5432/postgres?sslmode=disable"
		default:
			panic("unknown")
		}
		sqlDB, err := sql.Open(name, source)
		Expect(err).NotTo(HaveOccurred())
		query := "DROP TABLE IF EXISTS addresses;"
		_, err = sqlDB.Exec(query)
		Expect(err).NotTo(HaveOccurred())

		// foreign_key needs to be manually enabled for Sqlite
		if name == Sqlite {
			_, err := sqlDB.Exec("PRAGMA foreign_keys = ON;")
			Expect(err).NotTo(HaveOccurred())
		}
		return sqlDB
	}

	cleanup := func(db *sql.DB) {
		query := "DROP TABLE IF EXISTS addresses;"
		_, err := db.Exec(query)
		Expect(err).NotTo(HaveOccurred())
		Expect(db.Close()).Should(Succeed())
	}

	BeforeSuite(func() {
		os.Remove("./test.db")
	})

	AfterSuite(func() {
		os.Remove("./test.db")
	})

	for _, dbname := range testDBs {
		dbname := dbname

		Context(dbname, func() {
			Context("when initializing the store", func() {
				Context("when it's the first time to initialize", func() {
					It("should insert the bootstrap address into the store ", func() {
						db := init(dbname)
						defer cleanup(db)

						numBoostrapAddrs := 10
						store, err := New(db, RandomAddresses(numBoostrapAddrs))
						Expect(err).ShouldNot(HaveOccurred())

						size := store.Size()
						Expect(size).Should(Equal(numBoostrapAddrs))
					})
				})

				Context("when it's not the first time to initialize", func() {
					It("should construct the store without any error", func() {
						db := init(dbname)
						defer cleanup(db)

						numBoostrapAddrs := 10
						bootstraps := RandomAddresses(numBoostrapAddrs)
						_, err := New(db, bootstraps)
						Expect(err).ShouldNot(HaveOccurred())

						store, err := New(db, bootstraps)
						Expect(err).ShouldNot(HaveOccurred())

						size := store.Size()
						Expect(size).Should(Equal(numBoostrapAddrs))
					})
				})
			})

			Context("when interacting with db", func() {
				It("should be able to read and write addresses", func() {
					db := init(dbname)
					defer cleanup(db)

					numBoostrapAddrs := 10
					addrsMap := map[string]struct{}{}
					bootstraps := RandomAddresses(numBoostrapAddrs)
					for _, addr := range bootstraps {
						addrsMap[addr.String()] = struct{}{}
					}

					store, err := New(db, bootstraps)
					Expect(err).ShouldNot(HaveOccurred())

					addrs := RandomAddressesWithoutDuplicates(100, bootstraps)
					Expect(err).ShouldNot(HaveOccurred())

					Expect(store.Insert(addrs)).Should(Succeed())
					Expect(store.Size()).Should(Equal(100 + numBoostrapAddrs))
					for _, addr := range addrs {
						stored, err := store.Address(addr.ID().String())
						Expect(err).ShouldNot(HaveOccurred())
						Expect(addr.String()).Should(Equal(stored.String()))
					}
				})

				It("should update the addresses when it's in store", func() {
					db := init(dbname)
					defer cleanup(db)

					numBoostrapAddrs := 10
					bootstraps := RandomAddresses(numBoostrapAddrs)
					store, err := New(db, bootstraps)
					Expect(err).ShouldNot(HaveOccurred())

					addrs := RandomAddressesWithoutDuplicates(100, bootstraps)
					Expect(err).ShouldNot(HaveOccurred())

					Expect(store.Insert(addrs)).Should(Succeed())
					Expect(store.Size()).Should(Equal(100 + numBoostrapAddrs))
					for _, addr := range addrs {
						stored, err := store.Address(addr.ID().String())
						Expect(err).ShouldNot(HaveOccurred())
						Expect(addr.String()).Should(Equal(stored.String()))
					}
				})

				It("should be able to return the bootstrap addresses", func() {
					db := init(dbname)
					defer cleanup(db)

					numBoostrapAddrs := 10
					bootstraps := RandomAddresses(numBoostrapAddrs)
					addrsMap := map[string]struct{}{}
					for _, addr := range bootstraps {
						addrsMap[addr.String()] = struct{}{}
					}

					store, err := New(db, bootstraps)
					Expect(err).ShouldNot(HaveOccurred())

					addrs := store.BootstrapAddresses()
					Expect(len(addrs)).Should(Equal(len(bootstraps)))
					for _, addr := range addrs {
						_, ok := addrsMap[addr.String()]
						Expect(ok).Should(BeTrue())
					}
				})

				It("should be able to batch insert when giving a nil address list", func() {
					db := init(dbname)
					defer cleanup(db)

					store, err := New(db, nil)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(store.Insert(nil)).Should(Succeed())
					Expect(store.Size()).Should(Equal(0))
				})

				It("should return the size of the db", func() {
					db := init(dbname)
					defer cleanup(db)

					numBoostrapAddrs := 10
					bootstraps := RandomAddresses(numBoostrapAddrs)
					store, err := New(db, bootstraps)
					Expect(err).ShouldNot(HaveOccurred())

					numAddrs := 100
					addrs := RandomAddressesWithoutDuplicates(numAddrs, bootstraps)
					Expect(err).ShouldNot(HaveOccurred())
					for _, address := range addrs {
						Expect(store.Insert([]addr.MultiAddress{address})).Should(Succeed())
					}
					Expect(store.Size()).Should(Equal(numBoostrapAddrs + numAddrs))
				})
			})

			Context("when retrieving random addresses from the store", func() {
				It("should return non-duplicate random addresses from the store", func() {
					db := init(dbname)
					defer cleanup(db)

					numBoostrapAddrs := 10
					addrsMap := map[string]struct{}{}
					bootstraps := RandomAddresses(numBoostrapAddrs)
					for _, addr := range bootstraps {
						addrsMap[addr.String()] = struct{}{}
					}
					store, err := New(db, bootstraps)
					Expect(err).ShouldNot(HaveOccurred())

					numAddrs := 100
					addrs := RandomAddressesWithoutDuplicates(numAddrs, bootstraps)
					Expect(store.Insert(addrs)).Should(Succeed())
					for _, addr := range addrs {
						addrsMap[addr.String()] = struct{}{}
					}

					randAddrs := store.RandomAddresses(20)
					Expect(len(randAddrs)).Should(Equal(20))

					for _, addr := range randAddrs {
						_, ok := addrsMap[addr.String()]
						Expect(ok).Should(BeTrue())
						delete(addrsMap, addr.String())
					}
				})

				It("should return non-duplicate random bootstrap addresses from the store", func() {
					db := init(dbname)
					defer cleanup(db)

					numBoostrapAddrs := 10
					addrsMap := map[string]struct{}{}
					bootstraps := RandomAddresses(numBoostrapAddrs)
					for _, addr := range bootstraps {
						addrsMap[addr.String()] = struct{}{}
					}
					store, err := New(db, bootstraps)
					Expect(err).ShouldNot(HaveOccurred())

					// when n is less than total number bootstrap addresses
					randAddrs := store.RandomBootstraps(5)
					Expect(len(randAddrs)).Should(Equal(5))
					for _, addr := range randAddrs {
						_, ok := addrsMap[addr.String()]
						Expect(ok).Should(BeTrue())
					}

					// when n is greater than total number bootstrap addresses
					randAddrs = store.RandomBootstraps(100)
					Expect(len(randAddrs)).Should(Equal(numBoostrapAddrs))
					for _, addr := range randAddrs {
						_, ok := addrsMap[addr.String()]
						Expect(ok).Should(BeTrue())
					}
				})
			})

			Context("when deleting addresses in the store", func() {
				Context("when deleting stored addresses", func() {
					It("should be able to delete address", func() {
						db := init(dbname)
						defer cleanup(db)

						store, err := New(db, nil)
						Expect(err).ShouldNot(HaveOccurred())

						addrs := RandomAddresses(100)
						Expect(err).ShouldNot(HaveOccurred())
						addrsMap := map[string]struct{}{}

						for _, address := range addrs {
							Expect(store.Insert([]addr.MultiAddress{address})).Should(Succeed())
							addrsMap[address.String()] = struct{}{}
							stored, err := store.Address(address.ID().String())
							Expect(err).ShouldNot(HaveOccurred())
							Expect(address.String()).Should(Equal(stored.String()))

							id := address.ID().String()
							Expect(store.Delete([]string{id})).Should(Succeed())
							stored, err = store.Address(address.ID().String())
							Expect(err).Should(HaveOccurred())
						}

						Expect(store.Size()).Should(BeZero())
					})
				})

				Context("when deleting empty addresses", func() {
					It("should do nothing", func() {
						db := init(dbname)
						defer cleanup(db)

						numBoostrapAddrs := 10
						bootstraps := RandomAddresses(numBoostrapAddrs)
						store, err := New(db, bootstraps)
						Expect(err).ShouldNot(HaveOccurred())

						Expect(store.Delete(nil)).Should(Succeed())
						Expect(store.Delete([]string{})).Should(Succeed())
					})
				})
			})
		})
	}
})

func RandomAddresses(n int) addr.MultiAddresses {
	addrs := make(addr.MultiAddresses, n)
	duplicate := map[string]struct{}{}
	for i := 0; i < n; i++ {
		var addr addr.MultiAddress
		ok := true
		for ok {
			addr = testutil.RandomMultiAddress()
			_, ok = duplicate[addr.ID().String()]
		}
		addrs[i] = addr
	}
	return addrs
}

func RandomAddressesWithoutDuplicates(n int, addrs addr.MultiAddresses) addr.MultiAddresses {
	multis := make(addr.MultiAddresses, n)
	duplicate := map[string]struct{}{}
	for _, addr := range addrs {
		duplicate[addr.ID().String()] = struct{}{}
	}
	for i := range multis {
		var addr addr.MultiAddress
		ok := true
		for ok {
			addr = testutil.RandomMultiAddress()
			_, ok = duplicate[addr.ID().String()]
		}
		multis[i] = addr
	}
	return multis
}

func CompareAddresses(addrs1, addrs2 addr.MultiAddresses) bool {
	if len(addrs1) != len(addrs2) {
		return false
	}
	for i := range addrs1 {
		if addrs1[i].String() != addrs2[i].String() {
			return false
		}
	}
	return true
}
