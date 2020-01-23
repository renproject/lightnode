package db_test

import (
	"database/sql"
	"fmt"
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
		return sqlDB
	}

	cleanup := func(db *sql.DB) {
		shiftin := "DROP TABLE shiftin"
		_, err := db.Exec(shiftin)
		Expect(err).NotTo(HaveOccurred())

		shiftout := "DROP TABLE shiftout"
		_, err = db.Exec(shiftout)
		Expect(err).NotTo(HaveOccurred())

		Expect(db.Close()).Should(Succeed())
	}

	AfterSuite(func() {
		Expect(os.Remove("./test.db")).Should(BeNil())
	})

	for _, dbname := range testDBs {
		dbname := dbname
		Context(dbname, func() {
			Context("when initializing the db", func() {
				It("should create tables for both shiftIn and shiftOut if not exist", func() {
					sqlDB := init(dbname)
					defer cleanup(sqlDB)
					db := New(sqlDB)

					// table should not exist before creation
					Expect(CheckTableExistence(dbname, "shiftin", sqlDB)).Should(HaveOccurred())
					Expect(CheckTableExistence(dbname, "shiftout", sqlDB)).Should(HaveOccurred())

					// table should exist after creation
					Expect(db.Init()).To(Succeed())
					Expect(CheckTableExistence(dbname, "shiftin", sqlDB)).NotTo(HaveOccurred())
					Expect(CheckTableExistence(dbname, "shiftout", sqlDB)).NotTo(HaveOccurred())

					// Multiple call of the creation function should not have any effect on existing table.
					Expect(db.Init()).To(Succeed())
					Expect(CheckTableExistence(dbname, "tx", sqlDB)).NotTo(HaveOccurred())
				})
			})

			Context("when interacting with db", func() {
				It("should be able to read and write tx", func() {
					sqlDB := init(dbname)
					defer cleanup(sqlDB)
					db := New(sqlDB)

					test := func() bool {
						shiftIn := RandomShiftIn()
						shiftOut := RandomShiftOut()

						Expect(db.InsertShiftIn(shiftIn)).Should(Succeed())
						Expect(db.InsertShiftOut(shiftOut)).Should(Succeed())

						_shiftIn, err := db.ShiftIn(shiftIn.Hash)
						Expect(err).NotTo(HaveOccurred())
						_shiftOut, err := db.ShiftOut(shiftOut.Hash)
						Expect(err).NotTo(HaveOccurred())

						Expect(shiftIn).Should(Equal(_shiftIn))
						Expect(shiftOut).Should(Equal(_shiftOut))

						return true
					}

					Expect(quick.Check(test, nil)).NotTo(HaveOccurred())
				})

				It("should be able to get all pending txs", func() {

				})
			})

			Context("tx status", func() {
				It("should be able to get all pending txs", func() {
					sqlDB := init(dbname)
					defer sqlDB.Close()
					db := New(sqlDB)

					test := func() bool {
						Expect(db.Init()).To(Succeed())
						defer DropTable(sqlDB, "tx")

						txs := map[abi.B32]abi.Tx{}
						for i := 0; i < 100; i++ {
							tx := RandomShiftIn()
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
					sqlDB := init(dbname)
					defer sqlDB.Close()
					db := New(sqlDB)

					test := func() bool {
						Expect(db.Init()).To(Succeed())
						defer DropTable(sqlDB, "tx")

						tx := RandomShiftIn()
						Expect(db.InsertTx(tx)).To(Succeed())
						Expect(db.ConfirmTx(tx.Hash)).Should(Succeed())

						pendingTxs, err := db.PendingTxs()
						Expect(err).NotTo(HaveOccurred())
						return len(pendingTxs) == 0
					}

					Expect(quick.Check(test, &quick.Config{MaxCount: 20})).NotTo(HaveOccurred())
				})

				It("should not return txs which added more than 24 hours ago.", func() {
					sqlDB := init(dbname)
					defer sqlDB.Close()
					db := New(sqlDB)

					test := func() bool {
						Expect(db.Init()).To(Succeed())
						defer DropTable(sqlDB, "tx")

						// Insert a tx and update the created_time
						tx := RandomShiftIn()
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
