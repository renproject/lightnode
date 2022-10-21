package resolver

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/renproject/darknode/binding"
	"github.com/renproject/darknode/chainstate"
	"github.com/renproject/darknode/engine"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/id"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/lightnode/http"
	"github.com/renproject/multichain"
	"github.com/renproject/pack"
	"github.com/renproject/phi"
	"github.com/renproject/surge"
	"github.com/sirupsen/logrus"
)

// A txchecker reads SubmitTx requests from a channel and validates the details
// of the transaction. It will store the transaction if it is valid.
type txchecker struct {
	logger   logrus.FieldLogger
	bindings binding.Bindings
	requests <-chan http.RequestWithResponder
	verifier Verifier
	db       db.DB
	mu       *sync.Mutex
	screener Screener
}

type Verifier interface {
	VerifyTx(ctx context.Context, tx tx.Tx) error
}

type verifier struct {
	bindings binding.Bindings
	contract *chainstate.Contract
}

func NewVerifier(hostChains map[multichain.Chain]bool, bindings binding.Bindings, pubkey *id.PubKey) Verifier {
	// Convert host chains map to sorted list.
	chains := make([]string, 0, len(hostChains))
	for chain := range hostChains {
		chains = append(chains, string(chain))
	}
	sort.Strings(chains)

	// The verification for burn transactions uses the cross-chain contract to
	// verify the minted amount. As we do not keep track of the latest block
	// state inside the Lightnode, we assume the burned amount never exceeds the
	// tracked minted amount by setting it to the maximum U256 value.
	minted := make([]engine.XStateMinted, 0, len(chains))
	for _, chain := range chains {
		minted = append(minted, engine.XStateMinted{
			Chain:  multichain.Chain(chain),
			Amount: pack.MaxU256,
		})
	}
	shardState, err := pack.Encode(engine.XStateShardAccount{
		Nonce:   pack.NewU256([32]byte{}),
		Gnonces: []engine.XStateShardGnonce{},
	})
	if err != nil {
		panic(fmt.Sprintf("encoding shard state: %v", err))
	}
	// TODO: Once Key rotation is enabled, we will need to watch for epochs and
	// update the corresponding public keys.
	pubkeyBytes, err := surge.ToBinary(pubkey)
	if err != nil {
		panic(fmt.Sprintf("invalid renvm public Key: %v", err))
	}
	contractState, err := pack.Encode(engine.XState{
		LatestHeight:  pack.NewU256([32]byte{}),
		GasCap:        pack.NewU256([32]byte{}),
		GasLimit:      pack.NewU256([32]byte{}),
		GasPrice:      pack.NewU256([32]byte{}),
		MinimumAmount: pack.NewU256([32]byte{}),
		DustAmount:    pack.NewU256([32]byte{}),
		Shards: []engine.XStateShard{
			{
				Shard:  pack.Bytes32{},
				PubKey: pubkeyBytes,
				Queue:  []engine.XStateShardQueueItem{},
				State:  shardState,
			},
		},
		Minted: minted,
		Fees: engine.XStateFees{
			Unassigned: pack.NewU256([32]byte{}),
			Epochs:     []engine.XStateFeesEpoch{},
			Nodes:      []engine.XStateFeesNode{},
			HostChains: []engine.XStateFeesHostChains{},
		},
	})
	if err != nil {
		panic(fmt.Sprintf("encoding contract state: %v", err))
	}
	contract := chainstate.Contract{
		Address: "",
		State:   pack.Typed(contractState.(pack.Struct)),
	}
	return verifier{
		bindings: bindings,
		contract: &contract,
	}
}

func (v verifier) VerifyTx(ctx context.Context, transaction tx.Tx) error {
	// The verifier assumes all transactions are lock/mint/burn/release
	// transactions.
	return engine.XValidateLockMintBurnReleaseExtrinsicTx(chainstate.CodeContext{
		Context:  ctx,
		Bindings: v.bindings,
	}, v.contract, transaction)
}

// newTxChecker returns a new txchecker.
func newTxChecker(logger logrus.FieldLogger, requests <-chan http.RequestWithResponder, verifier Verifier, db db.DB, screener Screener, bindings binding.Bindings) txchecker {
	return txchecker{
		logger:   logger,
		bindings: bindings,
		requests: requests,
		verifier: verifier,
		db:       db,
		mu:       new(sync.Mutex),
		screener: screener,
	}
}

// Run starts the txchecker until the requests channel is closed.
func (tc *txchecker) Run() {
	workers := 2 * runtime.NumCPU()
	phi.ForAll(workers, func(_ int) {
		for req := range tc.requests {
			// Check if we already have the transaction
			params := req.Params.(jsonrpc.ParamsSubmitTx)
			_, err := tc.db.Tx(params.Tx.Hash)
			switch err {
			case sql.ErrNoRows:
				// continue verification
			case nil:
				// tx already exists, skip
				response := jsonrpc.ResponseSubmitTx{}
				req.Responder <- jsonrpc.NewResponse(req.ID, response, nil)
				continue
			default:
				// error getting transaction details from db
				req.RespondWithErr(jsonrpc.ErrorCodeInternal, fmt.Errorf("failed to check tx exsitence, err = %v", err))
				continue
			}

			// Verify the transaction details
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err = tc.verifier.VerifyTx(ctx, params.Tx)
			cancel()
			if err != nil {
				req.RespondWithErr(jsonrpc.ErrorCodeInvalidParams, err)
				continue
			}

			// If it's a mint, we want to check the tx sender address
			if params.Tx.Selector.IsMint() {
				chain := params.Tx.Selector.Source()
				txid := params.Tx.Input.Get("txid").(pack.Bytes)
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				senders, err := tc.bindings.TransactionSenders(ctx, chain, txid)
				cancel()
				if err != nil {
					tc.logger.Errorf("[txchecker] fail to screen address: %v", err)
				}
				for _, sender := range senders {
					sBlacklisted, err := tc.screener.IsBlacklisted(sender, chain)
					if err != nil {
						tc.logger.Errorf("[txchecker] fail to screen address: %v", err)
					}
					if sBlacklisted {
						req.RespondWithErr(jsonrpc.ErrorCodeInvalidParams, fmt.Errorf("sender address is blacklisted"))
						continue
					}
				}
			}

			chain := params.Tx.Selector.Destination()
			to := params.Tx.Input.Get("to").(pack.String)
			isBlacklisted, err := tc.screener.IsBlacklisted(to, chain)
			if err != nil {
				tc.logger.Errorf("[txchecker] fail to screen address: %v", err)
			}
			if isBlacklisted {
				req.RespondWithErr(jsonrpc.ErrorCodeInvalidParams, fmt.Errorf("target address is blacklisted"))
				continue
			}

			// Check if the transaction is a duplicate.
			if err := tc.checkDuplicate(params.Tx); err != nil {
				tc.logger.Errorf("[txchecker] cannot check tx duplication: %v", err)
				req.RespondWithErr(jsonrpc.ErrorCodeInternal, err)
				continue
			}

			// Write the response to the responder channel.
			response := jsonrpc.ResponseSubmitTx{}
			req.Responder <- jsonrpc.NewResponse(req.ID, response, nil)
		}
	})
}

func (tc *txchecker) checkDuplicate(transaction tx.Tx) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	_, err := tc.db.Tx(transaction.Hash)
	if err == sql.ErrNoRows {
		return tc.db.InsertTx(transaction)
	}
	return err
}
