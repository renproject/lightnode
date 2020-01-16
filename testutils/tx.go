package testutils

import (
	"fmt"
	"math/big"
	"math/rand"

	"github.com/renproject/darknode/abi"
	"github.com/renproject/darknode/testutil"
)

// RandomShiftInTx creates a random ShiftIn Tx.
func RandomShiftInTx() (abi.Tx, error) {
	contract := RandomMintMethod()
	testutil.RandomExtEthCompatAddress()
	token := testutil.RandomExtEthCompatAddress()
	toAddr := testutil.RandomExtEthCompatAddress()

	phashArg := abi.Arg{
		Name:  "phash",
		Type:  abi.TypeB32,
		Value: RandomB32(),
	}
	tokenArg := abi.Arg{
		Name:  "token",
		Type:  abi.ExtTypeEthCompatAddress,
		Value: token,
	}
	toArg := abi.Arg{
		Name:  "to",
		Type:  abi.ExtTypeEthCompatAddress,
		Value: toAddr,
	}
	nArg := abi.Arg{
		Name:  "n",
		Type:  abi.TypeB32,
		Value: RandomB32(),
	}
	utxo := abi.ExtBtcCompatUTXO{
		TxHash:       RandomB32(),
		VOut:         abi.U32{Int: big.NewInt(0)},
		ScriptPubKey: nil,
	}
	utxoArg := abi.Arg{
		Name:  "utxo",
		Type:  abi.ExtTypeBtcCompatUTXO,
		Value: utxo,
	}

	return abi.Tx{
		Hash: RandomB32(),
		To:   contract,
		In:   []abi.Arg{phashArg, tokenArg, toArg, nArg, utxoArg},
	}, nil
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

// RandomU32 returns a randomly generated unsigned 32-bit integer that is ABI
// compatible.
func RandomU32() abi.U32 {
	return abi.U32{Int: big.NewInt(int64(rand.Int31()))}
}

func RandomUtxo() abi.ExtBtcCompatUTXO {
	return abi.ExtBtcCompatUTXO{
		TxHash: RandomB32(),
		VOut:   RandomU32(),
	}
}

// RandomMintMethod returns a random method for minting tokens.
func RandomMintMethod() abi.Address {
	methods := make([]abi.Address, 0)
	methods = append(methods, abi.IntrinsicBTC0Btc2Eth.Address)
	methods = append(methods, abi.IntrinsicZEC0Zec2Eth.Address)
	methods = append(methods, abi.IntrinsicBCH0Bch2Eth.Address)
	return methods[rand.Intn(len(methods))]
}
