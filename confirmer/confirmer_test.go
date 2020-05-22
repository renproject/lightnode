package confirmer_test

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/confirmer"
	. "github.com/renproject/lightnode/testutils"

	"github.com/renproject/darknode"
	"github.com/renproject/darknode/abi"
	"github.com/renproject/darknode/testutil"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/lightnode/testutils"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Confirmer", func() {
	Context("when txs have received sufficient confirmations", func() {
		It("should mark them as confirmed", func() {
			// Initialise confirmer.
			logger := logrus.New()

			pollInterval := 2 * time.Second
			opts := Options{
				MinConfirmations: darknode.DefaultMinConfirmations(darknode.Devnet),
				PollInterval:     pollInterval,
				Expiry:           7 * 24 * time.Hour,
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			dispatcher := NewMockDispatcher()
			go dispatcher.Run(ctx)

			sqlDB, err := sql.Open("sqlite3", "./test.db")
			Expect(err).ToNot(HaveOccurred())
			sqlDB.SetMaxOpenConns(1)

			database := db.New(sqlDB)
			Expect(database.Init()).To(Succeed())

			maxAttempts := 2
			connPool := testutils.MockConnPool(logger, maxAttempts)

			confirmer := New(logger, opts, dispatcher, database, connPool)
			go confirmer.Run(ctx)

			// Insert random txs into database.
			hashes := make([]abi.B32, 100)
			for i := range hashes {
				tx := testutil.RandomTransformedMintingTx(abi.IntrinsicBTC0Btc2Eth.Address)
				Expect(database.InsertTx(tx, "", false)).To(Succeed())

				hashes[i] = tx.Hash
				status, err := database.TxStatus(tx.Hash)
				Expect(err).ToNot(HaveOccurred())
				Expect(status).To(Equal(db.TxStatusConfirming))
			}

			// Sleep and ensure the tx statuses have updated.
			time.Sleep(time.Duration(maxAttempts+1) * pollInterval)

			for i := range hashes {
				status, err := database.TxStatus(hashes[i])
				Expect(err).ToNot(HaveOccurred())
				Expect(status).To(Equal(db.TxStatusConfirmed))
			}
		})
	})
})
