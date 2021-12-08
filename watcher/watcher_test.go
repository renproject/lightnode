package watcher_test

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"testing/quick"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/alicebob/miniredis/v2"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcutil"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/ethereum/go-ethereum/crypto"
	filaddress "github.com/filecoin-project/go-address"
	"github.com/go-redis/redis/v7"
	"github.com/renproject/darknode/engine"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/jsonrpc/jsonrpcresolver"
	"github.com/renproject/id"
	"github.com/renproject/lightnode/compat/v0"
	"github.com/renproject/lightnode/watcher"
	"github.com/renproject/multichain"
	"github.com/renproject/multichain/chain/bitcoincash"
	"github.com/renproject/multichain/chain/terra"
	"github.com/renproject/multichain/chain/zcash"
	"github.com/renproject/pack"
	"github.com/renproject/surge"
)

type MockFetcher struct {
	handleLatestBlockHeight func(ctx context.Context) (uint64, error)
	handleFetchBurnLogs     func(ctx context.Context, from, to uint64) ([]watcher.EventInfo, error)
}

func (m *MockFetcher) LatestBlockHeight(ctx context.Context) (uint64, error) {
	return m.handleLatestBlockHeight(ctx)
}

func (m *MockFetcher) FetchBurnLogs(ctx context.Context, from, to uint64) ([]watcher.EventInfo, error) {
	return m.handleFetchBurnLogs(ctx, from, to)
}

func initRedis() redis.Cmdable {
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	return redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
}

var _ = Describe("Watcher", func() {
	Context("when watching events", func() {
		It("should query the latest block height once each time when pulling", func() {
			redisClient := initRedis()
			fetcher := &MockFetcher{}
			bindings := initBindings()
			resovler := jsonrpcresolver.OkResponder()
			options := watcher.DefaultOptions().
				WithNetwork(multichain.NetworkTestnet).
				WithChain(multichain.Ethereum).
				WithAssets([]multichain.Asset{multichain.BTC, multichain.LUNA}).
				WithPollInterval(3 * time.Second).
				WithConfidenceInterval(0)
			w := watcher.NewWatcher(options, fetcher, bindings, resovler, redisClient)

			// Make sure we are only pulling once
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			// Make sure the latestBlock is only called once
			counter := 0
			fetcher.handleFetchBurnLogs = func(ctx context.Context, from, to uint64) ([]watcher.EventInfo, error) {
				return nil, nil
			}
			fetcher.handleLatestBlockHeight = func(ctx context.Context) (uint64, error) {
				counter++
				return 100, nil
			}

			w.Run(ctx)
			Expect(counter).Should(Equal(1))
		})

		It("should modify the current height with a confidence interval for evm", func() {
			redisClient := initRedis()
			fetcher := &MockFetcher{}
			bindings := initBindings()
			resovler := jsonrpcresolver.OkResponder()
			confidenceInterval := uint64(10)
			options := watcher.DefaultOptions().
				WithNetwork(multichain.NetworkTestnet).
				WithChain(multichain.Ethereum).
				WithAssets([]multichain.Asset{multichain.BTC}).
				WithPollInterval(3 * time.Second).
				WithConfidenceInterval(confidenceInterval)
			w := watcher.NewWatcher(options, fetcher, bindings, resovler, redisClient)

			// Make sure we are only pulling once
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			// Make sure the latestBlock is only called once
			latestHeight := uint64(100)
			fetcher.handleFetchBurnLogs = func(ctx context.Context, from, to uint64) ([]watcher.EventInfo, error) {
				return nil, nil
			}
			fetcher.handleLatestBlockHeight = func(ctx context.Context) (uint64, error) {
				return latestHeight, nil
			}

			w.Run(ctx)
			height, err := redisClient.Get("Ethereum_lastCheckedBlock").Uint64()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(height).Should(Equal(latestHeight - confidenceInterval))
		})

		It("should limit the maximum of blocks to process each time when pulling", func() {
			redisClient := initRedis()
			fetcher := &MockFetcher{}
			bindings := initBindings()
			resovler := jsonrpcresolver.OkResponder()
			maxAdvance := uint64(10)
			options := watcher.DefaultOptions().
				WithNetwork(multichain.NetworkTestnet).
				WithChain(multichain.Ethereum).
				WithAssets([]multichain.Asset{multichain.BTC}).
				WithPollInterval(3 * time.Second).
				WithMaxBlockAdvance(maxAdvance).
				WithConfidenceInterval(0)
			w := watcher.NewWatcher(options, fetcher, bindings, resovler, redisClient)

			// Make sure we are only pulling once
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			// Make sure the latestBlock is only called once
			latestHeight := uint64(100)
			Expect(redisClient.Set("Ethereum_lastCheckedBlock", 0, 0).Err()).Should(Succeed())
			fetcher.handleFetchBurnLogs = func(ctx context.Context, from, to uint64) ([]watcher.EventInfo, error) {
				return nil, nil
			}
			fetcher.handleLatestBlockHeight = func(ctx context.Context) (uint64, error) {
				return latestHeight, nil
			}

			w.Run(ctx)
			height, err := redisClient.Get("Ethereum_lastCheckedBlock").Uint64()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(height).Should(Equal(maxAdvance))
		})

		It("should convert the burn log to a renvm tx and submit to the resolver", func() {
			redisClient := initRedis()
			fetcher := &MockFetcher{}
			bindings := initBindings()
			resovler := &jsonrpcresolver.Callbacks{}
			options := watcher.DefaultOptions().
				WithNetwork(multichain.NetworkTestnet).
				WithChain(multichain.Ethereum).
				WithAssets([]multichain.Asset{multichain.BTC})
			w := watcher.NewWatcher(options, fetcher, bindings, resovler, redisClient)

			latestBlock := uint64(0)
			test := func() bool {
				// Make sure we are only pulling once
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				// Simulate a burn event
				event := RandomEventInfo()
				fetcher.handleFetchBurnLogs = func(ctx context.Context, from, to uint64) ([]watcher.EventInfo, error) {
					return []watcher.EventInfo{event}, nil
				}
				fetcher.handleLatestBlockHeight = func(ctx context.Context) (uint64, error) {
					latestBlock += 10
					return latestBlock, nil
				}

				resovler.SubmitTxHandler = func(ctx context.Context, i interface{}, tx *jsonrpc.ParamsSubmitTx, request *http.Request) jsonrpc.Response {
					var input engine.LockMintBurnReleaseInput
					Expect(pack.Decode(&input, tx.Tx.Input)).Should(Succeed())
					Expect(input.Txid).Should(Equal(event.Txid))
					Expect(input.Amount.Equal(event.Amount)).Should(BeTrue())
					Expect(input.To).Should(Equal(pack.String(event.ToBytes)))
					Expect(input.Nonce).Should(Equal(event.Nonce))
					return jsonrpc.NewResponse(i, map[string]bool{"ok": true}, nil)
				}
				// Verify the burn tx sent to resolver
				w.Run(ctx)
				return true
			}

			Expect(quick.Check(test, nil)).NotTo(HaveOccurred())
		})

		It("should not progress the pointer if we get any error fetching the log", func() {
			redisClient := initRedis()
			fetcher := &MockFetcher{}
			bindings := initBindings()
			resovler := jsonrpcresolver.OkResponder()
			options := watcher.DefaultOptions().
				WithNetwork(multichain.NetworkTestnet).
				WithChain(multichain.Ethereum).
				WithAssets([]multichain.Asset{multichain.BTC}).
				WithPollInterval(3 * time.Second)
			w := watcher.NewWatcher(options, fetcher, bindings, resovler, redisClient)

			// Set a starting height
			startingHeight := uint64(rand.Int())
			Expect(redisClient.Set("Ethereum_lastCheckedBlock", startingHeight, 0).Err()).Should(Succeed())

			test := func() bool {
				// Make sure we are only pulling once
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				fetcher.handleFetchBurnLogs = func(ctx context.Context, from, to uint64) ([]watcher.EventInfo, error) {
					return nil, fmt.Errorf("testing")
				}
				fetcher.handleLatestBlockHeight = func(ctx context.Context) (uint64, error) {
					return startingHeight + 100, nil
				}
				w.Run(ctx)

				// Verify the pointer is not moving
				height, err := redisClient.Get("Ethereum_lastCheckedBlock").Uint64()
				Expect(err).ShouldNot(HaveOccurred())
				Expect(height).Should(Equal(startingHeight))
				return true
			}
			Expect(quick.Check(test, nil)).NotTo(HaveOccurred())
		})
	})
})

func RandomEventInfo() watcher.EventInfo {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	txid := RandomBytes(r, 20)
	addr := RandomGoodAddress(multichain.Bitcoin, multichain.NetworkTestnet)

	return watcher.EventInfo{
		Asset:       multichain.BTC,
		Txid:        txid,
		Amount:      pack.U256{}.Generate(r, 100).Interface().(pack.U256),
		ToBytes:     []byte(addr),
		Nonce:       pack.Bytes32{}.Generate(r, 100).Interface().(pack.Bytes32),
		BlockNumber: 0,
	}
}

func RandomBytes(r *rand.Rand, size int) pack.Bytes {
	data := make([]byte, size)
	if _, err := r.Read(data); err != nil {
		panic(err)
	}
	return data
}

func RandomGoodAddress(chain multichain.Chain, network multichain.Network) pack.String {
	key := id.NewPrivKey()

	switch chain {
	// UTXO-based chain
	case multichain.Bitcoin, multichain.DigiByte, multichain.Dogecoin:
		params := v0.NetParams(chain, network)
		key := btcec.PublicKey(key.PublicKey)
		pubKeyHash := btcutil.Hash160(key.SerializeCompressed())
		addr, err := btcutil.NewAddressPubKeyHash(pubKeyHash, params)
		if err != nil {
			panic(err)
		}
		return pack.String(addr.EncodeAddress())
	case multichain.BitcoinCash:
		key := btcec.PublicKey(key.PublicKey)
		params := v0.NetParams(chain, network)
		pubKeyHash := btcutil.Hash160(key.SerializeCompressed())
		addr, err := bitcoincash.NewAddressPubKeyHash(pubKeyHash, params)
		if err != nil {
			panic(err)
		}
		return pack.String(addr.EncodeAddress())
	case multichain.Zcash:
		key := btcec.PublicKey(key.PublicKey)
		pubKeyHash := btcutil.Hash160(key.SerializeCompressed())
		addr, err := zcash.NewAddressPubKeyHash(pubKeyHash, v0.ZcashNetParams(network))
		if err != nil {
			panic(err)
		}
		return pack.String(addr.EncodeAddress())

	// Ethereum-like chain
	case multichain.Avalanche, multichain.Ethereum, multichain.Goerli, multichain.BinanceSmartChain, multichain.Fantom, multichain.Polygon, multichain.Arbitrum, multichain.Moonbeam:
		addr := crypto.PubkeyToAddress(key.PublicKey)
		return pack.String(addr.Hex())

	// Account-based chain
	case multichain.Filecoin:
		serialisedPubKey := crypto.FromECDSAPub(&key.PublicKey)
		addr, err := filaddress.NewSecp256k1Address(serialisedPubKey)
		if err != nil {
			panic(err)
		}
		return pack.String(addr.String())
	case multichain.Terra:
		compressedPubKey, err := surge.ToBinary((id.PubKey)(key.PublicKey))
		if err != nil {
			panic(err)
		}
		pubKey := secp256k1.PubKey{Key: compressedPubKey}
		addrEncodeDecoder := terra.NewAddressEncodeDecoder()
		addr, err := addrEncodeDecoder.EncodeAddress(pubKey.Address().Bytes())
		if err != nil {
			panic(err)
		}
		return pack.String(addr)
	case multichain.Solana:
		// todo :
		return "FsaLodPu4VmSwXGr3gWfwANe4vKf8XSZcCh1CEeJ3jpD"
	default:
		panic(fmt.Errorf("AddressFromPubkey : unknown blockchain = %v", chain))
	}
}