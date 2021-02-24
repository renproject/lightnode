package watcher_test

import (
	"context"
	"time"

	_ "github.com/mattn/go-sqlite3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/watcher"

	"github.com/alicebob/miniredis"
	"github.com/go-redis/redis/v7"
	"github.com/renproject/darknode/jsonrpc/jsonrpcresolver"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/darknode/txengine/txenginebindings"
	"github.com/renproject/id"
	"github.com/renproject/multichain"
	"github.com/renproject/pack"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Watcher", func() {
	init := func(ctx context.Context, interval time.Duration) (Watcher, *redis.Client) {
		mr, err := miniredis.Run()
		if err != nil {
			panic(err)
		}

		client := redis.NewClient(&redis.Options{
			Addr: mr.Addr(),
		})

		logger := logrus.New()

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

		mockResolver := jsonrpcresolver.OkResponder()
		pubk := id.NewPrivKey().PubKey()

		ethClients := bindings.EthereumClients()
		ethClient := ethClients[multichain.Ethereum]

		gateways := bindings.EthereumGateways()
		btcGateway := gateways[multichain.Ethereum][multichain.BTC]

		watcher := NewWatcher(logger, multichain.NetworkDevnet, selector, bindings, ethClient, btcGateway, mockResolver, client, pubk, interval)

		go watcher.Run(ctx)

		return watcher, client
	}

	Context("when watching", func() {
		It("should initialize successfully and check for cached block height", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			_, redisClient := init(ctx, time.Second)
			defer redisClient.Close()

			Eventually(func() uint64 {
				lastBlock, err := redisClient.Get("BTCfromBitcoin_lastCheckedBlock").Uint64()
				// Cache hasn't been set yet, and that's OK
				if err == redis.Nil {
					err = nil
					lastBlock = 0
				}
				Expect(err).ShouldNot(HaveOccurred())
				return lastBlock
			}, 10*time.Second).ShouldNot(Equal(uint64(0)))
		})

		It("should encode and decode addresses", func() {
			validTestnetAddrs := []multichain.Address{
				"miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6",
				"bchtest:qq0j3wgesd5de3tuhkka25yjh2xselqvmvpxvx7863",
				"t28Tc2BUTHifXthexsohy89umGdqMWLSUqw",
			}
			chains := []multichain.Chain{multichain.Bitcoin, multichain.BitcoinCash, multichain.Zcash}
			networks := []multichain.Network{multichain.NetworkDevnet, multichain.NetworkMainnet, multichain.NetworkLocalnet}
			for i := range chains {
				for j := range networks {
					decoder := AddressEncodeDecoder(chains[i], networks[j])
					_, err := decoder.DecodeAddress(validTestnetAddrs[i])
					// If network is mainnet, fail to decode addresses
					// otherwise pass
					if j == 1 {
						Expect(err).To(HaveOccurred())
					} else {
						if i == 1 && j == 2 {
							// bcash has a different format for Localnet and Devnet
							// so it should fail when network is localnet, but pass when it is Devnet
							Expect(err).To(HaveOccurred())
						} else {
							Expect(err).NotTo(HaveOccurred())
						}
					}
				}
			}
		})
	})

})
