package db_test

import (
	"database/sql"
	"os"
	"testing/quick"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/db"
	. "github.com/renproject/lightnode/testutils"

	"github.com/renproject/darknode/abi"
)

const (
	Sqlite        = "sqlite3"
	Postgres      = "postgres"
	TestTableName = "tx"
)

var _ = Describe("Lightnode db", func() {

	testDBs := []string{Sqlite, Postgres}

	initDB := func(name string) *sql.DB {
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
		return sqlDB
	}

	AfterSuite(func() {
		Expect(os.Remove("./test.db")).Should(BeNil())
	})

	for _, dbname := range testDBs {
		dbname := dbname
		Context(dbname, func() {
			Context("when creating the tx table", func() {
				It("should only create the tx if not exists", func() {
					sqlDB := initDB(dbname)
					defer sqlDB.Close()
					defer DropTable(sqlDB, "tx")
					db := New(sqlDB)

					// table should not exist before creation
					Expect(CheckTableExistence(dbname, "tx", sqlDB)).Should(HaveOccurred())

					// table should exist after creation
					Expect(db.CreateTxTable()).To(Succeed())
					Expect(CheckTableExistence(dbname, "tx", sqlDB)).NotTo(HaveOccurred())

					// Multiple call of the creation function should not have any effect on existing table.
					Expect(db.CreateTxTable()).To(Succeed())
					Expect(CheckTableExistence(dbname, "tx", sqlDB)).NotTo(HaveOccurred())

					Expect(db.CreateTxTable()).To(Succeed())
					Expect(CheckTableExistence(dbname, "tx", sqlDB)).NotTo(HaveOccurred())
				})
			})

			Context("when processing txs", func() {
				It("should be able to read and write tx", func() {
					sqlDB := initDB(dbname)
					defer sqlDB.Close()
					defer DropTable(sqlDB, "tx")
					db := New(sqlDB)
					Expect(db.CreateTxTable()).To(Succeed())

					test := func() bool {
						tx := RandomTx()
						Expect(db.InsertTx(tx)).To(Succeed())

						stored, err := db.Tx(tx.Hash)
						Expect(err).NotTo(HaveOccurred())
						Expect(db.DeleteTx(tx.Hash)).Should(Succeed())
						return CompareTx(tx, stored)
					}

					Expect(quick.Check(test, nil)).NotTo(HaveOccurred())
				})

				It("should be able to delete tx", func() {
					sqlDB := initDB(dbname)
					defer sqlDB.Close()
					defer DropTable(sqlDB, "tx")
					db := New(sqlDB)
					Expect(db.CreateTxTable()).To(Succeed())

					test := func() bool {
						// Insert a random tx
						tx := RandomTx()
						Expect(db.InsertTx(tx)).To(Succeed())

						// Expect db has on data entry
						before, err := NumOfDataEntries(sqlDB, TestTableName)
						Expect(err).NotTo(HaveOccurred())
						Expect(before).Should(Equal(1))

						// Delete the data and expect no data in the db
						Expect(db.DeleteTx(tx.Hash)).Should(Succeed())
						after, err := NumOfDataEntries(sqlDB, TestTableName)
						Expect(err).NotTo(HaveOccurred())
						Expect(after).Should(BeZero())

						return true
					}

					Expect(quick.Check(test, nil)).NotTo(HaveOccurred())
				})
			})

			Context("tx status", func() {
				It("should be able to get all pending txs", func() {
					sqlDB := initDB(dbname)
					defer sqlDB.Close()
					db := New(sqlDB)

					test := func() bool {
						Expect(db.CreateTxTable()).To(Succeed())
						defer DropTable(sqlDB, "tx")

						txs := map[abi.B32]abi.Tx{}
						for i := 0; i < 100; i++ {
							tx := RandomTx()
							txs[tx.Hash] = tx
							Expect(db.InsertTx(tx)).To(Succeed())
						}
						pendingTxs, err := db.PendingTxs()
						Expect(err).NotTo(HaveOccurred())

						Expect(len(pendingTxs)).Should(Equal(len(txs)))
						for _, tx := range pendingTxs {
							_, ok := txs[tx.Hash]
							Expect(ok).Should(BeTrue())
							delete(txs, tx.Hash)
						}
						return true
					}

					Expect(quick.Check(test, &quick.Config{MaxCount: 20})).NotTo(HaveOccurred())
				})

				It("should not return confirmed tx when asking for pending txs", func() {
					sqlDB := initDB(dbname)
					defer sqlDB.Close()
					db := New(sqlDB)

					test := func() bool {
						Expect(db.CreateTxTable()).To(Succeed())
						defer DropTable(sqlDB, "tx")

						tx := RandomTx()
						Expect(db.InsertTx(tx)).To(Succeed())
						Expect(db.ConfirmTx(tx.Hash)).Should(Succeed())

						pendingTxs, err := db.PendingTxs()
						Expect(err).NotTo(HaveOccurred())
						return len(pendingTxs) == 0
					}

					Expect(quick.Check(test, &quick.Config{MaxCount: 20})).NotTo(HaveOccurred())
				})

				It("should not return txs which added more than 24 hours ago.", func() {
					sqlDB := initDB(dbname)
					defer sqlDB.Close()
					db := New(sqlDB)

					test := func() bool {
						Expect(db.CreateTxTable()).To(Succeed())
						defer DropTable(sqlDB, "tx")

						// Insert a tx and update the created_time
						tx := RandomTx()
						Expect(db.InsertTx(tx)).To(Succeed())
						Expect(UpdateTxCreatedTime(sqlDB, tx.Hash)).Should(Succeed())
						expired, err := db.Expired(tx.Hash)
						Expect(err).NotTo(HaveOccurred())
						Expect(expired).Should(BeTrue())

						pendingTxs, err := db.PendingTxs()
						Expect(err).NotTo(HaveOccurred())
						return len(pendingTxs) == 0
					}

					Expect(quick.Check(test, &quick.Config{MaxCount: 20})).NotTo(HaveOccurred())
				})
			})
		})
	}
})
