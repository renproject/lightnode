package v0

import (
	"context"
	"crypto/ecdsa"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-redis/redis/v7"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/darknode/txengine"
	"github.com/renproject/darknode/txengine/txenginebindings"
	"github.com/renproject/id"
	"github.com/renproject/multichain"
	"github.com/renproject/pack"
)

// Takes a QueryState rpc response and converts it into a QueryShards rpc response
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

func V1QueryTxFromQueryTx(ctx context.Context, queryTx ParamsQueryTx, store redis.Cmdable) (jsonrpc.ParamsQueryTx, error) {
	query := jsonrpc.ParamsQueryTx{}
	hash, err := store.Get(queryTx.TxHash.String()).Bytes()
	if err != nil {
		return query, err
	}
	txhash := [32]byte{}
	copy(txhash[:], hash)
	query.TxHash = txhash
	return query, nil
}

func V1TxParamsFromTx(ctx context.Context, transaction ParamsSubmitTx, bindings *txenginebindings.Bindings, pubkey *id.PubKey, store redis.Cmdable) (jsonrpc.ParamsSubmitTx, error) {
	utxo := transaction.Tx.In.Get("utxo").Value.(ExtBtcCompatUTXO)
	i := utxo.VOut.Int.Uint64()
	txindex := pack.NewU32(uint32(i))

	txidB, err := utxo.TxHash.MarshalBinary()

	// reverse the txhash bytes
	txl := len(txidB)
	for i := 0; i < txl/2; i++ {
		txidB[i], txidB[txl-1-i] = txidB[txl-1-i], txidB[i]
	}

	txid := pack.NewBytes(txidB)

	payload := pack.NewBytes(transaction.Tx.In.Get("p").Value.(ExtEthCompatPayload).Value[:])

	token := transaction.Tx.In.Get("token").Value.(ExtEthCompatAddress)
	asset, err := bindings.AssetFromTokenAddress(multichain.Ethereum, multichain.Address(token.String()))
	sel := tx.Selector(asset + "/toEthereum")

	phash := txengine.Phash(payload)

	to := pack.String(transaction.Tx.In.Get("to").Value.(ExtEthCompatAddress).String())

	nonce, err := transaction.Tx.In.Get("n").Value.(B32).MarshalBinary()
	var c [32]byte
	copy(c[:32], nonce)
	nonceP := pack.NewBytes32(c)

	minter, err := bindings.DecodeAddress(sel.Destination(), multichain.Address(to))

	ghash, err := txengine.V0Ghash(token[:], phash, minter, nonceP)

	nhash, err := txengine.V0Nhash(nonceP, txidB, txindex)

	// lets call the btc rpc endpoint because that's needed to get the correct amount
	out, err := bindings.UTXOLockInfo(ctx, multichain.Bitcoin, multichain.BTC, multichain.UTXOutpoint{
		Hash:  txid,
		Index: txindex,
	})

	pubkbytes := crypto.CompressPubkey((*ecdsa.PublicKey)(pubkey))
	if err != nil {
		return jsonrpc.ParamsSubmitTx{}, err
	}

	v1Transaction := jsonrpc.ParamsSubmitTx{
		Tx: tx.Tx{
			Version:  tx.Version0,
			Selector: sel,
			Input: []pack.StructField{
				{
					Name:  "phash",
					Value: phash,
				},
				{
					Name:  "ghash",
					Value: ghash,
				},
				{
					Name:  "nhash",
					Value: nhash,
				},
				{
					Name:  "nonce",
					Value: nonceP,
				},
				{
					Name:  "txid",
					Value: txid,
				},
				{
					Name:  "txindex",
					Value: txindex,
				},
				{
					Name:  "payload",
					Value: payload,
				},
				{
					Name:  "gpubkey",
					Value: pack.NewBytes(pubkbytes),
				},
				{
					Name:  "amount",
					Value: out.Value,
				},
				{
					Name:  "to",
					Value: to,
				},
			},
		},
	}

	h, err := tx.NewTxHash(tx.Version0, sel, v1Transaction.Tx.Input)
	v1Transaction.Tx.Hash = h
	// persist v0 hash for later query-lookup
	err = store.Set(transaction.Tx.Hash.String(), h.Bytes(), 0).Err()
	if err != nil {
		return v1Transaction, err
	}

	return v1Transaction, nil
}
