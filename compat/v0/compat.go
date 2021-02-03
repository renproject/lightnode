package v0

import (
	"context"
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

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

// ShardsResponseFromState takes a QueryState rpc response and converts it into a QueryShards rpc response
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

// TxFromV1Tx takes a V1 Tx and converts it to a V0 Tx, the given tx has to be a mint tx.
func TxFromV1Tx(t tx.Tx, hasOut bool, bindings txengine.Bindings) (Tx, error) {
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

	btcTxHash := t.Input.Get("txid").(pack.Bytes)
	btcTxHashReversed := make([]byte, len(btcTxHash))
	copy(btcTxHashReversed, btcTxHash)
	txl := len(btcTxHashReversed)
	for i := 0; i < txl/2; i++ {
		btcTxHashReversed[i], btcTxHashReversed[txl-1-i] = btcTxHashReversed[txl-1-i], btcTxHashReversed[i]
	}
	if err := utxo.TxHash.UnmarshalBinary(btcTxHashReversed); err != nil {
		return tx, nil
	}

	btcTxIndex := t.Input.Get("txindex").(pack.U32)
	utxo.VOut = U32{Int: big.NewInt(int64(btcTxIndex))}

	// utxo field `In` on has txHash and vout
	tx.In.Set(Arg{
		Name:  "utxo",
		Type:  "ext_btcCompatUTXO",
		Value: utxo,
	})

	inamount := t.Input.Get("amount").(pack.U256)
	utxo.Amount = U256{Int: inamount.Int()}
	utxo.GHash = B32(ghash)

	outpoint := multichain.UTXOutpoint{
		Hash:  btcTxHash,
		Index: btcTxIndex,
	}
	output, err := bindings.UTXOLockInfo(context.TODO(), t.Selector.Source(), t.Selector.Asset(), outpoint)
	if err != nil {
		return tx, nil
	}
	utxo.ScriptPubKey = B(output.PubKeyScript)

	tx.Autogen.Set(Arg{
		Name:  "utxo",
		Type:  "ext_btcCompatUTXO",
		Value: utxo,
	})

	// can't really re-create this correctly
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

	// rest of compat won't work beyond this point, so return early
	// in theory burns only need to check the status anyhow
	if t.Selector.IsBurn() || t.Selector.IsRelease() {
		return tx, nil
	}

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

	tokenAddrRaw, err := bindings.TokenAddressFromAsset(multichain.Ethereum, t.Selector.Asset())
	if err != nil {
		return tx, err
	}

	tokenAddr, err := ExtEthCompatAddressFromHex(hex.EncodeToString(tokenAddrRaw))
	if err != nil {
		return tx, err
	}

	tx.In.Set(Arg{
		Name:  "token",
		Type:  "ext_ethCompatAddress",
		Value: tokenAddr,
	})

	// use the in amount if we don't have an output yet
	// tx.Autogen.Set(Arg{
	// 	Name:  "amount",
	// 	Type:  "u256",
	// 	Value: U256{Int: inamount.Int()},
	// })

	sighash := [32]byte{}
	sender, err := ethereum.NewAddressFromHex(toAddr.String())
	if err != nil {
		return tx, err
	}

	tokenEthAddr, err := ethereum.NewAddressFromHex(tokenAddr.String())
	if err != nil {
		return tx, err
	}

	copy(sighash[:], crypto.Keccak256(ethereum.Encode(
		phash,
		inamount,
		tokenEthAddr,
		sender,
		nhash,
	)))

	// tx.Autogen.Set(Arg{
	// 	Name:  "sighash",
	// 	Type:  "b32",
	// 	Value: B32(sighash),
	// })

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

	tx.To = Address(ToFromV1Selector(t.Selector))
	v0hash := MintTxHash(t.Selector, ghash, btcTxHash, btcTxIndex)
	copy(tx.Hash[:],  v0hash[:])

	return tx, nil
}

// V1QueryTxFromQueryTx casts a v0 ParamsQueryTx to a v1 ParamsQueryTx
// by encoding the txhash in the appropriate manner
func V1QueryTxFromQueryTx(queryTx ParamsQueryTx) jsonrpc.ParamsQueryTx {
	query := jsonrpc.ParamsQueryTx{}
	hash := queryTx.TxHash[:]
	txhash := [32]byte{}
	copy(txhash[:], hash)
	query.TxHash = txhash
	return query
}

// ToFromV1Selector creates the "to" field in the v0 tx
// The "to" field in the v0 tx is the equivalent of a selector
// here we convert the v1 selector into the v0 format
func ToFromV1Selector(sel tx.Selector) string {
	source := strings.Title(strings.ToLower(string(sel.Source().NativeAsset())))
	dest := strings.Title(strings.ToLower(string(sel.Destination().NativeAsset())))
	return fmt.Sprintf("%s0%s2%s", sel.Asset(), source, dest)
}

// BurnTxHash creates V0 BurnTxHash from params available in V1
func BurnTxHash(sel tx.Selector, ref pack.U256) B32 {
	to := ToFromV1Selector(sel)
	txidString := fmt.Sprintf("txHash_%s_%s",
		to,
		ref)

	v0HashB := crypto.Keccak256([]byte(txidString))
	v0HashB32 := [32]byte{}
	copy(v0HashB32[:], v0HashB)
	return v0HashB32
}

// MintTxHash creates V0 MintTxHash from params avaialble in V1
func MintTxHash(sel tx.Selector, ghash pack.Bytes32, txid pack.Bytes, txindex pack.U32) B32 {
	// copy passed txid so that it doesn't modify the passed value...
	// v1 txid is reversed, so un-reverse it
	txl := len(txid)
	txidC := []byte{}
	for i := 1; i <= txl; i++ {
		txidC = append(txidC, txid[txl-i])
	}
	v0DepositID := fmt.Sprintf("%s_%s", base64.StdEncoding.EncodeToString(txidC), txindex)

	to := ToFromV1Selector(sel)
	txidString := fmt.Sprintf("txHash_%s_%s_%s",
		to,
		base64.StdEncoding.EncodeToString(ghash[:]),
		v0DepositID)

	v0HashB := crypto.Keccak256([]byte(txidString))
	v0HashB32 := [32]byte{}
	copy(v0HashB32[:], v0HashB)
	return v0HashB32
}

// V1TxParamsFromTx will create a v1 Tx from a v0 Tx
// Will attempt to check if we have already constructed the parameters previously,
// otherwise will construct a v1 tx using v0 parameters, and persist a mapping
// so that a v0 queryTX can find them
func V1TxParamsFromTx(ctx context.Context, params ParamsSubmitTx, bindings *txenginebindings.Bindings, pubkey *id.PubKey, store CompatStore) (jsonrpc.ParamsSubmitTx, error) {
	// It's a burn tx, we don't need to process it
	// as it should be picked up from the watcher
	// We pass the v0 hash along so that we can still
	// respond with the data that renjs-v1 requires
	if params.Tx.In.Get("utxo").Value == nil {
		refTx := params.Tx.In.Get("ref").Value.(U64).Int
		selString := fmt.Sprintf("%s/fromEthereum", params.Tx.To[0:3])
		sel := tx.Selector(selString)
		hash := BurnTxHash(sel, pack.NewU256FromInt(refTx))

		return jsonrpc.ParamsSubmitTx{Tx: tx.Tx{
			Selector: sel,
			Input:    pack.NewTyped("v0hash", pack.NewBytes32(hash)),
		}}, nil
	}

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
	if err != nil {
		return jsonrpc.ParamsSubmitTx{}, err
	}

	// reverse the utxo txhash bytes
	txl := len(txidB)
	for i := 0; i < txl/2; i++ {
		txidB[i], txidB[txl-1-i] = txidB[txl-1-i], txidB[i]
	}

	txid := pack.NewBytes(txidB)

	payload := pack.NewBytes(params.Tx.In.Get("p").Value.(ExtEthCompatPayload).Value[:])
	token := params.Tx.In.Get("token").Value.(ExtEthCompatAddress)
	asset, err := bindings.AssetFromTokenAddress(multichain.Ethereum, multichain.Address(strings.ToUpper("0x"+token.String())))
	if err != nil {
		return jsonrpc.ParamsSubmitTx{}, err
	}
	sel := tx.Selector(asset + "/toEthereum")

	phash := txengine.Phash(payload)

	to := pack.String(params.Tx.In.Get("to").Value.(ExtEthCompatAddress).String())

	nonce, err := params.Tx.In.Get("n").Value.(B32).MarshalBinary()
	var c [32]byte
	copy(c[:32], nonce)
	nonceP := pack.NewBytes32(c)

	minter, err := bindings.DecodeAddress(sel.Destination(), multichain.Address(to))
	if err != nil {
		return jsonrpc.ParamsSubmitTx{}, err
	}

	ghash, err := txengine.V0Ghash(token[:], phash, minter, nonceP)
	if err != nil {
		return jsonrpc.ParamsSubmitTx{}, err
	}

	nhash, err := txengine.V0Nhash(nonceP, txidB, txindex)
	if err != nil {
		return jsonrpc.ParamsSubmitTx{}, err
	}

	// check if we've seen this amount before
	// also a cheeky workaround to enable testability
	amount, err := store.GetAmountFromUTXO(utxo)
	if err == ErrNotFound {
		// lets call the btc rpc endpoint because that's needed to get the correct amount
		out, err := bindings.UTXOLockInfo(ctx, sel.Source(), sel.Asset(), multichain.UTXOutpoint{
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
	if err != nil {
		return jsonrpc.ParamsSubmitTx{}, err
	}

	v1Transaction := jsonrpc.ParamsSubmitTx{
		Tx: tx.Tx{
			Version:  tx.Version0,
			Selector: sel,
			Input:    pack.Typed(input.(pack.Struct)),
		},
	}

	h, err := tx.NewTxHash(tx.Version0, sel, v1Transaction.Tx.Input)
	if err != nil {
		return v1Transaction, err
	}
	v1Transaction.Tx.Hash = h

	params.Tx.Hash = MintTxHash(sel, ghash, txidB, txindex)

	err = store.PersistTxMappings(params.Tx, v1Transaction.Tx)
	if err != nil {
		return v1Transaction, err
	}

	return v1Transaction, nil
}
