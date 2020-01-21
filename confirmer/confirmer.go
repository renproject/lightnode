package confirmer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/renproject/darknode/abi"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/lightnode/blockchain"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/lightnode/http"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

// Options to initialize a confirmer.
type Options struct {
	MinConfirmations map[abi.Address]uint64
	PollInterval     time.Duration
}

// Confirmer handles requests which pass all validations. It checks if requests
// have reached enough confirmations. It stores requests which haven't reached
// enough confirmations and check them later.
type Confirmer struct {
	logger     logrus.FieldLogger
	options    Options
	dispatcher phi.Sender
	database   db.DB
	connPool   blockchain.ConnPool
}

// New returns a new Confirmer.
func New(logger logrus.FieldLogger, options Options, dispatcher phi.Sender, db db.DB, connPool blockchain.ConnPool) Confirmer {
	return Confirmer{
		logger:     logger,
		options:    options,
		dispatcher: dispatcher,
		database:   db,
		connPool:   connPool,
	}
}

// Run starts running the confirmer in background which periodically checks
// confirmations of pending txs.
func (confirmer *Confirmer) Run(ctx context.Context) {
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
}

func (confirmer *Confirmer) checkPendingTxs(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, confirmer.options.PollInterval)
	defer cancel()

	txs := confirmer.pendingTxs()
	phi.ParForAll(txs, func(i int) {
		tx := txs[i]
		var confirmed bool
		if confirmer.connPool.IsShiftIn(tx) {
			confirmed = confirmer.shiftInTxConfirmed(ctx, tx)
		} else {
			confirmed = confirmer.shiftOutTxConfirmed(ctx, tx)
		}

		if confirmed {
			confirmer.confirm(tx)
		}
	})
}

// pendingTxs loads all pending txs which are not expired from the database.
func (confirmer *Confirmer) pendingTxs() []abi.Tx {
	pendingTxs, err := confirmer.database.PendingTxs()
	if err != nil {
		panic(fmt.Sprintf("unable read pending pendingTxs from database, err = %v", err))
	}
	return pendingTxs
}

// confirm sends the tx to dispatcher and marks the tx as confirmed in db.
func (confirmer *Confirmer) confirm(tx abi.Tx) {
	request, err := txToJsonRequest(tx)
	if err != nil {
		confirmer.logger.Errorf("[confirmer] cannot convert tx to json request, err = %v", err)
	}
	req := http.NewRequestWithResponder(context.Background(), request, "")
	if ok := confirmer.dispatcher.Send(req); !ok {
		confirmer.logger.Errorf("[confirmer] cannot send message to dispatcher, too much back pressure")
		return
	}

	if err := confirmer.database.ConfirmTx(tx.Hash); err != nil {
		confirmer.logger.Errorf("[confirmer] cannot confirm tx in the db, err = %v", err)
	}
}

// shiftInTxConfirmed takes a shiftIn tx and check if it has enough confirmations.
func (confirmer *Confirmer) shiftInTxConfirmed(ctx context.Context, tx abi.Tx) bool {
	utxo := tx.In.Get("utxo").Value.(abi.ExtBtcCompatUTXO)

	confirmations, err := confirmer.connPool.UtxoConfirmations(ctx, tx.To, utxo.TxHash)
	if err != nil {
		confirmer.logger.Errorf("cannot get confirmation of [%v] request, txHash = %v, err = %v", tx.To, utxo.TxHash.String(), err)
		return false
	}
	minCon := confirmer.options.MinConfirmations[tx.To]
	return confirmations >= minCon
}

// shiftOutTxConfirmed takes a shiftOut tx and check if it has enough confirmations.
func (confirmer *Confirmer) shiftOutTxConfirmed(ctx context.Context, tx abi.Tx) bool {
	ref := tx.In.Get("ref").Value.(abi.U64)
	confirmations, err := confirmer.connPool.EventConfirmations(ctx, tx.To, ref.Int.Uint64())
	if err != nil {
		confirmer.logger.Errorf("cannot get confirmation of ethereum event log, err = %v", err)
		return false
	}
	minCon := confirmer.options.MinConfirmations[tx.To]
	return confirmations >= minCon
}

// txToJsonRequest converts a tx to its original jsonrpc.Requst.
func txToJsonRequest(tx abi.Tx) (jsonrpc.Request, error) {
	tx.Autogen = abi.Args{}
	utxo := tx.In.Get("utxo").Value.(abi.ExtBtcCompatUTXO)
	tx.In.Set("utxo", abi.ExtBtcCompatUTXO{
		TxHash: utxo.TxHash,
		VOut:   utxo.VOut,
	})

	if i := tx.In.Remove("amount"); i == -1 {
		return jsonrpc.Request{}, errors.New("missing amount argument")
	}
	data, err := json.Marshal(tx)
	if err != nil {
		return jsonrpc.Request{}, nil
	}
	return jsonrpc.Request{
		Version: "2.0",
		ID:      rand.Int63(),
		Method:  jsonrpc.MethodSubmitTx,
		Params:  data,
	}, nil
}
