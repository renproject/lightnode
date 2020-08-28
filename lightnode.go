package lightnode

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/go-redis/redis/v7"
	"github.com/renproject/darknode"
	"github.com/renproject/darknode/darknodeutil"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/darknode/txengine"
	"github.com/renproject/darknode/txengine/txenginebindings"
	"github.com/renproject/darknode/txpool/txpoolverifier"
	"github.com/renproject/kv"
	"github.com/renproject/lightnode/cacher"
	"github.com/renproject/lightnode/confirmer"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/lightnode/dispatcher"
	"github.com/renproject/lightnode/resolver"
	"github.com/renproject/lightnode/store"
	"github.com/renproject/lightnode/updater"
	"github.com/renproject/lightnode/watcher"
	"github.com/renproject/multichain"
	"github.com/renproject/multichain/api/gas"
	"github.com/renproject/multichain/chain/ethereum"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

// Lightnode is the top level container that encapsulates the functionality of
// the lightnode.
type Lightnode struct {
	options   Options
	logger    logrus.FieldLogger
	db        db.DB
	server    *jsonrpc.Server
	updater   updater.Updater
	confirmer confirmer.Confirmer
	watchers  map[multichain.Chain]map[multichain.Asset]watcher.Watcher

	// Tasks
	cacher     phi.Task
	dispatcher phi.Task
}

// New constructs a new Lightnode.
func New(options Options, ctx context.Context, logger logrus.FieldLogger, sqlDB *sql.DB, client *redis.Client) Lightnode {
	switch options.Network {
	case darknode.Mainnet, darknode.Testnet, darknode.Devnet:
	default:
		panic("unknown network")
	}
	if options.DistPubKey == nil {
		panic("distributed public key not specified")
	}
	if options.Port == "" {
		panic("port not specified")
	}
	if len(options.BootstrapAddrs) == 0 {
		panic("bootstrap addresses not specified")
	}

	// Define the options used for all Phi tasks.
	opts := phi.Options{Cap: options.Cap}

	// Initialise the database.
	db := db.New(sqlDB)
	if err := db.Init(); err != nil {
		logger.Panicf("failed to initialise db: %v", err)
	}

	// Define the options used for the server.
	serverOptions := jsonrpc.DefaultOptions().
		WithMaxBatchSize(options.MaxBatchSize).
		WithMaxPageSize(options.MaxPageSize).
		WithTimeout(options.ServerTimeout)

	// Initialise the multi-address store.
	table := kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses")
	multiStore := store.New(table, options.BootstrapAddrs)

	// Initialise the blockchain adapter.
	ethCompatClients, ethCompatContracts, accountTxBuilders, accountGasEstimators := darknodeutil.SetupTxengineAccountBindings(options.RPCs, options.Gateways)
	utxoCompatClients, utxoTxBuilders, utxoGasEstimators := darknodeutil.SetupTxengineUTXOBindings()
	gasEstimators := make(map[multichain.Chain]gas.Estimator, len(accountGasEstimators)+len(utxoGasEstimators))
	for k, v := range accountGasEstimators {
		gasEstimators[k] = v
	}
	for k, v := range utxoGasEstimators {
		gasEstimators[k] = v
	}
	bindings := txenginebindings.New(
		map[multichain.Chain]multichain.AccountClient{},
		accountTxBuilders,
		map[multichain.Chain]multichain.AddressEncodeDecoder{
			multichain.Ethereum: ethereum.NewAddressEncodeDecoder(),
		},
		map[multichain.Chain]multichain.ContractCaller{},
		gasEstimators,
		utxoCompatClients,
		utxoTxBuilders,
		ethCompatClients,
		ethCompatContracts,
		options.Confirmations,
	)

	updater := updater.New(logger, multiStore, options.UpdaterPollRate, options.ClientTimeout)
	dispatcher := dispatcher.New(logger, options.ClientTimeout, multiStore, opts)
	ttlCache := kv.NewTTLCache(ctx, kv.NewMemDB(kv.JSONCodec), "cacher", options.TTL)
	cacher := cacher.New(dispatcher, logger, ttlCache, opts, db)
	verifier := txpoolverifier.New(txengine.New(txengine.DefaultOptions(), nil, bindings))
	resolver := resolver.New(logger, cacher, multiStore, verifier, db, serverOptions)
	server := jsonrpc.NewServer(serverOptions, resolver)
	confirmer := confirmer.New(
		confirmer.DefaultOptions().
			WithLogger(logger).
			WithPollInterval(options.ConfirmerPollRate).
			WithExpiry(options.TransactionExpiry),
		dispatcher,
		db,
		bindings,
	)
	watchers := map[multichain.Chain]map[multichain.Asset]watcher.Watcher{}
	for chain, contracts := range ethCompatContracts {
		for asset, bindings := range contracts {
			selector := tx.Selector(fmt.Sprintf("%v/from%v", asset, chain))
			watchers[chain][asset] = watcher.NewWatcher(logger, selector, ethCompatClients[chain], bindings, resolver, client, options.WatcherPollRate)
		}
	}

	return Lightnode{
		options:    options,
		logger:     logger,
		db:         db,
		updater:    updater,
		dispatcher: dispatcher,
		cacher:     cacher,
		server:     server,
		confirmer:  confirmer,
		watchers:   watchers,
	}
}

// Run starts the `Lightnode`. This function call is blocking.
func (lightnode Lightnode) Run(ctx context.Context) {
	go lightnode.updater.Run(ctx)
	go lightnode.cacher.Run(ctx)
	go lightnode.dispatcher.Run(ctx)

	// Note: the following should be disabled when running locally.
	go lightnode.confirmer.Run(ctx)
	for _, assetMap := range lightnode.watchers {
		for _, watcher := range assetMap {
			go watcher.Run(ctx)
		}
	}

	lightnode.server.Listen(ctx, fmt.Sprintf(":%s", lightnode.options.Port))
}
