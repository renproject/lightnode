package http_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/http"
	. "github.com/renproject/lightnode/testutils"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/renproject/darknode/jsonrpc"
)

var (
	IP4  = "0.0.0.0"
	PORT = "5000"
)

var _ = Describe("Lightnode server", func() {

	Context("when initializing server options", func() {

		It("should panic if the port field is not set.", func() {
			options := Options{
				Port: "",
			}
			Expect(func() {
				options.SetZeroToDefault()
			}).Should(Panic())
		})

		It("should set zero values to default", func() {
			options := Options{
				Port: "12345",
			}
			options.SetZeroToDefault()
			Expect(options.MaxBatchSize).Should(Equal(10))
			Expect(options.Timeout).Should(Equal(15 * time.Second))
		})
	})

	Context("when running a server", func() {

		// TODO : ADD MORE TEST CASES.
		// It("should expose an endpoint for health check", func() {
		// 	ctx, cancel := context.WithCancel(context.Background())
		// 	defer cancel()
		// })

		It("should pass a valid message through", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			outMessages := InitServer(ctx, PORT)

			url := fmt.Sprintf("http://%s:%s", IP4, PORT)
			request := ValidRequest(jsonrpc.MethodSubmitTx)
			respChan, err := SendRequestAsync(request, url)
			Expect(err).NotTo(HaveOccurred())

			// Expect the request pass all the check and sent to validator.
			var req RequestWithResponder
			Eventually(outMessages).Should(Receive(&req))
			Expect(req.Request).To(Equal(request))
			Expect(req.Responder).To(Not(BeNil()))

			// Simulate a response been sent through the responder channel.
			response := ErrorResponse(req.Request.ID)
			req.Responder <- response
			var resp *jsonrpc.Response
			Eventually(respChan).Should(Receive(&resp))
			Expect(cmp.Equal(*response.Error, *resp.Error, cmpopts.EquateEmpty())).Should(BeTrue())
		})
	})
})
