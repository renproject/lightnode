package confirmer

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"strings"
	"time"

	"github.com/renproject/darknode/binding"
	"github.com/renproject/darknode/engine"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/lightnode/http"
	"github.com/renproject/multichain"
	"github.com/renproject/pack"
	"github.com/renproject/phi"
)

// Confirmer handles requests that have been validated. It checks if requests
// have reached sufficient confirmations and stores those that have not to be
// checked later.
type Confirmer struct {
	options    Options
	dispatcher phi.Sender
	database   db.DB
	bindings   binding.Bindings
}

// New returns a new Confirmer.
func New(options Options, dispatcher phi.Sender, db db.DB, bindings binding.Bindings) Confirmer {
	return Confirmer{
		options:    options,
		dispatcher: dispatcher,
		database:   db,
		bindings:   bindings,
	}
}

// Run starts running the confirmer in the background which periodically checks
// confirmations for pending transactions and prunes old transactions.
func (confirmer *Confirmer) Run(ctx context.Context) {
	phi.ParBegin(func() {
		ticker := time.NewTicker(confirmer.options.PollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				confirmer.checkPendingTxs(ctx)
			}
		}
	}, func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()

		confirmer.prune()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				confirmer.prune()
			}
		}
	})
}

// checkPendingTxs checks if any pending transactions have received sufficient
// confirmations.
func (confirmer *Confirmer) checkPendingTxs(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, confirmer.options.PollInterval)
	go func() {
		defer cancel()
		<-ctx.Done()
	}()

	txs, err := confirmer.database.PendingTxs(72 * time.Hour)
	if err != nil {
		confirmer.options.Logger.Errorf("[confirmer] failed to read pending txs from database: %v", err)
		return
	}

	phi.ParForAll(txs, func(i int) {
		tx := txs[i]
		var confirmed bool
		switch {
		case tx.Selector.IsBurn() && tx.Selector.IsMint():
			confirmed = confirmer.burnAndMintTxConfirmed(ctx, tx)
		case tx.Selector.IsLock():
			confirmed = confirmer.lockTxConfirmed(ctx, tx)
		case tx.Selector.IsBurn():
			confirmed = confirmer.burnTxConfirmed(ctx, tx)
		}

		if confirmed {
			confirmer.options.Logger.Infof("tx=%v has reached sufficient confirmations", tx.Hash.String())
			confirmer.confirm(ctx, tx)
		}
	})
}

// confirm sends the transaction to the dispatcher and marks it as confirmed if
// it receives a non-error response from the Darknodes.
func (confirmer *Confirmer) confirm(ctx context.Context, transaction tx.Tx) {
	request, err := submitTxRequest(transaction)
	if err != nil {
		confirmer.options.Logger.Errorf("[confirmer] cannot construct json request for transaction: %v", err)
		return
	}
	req := http.NewRequestWithResponder(ctx, request.ID, request.Method, request.Params, url.Values{})
	if ok := confirmer.dispatcher.Send(req); !ok {
		confirmer.options.Logger.Errorf("[confirmer] cannot send message to dispatcher: too much back pressure")
		return
	}

	go func() {
		select {
		case <-ctx.Done():
			return
		case response := <-req.Responder:
			if response.Error == nil ||
				strings.Contains(response.Error.Message, "status=done") ||
				strings.Contains(response.Error.Message, "status = done") {
				confirmer.options.Logger.Infof("✅ successfully submitted tx=%v to darknodes", transaction.Hash.String())
			} else {
				confirmer.options.Logger.Errorf("[confirmer] getting error back when submitting tx=%v: [%v] %v", transaction.Hash.String(), response.Error.Code, response.Error.Message)
				return
			}

			if err := confirmer.database.UpdateStatus(transaction.Hash, db.TxStatusConfirmed); err != nil {
				confirmer.options.Logger.Errorf("[confirmer] cannot update transaction status: %v", err)
			}
		}
	}()
}

// lockTxConfirmed checks if a given lock transaction has received sufficient
// confirmations.
func (confirmer *Confirmer) lockTxConfirmed(ctx context.Context, transaction tx.Tx) bool {
	lockChain := transaction.Selector.Source()
	mintChain := transaction.Selector.Destination()
	switch {
	case lockChain.IsUTXOBased():
		input := engine.LockMintBurnReleaseInput{}
		if err := pack.Decode(&input, transaction.Input); err != nil {
			confirmer.options.Logger.Errorf("[confirmer] failed to decode input for tx=%v: %v", transaction.Hash.String(), err)
			return false
		}
		_, err := confirmer.bindings.UTXOLockInfo(ctx, lockChain, multichain.UTXOutpoint{
			Hash:  input.Txid,
			Index: input.Txindex,
		})
		if err != nil {
			if !strings.Contains(err.Error(), "insufficient confirmations") {
				confirmer.options.Logger.Errorf("[confirmer] cannot get output for utxo tx=%v (%v): %v", input.Txid.String(), transaction.Selector.String(), err)
			} else {
				confirmer.options.Logger.Warnf("[confirmer] cannot get output for utxo tx=%v (%v): %v", input.Txid.String(), transaction.Selector.String(), err)
			}

			// If the UTXO has already been spent, that means the transaction
			// has already been processed by RenVM and it can be marked as
			// complete.
			if strings.Contains(err.Error(), "result is nil") {
				if err := confirmer.database.UpdateStatus(transaction.Hash, db.TxStatusConfirmed); err != nil {
					confirmer.options.Logger.Errorf("[confirmer] updating status for tx=%v: %v", transaction.Hash.String(), err)
					return false
				}
			}

			return false
		}
	case lockChain.IsAccountBased():
		input := engine.LockMintBurnReleaseInput{}
		if err := pack.Decode(&input, transaction.Input); err != nil {
			confirmer.options.Logger.Errorf("[confirmer] failed to decode input for tx=%v: %v", transaction.Hash.String(), err)
			return false
		}
		_, _, err := confirmer.bindings.AccountLockInfo(ctx, lockChain, mintChain, transaction.Selector.Asset(), input.Txid, input.Payload, input.Nonce)
		if err != nil {
			if !strings.Contains(err.Error(), "insufficient confirmations") {
				confirmer.options.Logger.Errorf("[confirmer] cannot get output for account tx=%v (%v): %v", input.Txid.String(), transaction.Selector.String(), err)
			} else {
				confirmer.options.Logger.Warnf("[confirmer] cannot get output for account tx=%v (%v): %v", input.Txid.String(), transaction.Selector.String(), err)
			}
			return false
		}
	default:
		return false
	}
	return true
}

// burnTxConfirmed checks if a given burn transaction has received sufficient
// confirmations.
func (confirmer *Confirmer) burnTxConfirmed(ctx context.Context, transaction tx.Tx) bool {
	burnChain := transaction.Selector.Source()
	txid, ok := transaction.Input.Get("txid").(pack.Bytes)
	if !ok {
		confirmer.options.Logger.Errorf("[confirmer] failed to get txid for tx=%v", transaction.Hash.String())
		return false
	}
	nonce, ok := transaction.Input.Get("nonce").(pack.Bytes32)
	if !ok {
		confirmer.options.Logger.Errorf("[confirmer] failed to get nonce for tx=%v", transaction.Hash.String())
		return false
	}

	_, _, _, err := confirmer.bindings.AccountBurnInfo(ctx, burnChain, transaction.Selector.Asset(), txid, nonce)
	if err != nil {
		if !strings.Contains(err.Error(), "insufficient confirmations") {
			confirmer.options.Logger.Errorf("[confirmer] cannot get burn info for tx=%v (%v): %v", transaction.Hash.String(), transaction.Selector.String(), err)
		} else {
			confirmer.options.Logger.Warnf("[confirmer] cannot get burn info for tx=%v (%v): %v", transaction.Hash.String(), transaction.Selector.String(), err)
		}
		return false
	}
	return true
}

// burnAndMintTxConfirmed checks if a given burn-and-mint transaction has
// received sufficient confirmations.
func (confirmer *Confirmer) burnAndMintTxConfirmed(ctx context.Context, transaction tx.Tx) bool {
	burnChain := transaction.Selector.Source()
	txid, ok := transaction.Input.Get("txid").(pack.Bytes)
	if !ok {
		confirmer.options.Logger.Errorf("[confirmer] failed to get txid for tx=%v", transaction.Hash.String())
		return false
	}
	nonce, ok := transaction.Input.Get("nonce").(pack.Bytes32)
	if !ok {
		confirmer.options.Logger.Errorf("[confirmer] failed to get nonce for tx=%v", transaction.Hash.String())
		return false
	}

	_, _, _, err := confirmer.bindings.AccountBurnToChainInfo(ctx, burnChain, transaction.Selector.Asset(), txid, nonce)
	if err != nil {
		if !strings.Contains(err.Error(), "insufficient confirmations") {
			confirmer.options.Logger.Errorf("[confirmer] cannot get burn info for burn-and-mint tx=%v (%v): %v", transaction.Hash.String(), transaction.Selector.String(), err)
		} else {
			confirmer.options.Logger.Warnf("[confirmer] cannot get burn info for burn-and-mint tx=%v (%v): %v", transaction.Hash.String(), transaction.Selector.String(), err)
		}
		return false
	}
	return true
}

// prune removes any expired transactions from the database.
func (confirmer *Confirmer) prune() {
	if err := confirmer.database.Prune(confirmer.options.Expiry); err != nil {
		confirmer.options.Logger.Errorf("[confirmer] cannot prune database: %v", err)
	}
}

// submitTxRequest converts a transaction to a `jsonrpc.Request`.
func submitTxRequest(transaction tx.Tx) (jsonrpc.Request, error) {
	data, err := json.Marshal(jsonrpc.ParamsSubmitTx{
		Tx: transaction,
	})
	if err != nil {
		return jsonrpc.Request{}, fmt.Errorf("failed to marshal tx: %v", err)
	}

	return jsonrpc.Request{
		Version: "2.0",
		ID:      rand.Int63(),
		Method:  jsonrpc.MethodSubmitTx,
		Params:  data,
	}, nil
}
