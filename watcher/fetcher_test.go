package watcher_test

import (
	"context"
	"fmt"
	"math/big"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	solanaRPC "github.com/dfuse-io/solana-go/rpc"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/renproject/darknode/binding"
	"github.com/renproject/lightnode/watcher"
	"github.com/renproject/multichain"
	"github.com/renproject/pack"
	"go.uber.org/zap"
)

var _ = Describe("fetcher", func() {
	Context("Ethereum fetcher", func() {
		It("should be able to fetch latest block height", func() {
			logger := logrus.New()
			binding := initBindings()
			assets := []multichain.Asset{multichain.BTC}
			fetcher := watcher.NewEthFetcher(logger, multichain.Ethereum, binding, assets)
			latestBlock, err := fetcher.LatestBlockHeight(context.Background())
			Expect(err).ShouldNot(HaveOccurred())

			// Check the returned block height has a timestamp within 1 minute
			client, err := ethclient.Dial("https://multichain-staging.renproject.io/testnet/kovan")
			Expect(err).ShouldNot(HaveOccurred())
			block, err := client.BlockByNumber(context.Background(), big.NewInt(int64(latestBlock)))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(time.Now().Unix() - int64(block.Time())).Should(BeNumerically("<", 60))
		})

		It("should be able to fetch burn event from the contract", func() {
			logger := logrus.New()
			binding := initBindings()
			assets := []multichain.Asset{
				multichain.BTC,
			}
			fetcher := watcher.NewEthFetcher(logger, multichain.Ethereum, binding, assets)

			// First btc burn on kovan is on block 18121453
			// Try a block range before that and expect zero event returned
			from, to := uint64(18058240), uint64(18058252)
			events, err := fetcher.FetchBurnLogs(context.Background(), from, to)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(events)).Should(Equal(0))

			// There's no burns in this range according to etherscan
			from, to = uint64(18058254), uint64(18058264)
			events, err = fetcher.FetchBurnLogs(context.Background(), from, to)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(events)).Should(Equal(0))

			// Try a range which has the first burn
			from, to = uint64(18058250), uint64(18058260)
			events, err = fetcher.FetchBurnLogs(context.Background(), from, to)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(events)).Should(Equal(1))
			Expect(events[0].BlockNumber).Should(Equal(pack.NewU64(18058253)))
		})

		It("should be able to fetch burn event for multiple assets on the same chain", func() {
			logger := logrus.New()
			binding := initBindings()
			assets := []multichain.Asset{
				multichain.BTC,  // 23520344
				multichain.LUNA, // 23528857
			}
			fetcher := watcher.NewEthFetcher(logger, multichain.Ethereum, binding, assets)
			from, to := uint64(23520000), uint64(23529000)
			events, err := fetcher.FetchBurnLogs(context.Background(), from, to)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(events)).Should(Equal(2))

			eventAssets := map[multichain.Asset]struct{}{}
			for _, event := range events {
				eventAssets[event.Asset] = struct{}{}
			}
			Expect(len(events)).Should(Equal(2))
		})
	})

	Context("Solana fetcher", func() {
		It("should be able to fetch latest burn for the asset", func() {
			binding := initBindings()
			gatewayAddr := binding.ContractGateway(multichain.Solana, multichain.BTC)
			Expect(gatewayAddr).ShouldNot(Equal(""))
			solClient := solanaRPC.NewClient("https://api.devnet.solana.com")

			// For Solana, the block height is actually the burn count
			logger := logrus.New()
			fetcher := watcher.NewSolFetcher(logger, solClient, multichain.BTC, string(gatewayAddr))
			latestBlock, err := fetcher.LatestBlockHeight(context.Background())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(latestBlock).Should(BeNumerically(">", 0))

			// Try latest 5 burn for time reason.
			for i := latestBlock - 5; i < latestBlock; i++ {
				events, err := fetcher.FetchBurnLogs(context.Background(), i, i+1)
				// There might be burns with invalid params which we want to skip
				if err != nil {
					continue
				}

				for _, event := range events {
					// Asset should be the expected asset
					Expect(event.Asset).Should(Equal(multichain.BTC))

					// Fields should be non-nil
					Expect(event.Txid).ShouldNot(Equal(pack.Bytes{}))
					Expect(event.Nonce).ShouldNot(Equal(pack.Bytes32{}))
					Expect(event.Amount.GreaterThan(pack.NewU256FromU64(0))).Should(BeTrue())
					Expect(event.ToBytes).ShouldNot(Equal(pack.Bytes{}))
				}
			}
		})
	})
})

func initBindings() *binding.Binding {
	loggerConfig := zap.NewProductionConfig()
	logger, err := loggerConfig.Build()
	if err != nil {
		panic(fmt.Errorf("cannot init logger: %v", err))
	}

	bindingsOpts := binding.DefaultOptions().
		WithLogger(logger).
		WithNetwork(multichain.NetworkTestnet).
		WithChainOptions(multichain.Bitcoin, binding.ChainOptions{
			RPC:           "https://multichain-staging.renproject.io/testnet/bitcoind",
			Confirmations: pack.U64(0),
		}).
		WithChainOptions(multichain.Dogecoin, binding.ChainOptions{
			RPC:           "https://multichain-staging.renproject.io/testnet/dogecoind",
			Confirmations: pack.U64(0),
		}).
		WithChainOptions(multichain.Terra, binding.ChainOptions{
			RPC:           "https://multichain-staging.renproject.io/testnet/terrad",
			Confirmations: pack.U64(0),
		}).
		WithChainOptions(multichain.Ethereum, binding.ChainOptions{
			RPC:           "https://multichain-staging.renproject.io/testnet/kovan",
			Confirmations: pack.U64(0),
			Registry:      "0x5076a1F237531fa4dC8ad99bb68024aB6e1Ff701",
			Extras: map[pack.String]pack.String{
				"protocol": "0x9e2Ed544eE281FBc4c00f8cE7fC2Ff8AbB4899D1",
			},
		}).
		WithChainOptions(multichain.Solana, binding.ChainOptions{
			RPC:           "https://api.devnet.solana.com",
			Confirmations: pack.U64(0),
			Registry:      "REGrPFKQhRneFFdUV3e9UDdzqUJyS6SKj88GdXFCRd2",
		})

	return binding.New(bindingsOpts, nil)
}
