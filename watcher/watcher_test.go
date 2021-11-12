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

// Flag to check whether ethereum client is progressing
// We need to wait a few seconds for new blocks,
// so it is something we only want to do once
var live = false

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
				Registry:      pack.String("0x707bBd01A54958d1c0303b29CAfA9D9fB2D61C10"),
				Extras: map[pack.String]pack.String{
					"protocol": "0x9e2Ed544eE281FBc4c00f8cE7fC2Ff8AbB4899D1",
				},
			})

		bindings := binding.New(bindingsOpts)
		if err != nil {
			logger.Panicf("bad bindings: %v", err)
		}

		ethClient := bindings.EthereumClient(multichain.Ethereum)
		fetcher := NewMockBurnLogFetcher(burnIn)
		heightFetcher := NewEthBlockHeightFetcher(ethClient)

		// Check if the ethereum client is progressing
		if live == false {
			h1, err := heightFetcher.FetchBlockHeight(ctx)
			time.Sleep(10 * time.Second)
			h2, err := heightFetcher.FetchBlockHeight(ctx)

			if h1 == h2 || err != nil {
				logger.Panicf("eth client has stalled: %v", err)
			}
			live = true
		}

		watcher := NewWatcher(logger, multichain.NetworkDevnet, selector, bindings, fetcher, heightFetcher, mockResolver, client, interval, 1000, 6)

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

			go func() {
				burnIn <- BurnLogResult{
					Result: BurnInfo{
						ToBytes: []byte("miMi2VET41YV1j6SDNTeZoPBbmH8B4nEx6"),
						Amount:  pack.NewU256FromU64(10000),
						Nonce:   pack.NewU256FromU64(0).Bytes32(),
					},
				}
			}()

			selector := tx.Selector("BTC/fromEthereum")
			v0Hash := v0.BurnTxHash(selector, pack.NewU256FromU8(0))

			Eventually(func() string {
				fmt.Printf("{%v}", redisClient.Keys("*"))
				fmt.Printf("{%v}", redisClient.Get("BTC/fromEthereum_lastCheckedBlock").Val())
				h := redisClient.Get(fmt.Sprintf("BTC/fromEthereum_%v", 0)).Val()
				return h
			}, 15*time.Second, time.Second).Should(Equal(v0Hash.String()))
		})

		It("should not process burn events in the future or the past", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

			defer cancel()
			watcher, redisClient, burnIn, _ := init(ctx, time.Second, true)
			defer redisClient.Close()

			go watcher.Run(ctx)
			// Wait for the block number to be picked up
			time.Sleep(time.Second)

			go func() {

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

			}()

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
				for range [100]bool{} {
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
					Registry:      pack.String("0x707bBd01A54958d1c0303b29CAfA9D9fB2D61C10"),
					Extras: map[pack.String]pack.String{
						"protocol": "0x9e2Ed544eE281FBc4c00f8cE7fC2Ff8AbB4899D1",
					},
				})

			bindings := binding.New(bindingsOpts)

			btcGateway := bindings.MintGateway(multichain.Ethereum, multichain.BTC)
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
				"t5d726GyG6Ejzu4nansAxgLY4VR5bMwDFD",
				"nXYwuk6dEF3Sm3jPgWjxz8brY6ArSzjvqz",
				"t14wczuvodunv3xzexobzywpbj6qpr6jwdrbkrmbq",
				"terra1ru5tjxvrdrwv2l9a4h24py4635x0crxmehw5hh",
				"t28Tc2BUTHifXthexsohy89umGdqMWLSUqw",
			}
			chains := []multichain.Chain{multichain.Bitcoin, multichain.BitcoinCash, multichain.DigiByte, multichain.Dogecoin, multichain.Filecoin, multichain.Terra, multichain.Zcash}
			networks := []multichain.Network{multichain.NetworkTestnet, multichain.NetworkMainnet}
			for i := range chains {
				for j := range networks {
					decoder := AddressEncodeDecoder(chains[i], networks[j])
					_, err := decoder.DecodeAddress(validTestnetAddrs[i])
					// If network is mainnet, fail to decode addresses
					// otherwise pass
					if networks[j] == multichain.NetworkMainnet && chains[i].IsUTXOBased() {
						// TODO: Test mainnet addresses for UTXO chains.
						Expect(err).To(HaveOccurred())
					} else {
						Expect(err).NotTo(HaveOccurred())
					}
				}
			}
		})

		It("should detect a burn on Solana", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			bindingsOpts := binding.DefaultOptions().WithNetwork("localnet").
				WithChainOptions(multichain.Bitcoin, binding.ChainOptions{
					RPC:           pack.String("https://multichain-staging.renproject.io/testnet/bitcoind"),
					Confirmations: pack.U64(0),
				}).
				// Tests against solana localnet
				WithChainOptions(multichain.Solana, binding.ChainOptions{
					RPC:      pack.String("http://0.0.0.0:8899"),
					Registry: pack.String("DHpzwsdvAzq61PN9ZwQWg2hzwX8gYNfKAdsNKKtdKDux"),
				})

			bindings := binding.New(bindingsOpts)
			solClient := solanaRPC.NewClient(bindingsOpts.Chains[multichain.Solana].RPC.String())
			btcGateway := bindings.ContractGateway(multichain.Solana, multichain.BTC)
			burnLogFetcher := NewSolFetcher(solClient, string(btcGateway))

			results, err := burnLogFetcher.FetchBurnLogs(ctx, 0, 0)
			Expect(err).ToNot(HaveOccurred())

			// wait to see if the channel picks anything up
			time.Sleep(time.Second)

			for r := range results {
				Expect(r).To(BeEmpty())
			}

			mr, err := miniredis.Run()
			if err != nil {
				panic(err)
			}

			client := redis.NewClient(&redis.Options{
				Addr: mr.Addr(),
			})

			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel)

			selector := tx.Selector("BTC/fromSolana")

			mockResolver := jsonrpcresolver.OkResponder()

			results, err = burnLogFetcher.FetchBurnLogs(ctx, 1, 2)
			Expect(err).ToNot(HaveOccurred())
			// We set the last checked block manually, because it will always start after the last checked burn
			client.Set("BTC/fromSolana_lastCheckedBlock", 1, 0)

			watcher := NewWatcher(logger, multichain.NetworkDevnet, selector, bindings, burnLogFetcher, burnLogFetcher, mockResolver, client, time.Second, 1000, 6)

			go watcher.Run(ctx)

			Eventually(func() string {
				h := client.Get(fmt.Sprintf("BTC/fromSolana_%v", 1)).Val()
				return h
			}, 15*time.Second).Should(Equal("t9INi66uVw1uUQ/Q3xcdnn5GuqJUiC+q7Ilr9Xot3rk="))
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
					Registry: pack.String("DHpzwsdvAzq61PN9ZwQWg2hzwX8gYNfKAdsNKKtdKDux"),
				})

			bindings := binding.New(bindingsOpts)
			solClient := solanaRPC.NewClient(bindingsOpts.Chains[multichain.Solana].RPC.String())
			btcGateway := bindings.ContractGateway(multichain.Solana, multichain.BTC)
			burnLogFetcher := NewSolFetcher(solClient, string(btcGateway))

			results, err := burnLogFetcher.FetchBurnLogs(ctx, 0, 0)
			Expect(err).ToNot(HaveOccurred())

			// wait to see if the channel picks anything up
			time.Sleep(time.Second)

			for r := range results {
				Expect(r).To(BeEmpty())
			}

			results, err = burnLogFetcher.FetchBurnLogs(ctx, 1, 2)
			Expect(err).ToNot(HaveOccurred())

			log := BurnLogResult{}

			Eventually(func() BurnLogResult {
				for r := range results {
					if r.Error != nil {
						results, err = burnLogFetcher.FetchBurnLogs(ctx, 1, 2)
						Expect(err).ToNot(HaveOccurred())
						continue
					}
					// We can't have reproducable signatures, so only check the other fields
					r.Result.Txid = []byte{}
					log = r
					return r
				}
				return log
			}, 15*time.Second).Should(Equal(BurnLogResult{Result: BurnInfo{
				Txid:        []byte{},
				Amount:      pack.NewU256FromUint64(100000000),
				ToBytes:     []byte("mumXH2WH8z8JMBuKrArV4XpNnf3xaR6Guy"),
				Nonce:       [32]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
				BlockNumber: 1,
			}}))

		})
	})

})
