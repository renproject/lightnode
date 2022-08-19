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
	. "github.com/renproject/lightnode/db"
	. "github.com/renproject/lightnode/testutils"

	"github.com/renproject/darknode/tx"
	"github.com/renproject/darknode/tx/txutil"
	"github.com/renproject/id"
	"github.com/renproject/pack"
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
		dropTxs := "DROP TABLE IF EXISTS txs; DROP TABLE IF EXISTS gateways;"
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
					Expect(CheckTableExistence(dbname, "gateways", sqlDB)).Should(HaveOccurred())

					// Tables should exist after creation.
					Expect(db.Init()).To(Succeed())
					Expect(CheckTableExistence(dbname, "txs", sqlDB)).NotTo(HaveOccurred())
					Expect(CheckTableExistence(dbname, "gateways", sqlDB)).NotTo(HaveOccurred())

					// Multiple calls of the creation function should not have
					// any effect on the existing tables.
					Expect(db.Init()).To(Succeed())
					Expect(CheckTableExistence(dbname, "txs", sqlDB)).NotTo(HaveOccurred())
					Expect(CheckTableExistence(dbname, "gateways", sqlDB)).NotTo(HaveOccurred())
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

				It("should be able to read and write gateways", func() {
					sqlDB := init(dbname)
					defer close(sqlDB)
					db := New(sqlDB)

					r := rand.New(rand.NewSource(GinkgoRandomSeed()))
					test := func() bool {
						Expect(db.Init()).Should(Succeed())
						defer cleanUp(sqlDB)
						transaction := txutil.RandomGoodTx(r)
						transaction.Output = nil
						gatewayAddress := "address"
						Expect(db.InsertGateway(gatewayAddress, transaction)).Should(Succeed())
						newTransaction, err := db.Gateway(gatewayAddress)
						Expect(err).NotTo(HaveOccurred())
						// We only store a partial tx
						Expect(transaction).ShouldNot(Equal(newTransaction))
						// Selector should still match
						Expect(newTransaction.Selector).Should(Equal(transaction.Selector))
						return true
					}

					Expect(quick.Check(test, nil)).NotTo(HaveOccurred())
				})

				It("should be able to write tx and query by txid", func() {
					sqlDB := init(dbname)
					defer close(sqlDB)
					db := New(sqlDB)

					r := rand.New(rand.NewSource(GinkgoRandomSeed()))
					test := func() bool {
						Expect(db.Init()).Should(Succeed())
						defer cleanUp(sqlDB)
						transaction := txutil.RandomGoodTx(r)
						transaction.Output = nil
						txid, ok := transaction.Input.Get("txid").(pack.Bytes)
						Expect(ok).To(Equal(true))
						Expect(db.InsertTx(transaction)).Should(Succeed())
						newTransaction, err := db.TxsByTxid(txid)
						Expect(err).NotTo(HaveOccurred())
						Expect(transaction).Should(Equal(newTransaction[0]))
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
					test := func(order bool) bool {
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

						txsPage, err := db.Txs(0, 10, order)
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

			Context("when querying txs by status", func() {
				It("should return all txs with the given status", func() {
					sqlDB := init(dbname)
					defer close(sqlDB)
					db := New(sqlDB)

					r := rand.New(rand.NewSource(GinkgoRandomSeed()))
					test := func() bool {
						Expect(db.Init()).Should(Succeed())
						defer cleanUp(sqlDB)

						pendingMap := map[id.Hash]tx.Tx{}
						confirmedMap := map[id.Hash]tx.Tx{}
						uncofirmedMap := map[id.Hash]tx.Tx{}
						for i := 0; i < 50; i++ {
							transaction := txutil.RandomGoodTx(r)
							transaction.Output = nil
							transaction.Version.Generate(r, 2)
							pendingMap[transaction.Hash] = transaction
							confirmedMap[transaction.Hash] = transaction
							uncofirmedMap[transaction.Hash] = transaction
							Expect(db.InsertTx(transaction)).To(Succeed())
						}

						pendingTxs, err := db.TxsByStatus(TxStatusConfirming, 0, 0, 0)
						Expect(err).NotTo(HaveOccurred())
						Expect(len(pendingTxs)).Should(Equal(len(pendingMap)))
						for _, tx := range pendingTxs {
							originTx, ok := pendingMap[tx.Hash]
							Expect(ok).Should(BeTrue())
							Expect(originTx).Should(Equal(tx))
							delete(pendingMap, tx.Hash)
						}
						Expect(pendingMap).To(HaveLen(0))

						// Update tx to be confirmed
						for hash := range confirmedMap {
							err = db.UpdateStatus(hash, TxStatusConfirmed)
							Expect(err).NotTo(HaveOccurred())
						}
						confirmedTxs, err := db.TxsByStatus(TxStatusConfirmed, 0, 0, 0)
						Expect(err).NotTo(HaveOccurred())
						Expect(len(confirmedTxs)).Should(Equal(len(confirmedMap)))
						for _, tx := range confirmedTxs {
							originTx, ok := confirmedMap[tx.Hash]
							Expect(ok).Should(BeTrue())
							Expect(originTx).Should(Equal(tx))
							delete(confirmedMap, tx.Hash)
						}
						Expect(confirmedMap).To(HaveLen(0))

						// Update tx to be unconfirmed
						for hash := range uncofirmedMap {
							err = db.UpdateStatus(hash, TxStatusUnconfirmed)
							Expect(err).NotTo(HaveOccurred())
						}
						unconfirmedTxs, err := db.TxsByStatus(TxStatusUnconfirmed, 0, 0, 0)
						Expect(err).NotTo(HaveOccurred())
						Expect(len(unconfirmedTxs)).Should(Equal(len(uncofirmedMap)))
						for _, tx := range unconfirmedTxs {
							originTx, ok := uncofirmedMap[tx.Hash]
							Expect(ok).Should(BeTrue())
							Expect(originTx).Should(Equal(tx))
							delete(uncofirmedMap, tx.Hash)
						}
						Expect(uncofirmedMap).To(HaveLen(0))

						return true
					}

					Expect(quick.Check(test, &quick.Config{MaxCount: 10})).NotTo(HaveOccurred())
				})

				It("should only return txs within a certain time", func() {
					sqlDB := init(dbname)
					defer close(sqlDB)
					db := New(sqlDB)

					r := rand.New(rand.NewSource(GinkgoRandomSeed()))
					test := func() bool {
						Expect(db.Init()).Should(Succeed())
						defer cleanUp(sqlDB)

						expectedTxs := map[id.Hash]tx.Tx{}
						for i := 0; i < 100; i++ {
							if i < 50 {
								transaction := txutil.RandomGoodTx(r)
								Expect(db.InsertTx(transaction)).To(Succeed())
								Expect(UpdateTxCreatedTime(sqlDB, "txs", transaction.Hash, time.Now().Unix()-24*3600)).Should(Succeed())
							} else {
								transaction := txutil.RandomGoodTx(r)
								transaction.Output = nil
								Expect(db.InsertTx(transaction)).To(Succeed())
								expectedTxs[transaction.Hash] = transaction
							}
						}
						txs, err := db.TxsByStatus(TxStatusConfirming, time.Hour, 0, 0)
						Expect(err).NotTo(HaveOccurred())
						Expect(len(txs)).To(Equal(len(expectedTxs)))
						for _, tx := range txs {
							originTx, ok := expectedTxs[tx.Hash]
							Expect(ok).Should(BeTrue())
							originTx.Output = nil
							Expect(originTx).Should(Equal(tx))
							delete(expectedTxs, tx.Hash)
						}
						Expect(expectedTxs).To(HaveLen(0))
						return true
					}

					Expect(quick.Check(test, &quick.Config{MaxCount: 10})).NotTo(HaveOccurred())
				})

				It("should only return txs beyond a certain time", func() {
					sqlDB := init(dbname)
					defer close(sqlDB)
					db := New(sqlDB)

					r := rand.New(rand.NewSource(GinkgoRandomSeed()))
					test := func() bool {
						Expect(db.Init()).Should(Succeed())
						defer cleanUp(sqlDB)

						expectedTxs := map[id.Hash]tx.Tx{}
						for i := 0; i < 100; i++ {
							if i < 50 {
								transaction := txutil.RandomGoodTx(r)
								transaction.Output = nil
								Expect(db.InsertTx(transaction)).To(Succeed())
								Expect(UpdateTxCreatedTime(sqlDB, "txs", transaction.Hash, time.Now().Unix()-24*3600)).Should(Succeed())
								expectedTxs[transaction.Hash] = transaction
							} else {
								transaction := txutil.RandomGoodTx(r)
								Expect(db.InsertTx(transaction)).To(Succeed())
							}
						}
						txs, err := db.TxsByStatus(TxStatusConfirming, 0, time.Hour-1, 0)
						Expect(err).NotTo(HaveOccurred())
						Expect(len(txs)).To(Equal(len(expectedTxs)))
						for _, tx := range txs {
							originTx, ok := expectedTxs[tx.Hash]
							Expect(ok).Should(BeTrue())
							Expect(originTx).Should(Equal(tx))
							delete(expectedTxs, tx.Hash)
						}
						Expect(expectedTxs).To(HaveLen(0))
						return true
					}

					Expect(quick.Check(test, &quick.Config{MaxCount: 10})).NotTo(HaveOccurred())
				})

				It("should not return more txs than the limit", func() {
					sqlDB := init(dbname)
					defer close(sqlDB)
					db := New(sqlDB)

					r := rand.New(rand.NewSource(GinkgoRandomSeed()))
					test := func() bool {
						Expect(db.Init()).Should(Succeed())
						defer cleanUp(sqlDB)

						numTxs := 50
						for i := 0; i < numTxs; i++ {
							transaction := txutil.RandomGoodTx(r)
							Expect(db.InsertTx(transaction)).To(Succeed())
						}

						for i := 1; i <= 100; i++ {
							txs, err := db.TxsByStatus(TxStatusConfirming, 0, 0, i)
							Expect(err).NotTo(HaveOccurred())

							if i > numTxs {
								Expect(len(txs)).To(Equal(numTxs))
							} else {
								Expect(len(txs)).To(Equal(i))
							}
						}

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

							Expect(db.UpdateStatus(transaction.Hash, TxStatusUnconfirmed)).To(Succeed())
							status, err = db.TxStatus(transaction.Hash)
							Expect(err).NotTo(HaveOccurred())
							Expect(status).Should(Equal(TxStatusUnconfirmed))
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
						gatewayAddress := "address"
						Expect(db.InsertGateway(gatewayAddress, transaction)).Should(Succeed())

						// Ensure no data gets pruned before it is expired.
						Expect(db.Prune(5 * time.Second)).Should(Succeed())
						numTxs, err := NumOfDataEntries(sqlDB, "txs")
						Expect(err).NotTo(HaveOccurred())
						Expect(numTxs).Should(Equal(1))
						numGateways, err := NumOfDataEntries(sqlDB, "gateways")
						Expect(err).NotTo(HaveOccurred())
						Expect(numGateways).Should(Equal(1))

						// Ensure data gets pruned once it has expired.
						Expect(UpdateTxCreatedTime(sqlDB, "txs", transaction.Hash, time.Now().Unix()-5)).Should(Succeed())
						Expect(UpdateGatewayCreatedTime(sqlDB, gatewayAddress, time.Now().Unix()-5)).Should(Succeed())
						Expect(db.Prune(time.Second)).Should(Succeed())
						numTxs, err = NumOfDataEntries(sqlDB, "txs")
						Expect(err).NotTo(HaveOccurred())
						Expect(numTxs).Should(BeZero())
						numGateways, err = NumOfDataEntries(sqlDB, "gateways")
						Expect(err).NotTo(HaveOccurred())
						Expect(numGateways).Should(BeZero())

						return true
					}

					Expect(quick.Check(test, &quick.Config{MaxCount: 10})).NotTo(HaveOccurred())
				})
			})
		})
	}
})
