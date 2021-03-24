package v1

import (
	v2 "github.com/renproject/lightnode/compat/v2"
	"github.com/renproject/multichain"
)

type QueryStateResponse struct {
	State StateResponse `json:"state"`
}

type StateResponse struct {
	Bitcoin     UTXOState    `json:"Bitcoin,omitempty"`
	Bitcoincash UTXOState    `json:"BitcoinCash,omitempty"`
	Digibyte    UTXOState    `json:"DigiByte,omitempty"`
	Dogecoin    UTXOState    `json:"Dogecoin,omitempty"`
	Filecoin    AccountState `json:"Filecoin,omitempty"`
	Terra       AccountState `json:"Terra,omitempty"`
	Zcash       UTXOState    `json:"Zcash,omitempty"`
}
type Outpoint struct {
	Hash  string `json:"hash"`
	Index string `json:"index"`
}
type Output struct {
	Outpoint     Outpoint `json:"outpoint"`
	Pubkeyscript string   `json:"pubKeyScript"`
	Value        string   `json:"value"`
}
type UTXOState struct {
	Address           string `json:"address"`
	Dust              string `json:"dust"`
	Gascap            string `json:"gasCap"`
	Gaslimit          string `json:"gasLimit"`
	Gasprice          string `json:"gasPrice"`
	Latestchainhash   string `json:"latestChainHash"`
	Latestchainheight string `json:"latestChainHeight"`
	Minimumamount     string `json:"minimumAmount"`
	Output            Output `json:"output"`
	Pubkey            string `json:"pubKey"`
}
type Gnonces struct {
	Address string `json:"address"`
	Nonce   string `json:"nonce"`
}
type AccountState struct {
	Address           string    `json:"address"`
	Gascap            string    `json:"gasCap"`
	Gaslimit          string    `json:"gasLimit"`
	Gasprice          string    `json:"gasPrice"`
	Gnonces           []Gnonces `json:"gnonces"`
	Latestchainhash   string    `json:"latestChainHash"`
	Latestchainheight string    `json:"latestChainHeight"`
	Minimumamount     string    `json:"minimumAmount"`
	Nonce             string    `json:"nonce"`
	Pubkey            string    `json:"pubKey"`
}

func QueryStateResponseFromState(utxostates map[string]v2.UTXOState, accountstates map[string]v2.AccountState) (QueryStateResponse, error) {
	stateResponse := StateResponse{}

	bitcoinS, ok := utxostates[string(multichain.Bitcoin.NativeAsset())]
	if ok && len(bitcoinS.Shards) != 0 {
		// do we want to log that we are missing bitcoin
		// return StateResponse{},
		// 	fmt.Errorf("Missing Bitcoin State")

		btcShard := bitcoinS.Shards[0]
		btcOutput := btcShard.State

		stateResponse.Bitcoin = UTXOState{
			Address:           btcShard.Shard.String(),
			Dust:              bitcoinS.DustAmount.String(),
			Gascap:            bitcoinS.GasCap.String(),
			Gaslimit:          bitcoinS.GasLimit.String(),
			Gasprice:          bitcoinS.GasPrice.String(),
			Latestchainheight: bitcoinS.LatestHeight.String(),
			Minimumamount:     bitcoinS.MinimumAmount.String(),
			Output: Output{
				Outpoint: Outpoint{
					Hash:  btcOutput.Hash.String(),
					Index: btcOutput.Index.String(),
				},
				Pubkeyscript: btcOutput.PubKeyScript.String(),
				Value:        btcOutput.Value.String(),
			},
			Pubkey: btcShard.PubKey.String(),
		}

	}

	zcashS, ok := utxostates[string(multichain.Zcash.NativeAsset())]
	if ok && len(zcashS.Shards) != 0 {

		zecShard := zcashS.Shards[0]

		zecOutput := zecShard.State

		stateResponse.Zcash = UTXOState{
			Address:           zecShard.Shard.String(),
			Dust:              zcashS.DustAmount.String(),
			Gascap:            zcashS.GasCap.String(),
			Gaslimit:          zcashS.GasLimit.String(),
			Gasprice:          zcashS.GasPrice.String(),
			Latestchainheight: zcashS.LatestHeight.String(),
			Minimumamount:     zcashS.MinimumAmount.String(),
			Output: Output{
				Outpoint: Outpoint{
					Hash:  zecOutput.Hash.String(),
					Index: zecOutput.Index.String(),
				},
				Pubkeyscript: zecOutput.PubKeyScript.String(),
				Value:        zecOutput.Value.String(),
			},
			Pubkey: zecShard.PubKey.String(),
		}
	}

	bitcoinCashS, ok := utxostates[string(multichain.BitcoinCash.NativeAsset())]
	if ok && len(bitcoinCashS.Shards) != 0 {

		bchShard := bitcoinCashS.Shards[0]

		bchOutput := bchShard.State

		stateResponse.Bitcoincash = UTXOState{
			Address:           bchShard.Shard.String(),
			Dust:              bitcoinCashS.DustAmount.String(),
			Gascap:            bitcoinCashS.GasCap.String(),
			Gaslimit:          bitcoinCashS.GasLimit.String(),
			Gasprice:          bitcoinCashS.GasPrice.String(),
			Latestchainheight: bitcoinCashS.LatestHeight.String(),
			Minimumamount:     bitcoinCashS.MinimumAmount.String(),
			Output: Output{
				Outpoint: Outpoint{
					Hash:  bchOutput.Hash.String(),
					Index: bchOutput.Index.String(),
				},
				Pubkeyscript: bchOutput.PubKeyScript.String(),
				Value:        bchOutput.Value.String(),
			},
			Pubkey: bchShard.PubKey.String(),
		}
	}

	digibyteS, ok := utxostates[string(multichain.DigiByte.NativeAsset())]
	if ok && len(digibyteS.Shards) != 0 {

		dgbShard := digibyteS.Shards[0]

		dgbOutput := dgbShard.State

		stateResponse.Digibyte = UTXOState{
			Address:           dgbShard.Shard.String(),
			Dust:              digibyteS.DustAmount.String(),
			Gascap:            digibyteS.GasCap.String(),
			Gaslimit:          digibyteS.GasLimit.String(),
			Gasprice:          digibyteS.GasPrice.String(),
			Latestchainheight: digibyteS.LatestHeight.String(),
			Minimumamount:     digibyteS.MinimumAmount.String(),
			Output: Output{
				Outpoint: Outpoint{
					Hash:  dgbOutput.Hash.String(),
					Index: dgbOutput.Index.String(),
				},
				Pubkeyscript: dgbOutput.PubKeyScript.String(),
				Value:        dgbOutput.Value.String(),
			},
			Pubkey: dgbShard.PubKey.String(),
		}

	}

	dogecoinS, ok := utxostates[string(multichain.DigiByte.NativeAsset())]
	if ok && len(dogecoinS.Shards) != 0 {

		dogeShard := dogecoinS.Shards[0]

		dogeOutput := dogeShard.State

		stateResponse.Dogecoin = UTXOState{
			Address:           dogeShard.Shard.String(),
			Dust:              dogecoinS.DustAmount.String(),
			Gascap:            dogecoinS.GasCap.String(),
			Gaslimit:          dogecoinS.GasLimit.String(),
			Gasprice:          dogecoinS.GasPrice.String(),
			Latestchainheight: dogecoinS.LatestHeight.String(),
			Minimumamount:     dogecoinS.MinimumAmount.String(),
			Output: Output{
				Outpoint: Outpoint{
					Hash:  dogeOutput.Hash.String(),
					Index: dogeOutput.Index.String(),
				},
				Pubkeyscript: dogeOutput.PubKeyScript.String(),
				Value:        dogeOutput.Value.String(),
			},
			Pubkey: dogeShard.PubKey.String(),
		}

	}

	terraS, ok := accountstates[string(multichain.Terra.NativeAsset())]
	if ok && len(terraS.Shards) != 0 {

		lunaShard := terraS.Shards[0]

		lunaOutput := lunaShard.State

		terra := AccountState{
			Address:           lunaShard.Shard.String(),
			Gascap:            terraS.GasCap.String(),
			Gaslimit:          terraS.GasLimit.String(),
			Gasprice:          terraS.GasPrice.String(),
			Latestchainheight: terraS.LatestHeight.String(),
			Minimumamount:     terraS.MinimumAmount.String(),
			Nonce:             lunaOutput.Nonce.String(),
			Pubkey:            lunaShard.PubKey.String(),
		}

		for _, v := range lunaOutput.Gnonces {
			gnonce := Gnonces{
				Address: v.Address.String(),
				Nonce:   v.Nonce.String(),
			}
			terra.Gnonces = append(terra.Gnonces, gnonce)
		}
		stateResponse.Terra = terra

	}

	filecoinS, ok := accountstates[string(multichain.Filecoin.NativeAsset())]
	if ok && len(filecoinS.Shards) != 0 {

		filShard := filecoinS.Shards[0]

		filOutput := filShard.State

		filecoin := AccountState{
			Address:           filShard.Shard.String(),
			Gascap:            filecoinS.GasCap.String(),
			Gaslimit:          filecoinS.GasLimit.String(),
			Gasprice:          filecoinS.GasPrice.String(),
			Latestchainheight: filecoinS.LatestHeight.String(),
			Minimumamount:     filecoinS.MinimumAmount.String(),
			Nonce:             filOutput.Nonce.String(),
			Pubkey:            filShard.PubKey.String(),
		}

		for _, v := range filOutput.Gnonces {
			gnonce := Gnonces{
				Address: v.Address.String(),
				Nonce:   v.Nonce.String(),
			}
			filecoin.Gnonces = append(filecoin.Gnonces, gnonce)
		}
		stateResponse.Filecoin = filecoin

	}

	return QueryStateResponse{
		State: stateResponse,
	}, nil
}
