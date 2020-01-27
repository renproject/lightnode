package watcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/go-redis/redis/v7"
	"github.com/renproject/darknode/abi"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/lightnode/blockchain"
	"github.com/renproject/lightnode/http"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

// Watcher watches event logs of a specific shifter contract on Ethereum. It
// extract information from the log and converts it an ShiftOut tx and send it
// to validator for next stage.
type Watcher struct {
	logger       logrus.FieldLogger
	addr         abi.Address
	pool         blockchain.ConnPool
	cache        *redis.Client
	validator    phi.Sender
	PollInterval time.Duration
}

// NewWatcher returns a new Watcher.
func NewWatcher(logger logrus.FieldLogger, addr abi.Address, pool blockchain.ConnPool, validator phi.Sender, cache *redis.Client, pollInterval time.Duration) Watcher {
	return Watcher{
		logger:       logger,
		addr:         addr,
		pool:         pool,
		cache:        cache,
		validator:    validator,
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

// watchLogShiftOuts checks logs which happens between current block number and
// last checked block number. It converts any ShiftOut log to a jsonrpc.Request
// and send it to validator for next stage.
func (watcher Watcher) watchLogShiftOuts(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, watcher.PollInterval)
	defer cancel()

	// Get current block number and last checked block number
	cur, err := watcher.currentBlockNumber(ctx)
	if err != nil {
		watcher.logger.Errorf("[watcher] error loading eth block header: %v", err)
		return
	}
	last := watcher.lastCheckedBlockNumber(cur)

	// Filter for all epoch events in this range of blocks
	shifter := watcher.pool.ShifterByAddress(watcher.addr)
	iter, err := shifter.FilterLogShiftOut(
		&bind.FilterOpts{
			Context: ctx,
			Start:   last + 1, // +1 to avoid duplication
			End:     &cur,
		},
		nil,
		nil,
	)
	if err != nil {
		watcher.logger.Errorf("error filtering LogShiftOut events from=%v to=%v: %v", last, cur, err)
		return
	}

	// Loop through the logs and check if there are ShiftOut events.
	for iter.Next() {
		ref := iter.Event.ShiftID.Uint64()
		amount := iter.Event.Amount.Uint64()
		log.Printf("DETECT %v SHIFT OUT: %v => %v SATs/ZATs", watcher.addr, ref, amount)

		// send the ShiftOut tx to validator
		req := watcher.shiftOutToRequest(ref)
		if ok := watcher.validator.Send(req); !ok {
			watcher.logger.Error("[watcher] fail to send request to validator")
			return
		}
	}
	if err := iter.Error(); err != nil {
		watcher.logger.Errorf("error iterating LogShiftOut events from=%v to=%v: %v", last, cur, err)
		return
	}

	if err := watcher.cache.Set(watcher.key(), cur, 0).Err(); err != nil {
		watcher.logger.Errorf("error setting last checked block number in redis: %v", err)
	}
}

// key returns the key which we use to store the last checked block in redis.
func (watcher Watcher) key() string {
	return fmt.Sprintf("%v_lastCheckedBlock", watcher.addr)
}

// currentBlockNumber returns the current block number of Etherem.
func (watcher Watcher) currentBlockNumber(ctx context.Context) (uint64, error) {
	client := watcher.pool.EthClient()
	currentBlock, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return 0, err
	}
	return currentBlock.Number.Uint64(), nil
}

// currentBlockNumber returns the current block number of Etherem.
func (watcher Watcher) lastCheckedBlockNumber(currentBlockN uint64) uint64 {
	last, err := watcher.cache.Get(watcher.key()).Uint64()
	if err != nil {
		// Init the pointer with last checked block number if not set.
		if err == redis.Nil {
			if err := watcher.cache.Set(watcher.key(), currentBlockN-1, 0).Err(); err != nil {
				watcher.logger.Panicf("cannot initialize last checked block in redis, err = %v", err)
			}
			last = currentBlockN - 1
		} else {
			watcher.logger.Errorf("cannot get last checked block in redis, err = %v", err)
			return 0
		}
	}
	return last
}

// shiftOutToRequest construct a new request with given ref.
func (watcher Watcher) shiftOutToRequest(ref uint64) http.RequestWithResponder {
	tx := abi.Tx{
		Hash: abi.B32{},
		To:   watcher.addr,
		In: abi.Args{{
			Name:  "ref",
			Type:  abi.TypeU64,
			Value: abi.U64{Int: big.NewInt(int64(ref))},
		}},
	}
	data, err := json.Marshal(jsonrpc.ParamsSubmitTx{Tx: tx})
	if err != nil {
		watcher.logger.Errorf("error marshaling ShiftOut event : %v", err)
		return http.RequestWithResponder{}
	}
	req := jsonrpc.Request{
		Version: "2.0",
		ID:      0,
		Method:  jsonrpc.MethodSubmitTx,
		Params:  data,
	}
	return http.NewRequestWithResponder(context.Background(), req, "")
}
