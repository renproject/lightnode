package main

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/renproject/multichain"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Lightnode cmd test", func() {
	Context("when receving a request that does not have a response in the cache", func() {
		It("should pass the request through", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			logger := logrus.New()
			conf := fetchConfig(ctx, "http://lightnode-new-testnet.herokuapp.com/", logger, time.Minute)
			bitcoinConfs := conf.Confirmations[multichain.Ethereum]
			Expect(bitcoinConfs).NotTo(BeZero())
		})
	})

})
