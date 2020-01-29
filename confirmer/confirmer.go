package confirmer

import (
	"context"
	"encoding/json"
	"errors"
	"log"
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
	Expiry           time.Duration
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
// confirmations of pending txs and prune txs which are too old.
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

// checkPendingTxs reads all pending txs from the database and checks if they
// have reached enough confirmations.
func (confirmer *Confirmer) checkPendingTxs(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, confirmer.options.PollInterval)
	defer cancel()

	// Read all pending txs.
	txs, err := confirmer.database.PendingTxs()
	if err != nil {
		confirmer.logger.Errorf("[confirmer] unable read pending pendingTxs from database, err = %v", err)
		return
	}
	phi.ParForAll(txs, func(i int) {
		tx := txs[i]
		var confirmed bool
		if blockchain.IsShiftIn(tx) {
			confirmed = confirmer.shiftInTxConfirmed(ctx, tx)
		} else {
			confirmed = confirmer.shiftOutTxConfirmed(ctx, tx)
		}

		if confirmed {
			log.Printf("tx = %v has reached enough confirmations", tx.Hash.String())
			confirmer.confirm(tx)
		}
	})
}

// confirm sends the tx to dispatcher and marks the tx as confirmed in db.
func (confirmer *Confirmer) confirm(tx abi.Tx) {
	request, err := txToJsonRequest(tx)
	if err != nil {
		confirmer.logger.Errorf("[confirmer] cannot convert tx to json request, err = %v", err)
		return
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
		confirmer.logger.Errorf("[confirmer] cannot get confirmation of [%v] request, txHash = %v, err = %v", tx.To, utxo.TxHash.String(), err)
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
		confirmer.logger.Errorf("[confirmer] cannot get confirmation of ethereum event log, err = %v", err)
		return false
	}
	minCon := confirmer.options.MinConfirmations[tx.To]
	return confirmations >= minCon
}

// prune deletes txs from the database which expire.
func (confirmer *Confirmer) prune() {
	if err := confirmer.database.Prune(confirmer.options.Expiry); err != nil {
		confirmer.logger.Errorf("[confirmer] cannot prune database, err = %v", err)
	}
}

// txToJsonRequest converts a tx to its original jsonrpc.Request.
func txToJsonRequest(tx abi.Tx) (jsonrpc.Request, error) {
	if blockchain.IsShiftIn(tx) {
		tx.Autogen = abi.Args{}
		utxo := tx.In.Get("utxo").Value.(abi.ExtBtcCompatUTXO)
		tx.In.Set("utxo", abi.ExtBtcCompatUTXO{
			TxHash: utxo.TxHash,
			VOut:   utxo.VOut,
		})

		if i := tx.In.Remove("amount"); i == -1 {
			return jsonrpc.Request{}, errors.New("missing amount argument")
		}
	} else {
		if i := tx.In.Remove("amount"); i == -1 {
			return jsonrpc.Request{}, errors.New("missing amount argument")
		}
		if i := tx.In.Remove("to"); i == -1 {
			return jsonrpc.Request{}, errors.New("missing to argument")
		}
	}

	data, err := json.Marshal(jsonrpc.ParamsSubmitTx{Tx: tx})
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
