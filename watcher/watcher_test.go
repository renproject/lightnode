package watcher_test

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	_ "github.com/mattn/go-sqlite3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v0 "github.com/renproject/lightnode/compat/v0"
	. "github.com/renproject/lightnode/watcher"

	"github.com/alicebob/miniredis"
	"github.com/go-redis/redis/v7"
	"github.com/renproject/darknode/jsonrpc/jsonrpcresolver"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/darknode/txengine/txenginebindings"
	"github.com/renproject/darknode/txengine/txenginebindings/ethereumbindings"
	"github.com/renproject/id"
	"github.com/renproject/multichain"
	"github.com/renproject/pack"
	"github.com/sirupsen/logrus"
)

type MockBurnLogFetcher struct {
	BurnIn chan ethereumbindings.MintGatewayLogicV1LogBurn
}

func NewMockBurnLogFetcher() MockBurnLogFetcher {
	return MockBurnLogFetcher{
		BurnIn: make(chan ethereumbindings.MintGatewayLogicV1LogBurn),
	}
}

func (fetcher MockBurnLogFetcher) FetchBurnLogs(ctx context.Context, from uint64, to uint64) (chan BurnLogResult, error) {
	x := make(chan BurnLogResult)

	go func() {
		for e := range fetcher.BurnIn {
			select {
			case <-ctx.Done():
				break
			}
			x <- BurnLogResult{Result: e}
		}
		close(x)
	}()

	return x, nil
}

var _ = Describe("Watcher", func() {
	init := func(ctx context.Context, interval time.Duration) (Watcher, *redis.Client, chan ethereumbindings.MintGatewayLogicV1LogBurn) {
		mr, err := miniredis.Run()
		if err != nil {
			panic(err)
		}

		client := redis.NewClient(&redis.Options{
			Addr: mr.Addr(),
		})

		logger := logrus.New()

		selector := tx.Selector("BTC/fromEthereum")

		mockResolver := jsonrpcresolver.OkResponder()
		pubk := id.NewPrivKey().PubKey()

		if err != nil {
			logger.Panicf("failed to create account client: %v", err)
		}

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

		ethClients := bindings.EthereumClients()
		ethClient := ethClients[multichain.Ethereum]

		burnLogFetcher := NewMockBurnLogFetcher()

		watcher := NewWatcher(logger, multichain.NetworkDevnet, selector, bindings, ethClient, burnLogFetcher, mockResolver, client, pubk, interval)

		go watcher.Run(ctx)

		return watcher, client, burnLogFetcher.BurnIn
	}

	Context("when watching", func() {
		It("should initialize successfully and check for cached block height", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			_, redisClient, _ := init(ctx, time.Second)
			defer redisClient.Close()

			Eventually(func() uint64 {
				lastBlock, err := redisClient.Get("BTC/fromEthereum_lastCheckedBlock").Uint64()
				// Cache hasn't been set yet, and that's OK
				if err == redis.Nil {
					err = nil
					lastBlock = 0
				}
				Expect(err).ShouldNot(HaveOccurred())
				return lastBlock
			}, 15*time.Second).ShouldNot(Equal(uint64(0)))
		})

		It("should detect burn events", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			_, redisClient, burnIn := init(ctx, time.Second)
			defer redisClient.Close()
			// hash, _, _, err := accountClient.BurnBeforeMint(ctx, multichain.BTC, "miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6", pack.NewU256FromU64(100000))
			// Wait for the block number to be picked up
			time.Sleep(time.Second)

			burnIn <- ethereumbindings.MintGatewayLogicV1LogBurn{
				To:        []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
				Amount:    big.NewInt(10000),
				N:         big.NewInt(0),
				IndexedTo: [32]byte{},
				Raw:       types.Log{},
			}

			selector := tx.Selector("BTC/fromEthereum")
			v0Hash := v0.BurnTxHash(selector, pack.NewU256FromU8(0))

			Eventually(func() string {
				h := redisClient.Get(fmt.Sprintf("BTC/fromEthereum_%v", 0)).Val()
				// println(h)
				// Expect().ShouldNot(HaveOccurred())
				return h
			}, 15*time.Second).Should(Equal(v0Hash.String()))
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
