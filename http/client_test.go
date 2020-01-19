package http_test

import (
	"context"
	"net/http/httptest"
	"testing/quick"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/http"
	. "github.com/renproject/lightnode/testutils"

	"github.com/renproject/darknode/jsonrpc"
)

var _ = Describe("Client", func() {
	Context("when sending requests", func() {
		It("should timeout after the set timeout", func() {
			client := NewClient(100 * time.Millisecond)
			server := httptest.NewServer(TimeoutHandler(time.Minute))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			request := RandomRequest(RandomMethod())
			_, err := client.SendRequest(ctx, server.URL, request, nil)
			Expect(err).Should(HaveOccurred())
		})

		It("should send to the expected url", func() {
			client := NewClient(DefaultClientTimeout)
			dataChan := make(chan jsonrpc.Request, 128)
			server := httptest.NewServer(ChanMiddleware(dataChan, OKHandler()))

			test := func() bool {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				// Send a random request to the test server
				request := RandomRequest(RandomMethod())
				_, err := client.SendRequest(ctx, server.URL, request, nil)
				Expect(err).NotTo(HaveOccurred())
				// Expect server receive the same request
				var received jsonrpc.Request
				Eventually(dataChan).Should(Receive(&received))
				Expect(request).Should(Equal(received))
				return true
			}

			Expect(quick.Check(test, nil)).NotTo(HaveOccurred())
		})

		It("should retry sending to the server when retryOption is not nil", func() {
			client := NewClient(DefaultClientTimeout)
			dataChan := make(chan jsonrpc.Request, 128)
			server := httptest.NewServer(ChanMiddleware(dataChan, NilHandler()))
			defer server.Close()

			// Send a random request to the test server
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			request := RandomRequest(RandomMethod())
			retryOpts := RetryOptions{
				Base:   time.Second,
				Max:    4 * time.Second,
				Factor: 0.3,
			}
			_, err := client.SendRequest(ctx, server.URL, request, &retryOpts)
			Expect(err).Should(HaveOccurred())

			// Expect the client has tried sending the request more than once.
			Expect(len(dataChan)).Should(BeNumerically(">", 1))
		})
	})
})
