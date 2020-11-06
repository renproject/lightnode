package v0

import (
	"fmt"

	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/multichain"
	"github.com/renproject/pack"
)

// Takes a QueryState rpc response and converts it into a QueryShards rpc response
func ShardsFromState(state jsonrpc.ResponseQueryState) (ResponseQueryShards, error) {
	bitcoinPubkey, ok := state.State[multichain.Bitcoin].Get("pubKey").(pack.String)
	fmt.Printf("%v", state.State[multichain.Bitcoin])

	if !ok {
		return ResponseQueryShards{},
			fmt.Errorf("unexpected type for Bitcoin pubKey: expected pack.String , got %v",
				state.State[multichain.Bitcoin].Get("pubKey").Type())
	}

	shards := make([]Shard, 1)
	shards[0] = Shard{
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
