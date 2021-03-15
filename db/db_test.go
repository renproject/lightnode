package db_test

import (
	"database/sql"
	"math/rand"
	"os"
	"testing/quick"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/renproject/id"
	. "github.com/renproject/lightnode/db"
	. "github.com/renproject/lightnode/testutils"

	"github.com/renproject/darknode/tx"
	"github.com/renproject/darknode/tx/txutil"
)

const (
	Sqlite   = "sqlite3"
	Postgres = "postgres"
)

var _ = Describe("Lightnode db", func() {

	testDBs := []string{Sqlite, Postgres}

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

	close := func(db *sql.DB) {
		Expect(db.Close()).Should(Succeed())
	}

	cleanUp := func(db *sql.DB) {
		dropTxs := "DROP TABLE IF EXISTS txs;"
		_, err := db.Exec(dropTxs)
		Expect(err).NotTo(HaveOccurred())
	}

	destroy := func(db *sql.DB) {
		cleanUp(db)
		close(db)
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
			Context("when initialising the db", func() {
				It("should create tables if they do not exist", func() {
					sqlDB := init(dbname)
					defer destroy(sqlDB)
					db := New(sqlDB)

					// Tables should not exist before creation.
					Expect(CheckTableExistence(dbname, "txs", sqlDB)).Should(HaveOccurred())

					// Tables should exist after creation.
					Expect(db.Init()).To(Succeed())
					Expect(CheckTableExistence(dbname, "txs", sqlDB)).NotTo(HaveOccurred())

					// Multiple calls of the creation function should not have
					// any effect on the existing tables.
					Expect(db.Init()).To(Succeed())
					Expect(CheckTableExistence(dbname, "txs", sqlDB)).NotTo(HaveOccurred())
				})
			})

			Context("when interacting with db", func() {
				It("should be able to read and write tx", func() {
					sqlDB := init(dbname)
					defer close(sqlDB)
					db := New(sqlDB)

					r := rand.New(rand.NewSource(GinkgoRandomSeed()))
					test := func() bool {
						Expect(db.Init()).Should(Succeed())
						defer cleanUp(sqlDB)
						transaction := txutil.RandomGoodTx(r)
						transaction.Output = nil
						Expect(db.InsertTx(transaction)).Should(Succeed())
						newTransaction, err := db.Tx(transaction.Hash)
						Expect(err).NotTo(HaveOccurred())
						Expect(transaction).Should(Equal(newTransaction))
						return true
					}

					Expect(quick.Check(test, nil)).NotTo(HaveOccurred())
				})
			})

			Context("when querying txs", func() {
				It("should return a page of txs", func() {
					sqlDB := init(dbname)
					defer close(sqlDB)
					db := New(sqlDB)

					r := rand.New(rand.NewSource(GinkgoRandomSeed()))
					test := func() bool {
						Expect(db.Init()).Should(Succeed())
						defer cleanUp(sqlDB)

						txs := map[id.Hash]tx.Tx{}
						for i := 0; i < 50; i++ {
							transaction := txutil.RandomGoodTx(r)
							transaction.Output = nil
							v := r.Intn(2)
							if v == 0 {
								transaction.Version = tx.Version0
							}
							txs[transaction.Hash] = transaction

							Expect(db.InsertTx(transaction)).To(Succeed())
						}

						txsPage, err := db.Txs(0, 10)
						Expect(err).NotTo(HaveOccurred())
						Expect(len(txsPage)).Should(Equal(10))
						for _, tx := range txsPage {
							originTx, ok := txs[tx.Hash]
							Expect(ok).Should(BeTrue())
							Expect(originTx).Should(Equal(tx))
							delete(txs, tx.Hash)
						}

						Expect(txs).To(HaveLen(40))
						return true
					}

					Expect(quick.Check(test, &quick.Config{MaxCount: 10})).NotTo(HaveOccurred())
				})
			})

			Context("when querying pending tx", func() {
				It("should return all txs which are not confirmed", func() {
					sqlDB := init(dbname)
					defer close(sqlDB)
					db := New(sqlDB)

					r := rand.New(rand.NewSource(GinkgoRandomSeed()))
					test := func() bool {
						Expect(db.Init()).Should(Succeed())
						defer cleanUp(sqlDB)

						txs := map[id.Hash]tx.Tx{}
						for i := 0; i < 50; i++ {
							transaction := txutil.RandomGoodTx(r)
							transaction.Output = nil
							transaction.Version.Generate(r, 2)
							txs[transaction.Hash] = transaction
							Expect(db.InsertTx(transaction)).To(Succeed())
						}

						pendingTxs, err := db.PendingTxs(time.Hour)
						Expect(err).NotTo(HaveOccurred())
						Expect(len(pendingTxs)).Should(Equal(len(txs)))
						for _, tx := range pendingTxs {
							originTx, ok := txs[tx.Hash]
							Expect(ok).Should(BeTrue())
							Expect(originTx).Should(Equal(tx))
							delete(txs, tx.Hash)
						}

						Expect(txs).To(HaveLen(0))
						return true
					}

					Expect(quick.Check(test, &quick.Config{MaxCount: 10})).NotTo(HaveOccurred())
				})

				It("should not return txs which added more than 24 hours ago", func() {
					sqlDB := init(dbname)
					defer close(sqlDB)
					db := New(sqlDB)

					r := rand.New(rand.NewSource(GinkgoRandomSeed()))
					test := func() bool {
						Expect(db.Init()).Should(Succeed())
						defer cleanUp(sqlDB)

						for i := 0; i < 50; i++ {
							transaction := txutil.RandomGoodTx(r)
							Expect(db.InsertTx(transaction)).To(Succeed())
							Expect(UpdateTxCreatedTime(sqlDB, "txs", transaction.Hash, time.Now().Unix()-24*3600)).Should(Succeed())
						}
						pendingTxs, err := db.PendingTxs(time.Hour)
						Expect(err).NotTo(HaveOccurred())
						Expect(pendingTxs).To(HaveLen(0))
						return true
					}

					Expect(quick.Check(test, &quick.Config{MaxCount: 10})).NotTo(HaveOccurred())
				})
			})

			Context("when updating tx status", func() {
				It("should returned the latest status of the tx", func() {
					sqlDB := init(dbname)
					defer close(sqlDB)
					db := New(sqlDB)

					r := rand.New(rand.NewSource(GinkgoRandomSeed()))
					test := func() bool {
						Expect(db.Init()).Should(Succeed())
						defer cleanUp(sqlDB)

						txs := map[id.Hash]tx.Tx{}
						for i := 0; i < 50; i++ {
							transaction := txutil.RandomGoodTx(r)
							txs[transaction.Hash] = transaction
							Expect(db.InsertTx(transaction)).To(Succeed())
							Expect(db.UpdateStatus(transaction.Hash, TxStatusConfirmed)).To(Succeed())

							status, err := db.TxStatus(transaction.Hash)
							Expect(err).NotTo(HaveOccurred())
							Expect(status).Should(Equal(TxStatusConfirmed))
						}

						return true
					}

					Expect(quick.Check(test, &quick.Config{MaxCount: 10})).NotTo(HaveOccurred())
				})
			})

			Context("when pruning the db", func() {
				It("should only prune data which is expired", func() {
					sqlDB := init(dbname)
					defer close(sqlDB)
					db := New(sqlDB)

					r := rand.New(rand.NewSource(GinkgoRandomSeed()))
					test := func() bool {
						Expect(db.Init()).Should(Succeed())
						defer cleanUp(sqlDB)

						transaction := txutil.RandomGoodTx(r)
						Expect(db.InsertTx(transaction)).To(Succeed())

						// Ensure no data gets pruned before it is expired.
						Expect(db.Prune(5 * time.Second)).Should(Succeed())
						numTxs, err := NumOfDataEntries(sqlDB, "txs")
						Expect(err).NotTo(HaveOccurred())
						Expect(numTxs).Should(Equal(1))

						// Ensure data gets pruned once it has expired.
						Expect(UpdateTxCreatedTime(sqlDB, "txs", transaction.Hash, time.Now().Unix()-5)).Should(Succeed())
						Expect(db.Prune(time.Second)).Should(Succeed())
						numTxs, err = NumOfDataEntries(sqlDB, "txs")
						Expect(err).NotTo(HaveOccurred())
						Expect(numTxs).Should(BeZero())

						return true
					}

					Expect(quick.Check(test, &quick.Config{MaxCount: 10})).NotTo(HaveOccurred())
				})
			})
		})
	}
})
