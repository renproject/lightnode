package lightnode

import (
	"context"
	"database/sql"

	"github.com/go-redis/redis/v7"
	"github.com/renproject/darknode"
	"github.com/renproject/lightnode/db"
	"github.com/sirupsen/logrus"
)

// Lightnode is the top level container that encapsulates the functionality of
// the lightnode.
type Lightnode struct {
	options Options
	logger  logrus.FieldLogger
	db      db.DB
	/* server     *jsonrpc.Server
	updater    updater.Updater
	confirmer  confirmer.Confirmer
	submitter  submitter.Submitter
	btcWatcher watcher.Watcher
	zecWatcher watcher.Watcher
	bchWatcher watcher.Watcher

	// Tasks
	cacher     phi.Task
	dispatcher phi.Task */
}

// New constructs a new Lightnode.
func New(options Options, ctx context.Context, logger logrus.FieldLogger, sqlDB *sql.DB, client *redis.Client) Lightnode {
	switch options.Network {
	case darknode.Mainnet, darknode.Testnet, darknode.Devnet:
	default:
		panic("unknown network")
	}
	if options.Key == nil {
		panic("private key for submitting gasless transactions not specified")
	}
	if options.DistPubKey == nil {
		panic("distributed public key not specified")
	}
	if options.Port == "" {
		panic("port not specified")
	}
	if options.ProtocolAddr == "" {
		panic("protocol contract address not specified")
	}
	if len(options.BootstrapAddrs) == 0 {
		panic("bootstrap addresses not specified")
	}

	// Define the options used for all Phi tasks.
	// opts := phi.Options{Cap: options.Cap}

	// Initialise the database.
	db := db.New(sqlDB)
	if err := db.Init(); err != nil {
		logger.Panicf("fail to initialize db, err = %v", err)
	}

	// Define the options used for the server.
	/* serverOptions := jsonrpc.DefaultOptions().
	WithMaxBatchSize(options.MaxBatchSize).
	WithMaxPageSize(options.MaxPageSize).
	WithTimeout(options.ServerTimeout)

	// TODO: Define minimum confirmations for each chain.
	confirmerOptions := confirmer.DefaultOptions().
		WithPollInterval(options.ConfirmerPollRate).
		WithExpiry(options.TransactionExpiry)

	// Initialise the multi-address store.
	table := kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses")
	multiStore := store.New(table, options.BootstrapAddrs)

	// Initialise the blockchain adapter.
	protocolAddr := common.HexToAddress(options.ProtocolAddr)
	btcClient := btcclient.NewClient(logger, darknode.BtcNetwork(types.Bitcoin, options.Network))
	zecClient := btcclient.NewClient(logger, darknode.BtcNetwork(types.ZCash, options.Network))
	bchClient := btcclient.NewClient(logger, darknode.BtcNetwork(types.BitcoinCash, options.Network))
	ethClient, err := ethclient.New(logger, darknode.EthShifterNetwork(options.Network))
	if err != nil {
		panic(fmt.Errorf("cannot initialise eth client: %v", err))
	}
	protocol, err := ethrpc.NewProtocol(ethClient.EthClient(), protocolAddr)
	if err != nil {
		panic(fmt.Errorf("cannot initialise protocol contract: %v", err))
	}
	bc := blockchain.New(logger, btcClient, zecClient, bchClient, ethClient, protocol)

	updater := updater.New(logger, multiStore, options.UpdaterPollRate, options.ClientTimeout)
	dispatcher := dispatcher.New(logger, options.ClientTimeout, multiStore, opts)
	ttlCache := kv.NewTTLCache(ctx, kv.NewMemDB(kv.JSONCodec), "cacher", options.TTL)
	cacher := cacher.New(dispatcher, logger, ttlCache, opts, db)
	resolver := resolver.New(logger, cacher, multiStore, *options.DisPubkey, bc, db, serverOptions)
	server := jsonrpc.NewServer(serverOptions, resolver)
	confirmer := confirmer.New(logger, confirmerOptions, dispatcher, db, bc)
	submitter := submitter.New(logger, dispatcher, db, ethClient, options.Key, options.SubmitterPollRate)
	btcWatcher := watcher.NewWatcher(logger, "BTC0Eth2Btc", bc, resolver, client, options.WatcherPollRate)
	zecWatcher := watcher.NewWatcher(logger, "ZEC0Eth2Zec", bc, resolver, client, options.WatcherPollRate)
	bchWatcher := watcher.NewWatcher(logger, "BCH0Eth2Bch", bc, resolver, client, options.WatcherPollRate) */

	return Lightnode{
		options: options,
		logger:  logger,
		db:      db,
		// server:     server,
		// updater:    updater,
		// confirmer:  confirmer,
		// submitter:  submitter,
		// btcWatcher: btcWatcher,
		// zecWatcher: zecWatcher,
		// bchWatcher: bchWatcher,

		// cacher:     cacher,
		// dispatcher: dispatcher,
	}
}

// Run starts the `Lightnode`. This function call is blocking.
func (lightnode Lightnode) Run(ctx context.Context) {
	/* go lightnode.updater.Run(ctx)
	go lightnode.cacher.Run(ctx)
	go lightnode.dispatcher.Run(ctx)

	// Note: the following should be disabled when running locally.
	go lightnode.confirmer.Run(ctx)
	go lightnode.btcWatcher.Run(ctx)
	go lightnode.zecWatcher.Run(ctx)
	go lightnode.bchWatcher.Run(ctx)
	go lightnode.submitter.Run(ctx)

	lightnode.server.Listen(ctx, fmt.Sprintf(":%s", lightnode.options.Port)) */
}
