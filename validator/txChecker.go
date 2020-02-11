package validator

import (
	"context"
	"crypto/ecdsa"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/renproject/darknode/abi"
	"github.com/renproject/darknode/abi/ethabi"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/lightnode/blockchain"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/lightnode/http"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

// MinShiftAmount for both ShiftIn and ShiftOut txs.
const MinShiftAmount = 10000

// A txChecker reads submitTx requests from a channel and validate the details
// of the tx. It will store the tx if it's valid.
type txChecker struct {
	mu        *sync.Mutex
	logger    logrus.FieldLogger
	requests  <-chan http.RequestWithResponder
	disPubkey ecdsa.PublicKey
	connPool  blockchain.ConnPool
	db        db.DB
}

// newTxChecker returns a new txChecker.
func newTxChecker(logger logrus.FieldLogger, requests <-chan http.RequestWithResponder, key ecdsa.PublicKey, pool blockchain.ConnPool, db db.DB) txChecker {
	return txChecker{
		mu:        new(sync.Mutex),
		logger:    logger,
		requests:  requests,
		disPubkey: key,
		connPool:  pool,
		db:        db,
	}
}

// Run starts the txChecker until the requests channel is closed.
func (tc *txChecker) Run() {
	workers := 2 * runtime.NumCPU()
	phi.ForAll(workers, func(_ int) {
		for req := range tc.requests {
			tx, err := tc.verify(req.Request)
			if err != nil {
				jsonErr := &jsonrpc.Error{Code: jsonrpc.ErrorCodeInvalidParams, Message: err.Error(), Data: nil}
				req.Responder <- jsonrpc.NewResponse(req.Request.ID, nil, jsonErr)
				continue
			}

			// Check for duplicate.
			storedTx, duplicate, err := tc.checkDuplicate(tx)
			if err != nil {
				tc.logger.Errorf("[txChecker] cannot check tx duplication, err = %v", err)
				continue
			}

			// Write the response to the responder channel.
			response := jsonrpc.ResponseSubmitTx{
				Tx: tx,
			}
			if duplicate {
				response.Tx = storedTx
			}
			req.Responder <- jsonrpc.NewResponse(req.Request.ID, response, nil)
		}
	})
}

func (tc *txChecker) verify(request jsonrpc.Request) (abi.Tx, error) {
	var submiTx jsonrpc.ParamsSubmitTx
	if err := json.Unmarshal(request.Params, &submiTx); err != nil {
		return abi.Tx{}, ErrInvalidParams
	}

	if err := tc.verifyArguments(submiTx.Tx); err != nil {
		return abi.Tx{}, err
	}

	tx, err := tc.verifyHash(submiTx.Tx)
	if err != nil {
		return abi.Tx{}, err
	}

	return tc.verifyUTXO(tx)
}

func (tc *txChecker) verifyArguments(tx abi.Tx) error {
	// Check that the contract exists.
	contract, ok := abi.Intrinsics[tx.To]
	if !ok {
		return errors.New("unknown contract address")
	}

	// Check the request has expected number of arguments.
	if len(contract.In) != len(tx.In) {
		return errors.New("incorrect number of arguments")
	}

	// Check the request has expected argument name and type.
	for _, formal := range contract.In {
		arg := tx.In.Get(formal.Name)
		if arg.IsNil() {
			return fmt.Errorf("missing argument [%v]", formal.Name)
		}
		if formal.Type != arg.Type {
			return fmt.Errorf("incorrect argument type for [%v], expect = %v, got = %v", formal.Name, formal.Type, arg.Type)
		}
	}
	return nil
}

func (tc *txChecker) verifyHash(tx abi.Tx) (abi.Tx, error) {
	if blockchain.IsShiftIn(tx) {
		ghash, nhash := abi.B32{}, abi.B32{}
		utxo := tx.In.Get("utxo").Value.(abi.ExtBtcCompatUTXO)

		// Calculate ghash and append it to the tx.
		copy(ghash[:], crypto.Keccak256(ethabi.Encode(abi.Args{
			tx.In.Get("phash"),
			tx.In.Get("token"),
			tx.In.Get("to"),
			tx.In.Get("n"),
		})))
		tx.Autogen.Append(abi.Arg{
			Name:  "ghash",
			Type:  abi.TypeB32,
			Value: ghash,
		})

		// Calculate nhash and append it to the tx.
		copy(nhash[:], crypto.Keccak256(ethabi.Encode(abi.Args{
			tx.In.Get("n"),
			abi.Arg{
				Name:  "txhash",
				Type:  abi.TypeB32,
				Value: utxo.TxHash,
			},
			abi.Arg{
				Name:  "vout",
				Type:  abi.TypeU32,
				Value: utxo.VOut,
			},
		})))
		tx.Autogen.Append(abi.Arg{
			Name:  "nhash",
			Type:  abi.TypeB32,
			Value: nhash,
		})

		// FIXME: This hash calculation should be imported directly from the
		// Darknode.
		copy(tx.Hash[:], crypto.Keccak256([]byte(fmt.Sprintf("txHash_%s_%s_%s_%d", tx.To, ghash, utxo.TxHash, utxo.VOut.Int.Int64()))))
	} else {
		ref := tx.In.Get("ref").Value.(abi.U64)
		copy(tx.Hash[:], crypto.Keccak256([]byte(fmt.Sprintf("txHash_%s_%d", tx.To, ref.Int.Int64()))))
	}
	return tx, nil
}

func (tc *txChecker) verifyUTXO(tx abi.Tx) (abi.Tx, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if blockchain.IsShiftIn(tx) {
		utxoValue := tx.In.Get("utxo").Value.(abi.ExtBtcCompatUTXO)

		// Verify existence of the provided UTXO.
		utxo, err := tc.connPool.Utxo(ctx, tx.To, utxoValue.TxHash, utxoValue.VOut)
		if err != nil {
			return abi.Tx{}, err
		}
		if int(utxo.Amount()) <= MinShiftAmount {
			return abi.Tx{}, fmt.Errorf("amount [%v] needs to be greater than minumum mint amount [%v]", utxo.Amount(), MinShiftAmount)
		}
		utxoValue.Amount = abi.U256{Int: big.NewInt(int64(utxo.Amount()))}
		tx.In.Append(abi.Arg{
			Name: "amount",
			Type: abi.TypeU256,
			Value: abi.U256{
				Int: big.NewInt(int64(utxo.Amount())),
			},
		})

		// Verify script public key.
		ghash := tx.Autogen.Get("ghash").Value.(abi.B32)
		if err := tc.connPool.VerifyScriptPubKey(tx.To, ghash[:], tc.disPubkey, utxo); err != nil {
			return abi.Tx{}, errors.New("invalid script pubkey")
		}
		utxoValue.ScriptPubKey = utxo.ScriptPubKey()
		if i := tx.In.Set("utxo", utxoValue); i == -1 {
			return abi.Tx{}, errors.New("failed to set the utxo with scriptPubkey and amount")
		}

		// Calculate the hash and append to the transaction.
		hash := abi.B32{}
		copy(hash[:], crypto.Keccak256(ethabi.Encode(abi.Args{
			tx.In.Get("phash"),
			tx.In.Get("amount"),
			tx.In.Get("token"),
			tx.In.Get("to"),
			tx.Autogen.Get("nhash"),
		})))
		tx.Autogen.Append(abi.Arg{
			Name:  "sighash",
			Type:  abi.TypeB32,
			Value: hash,
		})
	} else {
		ref := tx.In.Get("ref").Value.(abi.U64)
		to, amount, err := tc.connPool.ShiftOut(tx.To, ref.Int.Uint64())
		if err != nil {
			return abi.Tx{}, err
		}
		if amount <= MinShiftAmount {
			return abi.Tx{}, fmt.Errorf("amount [%v] needs be greater than minumum burn amount [%v]", amount, MinShiftAmount)
		}
		tx.In.Append(
			abi.Arg{
				Name:  "to",
				Type:  abi.TypeB,
				Value: abi.B(to),
			},
			abi.Arg{
				Name:  "amount",
				Type:  abi.TypeU256,
				Value: abi.U256{Int: big.NewInt(int64(amount))},
			},
		)
	}
	return tx, nil
}

func (tc *txChecker) checkDuplicate(tx abi.Tx) (abi.Tx, bool, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if blockchain.IsShiftIn(tx) {
		stored, err := tc.db.ShiftIn(tx.Hash)
		if err == sql.ErrNoRows {
			return tx, false, tc.db.InsertShiftIn(tx)
		}
		return stored, true, err
	} else {
		stored, err := tc.db.ShiftOut(tx.Hash)
		if err == sql.ErrNoRows {
			return tx, false, tc.db.InsertShiftOut(tx)
		}
		return stored, true, err
	}
}
