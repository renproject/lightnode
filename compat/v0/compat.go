package v0

import (
	"context"
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/darknode/txengine"
	"github.com/renproject/darknode/txengine/txenginebindings"
	"github.com/renproject/id"
	"github.com/renproject/multichain"
	"github.com/renproject/multichain/chain/ethereum"
	"github.com/renproject/pack"
)

// Takes a QueryState rpc response and converts it into a QueryShards rpc response
// It can be a standalone function as it has no dependencies
func ShardsResponseFromState(state jsonrpc.ResponseQueryState) (ResponseQueryShards, error) {
	bitcoinPubkey, ok := state.State[multichain.Bitcoin].Get("pubKey").(pack.String)
	if !ok {
		return ResponseQueryShards{},
			fmt.Errorf("unexpected type for Bitcoin pubKey: expected pack.String , got %v",
				state.State[multichain.Bitcoin].Get("pubKey").Type())
	}

	shards := make([]CompatShard, 1)
	shards[0] = CompatShard{
		DarknodesRootHash: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Gateways: []Gateway{
			{
				Asset:  "BTC",
				Hosts:  []string{"Ethereum"},
				Locked: "0",
				Origin: "Bitcoin",
				PubKey: bitcoinPubkey.String(),
			},
			{

				Asset:  "ZEC",
				Hosts:  []string{"Ethereum"},
				Locked: "0",
				Origin: "Zcash",
				PubKey: bitcoinPubkey.String(),
			},
			{
				Asset:  "BCH",
				Hosts:  []string{"Ethereum"},
				Locked: "0",
				Origin: "BitcoinCash",
				PubKey: bitcoinPubkey.String(),
			},
		},
		GatewaysRootHash: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Primary:          true,
		PubKey:           bitcoinPubkey.String(),
	}

	resp := ResponseQueryShards{
		Shards: shards,
	}
	return resp, nil
}

func TxFromV1Tx(t tx.Tx, hash B32, hasOut bool, bindings txengine.Bindings) (Tx, error) {
	tx := Tx{}

	phash := t.Input.Get("phash").(pack.Bytes32)
	tx.Autogen.Set(Arg{
		Name:  "phash",
		Type:  "b32",
		Value: B32(phash),
	})

	ghash := t.Input.Get("ghash").(pack.Bytes32)
	tx.Autogen.Set(Arg{
		Name:  "ghash",
		Type:  "b32",
		Value: B32(ghash),
	})

	nhash := t.Input.Get("nhash").(pack.Bytes32)
	tx.Autogen.Set(Arg{
		Name:  "nhash",
		Type:  "b32",
		Value: B32(nhash),
	})

	utxo := ExtBtcCompatUTXO{}

	inamount := t.Input.Get("amount").(pack.U256)
	utxo.Amount = U256{Int: inamount.Int()}
	utxo.GHash = B32(ghash)

	gpubkey := t.Input.Get("gpubkey").(pack.Bytes)
	utxo.ScriptPubKey = B(gpubkey)

	btcTxHash := t.Input.Get("txid").(pack.Bytes)
	btcTxHash32 := [32]byte{}
	copy(btcTxHash32[:], btcTxHash)
	//reverse it
	utxo.TxHash = B32(btcTxHash32)

	btcTxIndex := t.Input.Get("txindex").(pack.U32)
	utxo.VOut = U32{Int: big.NewInt(int64(btcTxIndex))}

	tx.Autogen.Set(Arg{
		Name:  "utxo",
		Type:  "ext_btcCompatUTXO",
		Value: utxo,
	})

	tx.In.Set(Arg{
		Name:  "utxo",
		Type:  "ext_btcCompatUTXO",
		Value: utxo,
	})

	// can't really re-create this correctly
	// pray it's OK
	payload := t.Input.Get("payload").(pack.Bytes)
	tx.In.Set(Arg{
		Name: "p",
		Type: "ext_ethCompatPayload",
		Value: ExtEthCompatPayload{
			ABI:   []byte("{}"),
			Value: B(payload),
			Fn:    []byte{},
		},
	})

	nonce := t.Input.Get("nonce").(pack.Bytes32)
	tx.In.Set(Arg{
		Name:  "n",
		Type:  "b32",
		Value: B32(nonce),
	})

	to := t.Input.Get("to").(pack.String)
	toAddr, err := ExtEthCompatAddressFromHex(to.String())
	if err != nil {
		return tx, err
	}

	tx.In.Set(Arg{
		Name:  "to",
		Type:  "ext_ethCompatAddress",
		Value: toAddr,
	})

	tokenAddrRaw, err := bindings.TokenAddressFromAsset(multichain.Ethereum, multichain.BTC)
	tokenAddr, err := ExtEthCompatAddressFromHex(hex.EncodeToString(tokenAddrRaw))

	tx.In.Set(Arg{
		Name:  "token",
		Type:  "ext_ethCompatAddress",
		Value: tokenAddr,
	})

	// use the in amount if we don't have an output yet
	tx.Autogen.Set(Arg{
		Name:  "amount",
		Type:  "u256",
		Value: U256{Int: inamount.Int()},
	})

	sighash := [32]byte{}
	sender, err := ethereum.NewAddressFromHex(toAddr.String())
	tokenEthAddr, err := ethereum.NewAddressFromHex(tokenAddr.String())

	copy(sighash[:], crypto.Keccak256(ethereum.Encode(
		phash,
		inamount,
		tokenEthAddr,
		sender,
		nhash,
	)))

	tx.Autogen.Set(Arg{
		Name:  "sighash",
		Type:  "b32",
		Value: B32(sighash),
	})

	if hasOut {
		outamount := t.Output.Get("amount").(pack.U256)
		tx.Autogen.Set(Arg{
			Name:  "amount",
			Type:  "u256",
			Value: U256{Int: outamount.Int()},
		})

		sig := t.Output.Get("sig").(pack.Bytes65)
		r := [32]byte{}
		copy(r[:], sig[:])

		s := [32]byte{}
		copy(s[:], sig[32:])

		v := sig[64:65]

		tx.Out.Set(Arg{
			Name:  "r",
			Type:  "b32",
			Value: B32(r),
		})

		tx.Out.Set(Arg{
			Name:  "s",
			Type:  "b32",
			Value: B32(s),
		})

		tx.Out.Set(Arg{
			Name:  "v",
			Type:  "b",
			Value: B(v[:]),
		})
	}

	tx.To = "BTC0Btc2Eth"
	tx.Hash = hash

	return tx, nil
}

// Don't lookup v1 txhash, just cast
func V1QueryTxFromQueryTx(queryTx ParamsQueryTx) jsonrpc.ParamsQueryTx {
	query := jsonrpc.ParamsQueryTx{}
	hash := queryTx.TxHash[:]
	txhash := [32]byte{}
	copy(txhash[:], hash)
	query.TxHash = txhash
	return query
}

// Will attempt to check if we have already constructed the parameters previously,
// otherwise will construct a v1 tx using v0 parameters, and persist a mapping
// so that a v0 queryTX can find them
func V1TxParamsFromTx(ctx context.Context, params ParamsSubmitTx, bindings *txenginebindings.Bindings, pubkey *id.PubKey, store CompatStore) (jsonrpc.ParamsSubmitTx, error) {
	v1tx, err := store.GetV1TxFromTx(params.Tx)
	if err == nil {
		// We have persisted this tx before, so let's use it
		return jsonrpc.ParamsSubmitTx{
			Tx: v1tx,
		}, err
	}
	if err != nil && err != ErrNotFound {
		// If there are errors with persistence, we won't be able to handle the tx
		// at a later state, so return an error early on
		return jsonrpc.ParamsSubmitTx{}, err
	}

	utxo := params.Tx.In.Get("utxo").Value.(ExtBtcCompatUTXO)
	i := utxo.VOut.Int.Uint64()
	txindex := pack.NewU32(uint32(i))

	txidB, err := utxo.TxHash.MarshalBinary()

	v0DepositId := base64.StdEncoding.EncodeToString(txidB) + "_" + utxo.VOut.Int.String()

	// reverse the txhash bytes
	txl := len(txidB)
	for i := 0; i < txl/2; i++ {
		txidB[i], txidB[txl-1-i] = txidB[txl-1-i], txidB[i]
	}

	txid := pack.NewBytes(txidB)

	payload := pack.NewBytes(params.Tx.In.Get("p").Value.(ExtEthCompatPayload).Value[:])

	token := params.Tx.In.Get("token").Value.(ExtEthCompatAddress)
	/// We only accept BTC/toEthereum / fromEthereum txs for compat
	/// We can't use the bindings, because the token addresses won't match
	// asset, err := bindings.AssetFromTokenAddress(multichain.Ethereum, multichain.Address(token.String()))
	// if err != nil {
	// 	return jsonrpc.ParamsSubmitTx{}, err
	// }
	// sel := tx.Selector(asset + "/toEthereum")
	sel := tx.Selector("BTC/toEthereum")

	phash := txengine.Phash(payload)

	to := pack.String(params.Tx.In.Get("to").Value.(ExtEthCompatAddress).String())

	nonce, err := params.Tx.In.Get("n").Value.(B32).MarshalBinary()
	var c [32]byte
	copy(c[:32], nonce)
	nonceP := pack.NewBytes32(c)

	minter, err := bindings.DecodeAddress(sel.Destination(), multichain.Address(to))

	ghash, err := txengine.V0Ghash(token[:], phash, minter, nonceP)

	nhash, err := txengine.V0Nhash(nonceP, txidB, txindex)

	// check if we've seen this amount before
	// also a cheeky workaround to enable testability
	amount, err := store.GetAmountFromUTXO(utxo)
	if err == ErrNotFound {
		// lets call the btc rpc endpoint because that's needed to get the correct amount
		out, err := bindings.UTXOLockInfo(ctx, multichain.Bitcoin, multichain.BTC, multichain.UTXOutpoint{
			Hash:  txid,
			Index: txindex,
		})
		if err != nil {
			return jsonrpc.ParamsSubmitTx{}, err
		}
		amount = out.Value.Int().Int64()

	} else if err != nil {
		return jsonrpc.ParamsSubmitTx{}, err
	}

	pubkbytes := crypto.CompressPubkey((*ecdsa.PublicKey)(pubkey))

	input, err := pack.Encode(txengine.CrossChainInput{
		Txid:    txid,
		Txindex: txindex,
		Amount:  pack.NewU256FromUint64(uint64(amount)),
		Payload: payload,
		Phash:   phash,
		To:      to,
		Nonce:   nonceP,
		Nhash:   nhash,
		Gpubkey: pack.NewBytes(pubkbytes),
		Ghash:   ghash,
	})

	v1Transaction := jsonrpc.ParamsSubmitTx{
		Tx: tx.Tx{
			Version:  tx.Version0,
			Selector: sel,
			Input:    pack.Typed(input.(pack.Struct)),
		},
	}

	h, err := tx.NewTxHash(tx.Version0, sel, v1Transaction.Tx.Input)
	v0HashB := crypto.Keccak256([]byte("txHash_BTC0Btc2Eth_" +
		base64.StdEncoding.EncodeToString(ghash[:]) + "_" +
		v0DepositId),
	)
	v0HashB32 := [32]byte{}
	copy(v0HashB32[:], v0HashB)
	params.Tx.Hash = v0HashB32

	v1Transaction.Tx.Hash = h
	err = store.PersistTxMappings(params.Tx, v1Transaction.Tx)
	if err != nil {
		return v1Transaction, err
	}

	return v1Transaction, nil
}
