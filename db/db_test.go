package db_test

/* import (
	"database/sql"
	"fmt"
	"os"
	"testing/quick"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/db"
	. "github.com/renproject/lightnode/testutils"

	"github.com/renproject/darknode/abi"
	"github.com/renproject/darknode/testutil"
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

	cleanup := func(db *sql.DB) {
		shiftin := "DROP TABLE IF EXISTS shiftin;"
		_, err := db.Exec(shiftin)
		Expect(err).NotTo(HaveOccurred())

		shiftout := "DROP TABLE IF EXISTS shiftout;"
		_, err = db.Exec(shiftout)
		Expect(err).NotTo(HaveOccurred())

		Expect(db.Close()).Should(Succeed())
	}

	dropTables := func(db *sql.DB, names ...string) {
		for _, name := range names {
			script := fmt.Sprintf("DROP TABLE %v;", name)
			_, err := db.Exec(script)
			Expect(err).NotTo(HaveOccurred())
		}
	}

	untransform := func(tx abi.Tx) abi.Tx {
		if abi.IsShiftIn(tx.To) {
			tx.Autogen = nil
		} else {
			tx.In = abi.Args{tx.In.Get("ref")}
		}
		return tx
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
			Context("when initializing the db", func() {
				It("should create tables for both shiftIn and shiftOut if not exist", func() {
					sqlDB := init(dbname)
					defer cleanup(sqlDB)
					db := New(sqlDB)

					// table should not exist before creation
					Expect(CheckTableExistence(dbname, "shift_in", sqlDB)).Should(HaveOccurred())
					Expect(CheckTableExistence(dbname, "shift_out", sqlDB)).Should(HaveOccurred())

					// table should exist after creation
					Expect(db.Init()).To(Succeed())
					Expect(CheckTableExistence(dbname, "shift_in", sqlDB)).NotTo(HaveOccurred())
					Expect(CheckTableExistence(dbname, "shift_out", sqlDB)).NotTo(HaveOccurred())

					// Multiple call of the creation function should not have any effect on existing table.
					Expect(db.Init()).To(Succeed())
					Expect(CheckTableExistence(dbname, "shift_in", sqlDB)).NotTo(HaveOccurred())
					Expect(CheckTableExistence(dbname, "shift_out", sqlDB)).NotTo(HaveOccurred())
				})
			})

			Context("when interacting with db", func() {
				It("should be able to read and write tx", func() {
					sqlDB := init(dbname)
					defer cleanup(sqlDB)
					db := New(sqlDB)

					test := func() bool {
						Expect(db.Init()).Should(Succeed())
						defer dropTables(sqlDB, "shift_in_autogen", "shift_in", "shift_out")
						tx := testutil.RandomTransformedTx()
						Expect(db.InsertTx(tx, abi.B32{}, true)).Should(Succeed())
						_tx, err := db.Tx(tx.Hash, true)
						Expect(err).NotTo(HaveOccurred())
						Expect(tx).Should(Equal(_tx))
						_tx, err = db.Tx(tx.Hash, false)
						Expect(err).NotTo(HaveOccurred())
						Expect(untransform(tx)).Should(Equal(_tx))
						return true
					}

					Expect(quick.Check(test, nil)).NotTo(HaveOccurred())
				})
			})

			Context("when querying pending tx", func() {
				It("should return all txs which are not confirmed", func() {
					sqlDB := init(dbname)
					defer cleanup(sqlDB)
					db := New(sqlDB)

					test := func() bool {
						Expect(db.Init()).Should(Succeed())
						defer dropTables(sqlDB, "shift_in_autogen", "shift_in", "shift_out")

						txs := map[abi.B32]abi.Tx{}
						for i := 0; i < 50; i++ {
							tx := testutil.RandomTransformedTx()
							txs[tx.Hash] = tx
							Expect(db.InsertTx(tx, abi.B32{}, true)).To(Succeed())
						}

						pendingTxs, err := db.PendingTxs(time.Hour)
						Expect(err).NotTo(HaveOccurred())
						Expect(len(pendingTxs)).Should(Equal(len(txs)))
						for _, tx := range pendingTxs {
							originTx, ok := txs[tx.Hash]
							Expect(ok).Should(BeTrue())
							Expect(untransform(originTx)).Should(Equal(tx))
							delete(txs, tx.Hash)
						}

						return len(txs) == 0
					}

					Expect(quick.Check(test, &quick.Config{MaxCount: 10})).NotTo(HaveOccurred())
				})

				It("should not return txs which added more than 24 hours ago", func() {
					sqlDB := init(dbname)
					defer sqlDB.Close()
					db := New(sqlDB)

					test := func() bool {
						Expect(db.Init()).Should(Succeed())
						defer dropTables(sqlDB, "shift_in_autogen", "shift_in", "shift_out")

						for i := 0; i < 50; i++ {
							tx := testutil.RandomTransformedTx()
							Expect(db.InsertTx(tx, abi.B32{}, true)).To(Succeed())
							Expect(UpdateTxCreatedTime(sqlDB, "shift_in", tx.Hash, time.Now().Unix()-24*3600)).Should(Succeed())
							Expect(UpdateTxCreatedTime(sqlDB, "shift_out", tx.Hash, time.Now().Unix()-24*3600)).Should(Succeed())
						}
						pendingTxs, err := db.PendingTxs(time.Hour)
						Expect(err).NotTo(HaveOccurred())
						return len(pendingTxs) == 0
					}

					Expect(quick.Check(test, &quick.Config{MaxCount: 10})).NotTo(HaveOccurred())
				})
			})

			Context("when querying unsubmitted txs", func() {
				It("should only return confirmed txs with payload", func() {
					sqlDB := init(dbname)
					defer cleanup(sqlDB)
					db := New(sqlDB)

					test := func() bool {
						Expect(db.Init()).Should(Succeed())
						defer dropTables(sqlDB, "shift_in_autogen", "shift_in", "shift_out")

						txs := map[abi.B32]abi.Tx{}
						for i := 0; i < 50; i++ {
							tx := testutil.RandomTransformedMintingTx("")
							Expect(db.InsertTx(tx, abi.B32{}, true)).To(Succeed())

							p := tx.In.Get("p")
							if !p.IsNil() {
								txs[tx.Hash] = tx
								Expect(db.UpdateStatus(tx.Hash, TxStatusConfirmed)).Should(Succeed())
							}
						}
						unsubmitted, err := db.UnsubmittedTxs(time.Hour)
						Expect(err).NotTo(HaveOccurred())
						for _, hash := range unsubmitted {
							_, ok := txs[hash]
							Expect(ok).Should(BeTrue())
							delete(txs, hash)
						}

						return len(txs) == 0
					}

					Expect(quick.Check(test, &quick.Config{MaxCount: 10})).NotTo(HaveOccurred())
				})
			})

			Context("when querying shiftIns with given status", func() {
				It("should only return txs with required status", func() {
					sqlDB := init(dbname)
					defer cleanup(sqlDB)
					db := New(sqlDB)

					test := func() bool {
						Expect(db.Init()).Should(Succeed())
						defer dropTables(sqlDB, "shift_in_autogen", "shift_in", "shift_out")

						txs := map[abi.B32]abi.Tx{}
						for i := 0; i < 50; i++ {
							tx := testutil.RandomTransformedMintingTx("")
							Expect(db.InsertTx(tx, abi.B32{}, true)).To(Succeed())
							txs[tx.Hash] = tx
							Expect(db.UpdateStatus(tx.Hash, TxStatusConfirmed)).Should(Succeed())
						}
						shiftIns, err := db.ShiftIns(TxStatusConfirmed, time.Hour, "")
						Expect(err).NotTo(HaveOccurred())
						for _, tx := range shiftIns {
							stored, ok := txs[tx.Hash]
							Expect(ok).Should(BeTrue())
							Expect(stored).Should(Equal(tx))
							delete(txs, tx.Hash)
						}

						return len(txs) == 0
					}

					Expect(quick.Check(test, &quick.Config{MaxCount: 10})).NotTo(HaveOccurred())
				})
			})

			Context("when updating tx status", func() {
				It("should returned the latest status of the tx", func() {
					sqlDB := init(dbname)
					defer cleanup(sqlDB)
					db := New(sqlDB)

					test := func() bool {
						Expect(db.Init()).Should(Succeed())
						defer dropTables(sqlDB, "shift_in_autogen", "shift_in", "shift_out")

						txs := map[abi.B32]abi.Tx{}
						for i := 0; i < 50; i++ {
							tx := testutil.RandomTransformedTx()
							txs[tx.Hash] = tx
							Expect(db.InsertTx(tx, abi.B32{}, true)).To(Succeed())
							Expect(db.UpdateStatus(tx.Hash, TxStatusConfirmed)).To(Succeed())

							status, err := db.TxStatus(tx.Hash)
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
					defer sqlDB.Close()
					db := New(sqlDB)

					test := func() bool {
						Expect(db.Init()).Should(Succeed())
						defer dropTables(sqlDB, "shift_in_autogen", "shift_in", "shift_out")

						shiftIn := testutil.RandomTransformedMintingTx("")
						shiftOut := testutil.RandomTransformedBurningTx("")
						Expect(db.InsertTx(shiftIn, abi.B32{}, true)).To(Succeed())
						Expect(db.InsertTx(shiftOut, abi.B32{}, true)).To(Succeed())

						// Expect no data gets pruned when they are not expired
						Expect(db.Prune(5 * time.Second)).Should(Succeed())
						numShiftIn, err := NumOfDataEntries(sqlDB, "shift_in")
						Expect(err).NotTo(HaveOccurred())
						Expect(numShiftIn).Should(Equal(1))
						numShiftOut, err := NumOfDataEntries(sqlDB, "shift_out")
						Expect(err).NotTo(HaveOccurred())
						Expect(numShiftOut).Should(Equal(1))

						// Expect data gets prunes when they are expired
						Expect(UpdateTxCreatedTime(sqlDB, "shift_in", shiftIn.Hash, time.Now().Unix()-5)).Should(Succeed())
						Expect(UpdateTxCreatedTime(sqlDB, "shift_out", shiftOut.Hash, time.Now().Unix()-5)).Should(Succeed())
						Expect(db.Prune(time.Second)).Should(Succeed())
						numShiftIn, err = NumOfDataEntries(sqlDB, "shift_in")
						Expect(err).NotTo(HaveOccurred())
						Expect(numShiftIn).Should(BeZero())
						numShiftInAutogen, err := NumOfDataEntries(sqlDB, "shift_in_autogen")
						Expect(err).NotTo(HaveOccurred())
						Expect(numShiftInAutogen).Should(BeZero())
						numShiftOut, err = NumOfDataEntries(sqlDB, "shift_out")
						Expect(err).NotTo(HaveOccurred())
						Expect(numShiftOut).Should(BeZero())

						return true
					}

					Expect(quick.Check(test, &quick.Config{MaxCount: 10})).NotTo(HaveOccurred())
				})
			})

			Context("when querying txs with a given tag", func() {
				It("should return the correct txs", func() {
					sqlDB := init(dbname)
					defer cleanup(sqlDB)
					db := New(sqlDB)

					test := func(tag abi.B32) bool {
						Expect(db.Init()).Should(Succeed())
						defer dropTables(sqlDB, "shift_in_autogen", "shift_in", "shift_out")

						pageSize := 2
						firstPage := map[abi.B32]abi.Tx{}
						secondPage := map[abi.B32]abi.Tx{}
						for i := 0; i < 2*pageSize; i++ {
							tx := testutil.RandomTransformedMintingTx("")
							Expect(db.InsertTx(tx, tag, true)).To(Succeed())

							if i < pageSize {
								firstPage[tx.Hash] = tx
							} else {
								secondPage[tx.Hash] = tx
							}

							// Sleep to ensure the timestamps are different.
							time.Sleep(time.Second)
						}

						// Validate the first page.
						matchingTxs, err := db.Txs(tag, 0, uint64(pageSize))
						Expect(err).NotTo(HaveOccurred())
						Expect(len(matchingTxs)).Should(Equal(pageSize))

						for _, tx := range matchingTxs {
							stored, ok := firstPage[tx.Hash]
							Expect(ok).Should(BeTrue())
							Expect(untransform(stored)).Should(Equal(tx))
							delete(firstPage, tx.Hash)
						}

						Expect(len(firstPage)).To(Equal(0))

						// Validate the second page.
						matchingTxs, err = db.Txs(tag, 1, uint64(pageSize))
						Expect(err).NotTo(HaveOccurred())
						Expect(len(matchingTxs)).Should(Equal(pageSize))

						for _, tx := range matchingTxs {
							stored, ok := secondPage[tx.Hash]
							Expect(ok).Should(BeTrue())
							Expect(untransform(stored)).Should(Equal(tx))
							delete(secondPage, tx.Hash)
						}

						Expect(len(secondPage)).To(Equal(0))

						// Validate the third page.
						matchingTxs, err = db.Txs(tag, 2, uint64(pageSize))
						Expect(err).NotTo(HaveOccurred())
						Expect(len(matchingTxs)).Should(Equal(0))

						return true
					}

					Expect(quick.Check(test, &quick.Config{MaxCount: 3})).NotTo(HaveOccurred())
				})
			})

			Context("when querying txs with a non-existent tag", func() {
				It("should return no txs", func() {
					sqlDB := init(dbname)
					defer cleanup(sqlDB)
					db := New(sqlDB)

					test := func(tag abi.B32) bool {
						Expect(db.Init()).Should(Succeed())
						defer dropTables(sqlDB, "shift_in_autogen", "shift_in", "shift_out")

						// Validate the third page.
						matchingTxs, err := db.Txs(tag, 0, 10)
						Expect(err).NotTo(HaveOccurred())

						return len(matchingTxs) == 0
					}

					Expect(quick.Check(test, &quick.Config{MaxCount: 10})).NotTo(HaveOccurred())
				})
			})
		})
	}
}) */
