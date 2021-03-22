package watcher

import (
	"context"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-redis/redis/v7"
	"github.com/jbenet/go-base58"
	"github.com/renproject/darknode/binding"
	"github.com/renproject/darknode/binding/ethereumbinding"
	"github.com/renproject/darknode/engine"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/id"
	v0 "github.com/renproject/lightnode/compat/v0"
	"github.com/renproject/multichain"
	"github.com/renproject/multichain/chain/bitcoin"
	"github.com/renproject/multichain/chain/bitcoincash"
	"github.com/renproject/multichain/chain/zcash"
	"github.com/renproject/pack"
	"github.com/sirupsen/logrus"
)

type BurnInfo struct {
	Txid        pack.Bytes
	Amount      pack.U256
	ToBytes     []byte
	Nonce       pack.Bytes32
	BlockNumber pack.U64
}

type BurnLogResult struct {
	Result BurnInfo
	Error  error
}

type BurnLogFetcher interface {
	FetchBurnLogs(ctx context.Context, from uint64, to uint64) (chan BurnLogResult, error)
}

type EthBurnLogFetcher struct {
	bindings *ethereumbinding.MintGatewayLogicV1
}

func NewBurnLogFetcher(bindings *ethereumbinding.MintGatewayLogicV1) EthBurnLogFetcher {
	return EthBurnLogFetcher{
		bindings: bindings,
	}
}

// This will fetch the burn event logs using the ethereum bindings and emit them via a channel
// We do this so that we can unit test the log handling without calling ethereum
func (fetcher EthBurnLogFetcher) FetchBurnLogs(ctx context.Context, from uint64, to uint64) (chan BurnLogResult, error) {
	iter, err := fetcher.bindings.FilterLogBurn(
		&bind.FilterOpts{
			Context: ctx,
			Start:   from,
			End:     &to,
		},
		nil,
		nil,
	)
	if err != nil {
		return nil, err
	}
	resultChan := make(chan BurnLogResult)

	go func() {
		func() {
			defer close(resultChan)
			for iter.Next() {
				nonce := iter.Event.N.Uint64()
				var nonceBytes pack.Bytes32
				copy(nonceBytes[:], pack.NewU256FromU64(pack.NewU64(nonce)).Bytes())

				result := BurnInfo{
					Txid:        iter.Event.Raw.TxHash.Bytes(),
					Amount:      pack.NewU256FromInt(iter.Event.Amount),
					ToBytes:     iter.Event.To,
					Nonce:       nonceBytes,
					BlockNumber: pack.NewU64(iter.Event.Raw.BlockNumber),
				}

				// Send the burn transaction to the resolver.
				resultChan <- BurnLogResult{Result: result}
				select {
				case <-ctx.Done():
					return
				}
			}
		}()

		// Always close the iter to clear the event subscription
		err = iter.Close()
		if err != nil {
			resultChan <- BurnLogResult{Error: err}
			return
		}

		// Iter should stop if an error occurs,
		// so no need to check on each iteration
		err := iter.Error()
		if err != nil {
			resultChan <- BurnLogResult{Error: err}
			return
		}

	}()

	return resultChan, nil
}

// Watcher watches for event logs for burn transactions. These transactions are
// then forwarded to the cacher.
type Watcher struct {
	network            multichain.Network
	logger             logrus.FieldLogger
	gpubkey            pack.Bytes
	selector           tx.Selector
	bindings           binding.Bindings
	burnLogFetcher     BurnLogFetcher
	ethClient          *ethclient.Client
	resolver           jsonrpc.Resolver
	cache              redis.Cmdable
	pollInterval       time.Duration
	maxBlockAdvance    uint64
	confidenceInterval uint64
}

// NewWatcher returns a new Watcher.
func NewWatcher(logger logrus.FieldLogger, network multichain.Network, selector tx.Selector, bindings binding.Bindings, burnLogFetcher BurnLogFetcher, ethClient *ethclient.Client, resolver jsonrpc.Resolver, cache redis.Cmdable, distPubKey *id.PubKey, pollInterval time.Duration, maxBlockAdvance uint64, confidenceInterval uint64) Watcher {
	gpubkey := (*btcec.PublicKey)(distPubKey).SerializeCompressed()
	return Watcher{
		logger:             logger,
		network:            network,
		gpubkey:            gpubkey,
		selector:           selector,
		bindings:           bindings,
		burnLogFetcher:     burnLogFetcher,
		ethClient:          ethClient,
		resolver:           resolver,
		cache:              cache,
		pollInterval:       pollInterval,
		maxBlockAdvance:    maxBlockAdvance,
		confidenceInterval: confidenceInterval,
	}
}

// Run starts the watcher until the context is canceled.
func (watcher Watcher) Run(ctx context.Context) {
	ticker := time.NewTicker(watcher.pollInterval)
	defer ticker.Stop()

	for {
		watcher.watchLogShiftOuts(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// watchLogShiftOuts checks logs that have occurred between current block number
// and the last checked block number. It constructs a `jsonrpc.Request` from
// these events and forwards them to the resolver.
func (watcher Watcher) watchLogShiftOuts(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, watcher.pollInterval)
	defer cancel()

	// Get current block number and last checked block number.
	cur, err := watcher.currentBlockNumber(ctx)
	if err != nil {
		watcher.logger.Errorf("[watcher] error loading eth block header: %v", err)
		return
	}
	last, err := watcher.lastCheckedBlockNumber(cur)
	if err != nil {
		watcher.logger.Errorf("[watcher] error loading last checked block number: %v", err)
		return
	}

	if cur <= last {
		watcher.logger.Warnf("[watcher] tried to process old blocks")
		// Make sure we do not process old events. This could occur if there is
		// an issue with the underlying blockchain node, for example if it needs
		// to resync.
		//
		// Processing old events is generally not an issue as the Lightnode will
		// drop transactions if it detects they have already been handled by the
		// Darknode, however in the case the transaction backlog builds up
		// substantially, it can cause the Lightnode to be rate limited by the
		// Darknode upon dispatching requests.
		return
	}

	// Only advance by a set number of blocks at a time to prevent over-subscription
	step := last + watcher.maxBlockAdvance
	if step < cur {
		cur = step
	}

	// avoid checking blocks that might have shuffled
	cur -= watcher.confidenceInterval

	// Fetch logs
	c, err := watcher.burnLogFetcher.FetchBurnLogs(ctx, last, cur)
	if err != nil {
		watcher.logger.Errorf("[watcher] error iterating LogBurn events from=%v to=%v: %v", last, cur, err)
		return
	}

	// Loop through the logs and check if there are burn events.
	for res := range c {
		if res.Error != nil {
			watcher.logger.Errorf("[watcher] error iterating LogBurn events from=%v to=%v: %v", last, cur, res.Error)
			return
		}
		burn := res.Result
		nonce := burn.Nonce
		amount := burn.Amount
		to := burn.ToBytes

		watcher.logger.Infof("[watcher] detected burn for %v (to=%v, amount=%v, nonce=%v)", watcher.selector.String(), string(to), amount, nonce)

		// Send the burn transaction to the resolver.
		params, err := watcher.burnToParams(burn.Txid, amount, to, nonce, watcher.gpubkey)
		if err != nil {
			watcher.logger.Errorf("[watcher] cannot get params from burn transaction (to=%v, amount=%v, nonce=%v): %v", to, amount, nonce, err)
			continue
		}
		response := watcher.resolver.SubmitTx(ctx, 0, &params, nil)
		if response.Error != nil {
			watcher.logger.Errorf("[watcher] invalid burn transaction %v: %v", params, response.Error.Message)
			continue
		}
	}

	if err := watcher.cache.Set(watcher.key(), cur, 0).Err(); err != nil {
		watcher.logger.Errorf("[watcher] error setting last checked block number in redis: %v", err)
		return
	}
}

// key returns the key that is used to store the last checked block.
func (watcher Watcher) key() string {
	return fmt.Sprintf("%v_lastCheckedBlock", watcher.selector.String())
}

// currentBlockNumber returns the current block number for the chain being watched
func (watcher Watcher) currentBlockNumber(ctx context.Context) (uint64, error) {
	currentBlock, err := watcher.ethClient.HeaderByNumber(ctx, nil)
	if err != nil {
		return 0, err
	}
	return currentBlock.Number.Uint64(), nil
}

// lastCheckedBlockNumber returns the last checked block number of Ethereum.
func (watcher Watcher) lastCheckedBlockNumber(currentBlockN uint64) (uint64, error) {
	last, err := watcher.cache.Get(watcher.key()).Uint64()
	// Initialise the pointer with current block number if it has not been yet.
	if err == redis.Nil {
		watcher.logger.Errorf("[watcher] last checked block number not initialised")
		if err := watcher.cache.Set(watcher.key(), currentBlockN-1, 0).Err(); err != nil {
			watcher.logger.Errorf("[watcher] cannot initialise last checked block in redis: %v", err)
			return 0, err
		}
		return currentBlockN - 1, nil
	}
	return last, err
}

// burnToParams constructs params for a SubmitTx request with given ref.
func (watcher Watcher) burnToParams(txid pack.Bytes, amount pack.U256, toBytes []byte, nonce pack.Bytes32, gpubkey pack.Bytes) (jsonrpc.ParamsSubmitTx, error) {

	// For v0 burn, `to` can be base58 encoded
	version := tx.Version1
	to := string(toBytes)
	switch watcher.selector.Asset() {
	case multichain.BTC, multichain.BCH, multichain.ZEC:
		decoder := AddressEncodeDecoder(watcher.selector.Asset().OriginChain(), watcher.network)
		_, err := decoder.DecodeAddress(multichain.Address(to))
		if err != nil {
			to = base58.Encode(toBytes)
			_, err = decoder.DecodeAddress(multichain.Address(to))
			if err != nil {
				return jsonrpc.ParamsSubmitTx{}, err
			}
			version = tx.Version0
		}
	}

	burnChain := watcher.selector.Destination()
	toBytes, err := watcher.bindings.DecodeAddress(burnChain, multichain.Address(to))
	if err != nil {
		return jsonrpc.ParamsSubmitTx{}, err
	}

	txindex := pack.U32(0)
	payload := pack.Bytes{}
	phash := engine.Phash(payload)
	nhash := engine.Nhash(nonce, txid, txindex)
	ghash := engine.Ghash(watcher.selector, phash, toBytes, nonce)
	input, err := pack.Encode(engine.LockMintBurnReleaseInput{
		Txid:    txid,
		Txindex: txindex,
		Amount:  amount,
		Payload: payload,
		Phash:   phash,
		To:      pack.String(to),
		Nonce:   nonce,
		Nhash:   nhash,
		Gpubkey: gpubkey,
		Ghash:   ghash,
	})
	if err != nil {
		return jsonrpc.ParamsSubmitTx{}, err
	}
	hash, err := tx.NewTxHash(version, watcher.selector, pack.Typed(input.(pack.Struct)))
	if err != nil {
		return jsonrpc.ParamsSubmitTx{}, err
	}
	transaction := tx.Tx{
		Hash:     hash,
		Version:  version,
		Selector: watcher.selector,
		Input:    pack.Typed(input.(pack.Struct)),
	}

	// Map the v0 burn txhash to v1 txhash so that it is still
	// queryable
	// We don't get the required data during tx submission rpc to track it there,
	// so we persist here in order to not re-filter all burn events
	v0Hash := v0.BurnTxHash(watcher.selector, pack.NewU256(nonce))
	watcher.cache.Set(v0Hash.String(), transaction.Hash.String(), 0)

	// Map the selector + burn ref to the v0 hash so that we can return something
	// to ren-js v1
	watcher.cache.Set(fmt.Sprintf("%s_%v", watcher.selector, pack.NewU256(nonce).String()), v0Hash.String(), 0)

	return jsonrpc.ParamsSubmitTx{Tx: transaction}, nil
}

func AddressEncodeDecoder(chain multichain.Chain, network multichain.Network) multichain.AddressEncodeDecoder {
	switch chain {
	case multichain.Bitcoin, multichain.DigiByte, multichain.Dogecoin:
		params := NetParams(network, chain)
		return bitcoin.NewAddressEncodeDecoder(params)
	case multichain.BitcoinCash:
		params := NetParams(network, chain)
		return bitcoincash.NewAddressEncodeDecoder(params)
	case multichain.Zcash:
		params := ZcashNetParams(network)
		return zcash.NewAddressEncodeDecoder(params)
	default:
		panic(fmt.Errorf("unknown chain %v", chain))
	}
}

func ZcashNetParams(network multichain.Network) *zcash.Params {
	switch network {
	case multichain.NetworkMainnet:
		return &zcash.MainNetParams
	case multichain.NetworkDevnet, multichain.NetworkTestnet:
		return &zcash.TestNet3Params
	default:
		return &zcash.RegressionNetParams
	}
}

func NetParams(network multichain.Network, chain multichain.Chain) *chaincfg.Params {
	switch chain {
	case multichain.Bitcoin, multichain.BitcoinCash:
		switch network {
		case multichain.NetworkMainnet:
			return &chaincfg.MainNetParams
		case multichain.NetworkDevnet, multichain.NetworkTestnet:
			return &chaincfg.TestNet3Params
		default:
			return &chaincfg.RegressionNetParams
		}
	default:
		panic(fmt.Errorf("cannot get network params: unknown chain %v", chain))
	}
}
