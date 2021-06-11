package watcher

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	solanaSDK "github.com/dfuse-io/solana-go"
	solanaRPC "github.com/dfuse-io/solana-go/rpc"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-redis/redis/v7"
	"github.com/jbenet/go-base58"
	"github.com/near/borsh-go"
	"github.com/renproject/darknode/binding"
	"github.com/renproject/darknode/binding/gatewaybinding"
	"github.com/renproject/darknode/binding/solanastate"
	"github.com/renproject/darknode/engine"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/id"
	v0 "github.com/renproject/lightnode/compat/v0"
	"github.com/renproject/multichain"
	"github.com/renproject/multichain/chain/bitcoin"
	"github.com/renproject/multichain/chain/bitcoincash"
	"github.com/renproject/multichain/chain/solana"
	"github.com/renproject/multichain/chain/zcash"
	"github.com/renproject/pack"
	"github.com/sirupsen/logrus"
)

type BurnInfo struct {
	Txid        pack.Bytes
	Amount      pack.U256
	ToBytes     []byte
	Nonce       pack.Bytes32
	BlockNumber pack.U64
}

type BurnLogResult struct {
	Result BurnInfo
	Error  error
}

type BurnLogFetcher interface {
	FetchBurnLogs(ctx context.Context, from uint64, to uint64) (chan BurnLogResult, error)
}

type EthBurnLogFetcher struct {
	bindings *gatewaybinding.MintGatewayLogicV1
}

func NewEthBurnLogFetcher(bindings *gatewaybinding.MintGatewayLogicV1) EthBurnLogFetcher {
	return EthBurnLogFetcher{
		bindings: bindings,
	}
}

// This will fetch the burn event logs using the ethereum bindings and emit them via a channel
// We do this so that we can unit test the log handling without calling ethereum
func (fetcher EthBurnLogFetcher) FetchBurnLogs(ctx context.Context, from uint64, to uint64) (chan BurnLogResult, error) {
	iter, err := fetcher.bindings.FilterLogBurn(
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
	resultChan := make(chan BurnLogResult)

	go func() {
		func() {
			defer close(resultChan)
			for iter.Next() {
				nonce := iter.Event.N.Uint64()
				var nonceBytes pack.Bytes32
				copy(nonceBytes[:], pack.NewU256FromU64(pack.NewU64(nonce)).Bytes())

				result := BurnInfo{
					Txid:        iter.Event.Raw.TxHash.Bytes(),
					Amount:      pack.NewU256FromInt(iter.Event.Amount),
					ToBytes:     iter.Event.To,
					Nonce:       nonceBytes,
					BlockNumber: pack.NewU64(iter.Event.Raw.BlockNumber),
				}

				// Send the burn transaction to the resolver.
				select {
				case <-ctx.Done():
					resultChan <- BurnLogResult{Error: ctx.Err()}
					return
				default:
					resultChan <- BurnLogResult{Result: result}
				}
			}
		}()

		// Always close the iter to clear the event subscription
		err = iter.Close()
		if err != nil {
			resultChan <- BurnLogResult{Error: err}
			return
		}

		// Iter should stop if an error occurs,
		// so no need to check on each iteration
		err := iter.Error()
		if err != nil {
			resultChan <- BurnLogResult{Error: err}
			return
		}

	}()

	return resultChan, nil
}

type SolFetcher struct {
	client           *solanaRPC.Client
	gatewayStatePubk solanaSDK.PublicKey
	gatewayAddress   string
}

func NewSolFetcher(client *solanaRPC.Client, gatewayAddress string) SolFetcher {
	seeds := []byte("GatewayStateV0.1.3")
	programDerivedAddress := solana.ProgramDerivedAddress(pack.Bytes(seeds), multichain.Address(gatewayAddress))
	programPubk, err := solanaSDK.PublicKeyFromBase58(string(programDerivedAddress))
	if err != nil {
		panic("invalid pubk")
	}

	return SolFetcher{
		client:           client,
		gatewayStatePubk: programPubk,
		gatewayAddress:   gatewayAddress,
	}
}

func (fetcher SolFetcher) FetchBurnLogs(ctx context.Context, from uint64, to uint64) (chan BurnLogResult, error) {
	resultChan := make(chan BurnLogResult)

	go func() {
		defer close(resultChan)
		for i := from; i < to; i++ {
			nonce := i

			b := make([]byte, 8)
			binary.LittleEndian.PutUint64(b, i)

			var nonceBytes pack.Bytes32
			copy(nonceBytes[:], pack.NewU256FromU64(pack.NewU64(nonce)).Bytes())

			programDerivedAddress := solana.ProgramDerivedAddress(b, multichain.Address(fetcher.gatewayAddress))

			programPubk, err := solanaSDK.PublicKeyFromBase58(string(programDerivedAddress))
			if err != nil {
				resultChan <- BurnLogResult{Error: fmt.Errorf("getting burn log account: %v", err)}
				return
			}

			// Fetch account data at gateway registry's state
			accountInfo, err := fetcher.client.GetAccountInfo(ctx, programPubk)
			if err != nil {
				resultChan <- BurnLogResult{Error: fmt.Errorf("getting burn log data for burn: %v err: %v", i, err)}
				return
			}
			data := accountInfo.Value.Data

			if len(data) != 41 {
				resultChan <- BurnLogResult{Error: fmt.Errorf("deserializing burn log data: expected data len 41, got %v", len(data))}
				return
			}
			amount := binary.LittleEndian.Uint64(data[:8])
			recipientLen := uint8(data[8:9][0])
			recipient := multichain.RawAddress(data[9 : 9+int(recipientLen)])

			signatures, err := fetcher.client.GetConfirmedSignaturesForAddress2(ctx, programPubk, &solanaRPC.GetConfirmedSignaturesForAddress2Opts{})
			if err != nil {
				resultChan <- BurnLogResult{Error: fmt.Errorf("getting burn log txes: %v", err)}
				return
			}

			if len(signatures) == 0 {
				resultChan <- BurnLogResult{Error: fmt.Errorf("Burn signature not confirmed")}
				return
			}

			result := BurnInfo{
				Txid:        base58.Decode(signatures[0].Signature),
				Amount:      pack.NewU256FromUint64(amount),
				ToBytes:     recipient[:],
				Nonce:       nonceBytes,
				BlockNumber: pack.NewU64(i),
			}

			// Send the burn transaction to the resolver.
			select {
			case <-ctx.Done():
				resultChan <- BurnLogResult{Error: ctx.Err()}
				return
			default:
				resultChan <- BurnLogResult{Result: result}
			}
		}
	}()

	return resultChan, nil
}

type BlockHeightFetcher interface {
	FetchBlockHeight(ctx context.Context) (uint64, error)
}

type EthBlockHeightFetcher struct {
	client *ethclient.Client
}

func NewEthBlockHeightFetcher(ethClient *ethclient.Client) EthBlockHeightFetcher {
	return EthBlockHeightFetcher{
		client: ethClient,
	}
}

// currentBlockNumber returns the current block number for the chain being watched
func (fetcher EthBlockHeightFetcher) FetchBlockHeight(ctx context.Context) (uint64, error) {
	currentBlock, err := fetcher.client.HeaderByNumber(ctx, nil)
	if err != nil {
		return 0, err
	}
	return currentBlock.Number.Uint64(), nil
}

// Behaves differently, as it checks the maximum burn index that should be fetched
func (fetcher SolFetcher) FetchBlockHeight(ctx context.Context) (uint64, error) {
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

// Watcher watches for event logs for burn transactions. These transactions are
// then forwarded to the cacher.
type Watcher struct {
	network            multichain.Network
	logger             logrus.FieldLogger
	gpubkey            pack.Bytes
	selector           tx.Selector
	bindings           binding.Bindings
	burnLogFetcher     BurnLogFetcher
	blockHeightFetcher BlockHeightFetcher
	resolver           jsonrpc.Resolver
	cache              redis.Cmdable
	pollInterval       time.Duration
	maxBlockAdvance    uint64
	confidenceInterval uint64
}

// NewWatcher returns a new Watcher.
func NewWatcher(logger logrus.FieldLogger, network multichain.Network, selector tx.Selector, bindings binding.Bindings, burnLogFetcher BurnLogFetcher, blockHeightFetcher BlockHeightFetcher, resolver jsonrpc.Resolver, cache redis.Cmdable, distPubKey *id.PubKey, pollInterval time.Duration, maxBlockAdvance uint64, confidenceInterval uint64) Watcher {
	gpubkey := (*btcec.PublicKey)(distPubKey).SerializeCompressed()
	return Watcher{
		logger:             logger,
		network:            network,
		gpubkey:            gpubkey,
		selector:           selector,
		bindings:           bindings,
		burnLogFetcher:     burnLogFetcher,
		blockHeightFetcher: blockHeightFetcher,
		resolver:           resolver,
		cache:              cache,
		pollInterval:       pollInterval,
		maxBlockAdvance:    maxBlockAdvance,
		confidenceInterval: confidenceInterval,
	}
}

// Run starts the watcher until the context is canceled.
func (watcher Watcher) Run(ctx context.Context) {
	ticker := time.NewTicker(watcher.pollInterval)
	defer ticker.Stop()

	for {
		watcher.watchLogShiftOuts(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// watchLogShiftOuts checks logs that have occurred between current block number
// and the last checked block number. It constructs a `jsonrpc.Request` from
// these events and forwards them to the resolver.
func (watcher Watcher) watchLogShiftOuts(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, watcher.pollInterval)
	defer cancel()

	// Get current block number and last checked block number.
	currentHeight, err := watcher.blockHeightFetcher.FetchBlockHeight(ctx)
	if err != nil {
		watcher.logger.Warnf("[watcher] error loading block header: %v", err)
		return
	}

	lastHeight, err := watcher.lastCheckedBlockNumber(currentHeight)
	if err != nil {
		watcher.logger.Errorf("[watcher] error loading last checked block number: %v", err)
		return
	}

	if currentHeight <= lastHeight {
		watcher.logger.Warnf("[watcher] tried to process old blocks")
		// Make sure we do not process old events. This could occur if there is
		// an issue with the underlying blockchain node, for example if it needs
		// to resync.
		//
		// Processing old events is generally not an issue as the Lightnode will
		// drop transactions if it detects they have already been handled by the
		// Darknode, however in the case the transaction backlog builds up
		// substantially, it can cause the Lightnode to be rate limited by the
		// Darknode upon dispatching requests.
		return
	}

	// Only advance by a set number of blocks at a time to prevent over-subscription
	step := lastHeight + watcher.maxBlockAdvance
	if step < currentHeight {
		currentHeight = step
	}

	// for Eth, avoid checking blocks that might have shuffled
	if watcher.selector.Source() != multichain.Solana {
		currentHeight -= watcher.confidenceInterval
	}

	// Fetch logs
	c, err := watcher.burnLogFetcher.FetchBurnLogs(ctx, lastHeight, currentHeight)
	if err != nil {
		watcher.logger.Warnf("[watcher] error fetching LogBurn events from=%v to=%v: %v", lastHeight, currentHeight, err)
		return
	}

	// Loop through the logs and check if there are burn events.
	for res := range c {
		if res.Error != nil {
			watcher.logger.Errorf("[watcher] error iterating LogBurn events from=%v to=%v: %v", lastHeight, currentHeight, res.Error)
			return
		}
		burn := res.Result
		nonce := burn.Nonce
		amount := burn.Amount
		to := burn.ToBytes

		watcher.logger.Infof("[watcher] detected burn for %v  with nonce=%v", watcher.selector.String(), nonce)

		// Send the burn transaction to the resolver.
		params, err := watcher.burnToParams(burn.Txid, amount, to, nonce, watcher.gpubkey)
		if err != nil {
			watcher.logger.Errorf("[watcher] cannot get params from burn transaction (to=%v, amount=%v, nonce=%v): %v", to, amount, nonce, err)
			continue
		}

		response := watcher.resolver.SubmitTx(ctx, 0, &params, nil)
		if response.Error != nil {
			watcher.logger.Errorf("[watcher] invalid burn transaction %v: %v", params, response.Error.Message)
			// return so that we retry, if the burnToParams are valid, the darknode should accept the tx
			// we assume that the only failure case would be RPC/darknode backpressure, so we backoff here
			return
		}
	}

	if err := watcher.cache.Set(watcher.key(), currentHeight, 0).Err(); err != nil {
		watcher.logger.Errorf("[watcher] error setting last checked block number in redis: %v", err)
		return
	}
}

// key returns the key that is used to store the last checked block.
func (watcher Watcher) key() string {
	return fmt.Sprintf("%v_lastCheckedBlock", watcher.selector.String())
}

// lastCheckedBlockNumber returns the last checked block number of Ethereum.
func (watcher Watcher) lastCheckedBlockNumber(currentBlockN uint64) (uint64, error) {
	last, err := watcher.cache.Get(watcher.key()).Uint64()
	// Initialise the pointer with current block number if it has not been yet.
	if err == redis.Nil {
		watcher.logger.Warnf("[watcher] last checked block number not initialised")
		if err := watcher.cache.Set(watcher.key(), currentBlockN, 0).Err(); err != nil {
			watcher.logger.Errorf("[watcher] cannot initialise last checked block in redis: %v", err)
			return 0, err
		}
		return currentBlockN, nil
	}

	return last, err
}

// burnToParams constructs params for a SubmitTx request with given ref.
func (watcher Watcher) burnToParams(txid pack.Bytes, amount pack.U256, toBytes []byte, nonce pack.Bytes32, gpubkey pack.Bytes) (jsonrpc.ParamsSubmitTx, error) {
	var version tx.Version
	var to multichain.Address
	var toDecoded []byte
	var err error
	burnChain := watcher.selector.Source()
	switch burnChain {
	case multichain.Solana:
		version, to, toDecoded, err = watcher.handleAssetAddrSolana(toBytes)
	default:
		version, to, toDecoded, err = watcher.handleAssetAddrEth(toBytes)
	}
	if err != nil {
		return jsonrpc.ParamsSubmitTx{}, err
	}

	watcher.logger.Infof("[watcher] burn parameters (to=%v, amount=%v, nonce=%v)", string(to), amount, nonce)

	txindex := pack.U32(0)
	payload := pack.Bytes{}
	phash := engine.Phash(payload)
	nhash := engine.Nhash(nonce, txid, txindex)
	ghash := engine.Ghash(watcher.selector, phash, toDecoded, nonce)
	input, err := pack.Encode(engine.LockMintBurnReleaseInput{
		Txid:    txid,
		Txindex: txindex,
		Amount:  amount,
		Payload: payload,
		Phash:   phash,
		To:      pack.String(to),
		Nonce:   nonce,
		Nhash:   nhash,
		Gpubkey: gpubkey,
		Ghash:   ghash,
	})
	if err != nil {
		return jsonrpc.ParamsSubmitTx{}, err
	}
	hash, err := tx.NewTxHash(version, watcher.selector, pack.Typed(input.(pack.Struct)))
	if err != nil {
		return jsonrpc.ParamsSubmitTx{}, err
	}
	transaction := tx.Tx{
		Hash:     hash,
		Version:  version,
		Selector: watcher.selector,
		Input:    pack.Typed(input.(pack.Struct)),
	}

	// Map the v0 burn txhash to v1 txhash so that it is still
	// queryable
	// We don't get the required data during tx submission rpc to track it there,
	// so we persist here in order to not re-filter all burn events
	v0Hash := v0.BurnTxHash(watcher.selector, pack.NewU256(nonce))
	watcher.cache.Set(v0Hash.String(), transaction.Hash.String(), 0)

	// Map the selector + burn ref to the v0 hash so that we can return something
	// to ren-js v1
	watcher.cache.Set(fmt.Sprintf("%s_%v", watcher.selector, pack.NewU256(nonce).String()), v0Hash.String(), 0)

	return jsonrpc.ParamsSubmitTx{Tx: transaction}, nil
}

func (watcher Watcher) handleAssetAddrEth(toBytes []byte) (tx.Version, multichain.Address, []byte, error) {
	// For v0 burn, `to` can be base58 encoded
	version := tx.Version1
	to := multichain.Address(toBytes)
	switch watcher.selector.Asset() {
	case multichain.BTC, multichain.BCH, multichain.ZEC:
		decoder := AddressEncodeDecoder(watcher.selector.Asset().OriginChain(), watcher.network)
		_, err := decoder.DecodeAddress(to)
		if err != nil {
			to = multichain.Address(base58.Encode(toBytes))
			_, err = decoder.DecodeAddress(to)
			if err != nil {
				return "-1", "", nil, err
			}
			version = tx.Version0
		}
	}

	burnChain := watcher.selector.Destination()
	toBytes, err := watcher.bindings.DecodeAddress(burnChain, to)
	if err != nil {
		return "-1", "", nil, err
	}

	return version, to, toBytes, nil
}

func (watcher Watcher) handleAssetAddrSolana(toBytes []byte) (tx.Version, multichain.Address, []byte, error) {
	encoder := AddressEncodeDecoder(watcher.selector.Asset().OriginChain(), watcher.network)
	to, err := encoder.EncodeAddress(toBytes)
	if err != nil {
		return "-1", "", nil, fmt.Errorf("encoding raw asset address returned by solana: %v", err)
	}
	return tx.Version1, to, toBytes, nil
}

func AddressEncodeDecoder(chain multichain.Chain, network multichain.Network) multichain.AddressEncodeDecoder {
	switch chain {
	case multichain.Bitcoin, multichain.DigiByte, multichain.Dogecoin:
		params := NetParams(network, chain)
		return bitcoin.NewAddressEncodeDecoder(params)
	case multichain.BitcoinCash:
		params := NetParams(network, chain)
		return bitcoincash.NewAddressEncodeDecoder(params)
	case multichain.Zcash:
		params := ZcashNetParams(network)
		return zcash.NewAddressEncodeDecoder(params)
	default:
		panic(fmt.Errorf("unknown chain %v", chain))
	}
}

func ZcashNetParams(network multichain.Network) *zcash.Params {
	switch network {
	case multichain.NetworkMainnet:
		return &zcash.MainNetParams
	case multichain.NetworkDevnet, multichain.NetworkTestnet:
		return &zcash.TestNet3Params
	default:
		return &zcash.RegressionNetParams
	}
}

func NetParams(network multichain.Network, chain multichain.Chain) *chaincfg.Params {
	switch chain {
	case multichain.Bitcoin, multichain.BitcoinCash:
		switch network {
		case multichain.NetworkMainnet:
			return &chaincfg.MainNetParams
		case multichain.NetworkDevnet, multichain.NetworkTestnet:
			return &chaincfg.TestNet3Params
		default:
			return &chaincfg.RegressionNetParams
		}
	default:
		panic(fmt.Errorf("cannot get network params: unknown chain %v", chain))
	}
}
