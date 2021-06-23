package v0

import (
	"context"
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/jbenet/go-base58"
	"github.com/renproject/darknode/binding"
	"github.com/renproject/darknode/engine"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/id"
	"github.com/renproject/multichain"
	"github.com/renproject/multichain/chain/bitcoin"
	"github.com/renproject/multichain/chain/bitcoincash"
	"github.com/renproject/multichain/chain/digibyte"
	"github.com/renproject/multichain/chain/dogecoin"
	"github.com/renproject/multichain/chain/ethereum"
	"github.com/renproject/multichain/chain/filecoin"
	"github.com/renproject/multichain/chain/terra"
	"github.com/renproject/multichain/chain/zcash"
	"github.com/renproject/pack"
)

// ShardsResponseFromState takes a QueryState rpc response and converts it into a QueryShards rpc response
// It can be a standalone function as it has no dependencies
func ShardsResponseFromSystemState(state engine.SystemState) (ResponseQueryShards, error) {
	shards := make([]CompatShard, 1)
	shards[0] = CompatShard{
		DarknodesRootHash: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Gateways: []Gateway{
			{
				Asset:  "BTC",
				Hosts:  []string{"Ethereum"},
				Locked: "0",
				Origin: "Bitcoin",
				PubKey: state.Shards.Primary[0].PubKey.String(),
			},
			{

				Asset:  "ZEC",
				Hosts:  []string{"Ethereum"},
				Locked: "0",
				Origin: "Zcash",
				PubKey: state.Shards.Primary[0].PubKey.String(),
			},
			{
				Asset:  "BCH",
				Hosts:  []string{"Ethereum"},
				Locked: "0",
				Origin: "BitcoinCash",
				PubKey: state.Shards.Primary[0].PubKey.String(),
			},
		},
		GatewaysRootHash: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Primary:          true,
		PubKey:           state.Shards.Primary[0].PubKey.String(),
	}

	resp := ResponseQueryShards{
		Shards: shards,
	}
	return resp, nil
}

// ShardsResponseFromState takes a QueryState rpc response and converts it into a QueryShards rpc response
// It can be a standalone function as it has no dependencies
func QueryFeesResponseFromState(state map[string]engine.XState) (ResponseQueryFees, error) {
	bitcoinS, ok := state[string(multichain.Bitcoin.NativeAsset())]
	if !ok {
		return ResponseQueryFees{},
			fmt.Errorf("Missing Bitcoin State")
	}
	bitcoinCap := bitcoinS.GasCap
	bitcoinLimit := bitcoinS.GasLimit
	bitcoinUnderlying := U64{Int: big.NewInt(int64(bitcoinCap.Int().Uint64() * bitcoinLimit.Int().Uint64()))}
	// TODO: These should be enabled once fees begin accuring inside RenVM.
	// bitcoinMintFee := U64{Int: big.NewInt(int64(bitcoinS.MintFee))}
	// bitcoinBurnFee := U64{Int: big.NewInt(int64(bitcoinS.BurnFee))}

	zcashS, ok := state[string(multichain.Zcash.NativeAsset())]
	if !ok {
		return ResponseQueryFees{},
			fmt.Errorf("Missing ZCash State")
	}

	zcashCap := zcashS.GasCap
	zcashLimit := zcashS.GasLimit
	zcashUnderlying := U64{Int: big.NewInt(int64(zcashCap.Int().Uint64() * zcashLimit.Int().Uint64()))}
	// zcashMintFee := U64{Int: big.NewInt(int64(zcashS.MintFee))}
	// zcashBurnFee := U64{Int: big.NewInt(int64(zcashS.BurnFee))}

	bitcoinCashS, ok := state[string(multichain.BitcoinCash.NativeAsset())]
	if !ok {
		return ResponseQueryFees{},
			fmt.Errorf("Missing BitcoinCash State")
	}
	bitcoinCashCap := bitcoinCashS.GasCap
	bitcoinCashLimit := bitcoinCashS.GasLimit
	bitcoinCashUnderlying := U64{Int: big.NewInt(int64(bitcoinCashCap.Int().Uint64() * bitcoinCashLimit.Int().Uint64()))}
	// bitcoinCashMintFee := U64{Int: big.NewInt(int64(bitcoinCashS.MintFee))}
	// bitcoinCashBurnFee := U64{Int: big.NewInt(int64(bitcoinCashS.BurnFee))}

	mintFee := U64{Int: big.NewInt(25)}
	burnFee := U64{Int: big.NewInt(10)}

	resp := ResponseQueryFees{
		Btc: Fees{
			Lock:    bitcoinUnderlying,
			Release: bitcoinUnderlying,
			Ethereum: MintAndBurnFees{
				Mint: mintFee,
				Burn: burnFee,
			},
		},
		Zec: Fees{
			Lock:    zcashUnderlying,
			Release: zcashUnderlying,
			Ethereum: MintAndBurnFees{
				Mint: mintFee,
				Burn: burnFee,
			},
		},
		Bch: Fees{
			Lock:    bitcoinCashUnderlying,
			Release: bitcoinCashUnderlying,
			Ethereum: MintAndBurnFees{
				Mint: mintFee,
				Burn: burnFee,
			},
		},
	}
	return resp, nil
}

func BurnTxFromV1Tx(t tx.Tx, bindings binding.Bindings) (Tx, error) {
	tx := Tx{}

	//nonce is ref in byte format
	nonce := t.Input.Get("nonce").(pack.Bytes32)
	ref := pack.NewU256(nonce)

	tx.Hash = BurnTxHash(t.Selector, ref)

	tx.To = Address(ToFromV1Selector(t.Selector))

	tx.In.Set(Arg{
		Name:  "ref",
		Type:  "u64",
		Value: U64{Int: ref.Int()},
	})

	to := t.Input.Get("to").(pack.String)

	tx.In.Set(Arg{
		Name:  "to",
		Type:  "b",
		Value: B(to),
	})

	inamount := t.Input.Get("amount").(pack.U256)
	castamount := U256{Int: inamount.Int()}

	tx.In.Set(Arg{
		Name:  "amount",
		Type:  "u256",
		Value: castamount,
	})

	return tx, nil
}

// TxFromV1Tx takes a V1 Tx and converts it to a V0 Tx.
func TxFromV1Tx(t tx.Tx, hasOut bool, bindings binding.Bindings) (Tx, error) {
	if t.Selector.IsBurn() || t.Selector.IsRelease() {
		return BurnTxFromV1Tx(t, bindings)
	}

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
	tx.Autogen.Set(Arg{
		Name:  "amount",
		Type:  "u256",
		Value: U256{Int: inamount.Int()},
	})

	sighash := [32]byte{}
	sender, err := ethereum.NewAddressFromHex(toAddr.String())
	if err != nil {
		return tx, err
	}

	tokenEthAddr, err := ethereum.NewAddressFromHex(tokenAddr.String())
	if err != nil {
		return tx, err
	}

	if hasOut {
		if t.Output.Get("amount") != nil {
			outamount := t.Output.Get("amount").(pack.U256)
			tx.Autogen.Set(Arg{
				Name:  "amount",
				Type:  "u256",
				Value: U256{Int: outamount.Int()},
			})

			copy(sighash[:], crypto.Keccak256(ethereum.Encode(
				phash,
				outamount,
				tokenEthAddr,
				sender,
				nhash,
			)))
		}

		if t.Output.Get("revert") != nil {
			reason := t.Output.Get("revert").(pack.String)

			tx.Out.Set(Arg{
				Name:  "revert",
				Type:  "str",
				Value: Str(reason),
			})
		}

		if t.Output.Get("sig") != nil {
			sig := t.Output.Get("sig").(pack.Bytes65)
			r := [32]byte{}
			copy(r[:], sig[:])

			s := [32]byte{}
			copy(s[:], sig[32:])

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
				Type:  "u8",
				Value: U8{Int: big.NewInt(int64(sig[64]))},
			})
		}
	}

	tx.Autogen.Set(Arg{
		Name:  "sighash",
		Type:  "b32",
		Value: B32(sighash),
	})

	tx.To = Address(ToFromV1Selector(t.Selector))
	v0hash := MintTxHash(t.Selector, ghash, btcTxHash, btcTxIndex)
	copy(tx.Hash[:], v0hash[:])

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
func V1TxParamsFromTx(ctx context.Context, params ParamsSubmitTx, bindings binding.Bindings, pubkey *id.PubKey, store CompatStore, network multichain.Network) (jsonrpc.ParamsSubmitTx, error) {
	// If it's a burn tx, we convert the tx to a v1 transaction and submit it
	if params.Tx.In.Get("utxo").Value == nil {
		selector := tx.Selector(fmt.Sprintf("%s/fromEthereum", params.Tx.To[0:3]))
		ref := params.Tx.In.Get("ref").Value.(U64)
		var nonce pack.Bytes32
		copy(nonce[:], pack.NewU256FromInt(ref.Int).Bytes())

		client := bindings.EthCompatClient(multichain.Ethereum)
		options := bindings.ChainOption(multichain.Ethereum)
		gatewayBinding := bindings.GatewayBinding(multichain.Ethereum, selector.Asset())

		details, err := gatewayBinding.GetBurn(&bind.CallOpts{}, ref.Int)
		if err != nil {
			return jsonrpc.ParamsSubmitTx{}, err
		}

		latestBlockHeader, err := client.HeaderByNumber(ctx, nil)
		if err != nil {
			return jsonrpc.ParamsSubmitTx{}, err
		}
		confirmations := new(big.Int).Sub(latestBlockHeader.Number, details.Blocknumber).Uint64()
		if pack.U64(confirmations) > options.MaxConfirmations {
			return jsonrpc.ParamsSubmitTx{}, fmt.Errorf("burn too old: confirmations=%v exceeds max=%v", confirmations, options.MaxConfirmations)
		}
		blockNumber := details.Blocknumber.Uint64()

		iter, err := gatewayBinding.FilterLogBurn(&bind.FilterOpts{
			Start:   blockNumber,
			End:     &blockNumber,
			Context: ctx,
		}, []*big.Int{ref.Int}, nil)
		if err != nil {
			return jsonrpc.ParamsSubmitTx{}, err
		}
		if iter == nil {
			return jsonrpc.ParamsSubmitTx{}, err
		}
		var txid pack.Bytes
		for iter.Next() {
			txid = iter.Event.Raw.TxHash.Bytes()
			break
		}
		if iter.Error() != nil {
			return jsonrpc.ParamsSubmitTx{}, err
		}
		amount := pack.NewU256FromInt(details.Amount)
		payload := details.Payload
		toBytes := details.To
		to := multichain.Address(toBytes)
		decoder := AddressEncodeDecoder(selector.Asset().OriginChain(), network)

		toDecode, err := decoder.DecodeAddress(to)
		if err != nil {
			to = multichain.Address(base58.Encode(toBytes))
			toDecode, err = decoder.DecodeAddress(to)
			if err != nil {
				return jsonrpc.ParamsSubmitTx{}, err
			}
		}

		phash := engine.Phash(payload)
		nhash := engine.Nhash(nonce, txid, 0)
		pubkbytes := crypto.CompressPubkey((*ecdsa.PublicKey)(pubkey))
		ghash := engine.Ghash(selector, phash, toDecode, nonce)
		input, err := pack.Encode(engine.LockMintBurnReleaseInput{
			Txid:    txid,
			Txindex: 0,
			Amount:  amount,
			Payload: payload,
			Phash:   phash,
			To:      pack.String(to),
			Nonce:   nonce,
			Nhash:   nhash,
			Gpubkey: pubkbytes,
			Ghash:   ghash,
		})
		if err != nil {
			return jsonrpc.ParamsSubmitTx{}, err
		}

		transaction := tx.Tx{
			Version:  tx.Version1,
			Selector: selector,
			Input:    pack.Typed(input.(pack.Struct)),
		}
		transaction.Hash, err = tx.NewTxHash(transaction.Version, transaction.Selector, transaction.Input)
		if err != nil {
			return jsonrpc.ParamsSubmitTx{}, err
		}

		return jsonrpc.ParamsSubmitTx{Tx: transaction}, nil
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

	phash := engine.Phash(payload)

	to := pack.String(params.Tx.In.Get("to").Value.(ExtEthCompatAddress).String())

	nonce, err := params.Tx.In.Get("n").Value.(B32).MarshalBinary()
	var c [32]byte
	copy(c[:32], nonce)
	nonceP := pack.NewBytes32(c)

	minter, err := bindings.DecodeAddress(sel.Destination(), multichain.Address(to))
	if err != nil {
		return jsonrpc.ParamsSubmitTx{}, err
	}

	ghash, err := engine.V0Ghash(token[:], phash, minter, nonceP)
	if err != nil {
		return jsonrpc.ParamsSubmitTx{}, err
	}

	nhash, err := engine.V0Nhash(nonceP, txidB, txindex)
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

	input, err := pack.Encode(engine.LockMintBurnReleaseInput{
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

func AddressEncodeDecoder(chain multichain.Chain, network multichain.Network) multichain.AddressEncodeDecoder {
	switch chain {
	case multichain.Bitcoin, multichain.DigiByte, multichain.Dogecoin:
		params := NetParams(chain, network)
		return bitcoin.NewAddressEncodeDecoder(params)
	case multichain.BitcoinCash:
		params := NetParams(chain, network)
		return bitcoincash.NewAddressEncodeDecoder(params)
	case multichain.Filecoin:
		return filecoin.NewAddressEncodeDecoder()
	case multichain.Terra:
		return terra.NewAddressEncodeDecoder()
	case multichain.Zcash:
		params := ZcashNetParams(network)
		return zcash.NewAddressEncodeDecoder(params)
	default:
		panic(fmt.Errorf("unknown chain %v", chain))
	}
}

func ZcashNetParams(network multichain.Network) *zcash.Params {
	switch network {
	case multichain.NetworkMainnet:
		return &zcash.MainNetParams
	case multichain.NetworkDevnet, multichain.NetworkTestnet:
		return &zcash.TestNet3Params
	default:
		return &zcash.RegressionNetParams
	}
}

func NetParams(chain multichain.Chain, net multichain.Network) *chaincfg.Params {
	switch chain {
	case multichain.Bitcoin, multichain.BitcoinCash:
		switch net {
		case multichain.NetworkDevnet, multichain.NetworkTestnet:
			return &chaincfg.TestNet3Params
		case multichain.NetworkMainnet:
			return &chaincfg.MainNetParams
		default:
			return &chaincfg.RegressionNetParams
		}
	case multichain.DigiByte:
		switch net {
		case multichain.NetworkDevnet, multichain.NetworkTestnet:
			return &digibyte.TestnetParams
		case multichain.NetworkMainnet:
			return &digibyte.MainNetParams
		default:
			return &digibyte.RegressionNetParams
		}
	case multichain.Dogecoin:
		switch net {
		case multichain.NetworkDevnet, multichain.NetworkTestnet:
			return &dogecoin.TestNetParams
		case multichain.NetworkMainnet:
			return &dogecoin.MainNetParams
		default:
			return &dogecoin.RegressionNetParams
		}
	default:
		panic(fmt.Errorf("cannot get network params: unknown chain %v", chain))
	}
}
