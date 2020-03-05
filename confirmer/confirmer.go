package confirmer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"time"

	"github.com/renproject/darknode/abi"
	"github.com/renproject/darknode/consensus/txcheck/transform/blockchain"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/lightnode/http"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

// Options for initialising a confirmer.
type Options struct {
	MinConfirmations map[abi.Address]uint64
	PollInterval     time.Duration
	Expiry           time.Duration
}

// Confirmer handles requests that have been validated. It checks if requests
// have reached sufficient confirmations and stores those that have not to be
// checked later.
type Confirmer struct {
	logger     logrus.FieldLogger
	options    Options
	dispatcher phi.Sender
	database   db.DB
	bc         blockchain.ConnPool
}

// New returns a new Confirmer.
func New(logger logrus.FieldLogger, options Options, dispatcher phi.Sender, db db.DB, bc blockchain.ConnPool) Confirmer {
	return Confirmer{
		logger:     logger,
		options:    options,
		dispatcher: dispatcher,
		database:   db,
		bc:         bc,
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

	txs, err := confirmer.database.PendingTxs(24 * time.Hour)
	if err != nil {
		confirmer.logger.Errorf("[confirmer] failed to read pending txs from database: %v", err)
		return
	}

	phi.ParForAll(txs, func(i int) {
		tx := txs[i]
		var confirmed bool
		if abi.IsShiftIn(tx.To) {
			confirmed = confirmer.shiftInTxConfirmed(ctx, tx)
		} else {
			confirmed = confirmer.shiftOutTxConfirmed(ctx, tx)
		}

		if confirmed {
			confirmer.logger.Infof("tx=%v has reached sufficient confirmations", tx.Hash.String())
			confirmer.confirm(ctx, tx)
		}
	})
}

// confirm sends the transaction to the dispatcher and marks it as confirmed if
// it receives a non-error response from the Darknodes.
func (confirmer *Confirmer) confirm(ctx context.Context, tx abi.Tx) {
	request, err := submitTxRequest(tx)
	if err != nil {
		confirmer.logger.Errorf("[confirmer] cannot construct json request for transaction: %v", err)
		return
	}
	req := http.NewRequestWithResponder(ctx, request, url.Values{})
	if ok := confirmer.dispatcher.Send(req); !ok {
		confirmer.logger.Errorf("[confirmer] cannot send message to dispatcher, too much back pressure")
		return
	}

	go func() {
		select {
		case <-ctx.Done():
			return
		case response := <-req.Responder:
			if response.Error != nil {
				confirmer.logger.Errorf("[confirmer] getting error back when submitting tx = %v: [%v] %v", tx.Hash.String(), response.Error.Code, response.Error.Message)
				return
			}
			confirmer.logger.Infof("✅ successfully submit tx = %v to darknodes", tx.Hash.String())

			if err := confirmer.database.UpdateStatus(tx.Hash, db.TxStatusConfirmed); err != nil {
				confirmer.logger.Errorf("[confirmer] cannot confirm tx in the database: %v", err)
			}
		}
	}()
}

// shiftInTxConfirmed checks if a given shift in transaction has received
// sufficient confirmations.
func (confirmer *Confirmer) shiftInTxConfirmed(ctx context.Context, tx abi.Tx) bool {
	utxoVal := tx.In.Get("utxo").Value.(abi.ExtBtcCompatUTXO)
	utxo, err := confirmer.bc.Utxo(ctx, tx.To, utxoVal.TxHash, utxoVal.VOut)
	if err != nil {
		confirmer.logger.Errorf("[confirmer] cannot get confirmation for tx=%v (%v): %v", utxoVal.TxHash.String(), tx.To, err)
		return false
	}

	return utxo.Confirmations() >= confirmer.options.MinConfirmations[tx.To]
}

// shiftOutTxConfirmed checks if a given shift out transaction has received
// sufficient confirmations.
func (confirmer *Confirmer) shiftOutTxConfirmed(ctx context.Context, tx abi.Tx) bool {
	ref := tx.In.Get("ref").Value.(abi.U64)

	confirmations, err := confirmer.bc.EventConfirmations(ctx, tx.To, ref.Int.Uint64())
	if err != nil {
		confirmer.logger.Errorf("[confirmer] cannot get confirmation for ethereum event (%v): %v", tx.To, err)
		return false
	}
	minCon := confirmer.options.MinConfirmations[tx.To]
	return confirmations >= minCon
}

// prune removes any expired transactions from the database.
func (confirmer *Confirmer) prune() {
	if err := confirmer.database.Prune(confirmer.options.Expiry); err != nil {
		confirmer.logger.Errorf("[confirmer] cannot prune database: %v", err)
	}
}

// submitTxRequest converts a transaction to a `jsonrpc.Request`.
func submitTxRequest(tx abi.Tx) (jsonrpc.Request, error) {
	data, err := json.Marshal(jsonrpc.ParamsSubmitTx{Tx: tx})
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
