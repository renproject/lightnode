package db_test

import (
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

	testDBs := []string{Sqlite/*, Postgres*/}

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

	// compare takes two abi.Txs and compare whether they are the same tx. It
	// ignores some fields and only compares fields which are stored by DB.
	compare := func(l ,r abi.Tx){
		if abi.IsShiftIn(l.To) && abi.IsShiftIn(r.To){
			Expect(l.Hash).Should(Equal(r.Hash))
			Expect(l.To).Should(Equal(r.To))

			Expect(l.In.Get("p")).Should(Equal(r.In.Get("p")))
			Expect(l.In.Get("token")).Should(Equal(r.In.Get("token")))
			Expect(l.In.Get("to")).Should(Equal(r.In.Get("to")))
			Expect(l.In.Get("n")).Should(Equal(r.In.Get("n")))
			lUtxo := l.In.Get("utxo").Value.(abi.ExtBtcCompatUTXO)
			rUtxo := r.In.Get("utxo").Value.(abi.ExtBtcCompatUTXO)
			Expect(lUtxo.TxHash).Should(Equal(rUtxo.TxHash))
			Expect(lUtxo.VOut).Should(Equal(rUtxo.VOut))

			Expect(l.Autogen.Get("phash")).Should(Equal(r.Autogen.Get("phash")))
			Expect(l.Autogen.Get("ghash")).Should(Equal(r.Autogen.Get("ghash")))
			Expect(l.Autogen.Get("nhash")).Should(Equal(r.Autogen.Get("nhash")))
			Expect(l.Autogen.Get("sighash")).Should(Equal(r.Autogen.Get("sighash")))
		} else {
			Expect(l).Should(Equal(r))
		}
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
					Expect(CheckTableExistence(dbname, "shiftin", sqlDB)).Should(HaveOccurred())
					Expect(CheckTableExistence(dbname, "shiftout", sqlDB)).Should(HaveOccurred())

					// table should exist after creation
					Expect(db.Init()).To(Succeed())
					Expect(CheckTableExistence(dbname, "shiftin", sqlDB)).NotTo(HaveOccurred())
					Expect(CheckTableExistence(dbname, "shiftout", sqlDB)).NotTo(HaveOccurred())

					// Multiple call of the creation function should not have any effect on existing table.
					Expect(db.Init()).To(Succeed())
					Expect(CheckTableExistence(dbname, "shiftin", sqlDB)).NotTo(HaveOccurred())
					Expect(CheckTableExistence(dbname, "shiftout", sqlDB)).NotTo(HaveOccurred())
				})
			})

			Context("when interacting with db", func() {
				It("should be able to read and write tx", func() {
					sqlDB := init(dbname)
					defer cleanup(sqlDB)
					db := New(sqlDB)

					test := func() bool {
						Expect(db.Init()).Should(Succeed())
						defer dropTables(sqlDB, "shiftin", "shiftout")

						tx := testutil.RandomTransformedTx()
						Expect(db.InsertTx(tx)).Should(Succeed())
						_tx, err := db.Tx(tx.Hash)
						Expect(err).NotTo(HaveOccurred())
						compare(tx, _tx)
						return true
					}

					Expect(quick.Check(test, nil)).NotTo(HaveOccurred())
				})
			})

			Context("when querying txs with specific status", func() {
				It("should return all txs with required status", func() {
					sqlDB := init(dbname)
					defer cleanup(sqlDB)
					db := New(sqlDB)

					test := func() bool {
						Expect(db.Init()).Should(Succeed())
						defer dropTables(sqlDB, "shiftin", "shiftout")

						txs := map[abi.B32]abi.Tx{}
						for i := 0; i < 50; i++ {
							tx := testutil.RandomTransformedTx()
							txs[tx.Hash] = tx
							Expect(db.InsertTx(tx)).To(Succeed())
						}

						pendingTxs, err := db.TxsWithStatus(TxStatusConfirming, time.Hour, "")
						Expect(err).NotTo(HaveOccurred())
						Expect(len(pendingTxs)).Should(Equal(len(txs)))
						for _, tx := range pendingTxs {
							originTx, ok := txs[tx.Hash]
							Expect(ok).Should(BeTrue())
							compare(originTx, tx)
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
						defer dropTables(sqlDB, "shiftin", "shiftout")

						for i := 0; i < 50; i++ {
							tx := testutil.RandomTransformedTx()
							Expect(db.InsertTx(tx)).To(Succeed())
							Expect(UpdateTxCreatedTime(sqlDB, "shiftin", tx.Hash, time.Now().Unix()-24*3600)).Should(Succeed())
							Expect(UpdateTxCreatedTime(sqlDB, "shiftout", tx.Hash, time.Now().Unix()-24*3600)).Should(Succeed())
						}
						pendingTxs, err := db.TxsWithStatus(TxStatusConfirming, time.Hour, "")
						Expect(err).NotTo(HaveOccurred())
						return len(pendingTxs) == 0
					}

					Expect(quick.Check(test, &quick.Config{MaxCount: 10})).NotTo(HaveOccurred())
				})

				It("should only return specific txs when a contract address is given", func(){

				})
			})

			Context("when updating tx status", func(){
				It("should returned the latest status of the tx", func() {
					sqlDB := init(dbname)
					defer cleanup(sqlDB)
					db := New(sqlDB)

					test := func() bool {
						Expect(db.Init()).Should(Succeed())
						defer dropTables(sqlDB, "shiftin", "shiftout")

						txs := map[abi.B32]abi.Tx{}
						for i := 0; i < 50; i++ {
							tx := testutil.RandomTransformedTx()
							txs[tx.Hash] = tx
							Expect(db.InsertTx(tx)).To(Succeed())
							Expect(db.UpdateStatus(tx.Hash, TxStatusConfirmed)).To(Succeed())

							status, err:= db.TxStatus(tx.Hash)
							Expect(err).NotTo(HaveOccurred())
							Expect(status).Should(Equal(TxStatusConfirmed))
						}

						confirmedTxs, err := db.TxsWithStatus(TxStatusConfirmed, time.Hour, "")
						Expect(err).NotTo(HaveOccurred())
						Expect(len(confirmedTxs)).Should(Equal(len(txs)))
						for _, tx := range confirmedTxs {
							originTx, ok := txs[tx.Hash]
							Expect(ok).Should(BeTrue())
							compare(originTx, tx)
							delete(txs, tx.Hash)
						}

						return len(txs) == 0
					}

					Expect(quick.Check(test, &quick.Config{MaxCount: 10})).NotTo(HaveOccurred())
				})
			})

			//
			// Context("when querying for confirmed tx", func() {
			// 	It("should only return whether the given tx has been confirmed on chain", func() {
			// 		sqlDB := init(dbname)
			// 		defer sqlDB.Close()
			// 		db := New(sqlDB)
			//
			// 		test := func() bool {
			// 			Expect(db.Init()).Should(Succeed())
			// 			defer dropTables(sqlDB, "shiftin", "shiftout")
			//
			// 			for i := 0; i < 50; i++ {
			// 				shiftIn := randomShiftIn()
			// 				Expect(db.InsertShiftIn(shiftIn)).To(Succeed())
			// 				confirmed, err := db.Confirmed(shiftIn.Hash)
			// 				Expect(err).NotTo(HaveOccurred())
			// 				Expect(confirmed).Should(BeFalse())
			// 				Expect(db.UpdateTxStatus(shiftIn.Hash, TxStatusConfirmed)).Should(Succeed())
			// 				confirmed, err = db.Confirmed(shiftIn.Hash)
			// 				Expect(err).NotTo(HaveOccurred())
			// 				Expect(confirmed).Should(BeTrue())
			//
			// 				shiftOut := testutil.RandomTransformedBurningTx("")
			// 				Expect(db.InsertShiftOut(shiftOut)).To(Succeed())
			// 				confirmed, err = db.Confirmed(shiftOut.Hash)
			// 				Expect(err).NotTo(HaveOccurred())
			// 				Expect(confirmed).Should(BeFalse())
			// 				Expect(db.UpdateTxStatus(shiftOut.Hash, TxStatusConfirmed)).Should(Succeed())
			// 				confirmed, err = db.Confirmed(shiftOut.Hash)
			// 				Expect(err).NotTo(HaveOccurred())
			// 				Expect(confirmed).Should(BeTrue())
			// 			}
			// 			pendingTxs, err := db.PendingTxs("")
			// 			Expect(err).NotTo(HaveOccurred())
			//
			// 			return len(pendingTxs) == 0
			// 		}
			//
			// 		Expect(quick.Check(test, &quick.Config{MaxCount: 20})).NotTo(HaveOccurred())
			// 	})
			// })

			Context("when pruning the db", func() {
				It("should only prune data which is expired", func() {
					sqlDB := init(dbname)
					defer sqlDB.Close()
					db := New(sqlDB)

					test := func() bool {
						Expect(db.Init()).Should(Succeed())
						defer dropTables(sqlDB, "shiftin", "shiftout")

						shiftIn := testutil.RandomTransformedMintingTx("")
						shiftOut := testutil.RandomTransformedBurningTx("")
						Expect(db.InsertTx(shiftIn)).To(Succeed())
						Expect(db.InsertTx(shiftOut)).To(Succeed())

						// Expect no data gets pruned when they are not expired
						Expect(db.Prune(5 * time.Second)).Should(Succeed())
						numShiftIn, err := NumOfDataEntries(sqlDB, "shiftin")
						Expect(err).NotTo(HaveOccurred())
						Expect(numShiftIn).Should(Equal(1))
						numShiftOut, err := NumOfDataEntries(sqlDB, "shiftout")
						Expect(err).NotTo(HaveOccurred())
						Expect(numShiftOut).Should(Equal(1))

						// Expect data gets prunes when they are expired
						Expect(UpdateTxCreatedTime(sqlDB, "shiftin", shiftIn.Hash, time.Now().Unix()-5)).Should(Succeed())
						Expect(UpdateTxCreatedTime(sqlDB, "shiftout", shiftOut.Hash, time.Now().Unix()-5)).Should(Succeed())
						Expect(db.Prune(time.Second)).Should(Succeed())
						numShiftIn, err = NumOfDataEntries(sqlDB, "shiftin")
						Expect(err).NotTo(HaveOccurred())
						Expect(numShiftIn).Should(BeZero())
						numShiftOut, err = NumOfDataEntries(sqlDB, "shiftout")
						Expect(err).NotTo(HaveOccurred())
						Expect(numShiftIn).Should(BeZero())

						return true
					}

					Expect(quick.Check(test, &quick.Config{MaxCount: 10})).NotTo(HaveOccurred())
				})
			})
		})
	}
})
