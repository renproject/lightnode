package watcher_test

import (
	"context"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/elliotchance/redismock/v7"
	"github.com/go-redis/redis/v7"

	_ "github.com/mattn/go-sqlite3"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/renproject/id"
	"github.com/renproject/lightnode/testutils"
	. "github.com/renproject/lightnode/watcher"
	"github.com/renproject/multichain"
	"github.com/renproject/pack"

	"github.com/renproject/darknode/tx"
	"github.com/renproject/darknode/txengine/txenginebindings"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Watcher", func() {
	init := func(ctx context.Context, interval time.Duration) (Watcher, *redis.Client, *redismock.ClientMock) {
		mr, err := miniredis.Run()
		if err != nil {
			panic(err)
		}

		client := redis.NewClient(&redis.Options{
			Addr: mr.Addr(),
		})

		logger := logrus.New()

		mockClient := redismock.NewNiceMock(client)

		selector := tx.Selector("BTCfromBitcoin")

		bindingsOpts := txenginebindings.DefaultOptions().
			WithNetwork("localnet")

		bindingsOpts.WithChainOptions(multichain.Bitcoin, txenginebindings.ChainOptions{
			RPC:           pack.String("https://multichain-staging.renproject.io/testnet/bitcoind"),
			Confirmations: pack.U64(0),
		})

		bindingsOpts.WithChainOptions(multichain.Ethereum, txenginebindings.ChainOptions{
			RPC:           pack.String("https://multichain-staging.renproject.io/testnet/geth"),
			Confirmations: pack.U64(0),
			Protocol:      pack.String("0x1CAD87e16b56815d6a0b4Cd91A6639eae86Fc53A"),
		})

		bindings, err := txenginebindings.New(bindingsOpts)
		if err != nil {
			logger.Panicf("bad bindings: %v", err)
		}

		mockResolver := testutils.NewMockResolver(logger)
		pubk := id.NewPrivKey().PubKey()

		ethClients := bindings.EthereumClients()
		ethClient := ethClients[multichain.Ethereum]

		gateways := bindings.EthereumGateways()
		btcGateway := gateways[multichain.Ethereum][multichain.BTC]

		watcher := NewWatcher(logger, selector, bindings, ethClient, btcGateway, mockResolver, mockClient, pubk, interval)

		go watcher.Run(ctx)

		return watcher, client, mockClient
	}

	Context("when watching", func() {
		It("should initialize successfully and check for cached block height", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			_, redisClient, redisMock := init(ctx, time.Second)
			defer redisClient.Close()
			redisMock.Mock.On("Get", "BTCfromBitcoin_lastCheckedBlock").Return(redis.NewStringCmd(1))

			Eventually(func() int {
				size := len(redisMock.Mock.Calls)
				return size
			}, 10*time.Second).Should(Equal(10))
		})
	})

})
