package resolver_test

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/rand"
	"testing/quick"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/ethereum/go-ethereum/crypto"
	ethSecp256 "github.com/ethereum/go-ethereum/crypto/secp256k1"
	filaddress "github.com/filecoin-project/go-address"
	"github.com/renproject/darknode/binding"
	"github.com/renproject/darknode/engine"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/darknode/tx/txutil"
	"github.com/renproject/id"
	"github.com/renproject/lightnode/resolver"
	"github.com/renproject/multichain"
	"github.com/renproject/multichain/api/utxo"
	"github.com/renproject/multichain/chain/bitcoin"
	"github.com/renproject/multichain/chain/bitcoincash"
	"github.com/renproject/multichain/chain/digibyte"
	"github.com/renproject/multichain/chain/dogecoin"
	"github.com/renproject/multichain/chain/ethereum"
	"github.com/renproject/multichain/chain/filecoin"
	"github.com/renproject/multichain/chain/solana"
	"github.com/renproject/multichain/chain/terra"
	"github.com/renproject/multichain/chain/zcash"
	"github.com/renproject/pack"
	"github.com/renproject/surge"
)

var _ = Describe("Verifier", func() {
	Context("when verifying a lock tx", func() {
		It("should reject a lock tx with invalid gpubkey", func() {
			network := multichain.NetworkMainnet
			r := rand.New(rand.NewSource(GinkgoRandomSeed()))
			bindings := new(binding.Callbacks)
			bindings.HandleDecodeAddress = func(chain multichain.Chain, addr multichain.Address) (multichain.RawAddress, error) {
				encodDecoder := AddressEncodeDecoder(chain, network)
				return encodDecoder.DecodeAddress(addr)
			}
			bindings.HandleAddressFromPubKey = func(chain multichain.Chain, pubKey *id.PubKey) (multichain.Address, error) {
				return binding.AddressFromPubKey(network, chain, pubKey)
			}

			hostChains := map[multichain.Chain]bool{
				multichain.Ethereum: true,
			}
			key := id.NewPrivKey()
			verifier := resolver.NewVerifier(hostChains, bindings, key.PubKey())

			test := func() bool {
				tx := RandomGoodLockTx(r, network, nil)
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()

				amount := tx.Input.Get("amount").(pack.U256)
				gpubkey := tx.Input.Get("gpubkey").(pack.Bytes)
				ghash := tx.Input.Get("ghash").(pack.Bytes32)
				bindings.HandleUTXOLockInfo = func(ctx context.Context, chain multichain.Chain, asset multichain.Asset, outpoint multichain.UTXOutpoint) (multichain.UTXOutput, error) {
					txid := tx.Input.Get("txid").(pack.Bytes)
					txindex := tx.Input.Get("txindex").(pack.U32)

					gwPubKeyScript, err := engine.UTXOGatewayPubKeyScript(tx.Selector.Source(), tx.Selector.Asset(), gpubkey, ghash)

					return multichain.UTXOutput{
						Outpoint: utxo.Outpoint{
							Hash:  txid,
							Index: txindex,
						},
						Value:        amount,
						PubKeyScript: gwPubKeyScript,
					}, err
				}
				bindings.HandleAccountLockInfo = func(ctx context.Context, sourceChain, destinationChain multichain.Chain, asset multichain.Asset, txid, payload pack.Bytes, nonce pack.Bytes32) (pack.U256, pack.String, error) {
					pubKey := id.PubKey{}
					if err := surge.FromBinary(&pubKey, gpubkey); err != nil {
						return pack.U256{}, "", err
					}
					ghashPrivKey, err := crypto.ToECDSA(ghash.Bytes())
					if err != nil {
						return pack.U256{}, "", err
					}
					ghashPubKey := (*id.PubKey)(&ghashPrivKey.PublicKey)
					toPubKey := &ecdsa.PublicKey{
						Curve: ethSecp256.S256(),
					}
					toPubKey.X, toPubKey.Y = toPubKey.Add(pubKey.X, pubKey.Y, ghashPubKey.X, ghashPubKey.Y)
					to, err := bindings.AddressFromPubKey(tx.Selector.Source(), (*id.PubKey)(toPubKey))
					if err != nil {
						return pack.U256{}, "", err
					}

					return amount, pack.String(to), nil
				}

				Expect(verifier.VerifyTx(ctx, tx).Error()).Should(ContainSubstring("bad gpubkey"))
				return true
			}

			Expect(quick.Check(test, nil)).NotTo(HaveOccurred())
		})

		It("should accept a lock tx with valid gpubkey", func() {
			network := multichain.NetworkMainnet
			r := rand.New(rand.NewSource(GinkgoRandomSeed()))
			bindings := new(binding.Callbacks)
			bindings.HandleDecodeAddress = func(chain multichain.Chain, addr multichain.Address) (multichain.RawAddress, error) {
				encodDecoder := AddressEncodeDecoder(chain, network)
				return encodDecoder.DecodeAddress(addr)
			}
			bindings.HandleAddressFromPubKey = func(chain multichain.Chain, pubKey *id.PubKey) (multichain.Address, error) {
				return binding.AddressFromPubKey(network, chain, pubKey)
			}

			hostChains := map[multichain.Chain]bool{
				multichain.Ethereum: true,
			}
			key := id.NewPrivKey()
			verifier := resolver.NewVerifier(hostChains, bindings, key.PubKey())

			test := func() bool {
				tx := RandomGoodLockTx(r, network, key.PubKey())
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()

				amount := tx.Input.Get("amount").(pack.U256)
				gpubkey := tx.Input.Get("gpubkey").(pack.Bytes)
				ghash := tx.Input.Get("ghash").(pack.Bytes32)
				bindings.HandleUTXOLockInfo = func(ctx context.Context, chain multichain.Chain, asset multichain.Asset, outpoint multichain.UTXOutpoint) (multichain.UTXOutput, error) {
					txid := tx.Input.Get("txid").(pack.Bytes)
					txindex := tx.Input.Get("txindex").(pack.U32)
					gwPubKeyScript, err := engine.UTXOGatewayPubKeyScript(tx.Selector.Source(), tx.Selector.Asset(), gpubkey, ghash)

					return multichain.UTXOutput{
						Outpoint: utxo.Outpoint{
							Hash:  txid,
							Index: txindex,
						},
						Value:        amount,
						PubKeyScript: gwPubKeyScript,
					}, err
				}
				bindings.HandleAccountLockInfo = func(ctx context.Context, sourceChain, destinationChain multichain.Chain, asset multichain.Asset, txid, payload pack.Bytes, nonce pack.Bytes32) (pack.U256, pack.String, error) {
					pubKey := id.PubKey{}
					if err := surge.FromBinary(&pubKey, gpubkey); err != nil {
						return pack.U256{}, "", err
					}
					ghashPrivKey, err := crypto.ToECDSA(ghash.Bytes())
					if err != nil {
						return pack.U256{}, "", err
					}
					ghashPubKey := (*id.PubKey)(&ghashPrivKey.PublicKey)
					toPubKey := &ecdsa.PublicKey{
						Curve: ethSecp256.S256(),
					}
					toPubKey.X, toPubKey.Y = toPubKey.Add(pubKey.X, pubKey.Y, ghashPubKey.X, ghashPubKey.Y)
					to, err := bindings.AddressFromPubKey(tx.Selector.Source(), (*id.PubKey)(toPubKey))
					if err != nil {
						return pack.U256{}, "", err
					}

					return amount, pack.String(to), nil
				}

				Expect(verifier.VerifyTx(ctx, tx)).Should(Succeed())
				return true
			}

			Expect(quick.Check(test, nil)).NotTo(HaveOccurred())
		})
	})
})

func RandomGoodLockTx(r *rand.Rand, network multichain.Network, gpubkey *id.PubKey) tx.Tx {
	// Generate a random transaction selector.
	randomSelector := RandomGoodLockSelector(r)

	input := RandomGoodTxInput(r, randomSelector, network, gpubkey)

	// Construct the transaction.
	transaction, err := tx.NewTx(randomSelector, input)
	if err != nil {
		panic(err)
	}
	transaction.Version = tx.Version1 // The inputs we generate are for version 1 transactions.
	transaction.Output = pack.NewTyped()

	// Compute the transaction hash correctly, based on the other randomly
	// generated fields.
	hash, err := tx.NewTxHash(transaction.Version, transaction.Selector, transaction.Input)
	if err != nil {
		panic(err)
	}
	transaction.Hash = hash

	return transaction
}

func RandomGoodLockSelector(r *rand.Rand) tx.Selector {
	assets := txutil.SupportedAssets()
	hosts := txutil.SupportedHostChains()
	selectors := make([]tx.Selector, 0, 2*len(assets)*len(hosts))
	for _, asset := range assets {
		for _, host := range hosts {
			host := host
			selectors = append(selectors, tx.Selector(fmt.Sprintf("%v/to%v", asset, host)))
		}
	}
	return selectors[r.Intn(len(selectors))]
}

func RandomGoodTx(r *rand.Rand, network multichain.Network, gpubkey *id.PubKey) tx.Tx {
	// Generate a random transaction selector.
	randomSelector := txutil.RandomGoodTxSelector(r)

	input := RandomGoodTxInput(r, randomSelector, network, gpubkey)

	// Construct the transaction.
	transaction, err := tx.NewTx(randomSelector, input)
	if err != nil {
		panic(err)
	}
	transaction.Version = tx.Version1 // The inputs we generate are for version 1 transactions.
	transaction.Output = pack.NewTyped()

	// Compute the transaction hash correctly, based on the other randomly
	// generated fields.
	hash, err := tx.NewTxHash(transaction.Version, transaction.Selector, transaction.Input)
	if err != nil {
		panic(err)
	}
	transaction.Hash = hash

	return transaction
}

func RandomGoodTxInput(r *rand.Rand, selector tx.Selector, network multichain.Network, gpubkey *id.PubKey) pack.Typed {
	txid := RandomGoodTxid(r, selector)
	txindex := RandomGoodTxindex(r, selector)
	amount := RandomGoodAmount(r, pack.NewU256FromU64(0))
	payload := pack.Bytes{}.Generate(r, r.Int()%100).Interface().(pack.Bytes)
	phash := engine.Phash(payload)
	to := RandomGoodAddress(selector.Destination(), network)
	encodeDecoder := AddressEncodeDecoder(selector.Destination(), network)
	toBytes, err := encodeDecoder.DecodeAddress(multichain.Address(to))
	if err != nil {
		panic(err)
	}
	nonce := pack.Bytes32{}.Generate(r, 1).Interface().(pack.Bytes32)
	if gpubkey == nil {
		gpubkey = id.NewPrivKey().PubKey()
	}
	gpubkeyByte := pack.Bytes(((*btcec.PublicKey)(gpubkey)).SerializeCompressed())
	ghash := engine.Ghash(selector, phash, toBytes, nonce)
	nhash := engine.Nhash(nonce, txid, txindex)

	input := pack.NewStruct(
		"txid", txid,
		"txindex", txindex,
		"amount", amount,
		"payload", payload,
		"phash", phash,
		"to", to,
		"nonce", nonce,
		"nhash", nhash,
		"gpubkey", gpubkeyByte,
		"ghash", ghash,
	)
	return pack.Typed(input)
}

func RandomGoodTxid(r *rand.Rand, selector tx.Selector) pack.Bytes {
	switch selector.Source() {
	// utxo chaisn
	case multichain.Bitcoin, multichain.BitcoinCash, multichain.Zcash, multichain.Dogecoin, multichain.DigiByte:
		return pack.Bytes{}.Generate(r, 32).Interface().(pack.Bytes)

	// Account based chains
	case multichain.Terra:
		return pack.Bytes{}.Generate(r, 32).Interface().(pack.Bytes)
	case multichain.Filecoin:
		return pack.Bytes{}.Generate(r, 38).Interface().(pack.Bytes)

	// Host chains
	case multichain.Arbitrum, multichain.Avalanche, multichain.BinanceSmartChain,
		multichain.Ethereum, multichain.Fantom, multichain.Goerli,
		multichain.Moonbeam, multichain.Polygon, multichain.Solana:
		return pack.Bytes{}.Generate(r, 32).Interface().(pack.Bytes)
	default:
		panic(fmt.Sprintf("unknown chain for txid = %v", selector.Source()))
	}
}

func RandomGoodTxindex(r *rand.Rand, selector tx.Selector) pack.U32 {
	if selector.IsLock() {
		source := selector.Source()
		if source.IsUTXOBased() {
			return pack.U32(0).Generate(r, 1).Interface().(pack.U32)
		} else {
			return pack.U32(0)
		}
	} else {
		return pack.U32(0)
	}
}

func RandomGoodAmount(r *rand.Rand, min pack.U256) pack.U256 {
	amount := pack.U256{}.Generate(r, 1).Interface().(pack.U256)
	return amount.Add(min)
}

func RandomGoodAddress(chain multichain.Chain, network multichain.Network) pack.String {
	key := id.NewPrivKey()

	switch chain {
	// UTXO-based chain
	case multichain.Bitcoin, multichain.DigiByte, multichain.Dogecoin:
		params := NetParams(chain, network)
		key := btcec.PublicKey(key.PublicKey)
		pubKeyHash := btcutil.Hash160(key.SerializeCompressed())
		addr, err := btcutil.NewAddressPubKeyHash(pubKeyHash, params)
		if err != nil {
			panic(err)
		}
		return pack.String(addr.EncodeAddress())
	case multichain.BitcoinCash:
		key := btcec.PublicKey(key.PublicKey)
		params := NetParams(chain, network)
		pubKeyHash := btcutil.Hash160(key.SerializeCompressed())
		addr, err := bitcoincash.NewAddressPubKeyHash(pubKeyHash, params)
		if err != nil {
			panic(err)
		}
		return pack.String(addr.EncodeAddress())
	case multichain.Zcash:
		key := btcec.PublicKey(key.PublicKey)
		pubKeyHash := btcutil.Hash160(key.SerializeCompressed())
		addr, err := zcash.NewAddressPubKeyHash(pubKeyHash, ZcashNetParams(network))
		if err != nil {
			panic(err)
		}
		return pack.String(addr.EncodeAddress())

	// Ethereum-like chain
	case multichain.Avalanche, multichain.Ethereum, multichain.Goerli, multichain.BinanceSmartChain, multichain.Fantom, multichain.Polygon, multichain.Arbitrum, multichain.Moonbeam:
		addr := crypto.PubkeyToAddress(key.PublicKey)
		return pack.String(addr.Hex())

	// Account-based chain
	case multichain.Filecoin:
		serialisedPubKey := crypto.FromECDSAPub(&key.PublicKey)
		addr, err := filaddress.NewSecp256k1Address(serialisedPubKey)
		if err != nil {
			panic(err)
		}
		return pack.String(addr.String())
	case multichain.Terra:
		compressedPubKey, err := surge.ToBinary((id.PubKey)(key.PublicKey))
		if err != nil {
			panic(err)
		}
		pubKey := secp256k1.PubKey{Key: compressedPubKey}
		addrEncodeDecoder := terra.NewAddressEncodeDecoder()
		addr, err := addrEncodeDecoder.EncodeAddress(pubKey.Address().Bytes())
		if err != nil {
			panic(err)
		}
		return pack.String(addr)
	case multichain.Solana:
		// todo :
		return "FsaLodPu4VmSwXGr3gWfwANe4vKf8XSZcCh1CEeJ3jpD"
	default:
		panic(fmt.Errorf("AddressFromPubkey : unknown blockchain = %v", chain))
	}
}

// NetParams returns the chain config for the given blockchain and network.
// It will panic for non-utxo-based chains.
func NetParams(chain multichain.Chain, network multichain.Network) *chaincfg.Params {
	switch chain {
	case multichain.Bitcoin, multichain.BitcoinCash:
		switch network {
		case multichain.NetworkMainnet:
			return &chaincfg.MainNetParams
		case multichain.NetworkTestnet:
			return &chaincfg.TestNet3Params
		default:
			return &chaincfg.RegressionNetParams
		}
	case multichain.Zcash:
		switch network {
		case multichain.NetworkMainnet:
			return zcash.MainNetParams.Params
		case multichain.NetworkTestnet:
			return zcash.TestNet3Params.Params
		default:
			return zcash.RegressionNetParams.Params
		}
	case multichain.DigiByte:
		switch network {
		case multichain.NetworkMainnet:
			return &digibyte.MainNetParams
		case multichain.NetworkTestnet:
			return &digibyte.TestnetParams
		default:
			return &digibyte.RegressionNetParams
		}
	case multichain.Dogecoin:
		switch network {
		case multichain.NetworkMainnet:
			return &dogecoin.MainNetParams
		case multichain.NetworkTestnet:
			return &dogecoin.TestNetParams
		default:
			return &dogecoin.RegressionNetParams
		}
	default:
		panic(fmt.Errorf("cannot get network params: unknown chain %v", chain))
	}
}

func ZcashNetParams(network multichain.Network) *zcash.Params {
	switch network {
	case multichain.NetworkMainnet:
		return &zcash.MainNetParams
	case multichain.NetworkTestnet:
		return &zcash.TestNet3Params
	default:
		return &zcash.RegressionNetParams
	}
}

func AddressEncodeDecoder(chain multichain.Chain, network multichain.Network) multichain.AddressEncodeDecoder {
	switch chain {
	case multichain.Bitcoin, multichain.DigiByte, multichain.Dogecoin:
		params := NetParams(chain, network)
		return bitcoin.NewAddressEncodeDecoder(params)
	case multichain.BitcoinCash:
		params := NetParams(chain, network)
		return bitcoincash.NewAddressEncodeDecoder(params)
	case multichain.Zcash:
		params := ZcashNetParams(network)
		return zcash.NewAddressEncodeDecoder(params)
	case multichain.Avalanche, multichain.BinanceSmartChain, multichain.Ethereum, multichain.Fantom, multichain.Polygon, multichain.Moonbeam, multichain.Arbitrum, multichain.Goerli:
		return ethereum.NewAddressEncodeDecoder()
	case multichain.Filecoin:
		return filecoin.NewAddressEncodeDecoder()
	case multichain.Solana:
		return solana.NewAddressEncodeDecoder()
	case multichain.Terra:
		return terra.NewAddressEncodeDecoder()
	default:
		panic(fmt.Errorf("AddressEncodeDecoder : unknown chain %v", chain))
	}
}