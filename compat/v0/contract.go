package v0

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"golang.org/x/crypto/sha3"
)

// A Shard stores multiple Contracts. The Txs sent to a Shard are guaranteed to
// be strictly ordered within that Shard.
type Shard [32]byte

// String implements the `fmt.Stringer` interface.
func (shard Shard) String() string {
	return base64.RawStdEncoding.EncodeToString(shard[:])
}

var (
	// IntrinsicDefault is the default Shard.
	IntrinsicDefault = Shard(sha3.Sum256([]byte{}))
	// IntrinsicBtc is the Shard for all assets that originate on the Bitcoin
	// blockchain.
	IntrinsicBtc = Shard(sha3.Sum256([]byte("Btc")))
	// IntrinsicZec is the Shard for all assets that originate on the ZCash
	// blockchain.
	IntrinsicZec = Shard(sha3.Sum256([]byte("Zec")))
	// IntrinsicBch is the Shard for all assets that originate on the Bitcoin
	// cash blockchain.
	IntrinsicBch = Shard(sha3.Sum256([]byte("Bch")))
	// IntrinsicEth is the Shard for all assets that originate on the Ethereum
	// blockchain.
	IntrinsicEth = Shard(sha3.Sum256([]byte("Eth")))
)

func ValidateAddress(addr Address) error {
	switch addr {
	case IntrinsicBTC0Btc2Eth.Address,
		IntrinsicZEC0Zec2Eth.Address,
		IntrinsicBCH0Bch2Eth.Address,
		IntrinsicBTC0Eth2Btc.Address,
		IntrinsicZEC0Eth2Zec.Address,
		IntrinsicBCH0Eth2Bch.Address:
		return nil
	default:
		return fmt.Errorf("invalid v0 contract address")
	}
}

// IsShiftIn returns if the given contract address is a shiftIn contract. Please
// only use this function when you know the contract exists, otherwise it will
// panic.
func IsShiftIn(addr Address) bool {
	switch addr {
	case IntrinsicBTC0Btc2Eth.Address, IntrinsicZEC0Zec2Eth.Address, IntrinsicBCH0Bch2Eth.Address:
		return true
	default:
		return false
	}
}

// IntrinsicBTC0Btc2Eth transfers BTC from the Bitcoin blockchain to the
// Ethereum blockchain.
var IntrinsicBTC0Btc2Eth = Contract{
	Address: "BTC0Btc2Eth",
	In: Formals{
		{"p", ExtTypeEthCompatPayload},     // The payload data
		{"token", ExtTypeEthCompatAddress}, // The ERC20 contract address on Etherum for ZBTC
		{"to", ExtTypeEthCompatAddress},    // The address on the Ethereum blockchain to which ZBTC will be transferred
		{"n", TypeB32},                     // The nonce is used to randomise the gateway
		{"utxo", ExtTypeBtcCompatUTXO},     // The UTXO sent to the gateway
	},
	Autogen: Formals{
		{"ghash", TypeB32},             // The hash returned by `keccak256(abi.encode(phash, token, to, n))`
		{"nhash", TypeB32},             // The hash returned by `keccak256(abi.encode(n, txhash))`
		{"phash", TypeB32},             // The hash of the payload data
		{"sighash", TypeB32},           // The hash returned by `keccak256(abi.encode(phash, amount, token, to, nhash))`
		{"amount", TypeU256},           // The amount of token to shift in.
		{"utxo", ExtTypeBtcCompatUTXO}, // The utxo which has all details filled by darknode.
	},
	Out: Formals{
		{"r", TypeB32}, // The R component of a signature over the input hash
		{"s", TypeB32}, // The S component of a signature over the input hash (lower range)
		{"v", TypeU8},  // The V component of a signature over the input hash
	},
}

// IntrinsicBTC0Eth2Btc transfers BTC from the Ethereum blockchain to the
// Bitcoin blockchain.
var IntrinsicBTC0Eth2Btc = Contract{
	Address: "BTC0Eth2Btc",
	In: Formals{
		{"ref", TypeU64}, // The ref of the `LogShiftOut` event from Ethereum
	},
	Out: Formals{
		{"txhash", TypeB}, // The Bitcoin transaction hash from the gateway to the RenVM address
	},
}

// IntrinsicBCH0Bch2Eth transfers BCH from the BitcoinCash blockchain to the
// Ethereum blockchain.
var IntrinsicBCH0Bch2Eth = Contract{
	Address: "BCH0Bch2Eth",
	In: Formals{
		{"p", ExtTypeEthCompatPayload},     // The payload data
		{"token", ExtTypeEthCompatAddress}, // The ERC20 contract address on Etherum for ZBCH
		{"to", ExtTypeEthCompatAddress},    // The address on the Ethereum blockchain to which ZBCH will be transferred
		{"n", TypeB32},                     // The nonce is used to randomise the gateway
		{"utxo", ExtTypeBtcCompatUTXO},     // The UTXO sent to the gateway
	},
	Autogen: Formals{
		{"ghash", TypeB32},             // The hash returned by `keccak256(abi.encode(phash, token, to, n))`
		{"nhash", TypeB32},             // The hash returned by `keccak256(abi.encode(n, txhash))`
		{"phash", TypeB32},             // The hash of the payload data
		{"sighash", TypeB32},           // The hash returned by `keccak256(abi.encode(phash, amount, token, to, nhash))`
		{"amount", TypeU256},           // The amount of token to shift in.
		{"utxo", ExtTypeBtcCompatUTXO}, // The utxo which has all details filled by darknode.
	},
	Out: Formals{
		{"r", TypeB32}, // The R component of a signature over the input hash
		{"s", TypeB32}, // The S component of a signature over the input hash (lower range)
		{"v", TypeU8},  // The V component of a signature over the input hash
	},
}

// IntrinsicBCH0Eth2Bch transfers BCH from the Ethereum blockchain to the
// BitcoinCash blockchain.
var IntrinsicBCH0Eth2Bch = Contract{
	Address: "BCH0Eth2Bch",
	In: Formals{
		{"ref", TypeU64}, // The ref of the `LogShiftOut` event from Ethereum
	},
	Out: Formals{
		{"txhash", TypeB}, // The BitcoinCash transaction hash from the gateway to the RenVM address
	},
}

// IntrinsicZEC0Zec2Eth transfers ZEC from the ZCash blockchain to the Ethereum
// blockchain.
var IntrinsicZEC0Zec2Eth = Contract{
	Address: "ZEC0Zec2Eth",
	In: Formals{
		{"p", ExtTypeEthCompatPayload},     // The payload data
		{"token", ExtTypeEthCompatAddress}, // The ERC20 contract address on Etherum for ZZEC
		{"to", ExtTypeEthCompatAddress},    // The address on the Ethereum blockchain to which ZZEC will be transferred
		{"n", TypeB32},                     // The nonce is used to randomise the gateway
		{"utxo", ExtTypeBtcCompatUTXO},     // The UTXO sent to the gateway
	},
	Autogen: Formals{
		{"ghash", TypeB32},             // The hash returned by `keccak256(abi.encode(phash, token, to, n))`
		{"nhash", TypeB32},             // The hash returned by `keccak256(abi.encode(n, txhash))`
		{"phash", TypeB32},             // The hash of the payload data
		{"sighash", TypeB32},           // The hash returned by `keccak256(abi.encode(phash, amount, token, to, nhash))`
		{"amount", TypeU256},           // The amount of token to shift in.
		{"utxo", ExtTypeBtcCompatUTXO}, // The utxo which has all details filled by darknode.
	},
	Out: Formals{
		{"r", TypeB32}, // The R component of a signature over the input hash
		{"s", TypeB32}, // The S component of a signature over the input hash (lower range)
		{"v", TypeU8},  // The V component of a signature over the input hash
	},
}

// IntrinsicZEC0Eth2Zec transfers ZEC from the Ethereum blockchain to the ZCash
// blockchain.
var IntrinsicZEC0Eth2Zec = Contract{
	Address: "ZEC0Eth2Zec",
	In: Formals{
		{"ref", TypeU64}, // The ref of the `LogShiftOut` event from Ethereum
	},
	Out: Formals{
		{"txhash", TypeB}, // The ZCash transaction hash from the gateway to the RenVM address
	},
}

// Intrinsics are Contracts that are always known to RenVM.
var Intrinsics = map[Address]Contract{
	IntrinsicBTC0Btc2Eth.Address: IntrinsicBTC0Btc2Eth,
	IntrinsicBTC0Eth2Btc.Address: IntrinsicBTC0Eth2Btc,
	IntrinsicZEC0Zec2Eth.Address: IntrinsicZEC0Zec2Eth,
	IntrinsicZEC0Eth2Zec.Address: IntrinsicZEC0Eth2Zec,
	IntrinsicBCH0Bch2Eth.Address: IntrinsicBCH0Bch2Eth,
	IntrinsicBCH0Eth2Bch.Address: IntrinsicBCH0Eth2Bch,
}

// Contracts is a wrapper type for the Contract slice type.
type Contracts []Contract

// A Contract ABI defines the expected components of a Contract. It must have a
// unique Addr, a slice of expected input Formals, and a slice of expected
// output Formals. Any Tx that is sent to a Contract must have Args that are
// compatible with the its input Formals.
type Contract struct {
	Address Address `json:"address"`
	In      Formals `json:"in"`
	Autogen Formals `json:"autogen"`
	Out     Formals `json:"out"`
}

// Formals is a wrapper type for the Formal slice type.
type Formals []Formal

// A Formal defines the expected name and type of a Value that will be passed
// in/out of a Contract. A Formal has no Value, because it is only a definition
// of the expected form of a Value.
type Formal struct {
	Name string `json:"name"`
	Type Type   `json:"type"`
}

// MarshalJSON implements the json.Marshaler interface for the Formal type.
func (formal Formal) MarshalJSON() ([]byte, error) {
	return json.Marshal(formal)
}

// UnmarshalJSON implements the json.Unmarshaler interface for the Formal type.
func (formal *Formal) UnmarshalJSON(data []byte) error {
	f := struct {
		Name string `json:"name"`
		Type Type   `json:"type"`
	}{}
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}

	switch f.Type {
	// Standard types
	case TypeAddress, TypeStr, TypeB32, TypeB, TypeI8, TypeI16, TypeI32, TypeI64, TypeI128, TypeI256, TypeU8, TypeU16, TypeU32, TypeU64, TypeU128, TypeU256:
		// Ok

	// Extended types
	case ExtTypeEthCompatAddress, ExtTypeBtcCompatUTXO, ExtTypeBtcCompatUTXOs, ExtTypeEthCompatTx, ExtTypeEthCompatPayload:
		// Ok

	default:
		return fmt.Errorf("unexpected type %s", f.Type)
	}

	formal.Name = f.Name
	formal.Type = f.Type
	return nil
}
