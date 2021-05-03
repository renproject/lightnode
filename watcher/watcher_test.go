package watcher_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	_ "github.com/mattn/go-sqlite3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/watcher"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v7"
	"github.com/renproject/darknode/binding"
	"github.com/renproject/darknode/jsonrpc/jsonrpcresolver"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/id"
	v0 "github.com/renproject/lightnode/compat/v0"
	"github.com/renproject/multichain"
	"github.com/renproject/pack"
	"github.com/sirupsen/logrus"

	solanaRPC "github.com/dfuse-io/solana-go/rpc"
)

type MockBurnLogFetcher struct {
	BurnIn chan BurnLogResult
	state  *MockState
}

type MockState struct {
	mu         sync.Mutex
	futureLogs []BurnLogResult
	logs       uint
}

func NewMockBurnLogFetcher(burnIn chan BurnLogResult) MockBurnLogFetcher {
	return MockBurnLogFetcher{
		BurnIn: burnIn,
		state: &MockState{
			futureLogs: make([]BurnLogResult, 0),
			logs:       0,
		},
	}
}

// Loop through the logs and pipe them to the channel if they are within the
// provided block range. Otherwise, store them in a list for later use.
func (fetcher MockBurnLogFetcher) FetchBurnLogs(ctx context.Context, from uint64, to uint64) (chan BurnLogResult, error) {
	x := make(chan BurnLogResult)

	go func() {

		newLogs := make([]BurnLogResult, 0)
		for i := range fetcher.state.futureLogs {
			e := fetcher.state.futureLogs[i]
			if e.Result.BlockNumber.Uint64() > to {
				newLogs = append(newLogs, e)
				continue
			}
			if e.Result.BlockNumber.Uint64() < to && e.Result.BlockNumber.Uint64() > from {
				x <- e
			}
		}

		fetcher.state.mu.Lock()
		fetcher.state.futureLogs = newLogs
		fetcher.state.mu.Unlock()
		// We always need to drain the channel,
		// even if the context finishes before the channel is drained,
		// to prevent blocking
		for e := range fetcher.BurnIn {
			// If we haven't set a block number, don't filter
			if e.Result.BlockNumber == 0 {
				x <- e
				continue
			}

			// If the event is in the future, cache it for later
			if e.Result.BlockNumber.Uint64() > to {

				fetcher.state.mu.Lock()
				fetcher.state.futureLogs = append(fetcher.state.futureLogs, e)
				fetcher.state.mu.Unlock()
				continue
			}

			if e.Result.BlockNumber.Uint64() > from {
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
		logger.SetLevel(logrus.ErrorLevel)

		selector := tx.Selector("BTC/fromEthereum")

		mockResolver := jsonrpcresolver.OkResponder()
		if !reliableResponder {
			mockResolver = jsonrpcresolver.RandomResponder()
		}

		pubk := id.NewPrivKey().PubKey()

		if err != nil {
			logger.Panicf("failed to create account client: %v", err)
		}

		burnIn := make(chan BurnLogResult)
		bindingsOpts := binding.DefaultOptions().
			WithNetwork("localnet").
			WithChainOptions(multichain.Bitcoin, binding.ChainOptions{
				RPC:           pack.String("https://multichain-staging.renproject.io/testnet/bitcoind"),
				Confirmations: pack.U64(0),
			}).
			WithChainOptions(multichain.Ethereum, binding.ChainOptions{
				RPC:           pack.String("https://multichain-staging.renproject.io/testnet/kovan"),
				Confirmations: pack.U64(0),
				Protocol:      pack.String("0x5045E727D9D9AcDe1F6DCae52B078EC30dC95455"),
			})

		bindings := binding.New(bindingsOpts)
		if err != nil {
			logger.Panicf("bad bindings: %v", err)
		}

		ethClients := bindings.EthereumClients()
		ethClient := ethClients[multichain.Ethereum]
		fetcher := NewMockBurnLogFetcher(burnIn)
		heightFetcher := NewEthBlockHeightFetcher(ethClient)

		watcher := NewWatcher(logger, multichain.NetworkDevnet, selector, bindings, fetcher, heightFetcher, mockResolver, client, pubk, interval, 1000, 6)

		return watcher, client, burnIn, mr
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
				Result: BurnInfo{
					ToBytes: []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
					Amount:  pack.NewU256FromU64(10000),
					Nonce:   pack.NewU256FromU64(0).Bytes32(),
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
				Result: BurnInfo{
					ToBytes:     []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
					Amount:      pack.NewU256FromU64(10000),
					Nonce:       pack.NewU256FromU64(0).Bytes32(),
					BlockNumber: pack.NewU64(1),
				},
			}

			burnIn <- BurnLogResult{
				Result: BurnInfo{
					ToBytes:     []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
					Amount:      pack.NewU256FromU64(10000),
					Nonce:       pack.NewU256FromU64(1).Bytes32(),
					BlockNumber: pack.NewU64(uint64(time.Now().Unix() + 1000000)),
				},
			}

			selector := tx.Selector("BTC/fromEthereum")
			v0Hash := v0.BurnTxHash(selector, pack.NewU256FromU8(0))

			time.Sleep(2 * time.Second)

			h := redisClient.Get(fmt.Sprintf("BTC/fromEthereum_%v", 0)).Val()
			Expect(h).NotTo(Equal(v0Hash.String()))

		})

		It("should process logs in block batches", func() {
			ethC, err := ethclient.Dial("https://multichain-staging.renproject.io/testnet/kovan")
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
				Result: BurnInfo{
					ToBytes:     []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
					Amount:      pack.NewU256FromU64(10000),
					Nonce:       pack.NewU256FromU64(0).Bytes32(),
					BlockNumber: pack.NewU64(uint64(blockNumber)),
				},
			}

			burnIn <- BurnLogResult{
				Result: BurnInfo{
					ToBytes:     []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
					Amount:      pack.NewU256FromU64(10000),
					Nonce:       pack.NewU256FromU64(1).Bytes32(),
					BlockNumber: pack.NewU64(uint64(blockNumber - 4500)),
				},
			}

			burnIn <- BurnLogResult{
				Result: BurnInfo{
					ToBytes:     []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
					Amount:      pack.NewU256FromU64(10000),
					Nonce:       pack.NewU256FromU64(2).Bytes32(),
					BlockNumber: pack.NewU64(uint64(blockNumber - 1500)),
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
					Result: BurnInfo{
						ToBytes: []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
						Amount:  pack.NewU256FromU64(10000),
						Nonce:   pack.NewU256FromU64(0).Bytes32(),
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
				Result: BurnInfo{
					ToBytes: []byte("not an address"),
					Amount:  pack.NewU256FromU64(10000),
					Nonce:   pack.NewU256FromU64(0).Bytes32(),
				},
			}

			selector := tx.Selector("BTC/fromEthereum")
			v0Hash := v0.BurnTxHash(selector, pack.NewU256FromU8(0))

			time.Sleep(2 * time.Second)

			h := redisClient.Get(fmt.Sprintf("BTC/fromEthereum_%v", 0)).Val()
			Expect(h).NotTo(Equal(v0Hash.String()))

			// check that we recover
			burnIn <- BurnLogResult{
				Result: BurnInfo{
					ToBytes: []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
					Amount:  pack.NewU256FromU64(10000),
					Nonce:   pack.NewU256FromU64(1).Bytes32(),
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
			redisClient.Set("BTC/fromEthereum_lastCheckedBlock", time.Now().Unix()+100000, 0)
			go watcher.Run(ctx)

			// channels will block until read from, so we make this concurrent
			go func() {
				burnIn <- BurnLogResult{
					Result: BurnInfo{
						ToBytes: []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
						Amount:  pack.NewU256FromU64(10000),
						Nonce:   pack.NewU256FromU64(0).Bytes32(),
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
					Result: BurnInfo{
						ToBytes: []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
						Amount:  pack.NewU256FromU64(10000),
						Nonce:   pack.NewU256FromU64(0).Bytes32(),
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
						Result: BurnInfo{
							ToBytes: []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
							Amount:  pack.NewU256FromU64(10000),
							Nonce:   pack.NewU256FromU64(0).Bytes32(),
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
					Result: BurnInfo{
						ToBytes: []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
						Amount:  pack.NewU256FromU64(10000),
						Nonce:   pack.NewU256FromU64(pack.U64(uint64(i))).Bytes32(),
					},
				}
			}
			close(burnIn)

			v0Hashes := make([]string, 5000)
			selector := tx.Selector("BTC/fromEthereum")
			for i := range v0Hashes {
				hash := v0.BurnTxHash(selector, pack.NewU256FromUint64(uint64(i))).String()
				v0Hashes = append(v0Hashes[:], hash)
			}

			Eventually(func() []string {
				hashes := make([]string, 5000)
				for i := range hashes {
					hash := redisClient.Get(fmt.Sprintf("BTC/fromEthereum_%v", i)).Val()
					hashes = append(hashes[:], hash)
				}
				return hashes
			}, 15*time.Second).Should(Equal(v0Hashes))
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
						Result: BurnInfo{
							ToBytes: []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
							Amount:  pack.NewU256FromU64(10000),
							Nonce:   pack.NewU256FromU64(0).Bytes32(),
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

		It("should be able to call filter logs on ethereum", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			bindingsOpts := binding.DefaultOptions().
				WithNetwork("localnet").
				WithChainOptions(multichain.Bitcoin, binding.ChainOptions{
					RPC:           pack.String("https://multichain-staging.renproject.io/testnet/bitcoind"),
					Confirmations: pack.U64(0),
				}).
				WithChainOptions(multichain.Ethereum, binding.ChainOptions{
					RPC:           pack.String("https://multichain-staging.renproject.io/testnet/kovan"),
					Confirmations: pack.U64(0),
					Protocol:      pack.String("0x59e23c087cA9bd9ce162875811CD6e99134D6d0F"),
				})

			bindings := binding.New(bindingsOpts)

			gateways := bindings.EthereumGateways()
			btcGateway := gateways[multichain.Ethereum][multichain.BTC]
			burnLogFetcher := NewEthBurnLogFetcher(btcGateway)

			results, err := burnLogFetcher.FetchBurnLogs(ctx, 0, 0)
			Expect(err).ToNot(HaveOccurred())

			// wait to see if the channel picks anything up
			time.Sleep(time.Second)

			for r := range results {
				Expect(r).To(BeEmpty())
			}

			results, err = burnLogFetcher.FetchBurnLogs(ctx, 23704992, 23704995)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() BurnLogResult {
				for r := range results {
					return r
				}
				return BurnLogResult{}
			}, 5*time.Second).Should(Equal(BurnLogResult{Result: BurnInfo{
				Txid:        []byte{42, 187, 84, 27, 111, 181, 207, 26, 30, 239, 244, 76, 7, 157, 157, 29, 159, 155, 62, 12, 65, 112, 124, 110, 85, 132, 116, 128, 171, 68, 197, 65},
				Amount:      pack.NewU256FromUint64(896853),
				ToBytes:     []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
				Nonce:       [32]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9, 231},
				BlockNumber: 23704993,
			}}))
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

		It("should be able to call filter logs on Solana", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			bindingsOpts := binding.DefaultOptions().
				WithNetwork("localnet").
				WithChainOptions(multichain.Bitcoin, binding.ChainOptions{
					RPC:           pack.String("https://multichain-staging.renproject.io/testnet/bitcoind"),
					Confirmations: pack.U64(0),
				}).
				// Tests against solana localnet
				WithChainOptions(multichain.Solana, binding.ChainOptions{
					RPC:      pack.String("http://0.0.0.0:8899"),
					Protocol: pack.String("DHpzwsdvAzq61PN9ZwQWg2hzwX8gYNfKAdsNKKtdKDux"),
				})

			bindings := binding.New(bindingsOpts)
			solClient := solanaRPC.NewClient(bindingsOpts.Chains[multichain.Solana].RPC.String())
			gateways := bindings.ContractGateways()
			btcGateway := gateways[multichain.Solana][multichain.BTC]
			burnLogFetcher := NewSolFetcher(solClient, string(btcGateway))

			results, err := burnLogFetcher.FetchBurnLogs(ctx, 0, 0)
			Expect(err).ToNot(HaveOccurred())

			// wait to see if the channel picks anything up
			time.Sleep(time.Second)

			for r := range results {
				Expect(r).To(BeEmpty())
			}

			results, err = burnLogFetcher.FetchBurnLogs(ctx, 0, 1)
			Expect(err).ToNot(HaveOccurred())

			log := BurnLogResult{}

			Eventually(func() BurnLogResult {
				for r := range results {
					log = r
					return r
				}
				return log
			}, 15*time.Second).Should(Equal(BurnLogResult{Result: BurnInfo{
				Txid:        []byte{153, 140, 140, 107, 117, 200, 223, 173, 87, 21, 66, 66, 195, 145, 206, 26, 31, 162, 156, 70, 162, 119, 69, 189, 118, 56, 220, 204, 164, 153, 85, 146, 25, 254, 167, 101, 114, 99, 107, 10, 56, 104, 100, 123, 11, 66, 173, 43, 231, 154, 180, 231, 178, 26, 4, 31, 178, 83, 116, 237, 166, 7, 179, 15},
				Amount:      pack.NewU256FromUint64(1000000000),
				ToBytes:     []byte{111, 156, 83, 29, 221, 210, 44, 11, 79, 156, 112, 96, 116, 20, 53, 247, 21, 98, 180, 2, 95, 155, 124, 199, 196},
				Nonce:       [32]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
				BlockNumber: 0,
			}}))

		})
	})

})
