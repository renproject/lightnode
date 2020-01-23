package testutils

import (
	"bytes"
	"math/big"
	"math/rand"

	"github.com/renproject/darknode/abi"
	"github.com/renproject/darknode/testutil"
)

func RandomShiftIn() abi.Tx {
	tx := testutil.RandomMintingTx("")
	amountArg := abi.Arg{
		Name:  "amount",
		Type:  abi.TypeU256,
		Value: abi.U256{Int: big.NewInt(int64(rand.Int31()))},
	}
	tx.In.Append(amountArg)
	ghashArg := abi.Arg{
		Name:  "ghash",
		Type:  abi.TypeB32,
		Value: testutil.RandomB32(),
	}
	nhashArg := abi.Arg{
		Name:  "nhash",
		Type:  abi.TypeB32,
		Value: testutil.RandomB32(),
	}
	sighashArg := abi.Arg{
		Name:  "sighash",
		Type:  abi.TypeB32,
		Value: testutil.RandomB32(),
	}
	tx.Autogen.Append(ghashArg, nhashArg, sighashArg)

	// Extra fields in utxo argument is not stored in db, so discard them in this case.
	utxo := tx.In.Get("utxo").Value.(abi.ExtBtcCompatUTXO)
	tx.In.Set("utxo", abi.ExtBtcCompatUTXO{
		TxHash: utxo.TxHash,
		VOut:   utxo.VOut,
	})
	return tx
}

func RandomShiftOut() abi.Tx {
	tx := testutil.RandomBurningTx("")
	b := testutil.RandomB32()
	toArg := abi.Arg{
		Name:  "to",
		Type:  abi.TypeB,
		Value: abi.B(b[:]),
	}
	amountArg := abi.Arg{
		Name:  "amount",
		Type:  abi.TypeU256,
		Value: abi.U256{Int: big.NewInt(int64(rand.Int31()))},
	}
	tx.In.Append(toArg, amountArg)
	return tx
}

func CompareTx(lhs, rhs abi.Tx) bool {
	lData, err := lhs.MarshalBinary()
	if err != nil {
		panic(err)
	}
	rData, err := rhs.MarshalBinary()
	if err != nil {
		panic(err)
	}
	return bytes.Equal(lData, rData)
}

func RandomUtxo() abi.ExtBtcCompatUTXO {
	return abi.ExtBtcCompatUTXO{
		TxHash: testutil.RandomB32(),
		VOut:   testutil.RandomU32(),
	}
}
