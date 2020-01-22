package lightnode

import (
	"context"
	"crypto/ecdsa"
	"database/sql"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-redis/redis/v7"
	"github.com/renproject/darknode"
	"github.com/renproject/darknode/abi"
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
	BtcShifterAddr    string
	ZecShifterAddr    string
	BchShifterAddr    string
	Cap               int
	MaxBatchSize      int
	ServerTimeout     time.Duration
	ClientTimeout     time.Duration
	TTL               time.Duration
	UpdaterPollRate   time.Duration
	ConfirmerPollRate time.Duration
	WatcherPollRate   time.Duration
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
	if options.BtcShifterAddr == "" {
		panic("btc shifter contract address is not defined")
	}
	if options.ZecShifterAddr == "" {
		panic("zec shifter contract address is not defined")
	}
	if options.BchShifterAddr == "" {
		panic("bch shifter contract address is not defined")
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
	if err := db.CreateTxTable(); err != nil {
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
		MinConfirmations: defaultMinConfirmations(options.Network),
		PollInterval:     options.ConfirmerPollRate,
	}

	// Create the store and insert the bootstrap addresses.
	multiStore := store.New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses"))
	for _, bootstrapAddr := range options.BootstrapAddrs {
		if err := multiStore.Insert(bootstrapAddr); err != nil {
			logger.Fatalf("cannot insert bootstrap address: %v", err)
		}
	}

	btcAddr := common.HexToAddress(options.BtcShifterAddr)
	zecAddr := common.HexToAddress(options.ZecShifterAddr)
	bchAddr := common.HexToAddress(options.BchShifterAddr)
	connPool := blockchain.New(logger, options.Network, btcAddr, zecAddr, bchAddr)
	updater := updater.New(logger, multiStore, options.UpdaterPollRate, options.ClientTimeout)
	dispatcher := dispatcher.New(logger, options.ClientTimeout, multiStore, opts)
	cacher := cacher.New(ctx, options.Network, dispatcher, logger, options.TTL, opts, db)
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
	go lightnode.updater.Run(ctx)
	go lightnode.validator.Run(ctx)
	go lightnode.cacher.Run(ctx)
	go lightnode.dispatcher.Run(ctx)
	go lightnode.confirmer.Run(ctx)
	go lightnode.btcWatcher.Run(ctx)
	go lightnode.zecWatcher.Run(ctx)
	go lightnode.bchWatcher.Run(ctx)

	lightnode.server.Listen(ctx)
}

func defaultMinConfirmations(network darknode.Network) map[abi.Address]uint64 {
	minConfirmations := make(map[abi.Address]uint64)
	switch network {
	case darknode.Devnet, darknode.Testnet, darknode.Chaosnet:
		minConfirmations[abi.IntrinsicBTC0Btc2Eth.Address] = 2
		minConfirmations[abi.IntrinsicZEC0Zec2Eth.Address] = 6
		minConfirmations[abi.IntrinsicBCH0Bch2Eth.Address] = 2
		minConfirmations[abi.IntrinsicBTC0Eth2Btc.Address] = 12
		minConfirmations[abi.IntrinsicZEC0Eth2Zec.Address] = 12
		minConfirmations[abi.IntrinsicBCH0Eth2Bch.Address] = 12
	default:
		for addr := range abi.Intrinsics {
			minConfirmations[addr] = 0
		}
	}
	return minConfirmations
}
