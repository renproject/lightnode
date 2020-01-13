package server_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/renproject/lightnode/server"
)

var _ = Describe("RateLimiter", func() {

	// TODO : Need to find a good way of testing whether the rateLimiter works
	//        as expected.

	Context("when receiving requests", func() {
		It("should always reject request with unknown method", func() {
			limiter := server.NewRateLimiter()
			Expect(limiter.Allow("random", "0.0.0.0")).Should(BeFalse())
		})
	})
})
