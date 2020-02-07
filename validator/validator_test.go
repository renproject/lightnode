package validator_test

import (
	"context"
	"database/sql"

	_ "github.com/mattn/go-sqlite3"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/ethereum/go-ethereum/common"
	"github.com/renproject/darknode"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/testutil"
	"github.com/renproject/kv"
	"github.com/renproject/lightnode/blockchain"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/lightnode/http"
	"github.com/renproject/lightnode/store"
	"github.com/renproject/lightnode/testutils"
	"github.com/renproject/lightnode/validator"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Validator", func() {

	init := func(ctx context.Context) (phi.Sender, <-chan phi.Message) {
		logger := logrus.New()
		inspector, messages := testutils.NewInspector(10)
		go inspector.Run(ctx)

		multiStore := store.New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses"))
		key, err := testutil.RandomEcdsaKey()
		Expect(err).NotTo(HaveOccurred())
		protocolAddr := common.HexToAddress("0x1deB773B50B66b0e65e62E41380355a1A2BEd2e1")
		connPool := blockchain.New(logger, darknode.Devnet, protocolAddr)
		sqlDB, err := sql.Open("sqlite3", "./test.db")
		Expect(err).NotTo(HaveOccurred())
		database := db.New(sqlDB)
		validator := validator.New(logger, inspector, multiStore, phi.Options{Cap: 10}, key.PublicKey, connPool, database)
		go validator.Run(ctx)
		return validator, messages
	}

	Context("When running a validator task", func() {
		It("Should pass a valid message through", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			validator, messages := init(ctx)

			for method := range jsonrpc.RPCs {
				// TODO: This method is not supported right now, but when it is
				// this case should be tested too.
				if method == jsonrpc.MethodQueryEpoch || method == jsonrpc.MethodSubmitTx {
					continue
				}

				validReq := testutils.ValidRequest(method)
				request := http.NewRequestWithResponder(ctx, validReq, "")
				Expect(validator.Send(request)).Should(BeTrue())

				var message phi.Message
				Eventually(messages).Should(Receive(&message))
				req, ok := message.(http.RequestWithResponder)
				Expect(ok).To(BeTrue())
				Expect(req.Request).To(Equal(validReq))
				Expect(req.Responder).To(Not(BeNil()))
				Eventually(req.Responder).ShouldNot(Receive())
			}
		})
	})
})
