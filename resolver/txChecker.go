package resolver

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/darknode/txengine"
	"github.com/renproject/darknode/txpool"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/lightnode/http"
	"github.com/renproject/pack"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

// A txchecker reads SubmitTx requests from a channel and validates the details
// of the transaction. It will store the transaction if it is valid.
type txchecker struct {
	logger   logrus.FieldLogger
	requests <-chan http.RequestWithResponder
	verifier txpool.Verifier
	db       db.DB
	mu       *sync.Mutex
}

// newTxChecker returns a new txchecker.
func newTxChecker(logger logrus.FieldLogger, requests <-chan http.RequestWithResponder, verifier txpool.Verifier, db db.DB) txchecker {
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
			err := tc.castToV1(&params.Tx)
			err = tc.verifier.VerifyTx(ctx, &params.Tx)
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

func (tc *txchecker) castToV1(transaction *tx.Tx) error {
	if transaction.Version == tx.Version1 {
		return nil
	}

	// utxoPack := pack.NewStruct(
	// 	"txHash": pack.Bytes32(),
	// 	"vOut": pack.String(),
	// )

	utxo, ok := transaction.Input.Get("utxo").(pack.Struct)
	if !ok {
		return fmt.Errorf("unexpected type for payload: expected pack.Struct, got %v", transaction.Input.Get("utxo").Type())
	}

	txid, ok := utxo.Get("txHash").(pack.Bytes)
	if !ok {
		return fmt.Errorf("unexpected type for utxo txHash: expected pack.Bytes, got %v", utxo.Get("txHash").Type())
	}

	txindex, ok := utxo.Get("vOut").(pack.U32)
	if !ok {
		return fmt.Errorf("unexpected type for utxo vOut: expected pack.u32, got %v", utxo.Get("vOut").Type())
	}

	payload, ok := transaction.Input.Get("p").(pack.Bytes)
	if !ok {
		return fmt.Errorf("unexpected type for payload: expected pack.Bytes, got %v", transaction.Input.Get("payload").Type())
	}

	// TODO: determine selector for given contract address
	token, ok := transaction.Input.Get("token").(pack.String)
	if !ok {
		return fmt.Errorf("unexpected type for payload: expected pack.Bytes, got %v", transaction.Input.Get("payload").Type())
	}

	legacyselectorParts := strings.Split(token.String(), "0")
	asset := legacyselectorParts[0]
	legacyselectorParts = strings.Split(legacyselectorParts[1], "2")
	destchain := legacyselectorParts[1]
	direction := "to"

	sel := tx.Selector(asset + "/" + direction + destchain)
	phash := txengine.Phash(payload)

	to, ok := transaction.Input.Get("to").(pack.String)
	if !ok {
		return fmt.Errorf("unexpected type for to: expected pack.String, got %v", transaction.Input.Get("to").Type())
	}

	nonce, ok := transaction.Input.Get("n").(pack.Bytes32)
	if !ok {
		return fmt.Errorf("unexpected type for nonce: expected pack.Bytes32, got %v", transaction.Input.Get("nonce").Type())
	}

	ghash := txengine.Ghash(sel, phash, []byte(to), nonce)

	// TODO: fetch public key for given asset/contract
	// TODO: figure out how to deserialize utxo from pack, so that we can get
	//     txid and txindex

	nhash := txengine.Nhash(nonce, txid, txindex)
	transaction.Version = tx.Version1
	transaction.Input.Set("phash", phash)
	transaction.Input.Set("ghash", ghash)
	transaction.Input.Set("nhash", nhash)
	transaction.Input.Set("txid", txid)
	transaction.Input.Set("txindex", txindex)
	transaction.Input.Set("gpubkey", pack.String("will_be_replaced"))

	return nil
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
