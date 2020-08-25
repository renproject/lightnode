package resolver

import (
	"context"
	"crypto/ecdsa"
	"database/sql"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/darknode/txengine"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/lightnode/http"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

// A txchecker reads SubmitTx requests from a channel and validates the details
// of the transaction. It will store the transaction if it is valid.
type txchecker struct {
	mu        *sync.Mutex
	logger    logrus.FieldLogger
	requests  <-chan http.RequestWithResponder
	disPubkey ecdsa.PublicKey
	bindings  txengine.Bindings
	db        db.DB
}

// newTxChecker returns a new txchecker.
func newTxChecker(logger logrus.FieldLogger, requests <-chan http.RequestWithResponder, key ecdsa.PublicKey, bindings txengine.Bindings, db db.DB) txchecker {
	return txchecker{
		mu:        new(sync.Mutex),
		logger:    logger,
		requests:  requests,
		disPubkey: key,
		bindings:  bindings,
		db:        db,
	}
}

// Run starts the txchecker until the requests channel is closed.
func (tc *txchecker) Run() {
	workers := 2 * runtime.NumCPU()
	phi.ForAll(workers, func(_ int) {
		for req := range tc.requests {
			params := req.Params.(jsonrpc.ParamsSubmitTx)
			tx, err := tc.verify(params)
			if err != nil {
				req.RespondWithErr(jsonrpc.ErrorCodeInvalidParams, err)
				continue
			}

			// Check if the transaction is a duplicate.
			tx, err = tc.checkDuplicate(tx)
			if err != nil {
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

func (tc *txchecker) verify(params jsonrpc.ParamsSubmitTx) (tx.Tx, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	transaction := &params.Tx
	switch {
	case params.Tx.Selector.IsLockAndMint():
		if err := txengine.UTXOLockAndMintPreparation(ctx, tc.bindings, transaction); err != nil {
			return tx.Tx{}, err
		}
		return *transaction, nil
	case params.Tx.Selector.IsBurnAndRelease():
		if err := txengine.UTXOBurnAndReleasePreparation(ctx, tc.bindings, transaction); err != nil {
			return tx.Tx{}, err
		}
		return *transaction, nil
	case params.Tx.Selector.IsBurnAndMint():
		if err := txengine.AccountBurnAndMintPreparation(ctx, tc.bindings, transaction); err != nil {
			return tx.Tx{}, err
		}
		return *transaction, nil
	default:
		return tx.Tx{}, fmt.Errorf("non-exhaustive pattern: selector %v", params.Tx.Selector)
	}
}

func (tc *txchecker) checkDuplicate(transaction tx.Tx) (tx.Tx, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	stored, err := tc.db.Tx(transaction.Hash)
	if err == sql.ErrNoRows {
		return transaction, tc.db.InsertTx(transaction)
	}
	return stored, err
}
