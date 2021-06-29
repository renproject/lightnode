package confirmer_test

import (
	"context"
	"database/sql"
	"math/rand"
	"time"

	_ "github.com/mattn/go-sqlite3"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/renproject/id"
	. "github.com/renproject/lightnode/confirmer"

	"github.com/renproject/darknode/tx/txutil"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/lightnode/testutils"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Confirmer", func() {
	cleanUp := func(db *sql.DB) {
		dropTxs := "DROP TABLE IF EXISTS txs;"
		_, err := db.Exec(dropTxs)
		Expect(err).NotTo(HaveOccurred())
	}

	Context("when txs have received sufficient confirmations", func() {
		It("should mark them as confirmed", func() {
			// Initialise confirmer.
			logger := logrus.New()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			dispatcher := testutils.NewMockDispatcher(false)
			go dispatcher.Run(ctx)

			sqlDB, err := sql.Open("sqlite3", "./test.db")
			Expect(err).ToNot(HaveOccurred())
			sqlDB.SetMaxOpenConns(1)
			defer cleanUp(sqlDB)

			database := db.New(sqlDB, 0)
			Expect(database.Init()).To(Succeed())

			maxAttempts := 2
			bindings := testutils.MockBindings(logger, maxAttempts)

			pollInterval := 2 * time.Second
			confirmer := New(
				DefaultOptions().
					WithLogger(logger).
					WithPollInterval(pollInterval).
					WithExpiry(7*24*time.Hour),
				dispatcher,
				database,
				bindings,
			)
			go confirmer.Run(ctx)

			// Insert random transactions into the database.
			hashes := make([]id.Hash, 100)
			r := rand.New(rand.NewSource(GinkgoRandomSeed()))
			for i := range hashes {
				transaction := txutil.RandomGoodTx(r)
				Expect(database.InsertTx(transaction)).To(Succeed())

				hashes[i] = transaction.Hash
				status, err := database.TxStatus(transaction.Hash)
				Expect(err).ToNot(HaveOccurred())
				Expect(status).To(Equal(db.TxStatusConfirming))
			}

			// Sleep and ensure the transaction statuses have updated.
			time.Sleep(time.Duration(maxAttempts+1) * pollInterval)

			for i := range hashes {
				status, err := database.TxStatus(hashes[i])
				Expect(err).ToNot(HaveOccurred())
				Expect(status).To(Equal(db.TxStatusConfirmed))
			}
		})

		It("should handle backpressure", func() {
			// Initialise confirmer.
			logger := logrus.New()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			dispatcher := testutils.NewMockDispatcher(true)
			go dispatcher.Run(ctx)

			sqlDB, err := sql.Open("sqlite3", "./test.db")
			Expect(err).ToNot(HaveOccurred())
			sqlDB.SetMaxOpenConns(1)

			defer cleanUp(sqlDB)

			database := db.New(sqlDB, 0)
			Expect(database.Init()).To(Succeed())

			maxAttempts := 2
			bindings := testutils.MockBindings(logger, maxAttempts)

			pollInterval := 2 * time.Second
			confirmer := New(
				DefaultOptions().
					WithLogger(logger).
					WithPollInterval(pollInterval).
					WithExpiry(7*24*time.Hour),
				dispatcher,
				database,
				bindings,
			)
			go confirmer.Run(ctx)

			// Insert random transactions into the database.
			hashes := make([]id.Hash, 100)
			r := rand.New(rand.NewSource(GinkgoRandomSeed()))
			for i := range hashes {
				transaction := txutil.RandomGoodTx(r)
				Expect(database.InsertTx(transaction)).To(Succeed())

				hashes[i] = transaction.Hash
				status, err := database.TxStatus(transaction.Hash)
				Expect(err).ToNot(HaveOccurred())
				Expect(status).To(Equal(db.TxStatusConfirming))
			}

			// Sleep and ensure the transaction statuses have updated.
			time.Sleep(time.Duration(maxAttempts+1) * pollInterval)

			for i := range hashes {
				status, err := database.TxStatus(hashes[i])
				Expect(err).ToNot(HaveOccurred())
				Expect(status).To(Equal(db.TxStatusConfirming))
			}
		})
	})
})
