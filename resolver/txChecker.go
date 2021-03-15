package resolver

import (
	"context"
	"database/sql"
	"runtime"
	"sync"
	"time"

	"github.com/renproject/darknode/binding"
	"github.com/renproject/darknode/chainstate"
	"github.com/renproject/darknode/engine"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/lightnode/http"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

// A txchecker reads SubmitTx requests from a channel and validates the details
// of the transaction. It will store the transaction if it is valid.
type txchecker struct {
	logger   logrus.FieldLogger
	requests <-chan http.RequestWithResponder
	verifier Verifier
	db       db.DB
	mu       *sync.Mutex
}

type Verifier interface {
	VerifyTx(ctx context.Context, tx tx.Tx) error
}

type verifier struct {
	bindings binding.Bindings
}

func NewVerifier(bindings binding.Bindings) Verifier {
	return verifier{
		bindings: bindings,
	}
}

func (v verifier) VerifyTx(ctx context.Context, tx tx.Tx) error {
	err := engine.XValidateLockMintBurnReleaseExtrinsicTx(chainstate.CodeContext{
		Context:  ctx,
		Bindings: v.bindings,
	}, nil, tx)
	return err
}

// newTxChecker returns a new txchecker.
func newTxChecker(logger logrus.FieldLogger, requests <-chan http.RequestWithResponder, verifier Verifier, db db.DB) txchecker {
	return txchecker{
		logger:   logger,
		requests: requests,
		verifier: verifier,
		db:       db,
		mu:       new(sync.Mutex),
	}
}

// Run starts the txchecker until the requests channel is closed.
func (tc *txchecker) Run() {
	workers := 2 * runtime.NumCPU()
	phi.ForAll(workers, func(_ int) {
		for req := range tc.requests {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

			params := req.Params.(jsonrpc.ParamsSubmitTx)

			err := tc.verifier.VerifyTx(ctx, params.Tx)
			cancel()
			if err != nil {
				req.RespondWithErr(jsonrpc.ErrorCodeInvalidParams, err)
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
