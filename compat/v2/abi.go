package v2

import (
	"github.com/renproject/darknode/engine"
	"github.com/renproject/pack"
)

type QueryBlockStateJSON struct {
	State State `json:"state"`
}

type UTXOShards struct {
	PubKey pack.Bytes             `json:"pubKey"`
	Queue  []interface{}          `json:"queue"`
	Shard  pack.Bytes32           `json:"shard"`
	State  engine.XStateShardUTXO `json:"state"`
}

type UTXOState struct {
	DustAmount    pack.U256    `json:"dustAmount"`
	GasCap        pack.U256    `json:"gasCap"`
	GasLimit      pack.U256    `json:"gasLimit"`
	GasPrice      pack.U256    `json:"gasPrice"`
	LatestHeight  pack.U256    `json:"latestHeight"`
	MinimumAmount pack.U256    `json:"minimumAmount"`
	Shards        []UTXOShards `json:"shards"`
}

type AccountState struct {
	DustAmount    pack.U256       `json:"dustAmount"`
	GasCap        pack.U256       `json:"gasCap"`
	GasLimit      pack.U256       `json:"gasLimit"`
	GasPrice      pack.U256       `json:"gasPrice"`
	LatestHeight  pack.U256       `json:"latestHeight"`
	MinimumAmount pack.U256       `json:"minimumAmount"`
	Shards        []AccountShards `json:"shards"`
}

type AccountShards struct {
	PubKey pack.Bytes                `json:"pubKey"`
	Queue  []interface{}             `json:"queue"`
	Shard  pack.Bytes32              `json:"shard"`
	State  engine.XStateShardAccount `json:"state"`
}

type State struct {
	BCH    UTXOState          `json:"BCH,omitempty"`
	BTC    UTXOState          `json:"BTC,omitempty"`
	DGB    UTXOState          `json:"DGB,omitempty"`
	DOGE   UTXOState          `json:"DOGE,omitempty"`
	FIL    AccountState       `json:"FIL,omitempty"`
	LUNA   AccountState       `json:"LUNA,omitempty"`
	System engine.SystemState `json:"System,omitempty"`
	ZEC    UTXOState          `json:"ZEC,omitempy"`
}
