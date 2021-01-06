package watcher

import (
	"context"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-redis/redis/v7"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/darknode/txengine"
	"github.com/renproject/darknode/txengine/txenginebindings/ethereumbindings"
	"github.com/renproject/id"
	v0 "github.com/renproject/lightnode/compat/v0"
	"github.com/renproject/multichain"
	"github.com/renproject/pack"
	"github.com/sirupsen/logrus"
)

// Watcher watches for event logs for burn transactions. These transactions are
// then forwarded to the cacher.
type Watcher struct {
	logger       logrus.FieldLogger
	gpubkey      pack.Bytes
	selector     tx.Selector
	bindings     txengine.Bindings
	ethClient    *ethclient.Client
	ethBindings  *ethereumbindings.MintGatewayLogicV1
	resolver     jsonrpc.Resolver
	cache        redis.Cmdable
	pollInterval time.Duration
}

// NewWatcher returns a new Watcher.
func NewWatcher(logger logrus.FieldLogger, selector tx.Selector, bindings txengine.Bindings, ethClient *ethclient.Client, ethBindings *ethereumbindings.MintGatewayLogicV1, resolver jsonrpc.Resolver, cache redis.Cmdable, distPubKey *id.PubKey, pollInterval time.Duration) Watcher {
	gpubkey := (*btcec.PublicKey)(distPubKey).SerializeCompressed()
	return Watcher{
		logger:       logger,
		gpubkey:      gpubkey,
		selector:     selector,
		bindings:     bindings,
		ethClient:    ethClient,
		ethBindings:  ethBindings,
		resolver:     resolver,
		cache:        cache,
		pollInterval: pollInterval,
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

	// Filter for all burn events in this range of blocks.
	iter, err := watcher.ethBindings.FilterLogBurn(
		&bind.FilterOpts{
			Context: ctx,
			Start:   last + 1, // Add one to avoid duplication.
			End:     &cur,
		},
		nil,
		nil,
	)
	if err != nil {
		watcher.logger.Errorf("[watcher] error filtering LogBurn events from=%v to=%v: %v", last, cur, err)
		return
	}

	// Loop through the logs and check if there are burn events.
	for iter.Next() {
		to := string(iter.Event.To)
		amount := iter.Event.Amount.Uint64()
		nonce := iter.Event.N.Uint64()
		watcher.logger.Infof("[watcher] detected burn for %v (to=%v, amount=%v, nonce=%v)", watcher.selector.String(), to, amount, nonce)

		var nonceBytes pack.Bytes32
		copy(nonceBytes[:], pack.NewU256FromU64(pack.NewU64(nonce)).Bytes())

		// Send the burn transaction to the resolver.
		params, err := watcher.burnToParams(iter.Event.Raw.TxHash.Bytes(), pack.NewU256FromU64(pack.NewU64(amount)), pack.String(to), nonceBytes, watcher.gpubkey)
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
	if err := iter.Error(); err != nil {
		watcher.logger.Errorf("[watcher] error iterating LogBurn events from=%v to=%v: %v", last, cur, err)
		return
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

// currentBlockNumber returns the current block number on Ethereum.
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
		if err := watcher.cache.Set(watcher.key(), currentBlockN-1, 0).Err(); err != nil {
			watcher.logger.Errorf("[watcher] cannot initialise last checked block in redis: %v", err)
			return 0, err
		}
		return currentBlockN - 1, nil
	}
	return last, err
}

// burnToParams constructs params for a SubmitTx request with given ref.
func (watcher Watcher) burnToParams(txid pack.Bytes, amount pack.U256, to pack.String, nonce pack.Bytes32, gpubkey pack.Bytes) (jsonrpc.ParamsSubmitTx, error) {
	burnChain := watcher.selector.Destination()
	toBytes, err := watcher.bindings.DecodeAddress(burnChain, multichain.Address(to))
	if err != nil {
		return jsonrpc.ParamsSubmitTx{}, err
	}

	txindex := pack.U32(0)
	payload := pack.Bytes{}
	phash := txengine.Phash(payload)
	nhash := txengine.Nhash(nonce, txid, txindex)
	ghash := txengine.Ghash(watcher.selector, phash, toBytes, nonce)
	input, err := pack.Encode(txengine.CrossChainInput{
		Txid:    txid,
		Txindex: txindex,
		Amount:  amount,
		Payload: payload,
		Phash:   phash,
		To:      to,
		Nonce:   nonce,
		Nhash:   nhash,
		Gpubkey: gpubkey,
		Ghash:   ghash,
	})
	if err != nil {
		return jsonrpc.ParamsSubmitTx{}, err
	}
	transaction, err := tx.NewTx(watcher.selector, pack.Typed(input.(pack.Struct)))
	if err != nil {
		return jsonrpc.ParamsSubmitTx{}, err
	}

	// Map the v0 burn txhash to v1 txhash so that it is still
	// queryable
	v0Hash := v0.TxHash(watcher.selector, ghash, txid, txindex)
	watcher.cache.Set(v0Hash.String(), transaction.Hash.String(), 0)

	return jsonrpc.ParamsSubmitTx{Tx: transaction}, nil
}
