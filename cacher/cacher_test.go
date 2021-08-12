package cacher_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
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

		database := db.New(sqlDB, 100)
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
				// Ignore these methods.
				switch method {
				case jsonrpc.MethodQueryTxs:
					continue
				}

				id, params := testutils.ValidRequest(method)
				request := http.NewRequestWithResponder(ctx, id, method, params, url.Values{})
				Expect(cacher.Send(request)).Should(BeTrue())

				var message phi.Message
				Eventually(messages).Should(Receive(&message))
				msgRequest, ok := message.(http.RequestWithResponder)
				Expect(ok).To(BeTrue())
				Expect(msgRequest.ID).To(Equal(request.ID))
				Expect(msgRequest.Method).To(Equal(request.Method))
				Expect(msgRequest.Params).To(Equal(request.Params))
				Expect(msgRequest.Query).To(Equal(request.Query))
				Expect(msgRequest.Responder).To(Not(BeNil()))
				Eventually(msgRequest.Responder).ShouldNot(Receive())
			}
		})
	})

	Context("when receiving a queryTx request", func() {
		It("should strip revert messages", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cacher, messages := init(ctx, time.Minute)
			defer cleanup()

			method := jsonrpc.MethodQueryTx
			// Send the first request and respond with an error
			id, params := testutils.ValidRequest(method)
			request := http.NewRequestWithResponder(ctx, id, method, params, url.Values{})
			Expect(cacher.Send(request)).Should(BeTrue())
			var message phi.Message
			Eventually(messages).Should(Receive(&message))
			req, ok := message.(http.RequestWithResponder)
			Expect(ok).To(BeTrue())
			queryTx := testutils.MockQueryTxResponse()
			resp := jsonrpc.NewResponse(request.ID, queryTx, nil)
			req.Responder <- resp

			// Expect receiving the response from the responder channel
			var receivedResp jsonrpc.Response
			Eventually(request.Responder).Should(Receive(&receivedResp))

			queryTxResp := receivedResp.Result.(jsonrpc.ResponseQueryTx)
			Expect(queryTxResp.Tx.Hash).To(Equal(queryTx.Tx.Hash))
			Expect(queryTxResp.Tx.Input).To(Equal(queryTx.Tx.Input))
			Expect(queryTxResp.Tx.Output).NotTo(Equal(queryTx.Tx.Output))
			res, err := queryTxResp.Tx.Output.MarshalJSON()
			Expect(fmt.Sprintf("%v", string(res))).ShouldNot(ContainSubstring("\"revert\":\"\""))
			Expect(fmt.Sprintf("%v", string(res))).Should(ContainSubstring("\"amount\":\"1698300\""))

			// Send the second request and expect a cached response
			newReq := http.NewRequestWithResponder(ctx, id, method, params, url.Values{})
			Expect(cacher.Send(newReq)).Should(BeTrue())

			var newResp jsonrpc.Response
			Eventually(newReq.Responder).Should(Receive(&newResp))

			respBytes, err := json.Marshal(resp)
			Expect(err).ToNot(HaveOccurred())
			newRespBytes, err := json.Marshal(resp)
			Expect(err).ToNot(HaveOccurred())
			Expect(respBytes).To(Equal(newRespBytes))
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
				case jsonrpc.MethodSubmitTx:
					continue
				}

				// Send the first request and respond with an error
				id, params := testutils.ValidRequest(method)
				request := http.NewRequestWithResponder(ctx, id, method, params, url.Values{})
				Expect(cacher.Send(request)).Should(BeTrue())
				var message phi.Message
				Eventually(messages).Should(Receive(&message))
				req, ok := message.(http.RequestWithResponder)
				Expect(ok).To(BeTrue())
				resp := testutils.ErrorResponse(request.ID)
				req.Responder <- resp

				// Expect receiving the response from the responder channel
				var receivedResp jsonrpc.Response
				Eventually(request.Responder).Should(Receive(&receivedResp))
				Expect(receivedResp).To(Equal(resp))

				// Send the second request and expect a cached response
				newReq := http.NewRequestWithResponder(ctx, id, method, params, url.Values{})
				Expect(cacher.Send(newReq)).Should(BeTrue())

				var newResp jsonrpc.Response
				Eventually(newReq.Responder).Should(Receive(&newResp))

				respBytes, err := json.Marshal(resp)
				Expect(err).ToNot(HaveOccurred())
				newRespBytes, err := json.Marshal(resp)
				Expect(err).ToNot(HaveOccurred())
				Expect(respBytes).To(Equal(newRespBytes))
			}
		})
	})
})
