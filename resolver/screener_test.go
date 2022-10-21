package resolver_test

import (
	"database/sql"
	"os"
	"strings"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/resolver"

	"github.com/renproject/multichain"
	"github.com/renproject/pack"
)

const (
	Sqlite   = "sqlite3"
	Postgres = "postgres"
)

var _ = FDescribe("Screening blacklisted addresses", func() {

	testDBs := []string{Sqlite}

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

	checkAddressFromDB := func(db *sql.DB, addr string) (bool, error) {
		row, err := db.Query("SELECT * FROM blacklist where address=$1", FormatAddress(addr))
		if err != nil {
			return false, err
		}
		count := 0
		for row.Next() {
			count++
		}
		if row.Err() != nil {
			return false, err
		}
		return count == 1, nil
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
			It("should return whether the addresses are blacklisted", func() {
				sqlDB := init(dbname)
				defer destroy(sqlDB)

				screeningKey := os.Getenv("SCREENING_KEY")
				screener := NewScreener(sqlDB, screeningKey)

				// Test a sanctioned address
				addr1 := pack.String("149w62rY42aZBox8fGcmqNsXUzSStKeq8C")
				ok, err := screener.IsBlacklisted(addr1, multichain.Bitcoin)
				Expect(err).NotTo(HaveOccurred())
				Expect(ok).Should(BeTrue())

				// Test a normal address
				addr2 := pack.String("0xEAF4a99DEA6fdc1e84996a2e61830222766D8303")
				ok, err = screener.IsBlacklisted(addr2, multichain.Ethereum)
				Expect(err).NotTo(HaveOccurred())
				Expect(ok).Should(BeFalse())
			})
		})

		Context("when the address is in our blacklist", func() {
			It("should return the address been blacklisted", func() {
				sqlDB := init(dbname)
				defer destroy(sqlDB)

				screeningKey := os.Getenv("SCREENING_KEY")
				screener := NewScreener(sqlDB, screeningKey)

				// Update the table with a list of addresses
				addrs := []string{
					"123",
					"abc",
					"htp9mgp8tig923zfy7qf2zzbmumynefrahsp7vsg4wxv",
					"cezn7mqp9xoxn2hdyw6fjej73t7qax9rp2zys6hb3ieu",
					"5wwbygqg6bderm2nnnyumqxfcunb68b6kesxbywh1j3n",
					"geeccgj9bezvbvor1njkbcciqxjbxvedhaxdcrbdbmuy",
				}
				for _, addr := range addrs {
					_, err := sqlDB.Exec("INSERT INTO blacklist (address) values ($1)", addr)
					Expect(err).NotTo(HaveOccurred())
				}

				for _, addr := range addrs {
					ok, err := screener.IsBlacklisted(pack.String(addr), multichain.Ethereum)
					Expect(err).NotTo(HaveOccurred())
					Expect(ok).Should(BeTrue())
				}

				// For address not listed in the db, it should query the external API
				addr1 := pack.String("149w62rY42aZBox8fGcmqNsXUzSStKeq8C")
				ok, err := screener.IsBlacklisted(addr1, multichain.Bitcoin)
				Expect(err).NotTo(HaveOccurred())
				Expect(ok).Should(BeTrue())

				// It should be able to figure out different format of the same address
				for _, addr := range addrs {

					// With up case letters
					ok, err := screener.IsBlacklisted(pack.String(strings.ToUpper(addr)), multichain.Ethereum)
					Expect(err).NotTo(HaveOccurred())
					Expect(ok).Should(BeTrue())

					// With space
					ok, err = screener.IsBlacklisted(pack.String(addr+"  "), multichain.Ethereum)
					Expect(err).NotTo(HaveOccurred())
					Expect(ok).Should(BeTrue())
					ok, err = screener.IsBlacklisted(pack.String("  "+addr), multichain.Ethereum)
					Expect(err).NotTo(HaveOccurred())
					Expect(ok).Should(BeTrue())

					// With 0x prefix
					ok, err = screener.IsBlacklisted(pack.String("0x"+addr), multichain.Ethereum)
					Expect(err).NotTo(HaveOccurred())
					Expect(ok).Should(BeTrue())
				}
			})
		})

		Context("when an address is blacklisted by the external api", func() {
			It("should store the result locally", func() {
				sqlDB := init(dbname)
				defer destroy(sqlDB)

				screeningKey := os.Getenv("SCREENING_KEY")
				screener := NewScreener(sqlDB, screeningKey)
				addr1 := pack.String("149w62rY42aZBox8fGcmqNsXUzSStKeq8C")

				// Check if the address is in the db
				exist, err := checkAddressFromDB(sqlDB, string(addr1))
				Expect(err).NotTo(HaveOccurred())
				Expect(exist).Should(BeFalse())

				// Test a sanctioned address
				ok, err := screener.IsBlacklisted(addr1, multichain.Bitcoin)
				Expect(err).NotTo(HaveOccurred())
				Expect(ok).Should(BeTrue())

				// Check if the address is in the db
				exist, err = checkAddressFromDB(sqlDB, string(addr1))
				Expect(err).NotTo(HaveOccurred())
				Expect(exist).Should(BeTrue())

			})
		})
	}
})
