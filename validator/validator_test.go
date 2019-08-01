package validator_test

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/lightnode/server"
	"github.com/renproject/lightnode/testutils"
	"github.com/renproject/lightnode/validator"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

func initValidator(ctx context.Context) (phi.Sender, <-chan phi.Message) {
	opts := phi.Options{Cap: 10}
	logger := logrus.New()
	inspector, messages := testutils.NewInspector(10)
	validator := validator.New(inspector, logger, opts)

	go validator.Run(ctx)
	go inspector.Run(ctx)

	return validator, messages
}

var _ = Describe("Validator", func() {
	Context("When running a validator task", func() {
		It("Should pass a valid message through", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			validator, messages := initValidator(ctx)

			for method, _ := range jsonrpc.RPCs {
				// TODO: This method is not supported right now, but when it is
				// this case should be tested too.
				if method == jsonrpc.MethodQueryEpoch {
					continue
				}

				request := testutils.ValidRequest(method)
				validator.Send(server.NewRequestWithResponder(request))

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

		It("Should return an error response when the jsonrpc field of the request is not 2.0", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			validator, messages := initValidator(ctx)

			// TODO: Is it worth fuzz testing on the other request fields?
			request := testutils.ValidRequest(jsonrpc.MethodQueryBlock)
			request.Version = "1.0"
			req := server.NewRequestWithResponder(request)
			validator.Send(req)

			select {
			case <-time.After(time.Second):
				Fail("timeout")
			case res := <-req.Responder:
				expectedErr := jsonrpc.NewError(jsonrpc.ErrorCodeInvalidRequest, fmt.Sprintf("invalid jsonrpc field: expected \"2.0\", got \"%s\"", request.Version), nil)

				Expect(res.Version).To(Equal("2.0"))
				Expect(res.ID).To(Equal(request.ID))
				Expect(res.Result).To(BeNil())
				Expect(*res.Error).To(Equal(expectedErr))
				Eventually(messages).ShouldNot(Receive())
			}
		})

		It("Should return an error response when the method is not supported", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			validator, messages := initValidator(ctx)

			// TODO: Is it worth fuzz testing on the other request fields?
			request := testutils.ValidRequest(jsonrpc.MethodQueryBlock)
			request.Method = "method"
			req := server.NewRequestWithResponder(request)
			validator.Send(req)

			select {
			case <-time.After(time.Second):
				Fail("timeout")
			case res := <-req.Responder:
				expectedErr := jsonrpc.NewError(jsonrpc.ErrorCodeMethodNotFound, fmt.Sprintf("unsupported method %s", request.Method), nil)

				Expect(res.Version).To(Equal("2.0"))
				Expect(res.ID).To(Equal(request.ID))
				Expect(res.Result).To(BeNil())
				Expect(*res.Error).To(Equal(expectedErr))
				Eventually(messages).ShouldNot(Receive())
			}
		})

		It("Should return an error response when the method does not match the parameters", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			validator, messages := initValidator(ctx)

			for method := range jsonrpc.RPCs {
				// TODO: Is it worth fuzz testing on the other request fields?
				var params json.RawMessage
				switch method {
				case jsonrpc.MethodSubmitTx,
					jsonrpc.MethodQueryTx:
					params = json.RawMessage{}
				default:
					// TODO: This method is either not supported, or does not
					// require any parameters.
					continue
				}
				request := testutils.ValidRequest(method)
				request.Params = params
				req := server.NewRequestWithResponder(request)
				validator.Send(req)

				select {
				case <-time.After(time.Second):
					Fail("timeout")
				case res := <-req.Responder:
					expectedErr := jsonrpc.NewError(server.ErrorCodeInvalidParams, "invalid parameters in request: parameters object does not match method", nil)

					Expect(res.Version).To(Equal("2.0"))
					Expect(res.ID).To(Equal(request.ID))
					Expect(res.Result).To(BeNil())
					Expect(*res.Error).To(Equal(expectedErr))
					Eventually(messages).ShouldNot(Receive())
				}
			}
		})
	})
})
