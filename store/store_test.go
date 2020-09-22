package store_test

import (
	"database/sql"
	"math/rand"
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

						size, err := store.Size()
						Expect(err).ShouldNot(HaveOccurred())
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

						size, err := store.Size()
						Expect(err).ShouldNot(HaveOccurred())
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

					for _, addr := range addrs {
						Expect(store.Insert(addr)).Should(Succeed())
						addrsMap[addr.String()] = struct{}{}
						stored, err := store.Address(addr.ID().String())
						Expect(err).ShouldNot(HaveOccurred())
						Expect(addr.String()).Should(Equal(stored.String()))
					}

					all, err := store.Addresses()
					Expect(err).ShouldNot(HaveOccurred())

					for _, addr := range all {
						_, ok := addrsMap[addr.String()]
						Expect(ok).Should(BeTrue())
						delete(addrsMap, addr.String())
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

				It("should be able to batch insert", func() {
					db := init(dbname)
					defer cleanup(db)

					store, err := New(db, nil)
					Expect(err).ShouldNot(HaveOccurred())

					addrs := RandomAddressesWithoutDuplicates(1000, nil)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(store.InsertAddresses(addrs)).Should(Succeed())

					size, err := store.Size()
					Expect(err).ShouldNot(HaveOccurred())
					Expect(size).Should(Equal(50))
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
					for _, addr := range addrs {
						Expect(store.Insert(addr)).Should(Succeed())
					}

					size, err := store.Size()
					Expect(err).ShouldNot(HaveOccurred())
					Expect(size).Should(Equal(numBoostrapAddrs + numAddrs))
				})

				It("should be able to delete address", func() {
					db := init(dbname)
					defer cleanup(db)

					store, err := New(db, nil)
					Expect(err).ShouldNot(HaveOccurred())

					addrs := RandomAddresses(100)
					Expect(err).ShouldNot(HaveOccurred())
					addrsMap := map[string]struct{}{}

					for _, addr := range addrs {
						Expect(store.Insert(addr)).Should(Succeed())
						addrsMap[addr.String()] = struct{}{}
						stored, err := store.Address(addr.ID().String())
						Expect(err).ShouldNot(HaveOccurred())
						Expect(addr.String()).Should(Equal(stored.String()))

						Expect(store.Delete(addr.ID().String())).Should(Succeed())
						stored, err = store.Address(addr.ID().String())
						Expect(err).Should(HaveOccurred())
					}

					size, err := store.Size()
					Expect(err).ShouldNot(HaveOccurred())
					Expect(size).Should(BeZero())
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
					Expect(err).ShouldNot(HaveOccurred())
					for _, addr := range addrs {
						Expect(store.Insert(addr)).Should(Succeed())
						addrsMap[addr.String()] = struct{}{}
					}

					randAddrs, err := store.RandomAddresses(20, false)
					Expect(err).ShouldNot(HaveOccurred())
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

					numAddrs := 100
					addrs := RandomAddressesWithoutDuplicates(numAddrs, bootstraps)
					Expect(err).ShouldNot(HaveOccurred())
					for _, addr := range addrs {
						Expect(store.Insert(addr)).Should(Succeed())
					}

					randAddrs, err := store.RandomAddresses(5, true)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(len(randAddrs)).Should(Equal(numBoostrapAddrs))

					for _, addr := range randAddrs {
						_, ok := addrsMap[addr.String()]
						Expect(ok).Should(BeTrue())
						delete(addrsMap, addr.String())
					}
				})
			})

			Context("when travelling through address in the store", func() {
				It("should return expected number of addresses in expected order", func() {
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
					Expect(err).ShouldNot(HaveOccurred())
					for _, addr := range addrs {
						Expect(store.Insert(addr)).Should(Succeed())
					}

					addrsAll := append(bootstraps, addrs...)

					index := 0
					for i := 0; i < 50; i++ {
						n := rand.Intn(20)
						addrs1, err := store.CycleThroughAddresses(n)
						Expect(err).ShouldNot(HaveOccurred())
						var addrs2 addr.MultiAddresses
						if index+n > len(addrsAll) {
							addrs2 = append(addrsAll[index:], addrsAll[:index][:index+n-len(addrsAll)]...)
						} else {
							addrs2 = addrsAll[index : index+n]
						}
						index = (index + n) % len(addrsAll)
						Expect(CompareAddresses(addrs1, addrs2)).Should(BeTrue())
					}
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
