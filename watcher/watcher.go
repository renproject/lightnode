package watcher

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/go-redis/redis/v7"
	"github.com/renproject/darknode/abi"
	"github.com/renproject/darknode/consensus/txcheck/transform/blockchain"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/lightnode/resolver"
	"github.com/sirupsen/logrus"
)

// Watcher watches for event logs for shift out transactions. These transactions
// are then forwarded to the cacher.
type Watcher struct {
	logger       logrus.FieldLogger
	addr         abi.Address
	pool         blockchain.ConnPool
	resolver     *resolver.Resolver
	cache        *redis.Client
	PollInterval time.Duration
}

// NewWatcher returns a new Watcher.
func NewWatcher(logger logrus.FieldLogger, addr abi.Address, pool blockchain.ConnPool, resolver *resolver.Resolver, cache *redis.Client, pollInterval time.Duration) Watcher {
	return Watcher{
		logger:       logger,
		addr:         addr,
		pool:         pool,
		resolver:     resolver,
		cache:        cache,
		PollInterval: pollInterval,
	}
}

// Run starts the watcher until the context is canceled.
func (watcher Watcher) Run(ctx context.Context) {
	ticker := time.NewTicker(watcher.PollInterval)
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
	ctx, cancel := context.WithTimeout(parent, watcher.PollInterval)
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

	// Filter for all shift out events in this range of blocks.
	shifter := watcher.pool.ShifterByAddress(watcher.addr)
	iter, err := shifter.FilterLogBurn(
		&bind.FilterOpts{
			Context: ctx,
			Start:   last + 1, // Add one to avoid duplication.
			End:     &cur,
		},
		nil,
		nil,
	)
	if err != nil {
		watcher.logger.Errorf("[watcher] error filtering LogShiftOut events from=%v to=%v: %v", last, cur, err)
		return
	}

	// Loop through the logs and check if there are ShiftOut events.
	for iter.Next() {
		ref := iter.Event.N.Uint64()
		amount := iter.Event.Amount.Uint64()
		watcher.logger.Infof("[watcher] detect shift out for %v, ref=%v, amount=%v SATs/ZATs", watcher.addr, ref, amount)

		// Send the ShiftOut tx to the resolver.
		params := watcher.shiftOutToParams(ref)
		watcher.resolver.SubmitTx(ctx, 0, &params, nil)
	}
	if err := iter.Error(); err != nil {
		watcher.logger.Errorf("[watcher] error iterating LogShiftOut events from=%v to=%v: %v", last, cur, err)
		return
	}

	if err := watcher.cache.Set(watcher.key(), cur, 0).Err(); err != nil {
		watcher.logger.Errorf("[watcher] error setting last checked block number in redis: %v", err)
		return
	}
}

// key returns the key that is used to store the last checked block.
func (watcher Watcher) key() string {
	return fmt.Sprintf("%v_lastCheckedBlock", watcher.addr)
}

// currentBlockNumber returns the current block number on Ethereum.
func (watcher Watcher) currentBlockNumber(ctx context.Context) (uint64, error) {
	client := watcher.pool.EthClient()
	currentBlock, err := client.HeaderByNumber(ctx, nil)
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

// shiftOutToParams constructs params for a SubmitTx request with given ref.
func (watcher Watcher) shiftOutToParams(ref uint64) jsonrpc.ParamsSubmitTx {
	tx := abi.Tx{
		Hash: abi.B32{},
		To:   watcher.addr,
		In: abi.Args{{
			Name:  "ref",
			Type:  abi.TypeU64,
			Value: abi.U64{Int: big.NewInt(int64(ref))},
		}},
	}
	return jsonrpc.ParamsSubmitTx{Tx: tx}
}
