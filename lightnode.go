package lightnode

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/go-redis/redis/v7"
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
	case multichain.NetworkMainnet, multichain.NetworkTestnet, multichain.NetworkDevnet, multichain.NetworkLocalnet:
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
	bindingsOpts := txenginebindings.DefaultOptions().
		WithNetwork(options.Network)
	for chain, chainOpts := range options.Chains {
		bindingsOpts = bindingsOpts.WithChainOptions(chain, chainOpts)
	}
	bindings, err := txenginebindings.New(bindingsOpts)
	if err != nil {
		logger.Panicf("bad bindings: %v", err)
	}

	// ==== BEGIN GROSS HACK
	//
	// TODO: For now we use a custom set of bindings for the transaction
	// verifier (with confirmations set to zero) as we want the initial
	// verification to succeed even if the transaction has not received any
	// confirmations.
	//

	verifierBindingsOpts := txenginebindings.DefaultOptions().
		WithNetwork(options.Network)
	for chain, chainOpts := range options.Chains {
		chainOpts.Confirmations = 0
		verifierBindingsOpts = verifierBindingsOpts.WithChainOptions(chain, chainOpts)
	}
	verifierBindings, err := txenginebindings.New(verifierBindingsOpts)
	if err != nil {
		logger.Panicf("bad bindings: %v", err)
	}

	// ==== END GROSS HACK
	//

	updater := updater.New(logger, multiStore, options.UpdaterPollRate, options.ClientTimeout)
	dispatcher := dispatcher.New(logger, options.ClientTimeout, multiStore, opts)
	ttlCache := kv.NewTTLCache(ctx, kv.NewMemDB(kv.JSONCodec), "cacher", options.TTL)
	cacher := cacher.New(dispatcher, logger, ttlCache, opts, db)
	whitelist := make(map[tx.Selector]bool, len(options.Whitelist))
	for i := range options.Whitelist {
		whitelist[options.Whitelist[i]] = true
	}
	engine := txengine.New(
		txengine.DefaultOptions().
			WithWhitelist(whitelist),
		nil,
		verifierBindings,
	)
	verifier := txpoolverifier.New(engine)
	resolverI := resolver.New(logger, cacher, multiStore, verifier, db, serverOptions)
	server := jsonrpc.NewServer(serverOptions, resolverI, resolver.NewVerifier(verifierBindings, options.DistPubKey, client, db))
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
	ethGateways := bindings.EthereumGateways()
	ethClients := bindings.EthereumClients()
	for chain, contracts := range ethGateways {
		for asset, bindings := range contracts {
			selector := tx.Selector(fmt.Sprintf("%v/from%v", asset, chain))
			if watchers[chain] == nil {
				watchers[chain] = map[multichain.Asset]watcher.Watcher{}
			}
			watchers[chain][asset] = watcher.NewWatcher(logger, selector, verifierBindings, ethClients[chain], bindings, resolverI, client, options.DistPubKey, options.WatcherPollRate)
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
