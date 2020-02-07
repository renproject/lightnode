package cacher_test

import (
	"context"
	"database/sql"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/cacher"

	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/kv"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/lightnode/http"
	"github.com/renproject/lightnode/testutils"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Cacher", func() {
	init := func(ctx context.Context, interval time.Duration) (phi.Sender, <-chan phi.Message) {
		inspector, messages := testutils.NewInspector(10)
		ttl := kv.NewTTLCache(ctx, kv.NewMemDB(kv.JSONCodec), "cacher", interval)

		sqlDB, err := sql.Open("sqlite3", "./test.db")
		Expect(err).NotTo(HaveOccurred())

		database := db.New(sqlDB)
		Expect(database.Init()).Should(Succeed())

		cacher := New(inspector, logrus.New(), ttl, phi.Options{Cap: 10}, database)
		go inspector.Run(ctx)
		go cacher.Run(ctx)

		return cacher, messages
	}

	cleanup := func() {
		Expect(os.Remove("./test.db")).Should(BeNil())
	}

	Context("when receving a request that does not have a response in the cache", func() {
		It("should pass the request through", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cacher, messages := init(ctx, time.Minute)
			defer cleanup()

			for method := range jsonrpc.RPCs {
				// TODO: This method is not supported right now, but when it is
				// this case should be tested too.
				if method == jsonrpc.MethodQueryEpoch {
					continue
				}

				request := http.NewRequestWithResponder(ctx, testutils.ValidRequest(method), "")
				Expect(cacher.Send(request)).Should(BeTrue())

				var message phi.Message
				Eventually(messages).Should(Receive(&message))
				req, ok := message.(http.RequestWithResponder)
				Expect(ok).To(BeTrue())
				Expect(req.Request).To(Equal(request.Request))
				Expect(req.Responder).To(Not(BeNil()))
				Eventually(req.Responder).ShouldNot(Receive())
			}
		})
	})

	Context("when receiving a request that has a response in the cache", func() {
		It("should return the cached response", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cacher, messages := init(ctx, time.Minute)
			defer cleanup()

			for method := range jsonrpc.RPCs {

				// Ignore these methods.
				switch method {
				case jsonrpc.MethodQueryEpoch, jsonrpc.MethodSubmitTx, jsonrpc.MethodQueryTx:
					continue
				}

				// Send the first request and respond with an error
				valid := testutils.ValidRequest(method)
				request := http.NewRequestWithResponder(ctx, valid, "")
				Expect(cacher.Send(request)).Should(BeTrue())
				var message phi.Message
				Eventually(messages).Should(Receive(&message))
				req, ok := message.(http.RequestWithResponder)
				Expect(ok).To(BeTrue())
				resp := testutils.ErrorResponse(request.Request.ID)
				req.Responder <- resp

				// Expect receiving the response from the responder channel
				var receivedResp jsonrpc.Response
				Eventually(request.Responder).Should(Receive(&receivedResp))
				Expect(receivedResp).To(Equal(resp))

				// Send the second request and expect a cached response
				newReq := http.NewRequestWithResponder(ctx, valid, "")
				Expect(cacher.Send(newReq)).Should(BeTrue())

				var newResp jsonrpc.Response
				Eventually(newReq.Responder).Should(Receive(&newResp))
				Expect(newResp).To(Equal(resp))
			}
		})
	})
})
