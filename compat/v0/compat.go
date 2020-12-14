package v0

import (
	"context"
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-redis/redis/v7"
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

func V0TxFromV1(t tx.Tx, hash B32, hasOut bool, bindings txengine.Bindings) (Tx, error) {
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

	// sighash
	// const encoded = rawEncode(
	//     [
	//         "bytes32",
	//         "uint256",
	//         v2 ? "bytes32" : "address",
	//         "address",
	//         "bytes32",
	//     ],
	//     [pHash, amount, Ox(tokenIdentifier), Ox(to), nonceHash],
	// const digest = keccak256(encoded);

	// sighashE :=
	//
	// ethargs := ethabi.Arguments{
	// 	{Type: b32},
	// 	{Type: addr},
	// 	{Type: addr},
	// 	{Type: b32},
	// }
	//
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
	// Expected
	/*
		{
		    "result": {
		        "txStatus": "confirming",
		        "tx": {
		            "autogen": [
		                {
		                    "value": "2zIFnPelBHr9RPNUkQlz4E0U3UYbzJ4nQ1xIw2L7yYg=",
		                    "type": "b32",
		                    "name": "phash"
		                },
		                {
		                    "value": "DEvQxAbrNQf/YxXDcgP/4Wd0AGNYa28eBDg6vFdC88I=",
		                    "type": "b32",
		                    "name": "ghash"
		                },
		                {
		                    "value": "VkKkSMRg/63q/ebb62HTMrEqdJuDWcSKx0LNGr+75+o=",
		                    "type": "b32",
		                    "name": "nhash"
		                },
		                {
		                    "value": "KxSqdrBA5fE14G1xB2G07zycRmQstj4/FavcCd+34po=",
		                    "type": "b32",
		                    "name": "sighash"
		                },
		                {
		                    "value": "100000",
		                    "type": "u256",
		                    "name": "amount"
		                },
		                {
		                    "value": {
		                        "ghash": "DEvQxAbrNQf/YxXDcgP/4Wd0AGNYa28eBDg6vFdC88I=",
		                        "amount": "200000",
		                        "scriptPubKey": "qRQFNngOJ3Dswbp9/JKCwaAqr5qhhIc=",
		                        "vOut": "0",
		                        "txHash": "1cuYQphEKlNQaeo7f5iZ5bWUp66nBII+KLJIXoy7YxY="
		                    },
		                    "type": "ext_btcCompatUTXO",
		                    "name": "utxo"
		                }
		            ],
		            "in": [
		                {
		                    "value": {
		                        "fn": "bWludA==",
		                        "value": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAACJGOCpLdxrU70fzhypE84q/owC0gAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAADQlRDAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		                        "abi": "W3siY29uc3RhbnQiOmZhbHNlLCJpbnB1dHMiOlt7InR5cGUiOiJzdHJpbmciLCJuYW1lIjoiX3N5bWJvbCJ9LHsidHlwZSI6ImFkZHJlc3MiLCJuYW1lIjoiX2FkZHJlc3MifSx7Im5hbWUiOiJfYW1vdW50IiwidHlwZSI6InVpbnQyNTYifSx7Im5hbWUiOiJfbkhhc2giLCJ0eXBlIjoiYnl0ZXMzMiJ9LHsibmFtZSI6Il9zaWciLCJ0eXBlIjoiYnl0ZXMifV0sIm91dHB1dHMiOltdLCJwYXlhYmxlIjp0cnVlLCJzdGF0ZU11dGFiaWxpdHkiOiJwYXlhYmxlIiwidHlwZSI6ImZ1bmN0aW9uIiwibmFtZSI6Im1pbnQifV0="
		                    },
		                    "type": "ext_ethCompatPayload",
		                    "name": "p"
		                },
		                {
		                    "value": "0a9add98c076448cbcfacf5e457da12ddbef4a8f",
		                    "type": "ext_ethCompatAddress",
		                    "name": "token"
		                },
		                {
		                    "value": "7ddfa2e5435027f6e13ca8db2f32ebd5551158bb",
		                    "type": "ext_ethCompatAddress",
		                    "name": "to"
		                },
		                {
		                    "value": "xi4j4cWhkCy6OeLbZL7Po52gJTfE01U2cb1782EqTMo=",
		                    "type": "b32",
		                    "name": "n"
		                },
		                {
		                    "value": {
		                        "ghash": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		                        "amount": "0",
		                        "vOut": "0",
		                        "txHash": "1cuYQphEKlNQaeo7f5iZ5bWUp66nBII+KLJIXoy7YxY="
		                    },
		                    "type": "ext_btcCompatUTXO",
		                    "name": "utxo"
		                }
		            ],
		            "to": "BTC0Btc2Eth",
		            "hash": "u2Rt4Yx6kP4fnLkhm9oPJLzWH78oYutzygknoTMIMj0="
		        }
		    },
		    "id": 1,
		    "jsonrpc": "2.0"
			}
	*/

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

func V1TxParamsFromTx(ctx context.Context, transaction ParamsSubmitTx, bindings *txenginebindings.Bindings, pubkey *id.PubKey, store redis.Cmdable) (jsonrpc.ParamsSubmitTx, error) {
	utxo := transaction.Tx.In.Get("utxo").Value.(ExtBtcCompatUTXO)
	i := utxo.VOut.Int.Uint64()
	txindex := pack.NewU32(uint32(i))

	txidB, err := utxo.TxHash.MarshalBinary()

	// deposit `${toBase64(fromHex(transaction.txHash))}_${transaction.vOut}`;
	v0DepositId := base64.StdEncoding.EncodeToString(txidB) + "_" + utxo.VOut.Int.String()

	// V0 deposit id in renjs = 06HQtSGSItb9R+kin2SCUhmJyCC1oALUe206azSbtUA=_0

	// reverse the txhash bytes
	txl := len(txidB)
	for i := 0; i < txl/2; i++ {
		txidB[i], txidB[txl-1-i] = txidB[txl-1-i], txidB[i]
	}

	txid := pack.NewBytes(txidB)

	payload := pack.NewBytes(transaction.Tx.In.Get("p").Value.(ExtEthCompatPayload).Value[:])

	token := transaction.Tx.In.Get("token").Value.(ExtEthCompatAddress)
	/// We only accept BTC/toEthereum / fromEthereum txs for compat
	/// We can't use the bindings, because the token addresses won't match
	// asset, err := bindings.AssetFromTokenAddress(multichain.Ethereum, multichain.Address(token.String()))
	// if err != nil {
	// 	return jsonrpc.ParamsSubmitTx{}, err
	// }
	// sel := tx.Selector(asset + "/toEthereum")
	sel := tx.Selector("BTC/toEthereum")

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

	input, err := pack.Encode(txengine.CrossChainInput{
		Txid:    txid,
		Txindex: txindex,
		Amount:  out.Value,
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
	transaction.Tx.Hash = v0HashB32

	v1Transaction.Tx.Hash = h
	fmt.Printf("\n\n\nsubmitting hash %s\n\n\n", h)

	// deposit `${toBase64(fromHex(transaction.txHash))}_${transaction.vOut}`;
	// const message = `txHash_${selector}_${encodedID}_${deposit}`;

	// persist v0 hash for later query-lookup
	err = store.Set(transaction.Tx.Hash.String(), h.String(), 0).Err()
	if err != nil {
		return v1Transaction, err
	}

	// Also allow for lookup by btc utxo; as we don't have the v0 hash at submission
	// Expire these because it's only useful during submission, not querying
	err = store.Set(utxo.TxHash.String(), h.String(), time.Duration(time.Hour*24*7)).Err()
	if err != nil {
		return v1Transaction, err
	}

	return v1Transaction, nil
}
