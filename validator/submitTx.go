package validator

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/renproject/darknode/abi"
	"github.com/renproject/darknode/abi/ethabi"
	"github.com/renproject/darknode/ethrpc/bindings"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/lightnode/confirmer"
	"github.com/renproject/lightnode/server"
	"github.com/renproject/mercury/sdk/client/btcclient"
	"github.com/renproject/mercury/sdk/gateway/btcgateway"
	"github.com/renproject/mercury/types"
	"github.com/renproject/mercury/types/btctypes"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

const MinShiftAmount = 10000

type TxValidator struct {
	logger     logrus.FieldLogger
	confirmer  phi.Sender
	requests   <-chan server.RequestWithResponder
	disPubkey  ecdsa.PublicKey
	btcClient  btcclient.Client
	zecClient  btcclient.Client
	bchClient  btcclient.Client
	btcShifter *bindings.Shifter
	zecShifter *bindings.Shifter
	bchShifter *bindings.Shifter
}

func (v *TxValidator) Run() {
	workers := runtime.NumCPU()
	phi.ForAll(workers, func(_ int) {
		for {
			for req := range v.requests {
				tx, err := v.verify(req.Request)
				if err != nil {
					jsonErr := &jsonrpc.Error{jsonrpc.ErrorCodeInvalidParams, err.Error(), nil}
					req.Responder <- jsonrpc.NewResponse(req.Request.ID, nil, jsonErr)
					continue
				}

				// Send the success response to user
				data, err:= json.Marshal(tx)
				if err != nil {
					v.logger.Errorf("cannot marshal tx, err = %v", err)
					continue
				}
				req.Responder <- jsonrpc.NewResponse(req.Request.ID, data, nil)

				// Send the verified tx to confirmer for confirmations
				v.confirmer.Send(confirmer.SubmitTx{Tx:tx})
			}
		}
	})
}

func (v *TxValidator) verify(request jsonrpc.Request) (abi.Tx, error) {
	var submiTx jsonrpc.ParamsSubmitTx
	if err := json.Unmarshal(request.Params, &submiTx); err != nil {
		return abi.Tx{}, ErrInvalidParams
	}

	if err := v.verifyArguments(submiTx.Tx); err != nil {
		return abi.Tx{}, err
	}
	// todo : check duplication

	tx, err := v.verifyHash(submiTx.Tx)
	if err != nil {
		return abi.Tx{}, err
	}

	return v.verifyUTXO(tx)
}

func (v *TxValidator) verifyArguments(tx abi.Tx) error {
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

func (v *TxValidator) verifyHash(tx abi.Tx) (abi.Tx, error) {
	if v.isShiftIn(tx) {
		ghash, nhash := abi.B32{}, abi.B32{}
		utxo := tx.In.Get("utxo").Value.(abi.ExtBtcCompatUTXO)

		// Calculate ghash and append to the tx
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

		// Calculate nhash and append to the tx
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

		// Calculate the txHash for the tx.
		copy(tx.Hash[:], crypto.Keccak256([]byte(fmt.Sprintf("txHash_%v_%v_%v_%v", tx.To, ghash, utxo.TxHash, utxo.VOut))))
	} else {
		// Calculate the txHash for the tx.
		ref := tx.In.Get("ref").Value.(abi.U64)
		copy(tx.Hash[:], crypto.Keccak256([]byte(fmt.Sprintf("txHash_%v_%v", tx.To, ref))))
	}
	return tx, nil
}

func (v *TxValidator) verifyUTXO(tx abi.Tx) (abi.Tx, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if v.isShiftIn(tx) {
		client := v.client(tx.To)
		utxoValue := tx.In.Get("utxo").Value.(abi.ExtBtcCompatUTXO)
		txHash := types.TxHash(hex.EncodeToString(utxoValue.TxHash[:]))

		// verify existence of the provided utxo
		outpoint := btctypes.NewOutPoint(txHash, uint32(utxoValue.VOut.Int.Uint64()))
		utxo, err := client.UTXO(ctx, outpoint)
		if err != nil {
			return abi.Tx{}, err
		}
		if int(utxo.Amount()) < MinShiftAmount {
			return abi.Tx{}, fmt.Errorf("amount [%v] lower than minumum mint amount [%v]", utxo.Amount(), MinShiftAmount)
		}
		utxoValue.Amount = abi.U256{Int: big.NewInt(int64(utxo.Amount()))}
		tx.In.Append(abi.Arg{
			Name: "amount",
			Type: abi.TypeU256,
			Value: abi.U256{
				Int: big.NewInt(int64(utxo.Amount())),
			},
		})

		// verify ScriptPubkey
		ghash := tx.Autogen.Get("ghash").Value.(abi.B32)
		expectedSPB, err := gatewayScriptPubkey(client, ghash[:], v.disPubkey)
		if err != nil {
			return abi.Tx{}, err
		}
		if !bytes.Equal(utxo.ScriptPubKey(), expectedSPB) {
			return abi.Tx{}, errors.New("invalid script pubkey")
		}
		utxoValue.ScriptPubKey = utxo.ScriptPubKey()
		if i := tx.In.Set("utxo", utxoValue); i == -1 {
			return abi.Tx{}, errors.New("failed to set the utxo with scriptPubkey and amount")
		}

		// Calculate hash and append to Tx
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
		to, amount, err := v.shiftOutRef(tx.To, ref.Int.Uint64())
		if err != nil {
			return abi.Tx{}, err
		}
		if amount < MinShiftAmount {
			return abi.Tx{}, fmt.Errorf("amount [%v] lower than minumum burn amount [%v]", amount, MinShiftAmount)
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

func (v *TxValidator) isShiftIn(tx abi.Tx) bool {
	switch tx.To {
	case abi.IntrinsicBTC0Btc2Eth.Address, abi.IntrinsicZEC0Zec2Eth.Address, abi.IntrinsicBCH0Bch2Eth.Address:
		return true
	case abi.IntrinsicBTC0Eth2Btc.Address, abi.IntrinsicZEC0Eth2Zec.Address, abi.IntrinsicBCH0Eth2Bch.Address:
		return false
	default:
		v.logger.Panicf("[validator] expected contract address = %v", tx.To)
		return false
	}
}

func (v *TxValidator) shiftOutRef(addr abi.Address, ref uint64) ([]byte, uint64, error) {
	shifter := v.shifter(addr)

	shiftID := big.NewInt(int64(ref))
	// Filter for all epoch events in this range of blocks
	iter, err := shifter.FilterLogShiftOut(nil, []*big.Int{shiftID}, nil)
	if err != nil {
		return nil, 0, err
	}

	for iter.Next() {
		to := iter.Event.To
		amount := iter.Event.Amount
		return to, amount.Uint64(), nil
	}

	return nil, 0, fmt.Errorf("invalid ref, no event with ref =%v can be found")
}

func (v *TxValidator) client(addr abi.Address) btcclient.Client {
	switch addr {
	case abi.IntrinsicBTC0Btc2Eth.Address:
		return v.btcClient
	case abi.IntrinsicZEC0Zec2Eth.Address:
		return v.zecClient
	case abi.IntrinsicBCH0Bch2Eth.Address:
		return v.bchClient
	default:
		v.logger.Panicf("[validator] invalid shiftIn address = %v", addr)
		return nil
	}
}

func (v *TxValidator) shifter(addr abi.Address) *bindings.Shifter {
	switch addr {
	case abi.IntrinsicBTC0Eth2Btc.Address:
		return v.btcShifter
	case abi.IntrinsicZEC0Eth2Zec.Address:
		return v.zecShifter
	case abi.IntrinsicBCH0Eth2Bch.Address:
		return v.bchShifter
	default:
		v.logger.Panicf("[validator] invalid shiftOut address = %v", addr)
		return nil
	}
}

func gatewayScriptPubkey(client btcclient.Client, ghash []byte, distPubKey ecdsa.PublicKey) ([]byte, error) {
	gateway := btcgateway.New(client, distPubKey, ghash)
	return btctypes.PayToAddrScript(gateway.Address(), client.Network())
}
