package lightnode

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/go-redis/redis/v7"
	"github.com/renproject/darknode/binding"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/kv"
	"github.com/renproject/lightnode/cacher"
	v0 "github.com/renproject/lightnode/compat/v0"
	v1 "github.com/renproject/lightnode/compat/v1"
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
	"go.uber.org/zap"

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
func New(options Options, ctx context.Context, logger logrus.FieldLogger, sqlDB *sql.DB, client redis.Cmdable) Lightnode {
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
	db := db.New(sqlDB, options.MaxGatewayCount)
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
	loggerConfig := zap.NewProductionConfig()
	loggerConfig.DisableStacktrace = true
	bindingsLogger, err := loggerConfig.Build()
	if err != nil {
		panic(fmt.Errorf("cannot init logger: %v", err))
	}
	bindingsOpts := binding.DefaultOptions().
		WithLogger(bindingsLogger).
		WithNetwork(options.Network)
	for chain, chainOpts := range options.Chains {
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
		WithLogger(bindingsLogger).
		WithNetwork(options.Network)
	for chain, chainOpts := range options.Chains {
		chainOpts.Confirmations = 0
		verifierBindingsOpts = verifierBindingsOpts.WithChainOptions(chain, chainOpts)
	}
	verifierBindings := binding.New(verifierBindingsOpts)

	// ==== END GROSS HACK
	//

	updater := updater.New(logger, multiStore, options.UpdaterPollRate, options.ClientTimeout)
	dispatcher := dispatcher.New(logger, options.ClientTimeout, multiStore, opts)
	ttlCache := kv.NewTTLCache(ctx, kv.NewMemDB(kv.JSONCodec), "cacher", options.TTL)
	cacher := cacher.New(dispatcher, logger, ttlCache, opts, db)

	versionStore := v0.NewCompatStore(db, client, options.TransactionExpiry)
	gpubkeyStore := v1.NewCompatStore(client)
	hostChains := map[multichain.Chain]bool{}
	for _, selector := range options.Whitelist {
		if selector.IsLock() && selector.IsMint() {
			hostChains[selector.Destination()] = true
		}
	}
	verifier := resolver.NewVerifier(hostChains, verifierBindings)
	resolverI := resolver.New(options.Network, logger, cacher, multiStore, db, serverOptions, versionStore, gpubkeyStore, bindings, verifier)
	limiter := resolver.NewRateLimiter(resolver.RateLimiterConf{
		GlobalMethodRate: options.LimiterGlobalRates,
		IpMethodRate:     options.LimiterIPRates,
		Ttl:              options.LimiterTTL,
		MaxClients:       options.LimiterMaxClients,
	})
	server := jsonrpc.NewServer(serverOptions, resolverI, resolver.NewValidator(options.Network, verifierBindings, options.DistPubKey, versionStore, gpubkeyStore, &limiter, logger))
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
	solClient := solanaRPC.NewClient(bindingsOpts.Chains[multichain.Solana].RPC.String())
	for _, selector := range options.Whitelist {
		if !selector.IsBurn() || !selector.IsRelease() {
			continue
		}
		chain := selector.Source()
		asset := selector.Asset()
		if watchers[chain] == nil {
			watchers[chain] = map[multichain.Asset]watcher.Watcher{}
		}
		if _, ok := watchers[chain][asset]; ok {
			continue
		}
		var burnLogFetcher watcher.BurnLogFetcher
		var blockHeightFetcher watcher.BlockHeightFetcher
		if chain == multichain.Solana {
			if bindings.ContractGateway(chain, asset) == "" {
				continue
			}
			burnLogFetcher = watcher.NewSolFetcher(solClient, string(bindings.ContractGateway(chain, asset)))
			blockHeightFetcher = watcher.NewSolFetcher(solClient, string(bindings.ContractGateway(chain, asset)))
		} else {
			burnLogFetcher = watcher.NewEthBurnLogFetcher(bindings.EthereumGateway(chain, asset))
			blockHeightFetcher = watcher.NewEthBlockHeightFetcher(bindings.EthereumClient(chain))
		}
		watchers[chain][selector.Asset()] = watcher.NewWatcher(logger, options.Network, selector, verifierBindings, burnLogFetcher, blockHeightFetcher, resolverI, client, options.WatcherPollRate, options.WatcherMaxBlockAdvance, options.WatcherConfidenceInterval)
		logger.Info("watching", selector)
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
