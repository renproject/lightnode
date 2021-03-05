package watcher_test

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	_ "github.com/mattn/go-sqlite3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v0 "github.com/renproject/lightnode/compat/v0"
	. "github.com/renproject/lightnode/watcher"

	"github.com/alicebob/miniredis/v2"
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
	BurnIn chan BurnLogResult
	state  *MockState
}

type MockState struct {
	futureLogs []BurnLogResult
	logs       uint
}

func NewMockBurnLogFetcher() MockBurnLogFetcher {
	return MockBurnLogFetcher{
		BurnIn: make(chan BurnLogResult),
		state: &MockState{
			futureLogs: make([]BurnLogResult, 0),
			logs:       0,
		},
	}
}

func (fetcher MockBurnLogFetcher) FetchBurnLogs(ctx context.Context, from uint64, to uint64) (chan BurnLogResult, error) {
	x := make(chan BurnLogResult)

	go func() {
		newLogs := make([]BurnLogResult, 0)
		for i := range fetcher.state.futureLogs {
			e := fetcher.state.futureLogs[i]
			if e.Result.Raw.BlockNumber > to {
				newLogs = append(newLogs, e)
				continue
			}
			if e.Result.Raw.BlockNumber < to && e.Result.Raw.BlockNumber > from {
				x <- e
			}
		}
		fetcher.state.futureLogs = newLogs
		// We always need to drain the channel,
		// even if the context finishes before the channel is drained,
		// to prevent blocking
		for e := range fetcher.BurnIn {
			fetcher.state.logs += 1
			// If we haven't set a block number, don't filter
			if e.Result.Raw.BlockNumber == 0 {
				x <- e
				continue
			}

			// If the event is in the future, cache it for later
			if e.Result.Raw.BlockNumber > to {
				fetcher.state.futureLogs = append(fetcher.state.futureLogs, e)
				continue
			}

			if e.Result.Raw.BlockNumber > from {
				x <- e
			}
		}
		close(x)
	}()

	return x, nil
}

var _ = Describe("Watcher", func() {
	init := func(ctx context.Context, interval time.Duration, reliableResponder bool) (Watcher, *redis.Client, chan BurnLogResult, *miniredis.Miniredis) {
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
		if !reliableResponder {
			mockResolver = jsonrpcresolver.RandomResponder()
		}

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

		return watcher, client, burnLogFetcher.BurnIn, mr
	}

	Context("when watching", func() {
		It("should initialize successfully and check for cached block height", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			watcher, redisClient, _, _ := init(ctx, time.Second, true)

			go watcher.Run(ctx)

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
			watcher, redisClient, burnIn, _ := init(ctx, time.Second, true)
			defer redisClient.Close()

			go watcher.Run(ctx)
			// Wait for the block number to be picked up
			time.Sleep(time.Second)

			burnIn <- BurnLogResult{
				Result: ethereumbindings.MintGatewayLogicV1LogBurn{
					To:        []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
					Amount:    big.NewInt(10000),
					N:         big.NewInt(0),
					IndexedTo: [32]byte{},
					Raw:       types.Log{},
				},
			}

			selector := tx.Selector("BTC/fromEthereum")
			v0Hash := v0.BurnTxHash(selector, pack.NewU256FromU8(0))

			Eventually(func() string {
				h := redisClient.Get(fmt.Sprintf("BTC/fromEthereum_%v", 0)).Val()
				return h
			}, 15*time.Second).Should(Equal(v0Hash.String()))
		})

		It("should not process burn events in the future or the past", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			watcher, redisClient, burnIn, _ := init(ctx, time.Second, true)
			defer redisClient.Close()

			go watcher.Run(ctx)
			// Wait for the block number to be picked up
			time.Sleep(time.Second)

			burnIn <- BurnLogResult{
				Result: ethereumbindings.MintGatewayLogicV1LogBurn{
					To:        []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
					Amount:    big.NewInt(10000),
					N:         big.NewInt(0),
					IndexedTo: [32]byte{},
					Raw: types.Log{
						BlockNumber: 1,
					},
				},
			}

			burnIn <- BurnLogResult{
				Result: ethereumbindings.MintGatewayLogicV1LogBurn{
					To:        []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
					Amount:    big.NewInt(10000),
					N:         big.NewInt(0),
					IndexedTo: [32]byte{},
					Raw: types.Log{
						BlockNumber: 100000000,
					},
				},
			}

			selector := tx.Selector("BTC/fromEthereum")
			v0Hash := v0.BurnTxHash(selector, pack.NewU256FromU8(0))

			time.Sleep(2 * time.Second)

			h := redisClient.Get(fmt.Sprintf("BTC/fromEthereum_%v", 0)).Val()
			Expect(h).NotTo(Equal(v0Hash.String()))

		})

		It("should process logs in block batches", func() {
			ethC, err := ethclient.Dial("https://multichain-staging.renproject.io/testnet/geth")
			Expect(err).NotTo(HaveOccurred())

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			watcher, redisClient, burnIn, _ := init(ctx, time.Second, true)
			defer redisClient.Close()

			currentBlock, err := ethC.HeaderByNumber(ctx, nil)
			Expect(err).NotTo(HaveOccurred())
			blockNumber := currentBlock.Number.Uint64()

			// Set the last checked block some time in the past
			redisClient.Set("BTC/fromEthereum_lastCheckedBlock", blockNumber-5000, 0)
			go watcher.Run(ctx)

			burnIn <- BurnLogResult{
				Result: ethereumbindings.MintGatewayLogicV1LogBurn{
					To:        []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
					Amount:    big.NewInt(10000),
					N:         big.NewInt(0),
					IndexedTo: [32]byte{},
					Raw: types.Log{
						BlockNumber: blockNumber,
					},
				},
			}

			burnIn <- BurnLogResult{
				Result: ethereumbindings.MintGatewayLogicV1LogBurn{
					To:        []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
					Amount:    big.NewInt(10000),
					N:         big.NewInt(1),
					IndexedTo: [32]byte{},
					Raw: types.Log{
						BlockNumber: blockNumber - 4500,
					},
				},
			}

			burnIn <- BurnLogResult{
				Result: ethereumbindings.MintGatewayLogicV1LogBurn{
					To:        []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
					Amount:    big.NewInt(10000),
					N:         big.NewInt(2),
					IndexedTo: [32]byte{},
					Raw: types.Log{
						BlockNumber: blockNumber - 1500,
					},
				},
			}

			close(burnIn)

			selector := tx.Selector("BTC/fromEthereum")
			v0Hash := v0.BurnTxHash(selector, pack.NewU256FromU8(0))

			time.Sleep(2 * time.Second)
			h := redisClient.Get(fmt.Sprintf("BTC/fromEthereum_%v", 0)).Val()
			// We should not see the first transaction, because it is too new
			Expect(h).NotTo(Equal(v0Hash.String()))

			v0Hash = v0.BurnTxHash(selector, pack.NewU256FromU8(1))
			h = redisClient.Get(fmt.Sprintf("BTC/fromEthereum_%v", 1)).Val()
			// We should see the second transaction, because it falls in the first batch
			Expect(h).To(Equal(v0Hash.String()))

			v0Hash = v0.BurnTxHash(selector, pack.NewU256FromU8(2))
			h = redisClient.Get(fmt.Sprintf("BTC/fromEthereum_%v", 2)).Val()
			// We should not see the third transaction, because it falls in the fourth batch
			Expect(h).NotTo(Equal(v0Hash.String()))

			// After a few more ticks, we should see the fourth tx
			time.Sleep(4 * time.Second)

			h = redisClient.Get(fmt.Sprintf("BTC/fromEthereum_%v", 2)).Val()
			Expect(h).To(Equal(v0Hash.String()))
		})

		It("should handle failures to fetch burn events", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			watcher, redisClient, burnIn, _ := init(ctx, time.Second, true)
			defer redisClient.Close()

			go watcher.Run(ctx)
			// Wait for the block number to be picked up
			time.Sleep(time.Second)

			burnIn <- BurnLogResult{
				Error: fmt.Errorf("failed to fetch burn logs"),
			}

			selector := tx.Selector("BTC/fromEthereum")
			v0Hash := v0.BurnTxHash(selector, pack.NewU256FromU8(0))

			// check that we recover
			// we need to retry a few times to simulate the logs being
			// re-fetched on failure
			for range [2]bool{} {
				burnIn <- BurnLogResult{
					Result: ethereumbindings.MintGatewayLogicV1LogBurn{
						To:        []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
						Amount:    big.NewInt(10000),
						N:         big.NewInt(0),
						IndexedTo: [32]byte{},
						Raw:       types.Log{},
					},
				}
			}

			Eventually(func() string {
				h := redisClient.Get(fmt.Sprintf("BTC/fromEthereum_%v", 0)).Val()
				return h
			}, 15*time.Second).Should(Equal(v0Hash.String()))
		})

		It("should handle malformed addresses", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			watcher, redisClient, burnIn, _ := init(ctx, time.Second, true)
			defer redisClient.Close()

			go watcher.Run(ctx)
			// Wait for the block number to be picked up
			time.Sleep(time.Second)

			burnIn <- BurnLogResult{
				Result: ethereumbindings.MintGatewayLogicV1LogBurn{
					To:        []byte("not a valid address"),
					Amount:    big.NewInt(10000),
					N:         big.NewInt(0),
					IndexedTo: [32]byte{},
					Raw:       types.Log{},
				},
			}

			selector := tx.Selector("BTC/fromEthereum")
			v0Hash := v0.BurnTxHash(selector, pack.NewU256FromU8(0))

			time.Sleep(2 * time.Second)

			h := redisClient.Get(fmt.Sprintf("BTC/fromEthereum_%v", 0)).Val()
			Expect(h).NotTo(Equal(v0Hash.String()))

			// check that we recover
			burnIn <- BurnLogResult{
				Result: ethereumbindings.MintGatewayLogicV1LogBurn{
					To:        []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
					Amount:    big.NewInt(10000),
					N:         big.NewInt(1),
					IndexedTo: [32]byte{},
					Raw:       types.Log{},
				},
			}

			v0Hash = v0.BurnTxHash(selector, pack.NewU256FromU8(1))

			Eventually(func() string {
				h := redisClient.Get(fmt.Sprintf("BTC/fromEthereum_%v", 1)).Val()
				return h
			}, 15*time.Second).Should(Equal(v0Hash.String()))
		})

		It("should handle last checked blocks ahead of eth node", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			watcher, redisClient, burnIn, _ := init(ctx, time.Second, true)
			defer redisClient.Close()

			// Set the block far in the future
			redisClient.Set("BTC/fromEthereum_lastCheckedBlock", 1000000000, 0)
			go watcher.Run(ctx)

			// channels will block until read from, so we make this concurrent
			go func() {
				burnIn <- BurnLogResult{
					Result: ethereumbindings.MintGatewayLogicV1LogBurn{
						To:        []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
						Amount:    big.NewInt(10000),
						N:         big.NewInt(0),
						IndexedTo: [32]byte{},
						Raw:       types.Log{},
					},
				}
				close(burnIn)
			}()

			selector := tx.Selector("BTC/fromEthereum")
			v0Hash := v0.BurnTxHash(selector, pack.NewU256FromU8(0))

			// Over 2 intervals, the mock burn should not be processed
			time.Sleep(2 * time.Second)

			h := redisClient.Get(fmt.Sprintf("BTC/fromEthereum_%v", 0)).Val()
			Expect(h).NotTo(Equal(v0Hash.String()))

			// clear the block, and check that we recover gracefully
			redisClient.Del("BTC/fromEthereum_lastCheckedBlock")

			Eventually(func() string {
				h := redisClient.Get(fmt.Sprintf("BTC/fromEthereum_%v", 0)).Val()
				return h
			}, 15*time.Second).Should(Equal(v0Hash.String()))
		})

		It("should handle redis failures", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			watcher, redisClient, burnIn, redismock := init(ctx, time.Second, true)
			defer redisClient.Close()
			// Pause during latest block collection
			redismock.SetError("test error")
			go watcher.Run(ctx)

			// channels will block until read from, so we make this concurrent
			go func() {
				burnIn <- BurnLogResult{
					Result: ethereumbindings.MintGatewayLogicV1LogBurn{
						To:        []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
						Amount:    big.NewInt(10000),
						N:         big.NewInt(0),
						IndexedTo: [32]byte{},
						Raw:       types.Log{},
					},
				}
				close(burnIn)
			}()

			selector := tx.Selector("BTC/fromEthereum")
			v0Hash := v0.BurnTxHash(selector, pack.NewU256FromU8(0))

			// Over 2 intervals, the mock burn should not be processed
			time.Sleep(2 * time.Second)

			h := redisClient.Get(fmt.Sprintf("BTC/fromEthereum_%v", 0)).Val()
			Expect(h).NotTo(Equal(v0Hash.String()))

			// Unset error so that we can pass
			redismock.SetError("")
			Eventually(func() string {
				h := redisClient.Get(fmt.Sprintf("BTC/fromEthereum_%v", 0)).Val()
				return h
			}, 15*time.Second).Should(Equal(v0Hash.String()))
		})

		It("should handle duplication", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			watcher, redisClient, burnIn, _ := init(ctx, time.Second, true)
			defer redisClient.Close()
			go watcher.Run(ctx)

			func() {
				for range [100]bool{} {
					burnIn <- BurnLogResult{
						Result: ethereumbindings.MintGatewayLogicV1LogBurn{
							To:        []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
							Amount:    big.NewInt(10000),
							N:         big.NewInt(0),
							IndexedTo: [32]byte{},
							Raw:       types.Log{},
						},
					}

					select {
					case <-ctx.Done():
						return
					}
				}
			}()
			close(burnIn)

			selector := tx.Selector("BTC/fromEthereum")
			v0Hash := v0.BurnTxHash(selector, pack.NewU256FromU8(0))

			Eventually(func() string {
				h := redisClient.Get(fmt.Sprintf("BTC/fromEthereum_%v", 0)).Val()
				return h
			}, 15*time.Second).Should(Equal(v0Hash.String()))
		})

		It("should process 5k burns", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			watcher, redisClient, burnIn, _ := init(ctx, time.Second, true)
			defer redisClient.Close()
			go watcher.Run(ctx)

			for i := range [5000]bool{} {
				burnIn <- BurnLogResult{
					Result: ethereumbindings.MintGatewayLogicV1LogBurn{
						To:        []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
						Amount:    big.NewInt(10000),
						N:         big.NewInt(int64(i)),
						IndexedTo: [32]byte{},
						Raw:       types.Log{},
					},
				}
			}
			close(burnIn)

			selector := tx.Selector("BTC/fromEthereum")
			v0Hash := v0.BurnTxHash(selector, pack.NewU256FromU64(4999))

			Eventually(func() string {
				h := redisClient.Get(fmt.Sprintf("BTC/fromEthereum_%v", 4999)).Val()
				return h
			}, 15*time.Second).Should(Equal(v0Hash.String()))
		})

		It("should handle an intermittent responder", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			watcher, redisClient, burnIn, _ := init(ctx, time.Second, false)
			defer redisClient.Close()
			go watcher.Run(ctx)

			// channels will block until read from, so we make this concurrent
			go func() {
				for range [1000]bool{} {
					burnIn <- BurnLogResult{
						Result: ethereumbindings.MintGatewayLogicV1LogBurn{
							To:        []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
							Amount:    big.NewInt(10000),
							N:         big.NewInt(0),
							IndexedTo: [32]byte{},
							Raw:       types.Log{},
						},
					}
				}
				close(burnIn)
			}()

			selector := tx.Selector("BTC/fromEthereum")
			v0Hash := v0.BurnTxHash(selector, pack.NewU256FromU8(0))

			Eventually(func() string {
				h := redisClient.Get(fmt.Sprintf("BTC/fromEthereum_%v", 0)).Val()
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
