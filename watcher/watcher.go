package watcher

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v7"
	"github.com/jbenet/go-base58"
	"github.com/renproject/darknode/binding"
	"github.com/renproject/darknode/engine"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/lightnode/compat/v0"
	"github.com/renproject/multichain"
	"github.com/renproject/pack"
)

type Watcher struct {
	opts     Options
	fetcher  Fetcher
	bindings binding.Bindings
	resolver jsonrpc.Resolver
	cache    redis.Cmdable
}

func NewWatcher(opts Options, fetcher Fetcher, binding binding.Bindings, resolver jsonrpc.Resolver, cache redis.Cmdable) Watcher {
	if opts.Chain == multichain.Solana {
		if len(opts.Assets) != 1 {
			panic("Solana needs to have one watcher per asset")
		}
	}
	return Watcher{
		opts:     opts,
		fetcher:  fetcher,
		bindings: binding,
		resolver: resolver,
		cache:    cache,
	}
}

func (watcher Watcher) Run(ctx context.Context) {
	ticker := time.NewTicker(watcher.opts.PollInterval)
	defer ticker.Stop()

	for {

		func() {
			innerCtx, innerCancel := context.WithTimeout(ctx, watcher.opts.PollInterval)
			defer innerCancel()

			watcher.watchLogs(innerCtx)
		}()

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (watcher Watcher) watchLogs(ctx context.Context) {
	// Fetch the latest block height of the chain
	currentHeight, err := watcher.fetcher.LatestBlockHeight(ctx)
	if err != nil {
		return
	}

	// for Eth, avoid checking blocks that might have shuffled
	if watcher.opts.Chain != multichain.Solana {
		currentHeight -= watcher.opts.ConfidenceInterval
	}

	// Fetch last checked block height from the cache
	lastHeight, err := watcher.lastCheckedBlockNumber(currentHeight)
	if err != nil {
		watcher.opts.Logger.Errorf("[watcher] error loading last checked block number: %v", err)
		return
	}

	if currentHeight <= lastHeight {
		watcher.opts.Logger.Debug("[watcher] tried to process old blocks")
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
	step := lastHeight + watcher.opts.MaxBlockAdvance
	if step < currentHeight {
		currentHeight = step
	}

	burnLogs, err := watcher.fetcher.FetchBurnLogs(ctx, lastHeight, currentHeight)
	if err != nil {
		watcher.opts.Logger.Warnf("[watcher] error fetching LogBurn events from=%v to=%v: %v", lastHeight, currentHeight, err)
		return
	}

	for _, log := range burnLogs {
		watcher.opts.Logger.Infof("[watcher] detected burn for %v on %v with nonce=%v", log.Asset, watcher.opts.Chain, log.Nonce)

		// Send the burn transaction to the resolver.
		params, err := watcher.burnToParams(log)
		if err != nil {
			watcher.opts.Logger.Errorf("[watcher] cannot convert %v burn on %v to renvm tx: %v", log.Asset, watcher.opts.Chain, err)
			continue
		}

		response := watcher.resolver.SubmitTx(ctx, 0, &params, nil)
		if response.Error != nil {
			watcher.opts.Logger.Errorf("[watcher] invalid burn transaction %v: %v", params, response.Error.Message)
			// return so that we retry, if the burnToParams are valid, the darknode should accept the tx
			// we assume that the only failure case would be RPC/darknode backpressure, so we backoff here
			return
		}
	}

	if err := watcher.cache.Set(watcher.key(), currentHeight, 0).Err(); err != nil {
		watcher.opts.Logger.Errorf("[watcher] error setting last checked block number in redis: %v", err)
		return
	}
}

// lastCheckedBlockNumber returns the last checked block number of Ethereum.
func (watcher Watcher) lastCheckedBlockNumber(currentBlockN uint64) (uint64, error) {
	last, err := watcher.cache.Get(watcher.key()).Uint64()
	if err == redis.Nil {
		// Initialise the pointer with current block number if it has not been yet.
		watcher.opts.Logger.Infof("[watcher] last checked block number not initialised, setting to %v", currentBlockN)
		if err := watcher.cache.Set(watcher.key(), currentBlockN, 0).Err(); err != nil {
			watcher.opts.Logger.Errorf("[watcher] cannot initialise last checked block in redis: %v", err)
			return 0, err
		}
		return currentBlockN, nil
	}

	return last, err
}

// key returns the key that is used to store the last checked block.
func (watcher Watcher) key() string {
	if watcher.opts.Chain == multichain.Solana {
		return fmt.Sprintf("%v_%v_lastCheckedBlock", watcher.opts.Chain, watcher.opts.Assets[0])
	}
	return fmt.Sprintf("%v_lastCheckedBlock", watcher.opts.Chain)
}

// burnToParams constructs params for a SubmitTx request with given ref.
func (watcher Watcher) burnToParams(eventLog EventInfo) (jsonrpc.ParamsSubmitTx, error) {
	selector := tx.Selector(fmt.Sprintf("%v/from%v", eventLog.Asset, watcher.opts.Chain))
	to, toDecoded, err := watcher.decodeToAddress(eventLog.Asset, eventLog.ToBytes)
	if err != nil {
		return jsonrpc.ParamsSubmitTx{}, err
	}
	watcher.opts.Logger.Infof("[watcher] %v burn parameters (to=%v, amount=%v, nonce=%v)", selector, string(to), eventLog.Amount, eventLog.Nonce)

	txindex := pack.U32(0)
	payload := pack.Bytes{}
	phash := engine.Phash(payload)
	nhash := engine.Nhash(eventLog.Nonce, eventLog.Txid, txindex)
	ghash := engine.Ghash(selector, phash, toDecoded, eventLog.Nonce)
	input, err := pack.Encode(engine.LockMintBurnReleaseInput{
		Txid:    eventLog.Txid,
		Txindex: txindex,
		Amount:  eventLog.Amount,
		Payload: payload,
		Phash:   phash,
		To:      pack.String(to),
		Nonce:   eventLog.Nonce,
		Nhash:   nhash,
		Gpubkey: pack.Bytes{},
		Ghash:   ghash,
	})
	if err != nil {
		return jsonrpc.ParamsSubmitTx{}, err
	}
	hash, err := tx.NewTxHash(tx.Version1, selector, pack.Typed(input.(pack.Struct)))
	if err != nil {
		return jsonrpc.ParamsSubmitTx{}, err
	}
	transaction := tx.Tx{
		Hash:     hash,
		Version:  tx.Version1,
		Selector: selector,
		Input:    pack.Typed(input.(pack.Struct)),
	}

	// Map the v0 burn txhash to v1 txhash so that it is still
	// queryable
	// We don't get the required data during tx submission rpc to track it there,
	// so we persist here in order to not re-filter all burn events
	v0Hash := v0.BurnTxHash(selector, pack.NewU256(eventLog.Nonce))
	watcher.cache.Set(v0Hash.String(), transaction.Hash.String(), 0)

	// Map the selector + burn ref to the v0 hash so that we can return something
	// to ren-js v1
	watcher.cache.Set(fmt.Sprintf("%s_%v", selector, pack.NewU256(eventLog.Nonce).String()), v0Hash.String(), 0)

	return jsonrpc.ParamsSubmitTx{Tx: transaction}, nil
}

func (watcher Watcher) decodeToAddress(asset multichain.Asset, toBytes []byte) (multichain.Address, []byte, error) {
	// For v0 burn, `to` can be base58 encoded
	to := multichain.Address(toBytes)
	switch asset {
	case multichain.BTC, multichain.BCH, multichain.ZEC:
		decoder := v0.AddressEncodeDecoder(asset.OriginChain(), watcher.opts.Network)
		_, err := decoder.DecodeAddress(to)
		if err != nil {
			to = multichain.Address(base58.Encode(toBytes))
			_, err = decoder.DecodeAddress(to)
			if err != nil {
				return "", nil, err
			}
		}
	}

	toBytes, err := watcher.bindings.DecodeAddress(asset.OriginChain(), to)
	return to, toBytes, err
}
