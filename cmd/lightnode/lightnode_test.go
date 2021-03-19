package main

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/renproject/aw/wire"
	"github.com/renproject/multichain"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Lightnode cmd test", func() {
	It("should fetch config from an rpc endpoint", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		logger := logrus.New()
		conf, err := fetchConfig(ctx, "http://lightnode-new-testnet.herokuapp.com/", logger, time.Minute)
		Expect(err).ShouldNot(HaveOccurred())
		ethConfs := conf.Confirmations[multichain.Ethereum]
		Expect(ethConfs).NotTo(BeZero())
	})

	It("should fail if there are no bootstrap nodes to fetch config from", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		logger := logrus.New()
		conf, err := getConfigFromBootstrap(ctx, logger, []wire.Address{})
		Expect(conf).To(BeZero())
		Expect(err).Should(HaveOccurred())
	})

	It("should fail if no bootstrap nodes to return configs from", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		logger := logrus.New()
		addrs := make([]wire.Address, 3)
		conf, err := getConfigFromBootstrap(ctx, logger, addrs)
		Expect(conf).To(BeZero())
		Expect(err).Should(HaveOccurred())
	})

	It("should pass if one of the bootstrap nodes returns a config", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		logger := logrus.New()
		addrs := make([]wire.Address, 3)
		addrs[1] = wire.Address{
			Protocol:  0,
			Value:     "lightnode-new-testnet.herokuapp.com:79",
			Nonce:     0,
			Signature: [65]byte{},
		}
		conf, err := getConfigFromBootstrap(ctx, logger, addrs)
		Expect(conf).NotTo(BeZero())
		Expect(err).ShouldNot(HaveOccurred())
	})

})
