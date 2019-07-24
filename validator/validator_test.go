package validator_test

import (
	"context"
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
				}
			}
		})
	})
})
