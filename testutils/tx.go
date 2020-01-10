package testutils

import (
	"fmt"
	"math/rand"

	"github.com/ethereum/go-ethereum/common"
	"github.com/renproject/darknode"
	"github.com/renproject/darknode/abi"
)

// RandomShiftInTx creates a random ShiftIn Tx.
func RandomShiftInTx() (abi.Tx, error) {
	contract := RandomMintMethod()
	tokenAddr := darknode.TokenAddresses[darknode.Localnet][contract]
	token, err := abi.B20FromHex(tokenAddr.Hex())
	if err != nil {
		return abi.Tx{}, err
	}

	to := common.Address(RandomB20())
	toAddr, err := abi.B20FromHex(to.Hex())
	if err != nil {
		return abi.Tx{}, err
	}

	amount := abi.U64(rand.Intn(100000000))
	phashArg := abi.Arg{
		Name:  "phash",
		Type:  abi.TypeB32,
		Value: RandomB32(),
	}
	amountArg := abi.Arg{
		Name:  "amount",
		Type:  abi.TypeU64,
		Value: amount,
	}
	tokenArg := abi.Arg{
		Name:  "token",
		Type:  abi.TypeB20,
		Value: token,
	}
	toArg := abi.Arg{
		Name:  "to",
		Type:  abi.TypeB20,
		Value: toAddr,
	}
	nArg := abi.Arg{
		Name:  "n",
		Type:  abi.TypeB32,
		Value: RandomB32(),
	}
	utxo := abi.ExtBtcCompatUTXO{
		TxHash:       RandomB32(),
		VOut:         0,
		ScriptPubKey: nil,
		Amount:       amount,
		GHash:        RandomB32(),
	}
	utxoArg := abi.Arg{
		Name:  "utxo",
		Type:  abi.ExtTypeBtcCompatUTXO,
		Value: utxo,
	}

	return abi.Tx{
		Hash: RandomB32(),
		To:   contract,
		Args: []abi.Arg{phashArg, amountArg, tokenArg, toArg, nArg, utxoArg},
	}, nil
}

// RandomB20 returns a randomly generated 20-byte array that is ABI compatible.
func RandomB20() abi.B20 {
	b20 := abi.B20{}
	_, err := rand.Read(b20[:])
	if err != nil {
		panic(fmt.Sprintf("cannot create random Tx object, err = %v", err))
	}
	return b20
}

// RandomB32 returns a randomly generated 32-byte array that is ABI compatible.
func RandomB32() abi.B32 {
	b32 := abi.B32{}
	_, err := rand.Read(b32[:])
	if err != nil {
		panic(fmt.Sprintf("cannot create random Tx object, err = %v", err))
	}
	return b32
}

// RandomMintMethod returns a random method for minting tokens.
func RandomMintMethod() abi.Addr {
	methods := make([]abi.Addr, 0)
	methods = append(methods, abi.IntrinsicBTC0Btc2Eth.Addr)
	methods = append(methods, abi.IntrinsicZEC0Zec2Eth.Addr)
	methods = append(methods, abi.IntrinsicBCH0Bch2Eth.Addr)
	return methods[rand.Intn(len(methods))]
}
