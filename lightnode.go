package lightnode

import (
	"context"
	"crypto/ecdsa"
	"database/sql"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-redis/redis/v7"
	"github.com/renproject/darknode"
	"github.com/renproject/darknode/addr"
	"github.com/renproject/darknode/consensus/txcheck/transform/blockchain"
	"github.com/renproject/darknode/ethrpc"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/kv"
	"github.com/renproject/lightnode/cacher"
	"github.com/renproject/lightnode/confirmer"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/lightnode/dispatcher"
	"github.com/renproject/lightnode/resolver"
	"github.com/renproject/lightnode/store"
	"github.com/renproject/lightnode/submitter"
	"github.com/renproject/lightnode/updater"
	"github.com/renproject/lightnode/watcher"
	"github.com/renproject/mercury/sdk/client/btcclient"
	"github.com/renproject/mercury/sdk/client/ethclient"
	"github.com/renproject/mercury/types"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

// Options for setting up a Lightnode, usually parsed from environment variables.
type Options struct {
	Network           darknode.Network
	Key               *ecdsa.PrivateKey
	DisPubkey         *ecdsa.PublicKey
	Port              string
	ProtocolAddr      string
	Cap               int
	MaxBatchSize      int
	MaxPageSize       int
	ServerTimeout     time.Duration
	ClientTimeout     time.Duration
	TTL               time.Duration
	UpdaterPollRate   time.Duration
	ConfirmerPollRate time.Duration
	WatcherPollRate   time.Duration
	SubmitterPollRate time.Duration
	Expiry            time.Duration
	BootstrapAddrs    addr.MultiAddresses
}

// SetZeroToDefault does basic verification of options and set fields with zero
// value to default.
func (options *Options) SetZeroToDefault() {
	switch options.Network {
	case darknode.Mainnet, darknode.Chaosnet:
	case darknode.Testnet, darknode.Devnet, darknode.Localnet:
	default:
		panic("unknown networks")
	}
	if options.Key == nil {
		panic("please specify the key of lightnode account for submitting gasless txs.")
	}
	if options.DisPubkey == nil {
		panic("distributed public key is not initialized in the options")
	}
	if options.Port == "" {
		panic("port is not set in the options")
	}
	if options.ProtocolAddr == "" {
		panic("protocol contract address is not defined")
	}
	if len(options.BootstrapAddrs) == 0 {
		panic("bootstrap addresses are not set in the options")
	}
	if options.Cap == 0 {
		options.Cap = 128
	}
	if options.MaxBatchSize == 0 {
		options.MaxBatchSize = 10
	}
	if options.MaxPageSize == 0 {
		options.MaxPageSize = 10
	}
	if options.ServerTimeout == 0 {
		options.ServerTimeout = 15 * time.Second
	}
	if options.ClientTimeout == 0 {
		options.ClientTimeout = time.Minute
	}
	if options.TTL == 0 {
		options.TTL = 3 * time.Second
	}
	if options.UpdaterPollRate == 0 {
		options.UpdaterPollRate = 5 * time.Minute
	}
	if options.ConfirmerPollRate == 0 {
		options.ConfirmerPollRate = 30 * time.Second
	}
	if options.WatcherPollRate == 0 {
		options.WatcherPollRate = 15 * time.Second
	}
	if options.SubmitterPollRate == 0 {
		options.SubmitterPollRate = 15 * time.Second
	}
	if options.Expiry == 0 {
		options.Expiry = 14 * 24 * time.Hour
	}
}

// Lightnode is the top level container that encapsulates the functionality of
// the lightnode.
type Lightnode struct {
	options    Options
	logger     logrus.FieldLogger
	db         db.DB
	server     *jsonrpc.Server
	updater    updater.Updater
	confirmer  confirmer.Confirmer
	submitter  submitter.Submitter
	btcWatcher watcher.Watcher
	zecWatcher watcher.Watcher
	bchWatcher watcher.Watcher

	// Tasks
	cacher     phi.Task
	dispatcher phi.Task
}

// New constructs a new `Lightnode`.
func New(ctx context.Context, options Options, logger logrus.FieldLogger, sqlDB *sql.DB, client *redis.Client) Lightnode {
	options.SetZeroToDefault()
	// Define the options used for all Phi tasks.
	opts := phi.Options{Cap: options.Cap}

	// Initialise the database.
	db := db.New(sqlDB)
	if err := db.Init(); err != nil {
		logger.Panicf("fail to initialize db, err = %v", err)
	}

	// Define the options used for the server.
	serverOptions := jsonrpc.DefaultOptions()
	serverOptions.MaxBatchSize = options.MaxBatchSize
	serverOptions.MaxPageSize = options.MaxPageSize
	serverOptions.Timeout = options.ServerTimeout

	// TODO: These are currently not configurable from environment variables.
	confirmerOptions := confirmer.Options{
		MinConfirmations: darknode.DefaultMinConfirmations(options.Network),
		PollInterval:     options.ConfirmerPollRate,
		Expiry:           options.Expiry,
	}

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
	bchWatcher := watcher.NewWatcher(logger, "BCH0Eth2Bch", bc, resolver, client, options.WatcherPollRate)

	return Lightnode{
		options:    options,
		logger:     logger,
		db:         db,
		server:     server,
		updater:    updater,
		confirmer:  confirmer,
		submitter:  submitter,
		btcWatcher: btcWatcher,
		zecWatcher: zecWatcher,
		bchWatcher: bchWatcher,

		cacher:     cacher,
		dispatcher: dispatcher,
	}
}

// Run starts the `Lightnode`. This function call is blocking.
func (lightnode Lightnode) Run(ctx context.Context) {
	go lightnode.updater.Run(ctx)
	go lightnode.cacher.Run(ctx)
	go lightnode.dispatcher.Run(ctx)
	go lightnode.confirmer.Run(ctx)
	go lightnode.btcWatcher.Run(ctx)
	go lightnode.zecWatcher.Run(ctx)
	go lightnode.bchWatcher.Run(ctx)
	go lightnode.submitter.Run(ctx)

	lightnode.server.Listen(ctx, fmt.Sprintf(":%s", lightnode.options.Port))
}
