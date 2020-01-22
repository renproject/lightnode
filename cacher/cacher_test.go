package cacher_test

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/renproject/darknode"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/lightnode/cacher"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/lightnode/server"
	"github.com/renproject/lightnode/testutils"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

func initDB() db.DB {
	sqlDB, err := sql.Open("sqlite3", "./test.db")
	if err != nil {
		panic(err)
	}
	return db.NewSQLDB(sqlDB)
}

func initCacher(ctx context.Context, cacheCap int, ttl time.Duration) (phi.Sender, <-chan phi.Message) {
	opts := phi.Options{Cap: 10}
	logger := logrus.New()
	inspector, messages := testutils.NewInspector(10)
	cacher := cacher.New(ctx, darknode.Localnet, initDB(), inspector, logger, cacheCap, ttl, 24*time.Hour, opts)

	go cacher.Run(ctx)
	go inspector.Run(ctx)

	return cacher, messages
}

var _ = Describe("Cacher", func() {
	Context("When receving a request that does not have a response in the cache", func() {
		It("Should pass the request through", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cacher, messages := initCacher(ctx, 10, time.Second)

			for method, _ := range jsonrpc.RPCs {
				// TODO: This method is not supported right now, but when it is
				// this case should be tested too.
				if method == jsonrpc.MethodQueryEpoch {
					continue
				}

				request := testutils.ValidRequest(method)
				cacher.Send(server.NewRequestWithResponder(request, ""))

				select {
				case <-time.After(time.Second):
					Fail("timeout")
				case message := <-messages:
					req, ok := message.(server.RequestWithResponder)
					Expect(ok).To(BeTrue())
					Expect(req.Request).To(Equal(request))
					Expect(req.Responder).To(Not(BeNil()))
					Eventually(req.Responder).ShouldNot(Receive())
				}
			}
		})
	})

	Context("when receiving a request that has a response in the cache", func() {
		It("Should return the cached response", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cacher, messages := initCacher(ctx, 10, 1*time.Second)

			for method, _ := range jsonrpc.RPCs {
				// TODO: This method is not supported right now, but when it is
				// this case should be tested too.
				if method == jsonrpc.MethodQueryEpoch {
					continue
				}

				// We have disabled caching for these methods.
				if method == jsonrpc.MethodSubmitTx || method == jsonrpc.MethodQueryTx {
					continue
				}

				// Send the first request
				request := testutils.ValidRequest(method)
				reqWithRes := server.NewRequestWithResponder(request, "")
				cacher.Send(reqWithRes)
				forwardedReq := <-messages
				res := testutils.ErrorResponse(request.ID)
				forwardedReq.(server.RequestWithResponder).Responder <- res

				select {
				case <-time.After(time.Second):
					Fail("timeout")
				case response := <-reqWithRes.Responder:
					Expect(response).To(Equal(res))
				}

				// Send the second request and expect a cached response
				request = testutils.ValidRequest(method)
				reqWithRes = server.NewRequestWithResponder(request, "")
				cacher.Send(reqWithRes)

				select {
				case <-time.After(time.Second):
					Fail("timeout")
				case response := <-reqWithRes.Responder:
					Expect(response).To(Equal(res))
					Eventually(messages).ShouldNot(Receive())
				}
			}
		})
	})
})
