package watcher

import (
	"context"
	"encoding/binary"
	"fmt"

	solanaSDK "github.com/dfuse-io/solana-go"
	solanaRPC "github.com/dfuse-io/solana-go/rpc"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/jbenet/go-base58"
	"github.com/near/borsh-go"
	"github.com/renproject/darknode/binding"
	"github.com/renproject/darknode/binding/solanastate"
	"github.com/renproject/multichain"
	"github.com/renproject/multichain/chain/solana"
	"github.com/renproject/pack"
)

type EventInfo struct {
	Asset       multichain.Asset
	Txid        pack.Bytes
	Amount      pack.U256
	ToBytes     []byte
	Nonce       pack.Bytes32
	BlockNumber pack.U64
}

type Fetcher interface {
	LatestBlockHeight(ctx context.Context) (uint64, error)

	FetchBurnLogs(ctx context.Context, from, to uint64) ([]EventInfo, error)
}

type ethFetcher struct {
	chain    multichain.Chain
	assets   []multichain.Asset
	bindings *binding.Binding
}

func NewEthFetcher(chain multichain.Chain, bindings *binding.Binding, assets []multichain.Asset) Fetcher {

	// Make sure we have initialized the gateway for all supported assets
	for _, asset := range assets {
		gateway := bindings.MintGateway(chain, asset)
		if gateway == nil {
			panic(fmt.Sprintf("gateway for %v on %v is not initialized", chain, asset))
		}
	}

	return ethFetcher{
		chain:    chain,
		assets:   assets,
		bindings: bindings,
	}
}

func (fetcher ethFetcher) LatestBlockHeight(ctx context.Context) (uint64, error) {
	latestBlock, err := fetcher.bindings.LatestBlock(ctx, fetcher.chain)
	if err != nil {
		return 0, err
	}
	return latestBlock.Uint64(), nil
}

func (fetcher ethFetcher) FetchBurnLogs(ctx context.Context, from, to uint64) ([]EventInfo, error) {
	var events []EventInfo
	for _, asset := range fetcher.assets {
		gateway := fetcher.bindings.MintGateway(fetcher.chain, asset)
		iter, err := gateway.FilterLogBurn(
			&bind.FilterOpts{
				Context: ctx,
				Start:   from,
				End:     &to,
			},
			nil,
			nil,
		)
		if err != nil {
			return nil, err
		}
		defer iter.Close()

		for iter.Next() {
			nonce := iter.Event.BurnNonce.Uint64()
			var nonceBytes pack.Bytes32
			copy(nonceBytes[:], pack.NewU256FromU64(pack.NewU64(nonce)).Bytes())
			event := EventInfo{
				Asset:       asset,
				Txid:        iter.Event.Raw.TxHash.Bytes(),
				Amount:      pack.NewU256FromInt(iter.Event.Amount),
				ToBytes:     iter.Event.To,
				Nonce:       nonceBytes,
				BlockNumber: pack.NewU64(iter.Event.Raw.BlockNumber),
			}

			events = append(events, event)
		}
		if iter.Error() != nil {
			return nil, iter.Error()
		}
	}

	return events, nil
}

type solFetcher struct {
	client           *solanaRPC.Client
	asset            multichain.Asset
	gatewayStatePubk solanaSDK.PublicKey
	gatewayAddress   string
}

func NewSolFetcher(client *solanaRPC.Client, asset multichain.Asset, gatewayAddr string) Fetcher {
	seeds := []byte("GatewayStateV0.1.4")
	programDerivedAddress := solana.ProgramDerivedAddress(seeds, multichain.Address(gatewayAddr))
	programPubk, err := solanaSDK.PublicKeyFromBase58(string(programDerivedAddress))
	if err != nil {
		panic("invalid pubkey")
	}
	return solFetcher{
		client:           client,
		asset:            asset,
		gatewayStatePubk: programPubk,
		gatewayAddress:   gatewayAddr,
	}
}

func (fetcher solFetcher) LatestBlockHeight(ctx context.Context) (uint64, error) {
	accountData, err := fetcher.client.GetAccountInfo(ctx, fetcher.gatewayStatePubk)
	if err != nil {
		return 0, fmt.Errorf("getting gateway data: %v", err)
	}

	// Deserialize the account data into registry state's structure.
	gateway := solanastate.Gateway{}
	if err = borsh.Deserialize(&gateway, accountData.Value.Data); err != nil {
		return 0, fmt.Errorf("deserializing account data: %v", err)
	}

	// We increment the burnCount by 1, as internally its indexes start at 1
	return uint64(gateway.BurnCount) + 1, nil
}

func (fetcher solFetcher) FetchBurnLogs(ctx context.Context, from, to uint64) ([]EventInfo, error) {
	var events []EventInfo

	for i := from; i < to; i++ {
		nonce := i

		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, i)

		var nonceBytes pack.Bytes32
		copy(nonceBytes[:], pack.NewU256FromU64(pack.NewU64(nonce)).Bytes())

		burnLogDerivedAddress := solana.ProgramDerivedAddress(b, multichain.Address(fetcher.gatewayAddress))

		burnLogPubk, err := solanaSDK.PublicKeyFromBase58(string(burnLogDerivedAddress))
		if err != nil {
			continue
		}

		// Fetch account data at gateway's state
		accountInfo, err := fetcher.client.GetAccountInfo(ctx, burnLogPubk)
		if err != nil {
			return nil, err
		}
		data := accountInfo.Value.Data
		amount, recipient, err := solanastate.DecodeBurnLog(data)
		if err != nil {
			return nil, err
		}

		signatures, err := fetcher.client.GetSignaturesForAddress(ctx, burnLogPubk, &solanaRPC.GetSignaturesForAddressOpts{})
		if err != nil {
			legacySignatures, err2 := fetcher.client.GetConfirmedSignaturesForAddress2(ctx, burnLogPubk, &solanaRPC.GetConfirmedSignaturesForAddress2Opts{})
			if err2 != nil {
				return nil, err2
			}
			signatures = solanaRPC.GetSignaturesForAddressResult(legacySignatures)
		}

		// NOTE: We assume the burn watcher will always run before a signature gets pruned
		// manual intervention will be required to skip a burns where the signatures are no longer
		// returned by the nodes
		if len(signatures) == 0 {
			return nil, fmt.Errorf("burn signature not confirmed")
		}

		event := EventInfo{
			Asset:       fetcher.asset,
			Txid:        base58.Decode(signatures[0].Signature),
			Amount:      amount,
			ToBytes:     []byte(recipient),
			Nonce:       nonceBytes,
			BlockNumber: pack.NewU64(i),
		}
		events = append(events, event)
	}
	return events, nil
}
