package v0

import (
	"fmt"

	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/darknode/txengine"
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

func V1TxParamsFromTx(transaction ParamsSubmitTx) (jsonrpc.ParamsSubmitTx, error) {
	utxo := transaction.Tx.In.Get("utxo").Value.(ExtBtcCompatUTXO)
	i := utxo.VOut.Int.Uint64()
	txindex := pack.NewU32(uint32(i))

	payload := transaction.Tx.In.Get("p").Value.(ExtEthCompatPayload)

	// TODO: determine selector for given contract address
	token := transaction.Tx.In.Get("token").Value.(ExtEthCompatAddress)
	sel := tx.Selector("BTC/toEthereum")
	switch token.String() {
	case "0A9ADD98C076448CBcFAcf5E457DA12ddbEF4A8f":
		sel = tx.Selector("BTC/toEthereum")
	}

	bytes, err := payload.MarshalBinary()
	if err != nil {
		return jsonrpc.ParamsSubmitTx{}, fmt.Errorf("Failed to marshal payload binary: %v", err)
	}
	phash := txengine.Phash(pack.NewBytes(bytes))

	to := transaction.Tx.In.Get("to").Value.(ExtEthCompatAddress).String()

	nonce, err := transaction.Tx.In.Get("n").Value.(B32).MarshalBinary()
	var c [32]byte
	copy(c[:32], nonce)
	nonceP := pack.NewBytes32(c)

	ghash := txengine.Ghash(sel, phash, []byte(to), nonceP)

	// TODO: fetch public key for given asset/contract
	// TODO: figure out how to deserialize utxo from pack, so that we can get
	//     txid and txindex
	txidB, err := utxo.TxHash.MarshalBinary()
	txid := pack.NewBytes(txidB)

	nhash := txengine.Nhash(nonceP, txid, txindex)
	v1Transaction := jsonrpc.ParamsSubmitTx{
		Tx: tx.Tx{
			Hash:     phash,
			Version:  tx.Version1,
			Selector: sel,
			Input:    []pack.StructField{},
			Output:   []pack.StructField{},
		},
	}
	v1Transaction.Tx.Input.Set("phash", phash)
	v1Transaction.Tx.Input.Set("ghash", ghash)
	v1Transaction.Tx.Input.Set("nhash", nhash)
	v1Transaction.Tx.Input.Set("txid", txid)
	v1Transaction.Tx.Input.Set("txindex", txindex)
	v1Transaction.Tx.Input.Set("gpubkey", pack.String("will_be_replaced"))

	return v1Transaction, nil
}
