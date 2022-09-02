package resolver_test

import (
	"database/sql"
	"os"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/resolver"

	"github.com/renproject/pack"
)

const (
	Sqlite   = "sqlite3"
	Postgres = "postgres"
)

var _ = Describe("Screening sanction addresses", func() {

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
		dropTxs := "DROP TABLE IF EXISTS blacklist;"
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

		Context("when sending a list of addresses to the API", func() {
			It("should return whether the addresses are sanctioned", func() {
				sqlDB := init(dbname)
				defer destroy(sqlDB)
				screener := NewScreener(sqlDB, "")

				// Test a sanctioned address
				addr1 := pack.String("149w62rY42aZBox8fGcmqNsXUzSStKeq8C")
				ok, err := screener.IsSanctioned(addr1)
				Expect(err).NotTo(HaveOccurred())
				Expect(ok).Should(BeTrue())

				// Test a normal address
				addr2 := pack.String("0xEAF4a99DEA6fdc1e84996a2e61830222766D8303")
				ok, err = screener.IsSanctioned(addr2)
				Expect(err).NotTo(HaveOccurred())
				Expect(ok).Should(BeFalse())
			})
		})

		Context("when the address is in our blacklist", func() {
			It("should return the address been sanctioned", func() {
				sqlDB := init(dbname)
				defer destroy(sqlDB)
				screener := NewScreener(sqlDB, "")

				// Update the table with a list of addresses
				addrs := []string{
					"123",
					"abc",
					"Htp9MGP8Tig923ZFY7Qf2zzbMUmYneFRAhSp7vSg4wxV",
					"CEzN7mqP9xoxn2HdyW6fjEJ73t7qaX9Rp2zyS6hb3iEu",
					"5WwBYgQG6BdErM2nNNyUmQXfcUnB68b6kesxBywh1J3n",
					"GeEccGJ9BEzVbVor1njkBCCiqXJbXVeDHaXDCrBDbmuy",
				}
				for _, addr := range addrs {
					_, err := sqlDB.Exec("INSERT INTO blacklist (address) values ($1)", addr)
					Expect(err).NotTo(HaveOccurred())
				}

				for _, addr := range addrs {
					ok, err := screener.IsSanctioned(pack.String(addr))
					Expect(err).NotTo(HaveOccurred())
					Expect(ok).Should(BeTrue())
				}

				// For address not listed in the db, it should query the external API
				addr1 := pack.String("149w62rY42aZBox8fGcmqNsXUzSStKeq8C")
				ok, err := screener.IsSanctioned(addr1)
				Expect(err).NotTo(HaveOccurred())
				Expect(ok).Should(BeTrue())

				// Test a normal address
				addr2 := pack.String("0xEAF4a99DEA6fdc1e84996a2e61830222766D8303")
				ok, err = screener.IsSanctioned(addr2)
				Expect(err).NotTo(HaveOccurred())
				Expect(ok).Should(BeFalse())
			})
		})
	}
})
