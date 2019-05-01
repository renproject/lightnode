package testutils

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/republicprotocol/renvm-go/core/vm/asm"
	"github.com/republicprotocol/renvm-go/core/vm/macro"
	"github.com/republicprotocol/renvm-go/foundation/crypto/algebra"
	"github.com/republicprotocol/renvm-go/foundation/crypto/shamir"
	"github.com/tyler-smith/go-bip39"
)

func BuildSigners(n, k uint64) []macro.SignerBlob {
	privKey, err := loadKey("", "", 44, 1, 0, 0, 0)
	if err != nil {
		panic(err)
	}
	qOne, _ := big.NewInt(0).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)
	fieldOne := algebra.NewField(qOne)
	privKeyShares := split(fieldOne.NewInField(privKey.D), n, (k+1)/2, fieldOne)
	rho := fieldOne.NewInField(big.NewInt(69))
	sigma := fieldOne.NewInField(big.NewInt(69))
	rhoShares := split(rho, n, k, fieldOne)
	sigmaShares := split(sigma, n, (k+1)/2, fieldOne)
	signers := make([]macro.SignerBlob, n)
	for i := 0; i < int(n); i++ {
		signer := macro.NewSigner(privKey.PublicKey, privKeyShares[i], rhoShares[i], sigmaShares[i])
		if err := signers[i].FromSigner(signer); err != nil {
			panic(err)
		}
	}
	return signers
}

func loadKey(mnemonic, passphrase string, path ...uint32) (*ecdsa.PrivateKey, error) {
	key, err := loadMasterKey(mnemonic, passphrase, path[1])
	if err != nil {
		return nil, err
	}
	for _, val := range path {
		key, err = key.Child(val)
		if err != nil {
			return nil, err
		}
	}
	privKey, err := key.ECPrivKey()
	if err != nil {
		return nil, err
	}
	return privKey.ToECDSA(), nil
}

func loadMasterKey(mnemonic, passphrase string, network uint32) (*hdkeychain.ExtendedKey, error) {
	switch network {
	case 1:
		seed := bip39.NewSeed(mnemonic, passphrase)
		return hdkeychain.NewMaster(seed, &chaincfg.TestNet3Params)
	case 0:
		seed := bip39.NewSeed(mnemonic, passphrase)
		return hdkeychain.NewMaster(seed, &chaincfg.MainNetParams)
	default:
		return nil, fmt.Errorf("network id: %d", network)
	}
}

func split(value algebra.FpElement, n, k uint64, fpp algebra.Fp) []asm.ValuePrivate {
	poly := algebra.NewRandomPolynomial(fpp, uint(k-1), value)
	shares := shamir.Split(poly, n)
	values := make([]asm.ValuePrivate, n)
	for i := range values {
		values[i] = asm.NewValuePrivate(shares[i])
	}
	return values
}
