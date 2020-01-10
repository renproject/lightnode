package lightnode

import (
	"context"
	"time"

	"github.com/renproject/darknode"
	"github.com/renproject/darknode/addr"
	"github.com/renproject/kv"
	"github.com/renproject/lightnode/cacher"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/lightnode/dispatcher"
	"github.com/renproject/lightnode/server"
	"github.com/renproject/lightnode/store"
	"github.com/renproject/lightnode/updater"
	"github.com/renproject/lightnode/validator"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

// Options for setting up a Lightnode, usually parsed from environment variables.
type Options struct {
	Network        darknode.Network
	Port           string
	Cap            int
	CacheCap       int
	MaxBatchSize   int
	Timeout        time.Duration
	TTL            time.Duration
	PollRate       time.Duration
	BootstrapAddrs addr.MultiAddresses
}

// SetZeroToDefault does basic verification of options and set fields with zero value to default.
func (options Options) SetZeroToDefault() {
	switch options.Network {
	case darknode.Mainnet, darknode.Chaosnet, darknode.Testnet, darknode.Devnet, darknode.Localnet:
	default:
		panic("unknown networks")
	}
	if options.Port == "" {
		panic("port is not set in the options")
	}
	if len(options.BootstrapAddrs) == 0 {
		panic("bootstrap addresses are not set in the options")
	}
	if options.Cap == 0 {
		options.Cap = 128
	}
	if options.CacheCap == 0 {
		options.CacheCap = 128
	}
	if options.MaxBatchSize == 0 {
		options.MaxBatchSize = 10
	}
	if options.Timeout == 0 {
		options.Timeout = time.Minute
	}
	if options.TTL == 0 {
		options.TTL = 3 * time.Second
	}
	if options.PollRate == 0 {
		options.PollRate = 5 * time.Minute
	}
}

// Lightnode is the top level container that encapsulates the functionality of
// the lightnode.
type Lightnode struct {
	options Options
	logger  logrus.FieldLogger
	server  *server.Server
	updater updater.Updater

	// Tasks
	validator  phi.Task
	cacher     phi.Task
	dispatcher phi.Task
}

// New constructs a new `Lightnode`.
func New(ctx context.Context, options Options, logger logrus.FieldLogger, db db.DB) Lightnode {
	// All tasks have the same capacity, and no scaling
	opts := phi.Options{Cap: options.Cap}

	// Server options
	serverOptions := server.Options{MaxBatchSize: options.MaxBatchSize}

	// Create the store and insert the bootstrap addresses.
	multiStore := store.New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses"), options.BootstrapAddrs[0])
	for _, bootstrapAddr := range options.BootstrapAddrs {
		if err := multiStore.Insert(bootstrapAddr); err != nil {
			logger.Fatalf("cannot insert bootstrap address: %v", err)
		}
	}

	updater := updater.New(logger, options.BootstrapAddrs, multiStore, options.PollRate, options.Timeout)
	dispatcher := dispatcher.New(logger, options.Timeout, multiStore, opts)
	cacher := cacher.New(ctx, options.Network, db, dispatcher, logger, options.CacheCap, options.TTL, opts)
	validator := validator.New(logger, cacher, multiStore, opts)
	server := server.New(logger, options.Port, serverOptions, validator)

	return Lightnode{
		options:    options,
		logger:     logger,
		server:     server,
		updater:    updater,

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

	lightnode.server.Run()
}
