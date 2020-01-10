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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var _ = Describe("Lightnode db", func() {

	Context("Sqlite", func() {

		initDB := func() *sql.DB {
			sqlDB, err := sql.Open("sqlite3", "./test.db")
			if err != nil {
				Expect(err).NotTo(HaveOccurred())
			}
			return sqlDB
		}

		AfterEach(func() {
			Expect(os.Remove("./test.db")).Should(BeNil())
		})

		Context("when creating the tx table", func() {
			It("should only create the tx if not exists", func() {
				sqlite := initDB()
				defer sqlite.Close()
				db := New(sqlite)

				// table should not exist before creation
				Expect(CheckTableExistenceSqlite(sqlite, "tx")).Should(HaveOccurred())

				// table should exist after creation
				Expect(db.CreateTxTable()).To(Succeed())
				Expect(CheckTableExistenceSqlite(sqlite, "tx")).NotTo(HaveOccurred())

				// Multiple call of the creation function should not have any effect on existing table.
				Expect(db.CreateTxTable()).To(Succeed())
				Expect(CheckTableExistenceSqlite(sqlite, "tx")).NotTo(HaveOccurred())

				Expect(db.CreateTxTable()).To(Succeed())
				Expect(CheckTableExistenceSqlite(sqlite, "tx")).NotTo(HaveOccurred())
			})
		})

		Context("when processing txs", func() {
			It("should be able to read and write tx", func() {
				sqlite := initDB()
				defer sqlite.Close()
				db := New(sqlite)
				Expect(db.CreateTxTable()).To(Succeed())

				test := func() bool {
					tx, err := RandomShiftInTx()
					Expect(err).NotTo(HaveOccurred())
					Expect(db.InsertTx(tx)).To(Succeed())

					stored, err := db.GetTx(tx.Hash)
					Expect(err).NotTo(HaveOccurred())
					return cmp.Equal(tx, stored, cmpopts.EquateEmpty())
				}

				Expect(quick.Check(test, nil)).NotTo(HaveOccurred())
			})

			It("should be able to delete tx", func() {
				sqlite := initDB()
				defer sqlite.Close()
				db := New(sqlite)
				Expect(db.CreateTxTable()).To(Succeed())

				test := func() bool {
					// Insert a random tx
					tx, err := RandomShiftInTx()
					Expect(err).NotTo(HaveOccurred())
					Expect(db.InsertTx(tx)).To(Succeed())

					// Expect db has on data entry
					before, err := NumOfDataEntries(sqlite)
					Expect(err).NotTo(HaveOccurred())
					Expect(before).Should(Equal(1))

					// Delete the data and expect no data in the db
					Expect(db.DeleteTx(tx.Hash)).Should(Succeed())
					after, err := NumOfDataEntries(sqlite)
					Expect(err).NotTo(HaveOccurred())
					Expect(after).Should(BeZero())

					return true
				}

				Expect(quick.Check(test, nil)).NotTo(HaveOccurred())
			})
		})
	})

	Context("Postgres", func() {

		// Please make sure you have turned postgres on and have set up a db called testDatabase
		// $ pg_ctl -D /usr/local/var/postgres start
		// $ createdb testDatabase
		initDB := func() *sql.DB {
			sqlDB, err := sql.Open("postgres", "postgresql://postgres:postgres@localhost:5432/postgres?sslmode=disable")
			if err != nil {
				Expect(err).NotTo(HaveOccurred())
			}
			return sqlDB
		}

		Context("when creating the tx table", func() {
			It("should only create the tx if not exists", func() {
				pq := initDB()
				defer pq.Close()
				db := New(pq)

				// table should not exist before creation
				Expect(CheckTableExistencePostgres(pq, "tx")).Should(HaveOccurred())

				// table should exist after creation
				Expect(db.CreateTxTable()).To(Succeed())
				Expect(CheckTableExistencePostgres(pq, "tx")).NotTo(HaveOccurred())

				// Multiple call of the creation function should not have any effect on existing table.
				Expect(db.CreateTxTable()).To(Succeed())
				Expect(CheckTableExistencePostgres(pq, "tx")).NotTo(HaveOccurred())

				Expect(db.CreateTxTable()).To(Succeed())
				Expect(CheckTableExistencePostgres(pq, "tx")).NotTo(HaveOccurred())
			})
		})

		Context("when processing txs", func() {
			It("should be able to read and write tx", func() {
				pq := initDB()
				defer pq.Close()
				db := New(pq)
				Expect(db.CreateTxTable()).To(Succeed())

				test := func() bool {
					tx, err := RandomShiftInTx()
					Expect(err).NotTo(HaveOccurred())
					Expect(db.InsertTx(tx)).To(Succeed())

					stored, err := db.GetTx(tx.Hash)
					Expect(err).NotTo(HaveOccurred())
					Expect(cmp.Equal(tx, stored, cmpopts.EquateEmpty())).Should(BeTrue())

					Expect(db.DeleteTx(tx.Hash)).Should(Succeed())

					return true
				}

				Expect(quick.Check(test, nil)).NotTo(HaveOccurred())

			})

			It("should be able to delete tx", func() {
				pq := initDB()
				defer pq.Close()
				db := New(pq)
				Expect(db.CreateTxTable()).To(Succeed())

				test := func() bool {
					// Insert a random tx
					tx, err := RandomShiftInTx()
					Expect(err).NotTo(HaveOccurred())
					Expect(db.InsertTx(tx)).To(Succeed())

					// Expect db has on data entry
					before, err := NumOfDataEntries(pq)
					Expect(err).NotTo(HaveOccurred())
					Expect(before).Should(Equal(1))

					// Delete the data and expect no data in the db
					Expect(db.DeleteTx(tx.Hash)).Should(Succeed())
					after, err := NumOfDataEntries(pq)
					Expect(err).NotTo(HaveOccurred())
					Expect(after).Should(BeZero())

					return true
				}

				Expect(quick.Check(test, nil)).NotTo(HaveOccurred())
			})
		})
	})
})
