package lightnode

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/go-redis/redis/v7"
	"github.com/renproject/darknode/binding"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/kv"
	"github.com/renproject/lightnode/cacher"
	v0 "github.com/renproject/lightnode/compat/v0"
	"github.com/renproject/lightnode/confirmer"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/lightnode/dispatcher"
	"github.com/renproject/lightnode/resolver"
	"github.com/renproject/lightnode/store"
	"github.com/renproject/lightnode/updater"
	"github.com/renproject/lightnode/watcher"
	"github.com/renproject/multichain"
	"github.com/renproject/pack"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"

	solanaRPC "github.com/dfuse-io/solana-go/rpc"
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
	bindingsOpts := binding.DefaultOptions().
		WithNetwork(options.Network)
	for chain, chainOpts := range options.Chains {
		chainOpts.MaxConfirmations = pack.MaxU64 // TODO: Eventually we will want to fetch this from the Darknode.
		bindingsOpts = bindingsOpts.WithChainOptions(chain, chainOpts)
	}
	bindings := binding.New(bindingsOpts)

	// ==== BEGIN GROSS HACK
	//
	// TODO: For now we use a custom set of bindings for the transaction
	// verifier (with confirmations set to zero) as we want the initial
	// verification to succeed even if the transaction has not received any
	// confirmations.
	//

	verifierBindingsOpts := binding.DefaultOptions().
		WithNetwork(options.Network)
	for chain, chainOpts := range options.Chains {
		chainOpts.Confirmations = 0
		chainOpts.MaxConfirmations = pack.MaxU64 // TODO: Eventually we will want to fetch this from the Darknode.
		verifierBindingsOpts = verifierBindingsOpts.WithChainOptions(chain, chainOpts)
	}
	verifierBindings := binding.New(verifierBindingsOpts)

	// ==== END GROSS HACK
	//

	updater := updater.New(logger, multiStore, options.UpdaterPollRate, options.ClientTimeout)
	dispatcher := dispatcher.New(logger, options.ClientTimeout, multiStore, opts)
	ttlCache := kv.NewTTLCache(ctx, kv.NewMemDB(kv.JSONCodec), "cacher", options.TTL)
	cacher := cacher.New(dispatcher, logger, ttlCache, opts, db)

	compatStore := v0.NewCompatStore(db, client)
	hostChains := map[multichain.Chain]bool{}
	for _, selector := range options.Whitelist {
		if selector.IsLock() && selector.IsMint() {
			hostChains[selector.Destination()] = true
		}
	}
	verifier := resolver.NewVerifier(hostChains, verifierBindings)
	resolverI := resolver.New(options.Network, logger, cacher, multiStore, db, serverOptions, compatStore, bindings, verifier)
	limiter := resolver.NewRateLimiter(resolver.RateLimiterConf{
		GlobalMethodRate: options.LimiterGlobalRates,
		IpMethodRate:     options.LimiterIPRates,
		Ttl:              options.LimiterTTL,
		MaxClients:       options.LimiterMaxClients,
	})
	server := jsonrpc.NewServer(serverOptions, resolverI, resolver.NewValidator(verifierBindings, options.DistPubKey, compatStore, &limiter, logger))
	confirmer := confirmer.New(
		confirmer.DefaultOptions().
			WithLogger(logger).
			WithPollInterval(options.ConfirmerPollRate).
			WithExpiry(options.TransactionExpiry),
		dispatcher,
		db,
		bindings,
	)

	whitelistMap := map[tx.Selector]bool{}
	for _, i := range options.Whitelist {
		whitelistMap[i] = true
	}

	watchers := map[multichain.Chain]map[multichain.Asset]watcher.Watcher{}

	// Ethereum watchers
	ethGateways := bindings.EthereumGateways()
	ethClients := bindings.EthereumClients()
	for chain, contracts := range ethGateways {
		for asset, bindings := range contracts {
			selector := tx.Selector(fmt.Sprintf("%v/from%v", asset, chain))
			if !whitelistMap[selector] {
				logger.Info("not watching", selector)
				continue
			}
			if watchers[chain] == nil {
				watchers[chain] = map[multichain.Asset]watcher.Watcher{}
			}
			burnLogFetcher := watcher.NewEthBurnLogFetcher(bindings)
			blockHeightFetcher := watcher.NewEthBlockHeightFetcher(ethClients[chain])
			watchers[chain][selector.Asset()] = watcher.NewWatcher(logger, options.Network, selector, verifierBindings, burnLogFetcher, blockHeightFetcher, resolverI, client, options.DistPubKey, options.WatcherPollRate, options.WatcherMaxBlockAdvance, options.WatcherConfidenceInterval)
			logger.Info("watching", selector)
		}
	}

	// Solana watchers
	solanaGateways := bindings.ContractGateways()[multichain.Solana]
	solClient := solanaRPC.NewClient(bindingsOpts.Chains[multichain.Solana].RPC.String())
	for asset, bindings := range solanaGateways {
		chain := multichain.Solana
		selector := tx.Selector(fmt.Sprintf("%v/from%v", asset, chain))
		if !whitelistMap[selector] {
			logger.Info("not watching ", selector)
			continue
		}
		if watchers[chain] == nil {
			watchers[chain] = map[multichain.Asset]watcher.Watcher{}
		}
		solanaFetcher := watcher.NewSolFetcher(solClient, string(bindings))
		watchers[chain][selector.Asset()] = watcher.NewWatcher(logger, options.Network, selector, verifierBindings, solanaFetcher, solanaFetcher, resolverI, client, options.DistPubKey, options.WatcherPollRate, options.WatcherMaxBlockAdvance, options.WatcherConfidenceInterval)
		logger.Info("watching ", selector)
		logger.Info("at ", bindings)
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
