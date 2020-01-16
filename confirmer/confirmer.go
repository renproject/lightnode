package confirmer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/renproject/darknode/abi"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/lightnode/blockchain"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/lightnode/server"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

type Options struct {
	MinConfirmations map[abi.Address]uint64
	PollInterval     time.Duration
	Phi              phi.Options
}

type Confirmer struct {
	logger     logrus.FieldLogger
	options    Options
	dispatcher phi.Sender
	database   db.DB
	connPool   blockchain.ConnPool

	txMu   *sync.RWMutex
	txs    map[abi.B32]abi.Tx
	txReqs map[abi.B32]jsonrpc.Request
}

func New(logger logrus.FieldLogger, options Options, dispatcher phi.Sender, db db.DB, connPool blockchain.ConnPool) phi.Task {
	// Load all pending pendingTxs from db into memory
	pendingTxs, err := db.PendingTxs()
	if err != nil {
		panic(fmt.Sprintf("unable read pending pendingTxs from database, err = %v", err))
	}
	txs := map[abi.B32]abi.Tx{}
	for _, tx := range pendingTxs {
		txs[tx.Hash] = tx
	}

	return phi.New(&Confirmer{
		logger:     logger,
		options:    Options{},
		dispatcher: dispatcher,
		database:   db,
		connPool:   connPool,
		txMu:       new(sync.RWMutex),
		txs:        txs,
		txReqs:     map[abi.B32]jsonrpc.Request{},
	}, options.Phi)
}

// Handle implements the `phi.Handler` interface.
func (confirmer *Confirmer) Handle(_ phi.Task, message phi.Message) {
	submiTx, ok := message.(SubmitTx)
	if !ok {
		confirmer.logger.Panicf("[confirmer] unexpected message type %T", message)
	}

	// Add tx to DB and in-mem tx pool
	if err := confirmer.database.InsertTx(submiTx.Tx); err != nil {
		confirmer.logger.Panicf("[confirmer] cannot insert tx into db, err = %v", err)
	}
	confirmer.txMu.Lock()
	defer confirmer.txMu.Unlock()
	confirmer.txs[submiTx.Tx.Hash] = submiTx.Tx
	confirmer.txReqs[submiTx.Tx.Hash] = submiTx.Request
}

func (confirmer *Confirmer) Run(ctx context.Context) {
	ticker := time.NewTicker(confirmer.options.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			confirmer.checkConfirmations(ctx)
		}
	}
}

func (confirmer *Confirmer) checkConfirmations(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, confirmer.options.PollInterval)
	defer cancel()

	txs := confirmer.pendingTxs()
	// TODO : IF THE CPU ONLY HAS ONE CORE AND THE FIRST TX IS BLOCKING
	// TODO : IT WILL CAUSING ISSUES
	phi.ForAll(txs, func(i int) {
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

func (confirmer *Confirmer) pendingTxs() []abi.Tx {
	confirmer.txMu.RLock()
	defer confirmer.txMu.Unlock()

	txs := make([]abi.Tx, 0, len(confirmer.txs))
	for _, tx := range confirmer.txs {
		txs = append(txs, tx)
	}
	return txs
}

func (confirmer *Confirmer) confirm(tx abi.Tx) {
	confirmer.txMu.Lock()
	defer confirmer.txMu.Unlock()

	// Send the tx to dispatcher
	request := confirmer.txReqs[tx.Hash]
	confirmer.dispatch(request)

	// Remove from the memory tx pool
	delete(confirmer.txs, tx.Hash)
	delete(confirmer.txReqs, tx.Hash)

	// Mark the txs as confirmed in DB
	if err := confirmer.database.ConfirmTx(tx.Hash); err != nil {
		confirmer.logger.Errorf("[confirmer] cannot confirm tx in the db, err = %v", err)
	}
}

// dispatch the confirmed request through dispatcher.
func (confirmer *Confirmer) dispatch(request jsonrpc.Request) {
	req := server.NewRequestWithResponder(request, "")
	if ok := confirmer.dispatcher.Send(req); !ok {
		confirmer.logger.Errorf("[confirmer] cannot send message to dispatcher, too much back pressure")
		return
	}
	// todo : handle if the dispatcher failed to send the tx to darknode.
}

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

type SubmitTx struct {
	Request jsonrpc.Request
	Tx      abi.Tx
}

func (SubmitTx) IsMessage() {}
