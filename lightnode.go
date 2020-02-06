package lightnode

import (
	"context"
	"crypto/ecdsa"
	"database/sql"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-redis/redis/v7"
	"github.com/renproject/darknode"
	"github.com/renproject/darknode/addr"
	"github.com/renproject/kv"
	"github.com/renproject/lightnode/blockchain"
	"github.com/renproject/lightnode/cacher"
	"github.com/renproject/lightnode/confirmer"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/lightnode/dispatcher"
	"github.com/renproject/lightnode/http"
	"github.com/renproject/lightnode/store"
	"github.com/renproject/lightnode/updater"
	"github.com/renproject/lightnode/validator"
	"github.com/renproject/lightnode/watcher"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

// Options for setting up a Lightnode, usually parsed from environment variables.
type Options struct {
	Network           darknode.Network
	DisPubkey         *ecdsa.PublicKey
	Port              string
	ProtocolAddr      string
	Cap               int
	MaxBatchSize      int
	ServerTimeout     time.Duration
	ClientTimeout     time.Duration
	TTL               time.Duration
	UpdaterPollRate   time.Duration
	ConfirmerPollRate time.Duration
	WatcherPollRate   time.Duration
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
		options.ConfirmerPollRate = 10 * time.Second
	}
	if options.WatcherPollRate == 0 {
		options.WatcherPollRate = 10 * time.Second
	}
	if options.Expiry == 0 {
		options.Expiry = 7 * 24 * time.Hour
	}
}

// Lightnode is the top level container that encapsulates the functionality of
// the lightnode.
type Lightnode struct {
	options    Options
	logger     logrus.FieldLogger
	server     *http.Server
	updater    updater.Updater
	confirmer  confirmer.Confirmer
	btcWatcher watcher.Watcher
	zecWatcher watcher.Watcher
	bchWatcher watcher.Watcher

	// Tasks
	validator  phi.Task
	cacher     phi.Task
	dispatcher phi.Task
}

// New constructs a new `Lightnode`.
func New(ctx context.Context, options Options, logger logrus.FieldLogger, sqlDB *sql.DB, client *redis.Client) Lightnode {
	options.SetZeroToDefault()
	// All tasks have the same capacity, and no scaling
	opts := phi.Options{Cap: options.Cap}

	// Initialize the databae
	db := db.New(sqlDB)
	if err := db.Init(); err != nil {
		logger.Panicf("fail to initialize db, err = %v", err)
	}

	// Server options
	serverOptions := http.Options{
		Port:         options.Port,
		MaxBatchSize: options.MaxBatchSize,
		Timeout:      options.ServerTimeout,
	}

	// TODO: This is currently not configurable from the ENV variables
	confirmerOptions := confirmer.Options{
		MinConfirmations: darknode.DefaultMinConfirmations(options.Network),
		PollInterval:     options.ConfirmerPollRate,
		Expiry:           options.Expiry,
	}

	// Create the store and insert the bootstrap addresses.
	multiStore := store.New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses"))
	for _, bootstrapAddr := range options.BootstrapAddrs {
		if err := multiStore.Insert(bootstrapAddr); err != nil {
			logger.Fatalf("cannot insert bootstrap address: %v", err)
		}
	}

	protocolAddr := common.HexToAddress(options.ProtocolAddr)
	connPool := blockchain.New(logger, options.Network, protocolAddr)
	updater := updater.New(logger, multiStore, options.UpdaterPollRate, options.ClientTimeout)
	dispatcher := dispatcher.New(logger, options.ClientTimeout, multiStore, opts)
	ttlCache := kv.NewTTLCache(ctx, kv.NewMemDB(kv.JSONCodec), "cacher", options.TTL)
	cacher := cacher.New(dispatcher, logger, ttlCache, opts, db)
	validator := validator.New(logger, cacher, multiStore, opts, *options.DisPubkey, connPool, db)
	server := http.New(logger, serverOptions, validator)
	confirmer := confirmer.New(logger, confirmerOptions, dispatcher, db, connPool)
	btcWatcher := watcher.NewWatcher(logger, "BTC0Eth2Btc", connPool, validator, client, options.WatcherPollRate)
	zecWatcher := watcher.NewWatcher(logger, "ZEC0Eth2Zec", connPool, validator, client, options.WatcherPollRate)
	bchWatcher := watcher.NewWatcher(logger, "BCH0Eth2Bch", connPool, validator, client, options.WatcherPollRate)

	return Lightnode{
		options:    options,
		logger:     logger,
		server:     server,
		updater:    updater,
		confirmer:  confirmer,
		btcWatcher: btcWatcher,
		zecWatcher: zecWatcher,
		bchWatcher: bchWatcher,

		validator:  validator,
		cacher:     cacher,
		dispatcher: dispatcher,
	}
}

// Run starts the `Lightnode`. This function call is blocking.
func (lightnode Lightnode) Run(ctx context.Context) {
	updater := os.Getenv("UPDATER")
	if updater == "1" {
		go lightnode.updater.Run(ctx)
	}

	go lightnode.validator.Run(ctx)
	go lightnode.cacher.Run(ctx)
	go lightnode.dispatcher.Run(ctx)
	go lightnode.confirmer.Run(ctx)
	go lightnode.btcWatcher.Run(ctx)
	go lightnode.zecWatcher.Run(ctx)
	go lightnode.bchWatcher.Run(ctx)

	lightnode.server.Listen(ctx)
}
