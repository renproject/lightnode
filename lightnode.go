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

// Lightnode is the top level container that encapsulates the functionality of
// the lightnode.
type Lightnode struct {
	logger logrus.FieldLogger
	server *server.Server

	// Tasks
	validator  phi.Task
	cacher     phi.Task
	dispatcher phi.Task

	updater updater.Updater
}

// New constructs a new `Lightnode`.
func New(ctx context.Context, network darknode.Network, db db.DB, logger logrus.FieldLogger, cap, cacheCap, maxBatchSize int, timeout, minTTL, maxTTL, pollRate time.Duration, port string, bootstrapAddrs addr.MultiAddresses) Lightnode {
	// All tasks have the same capacity, and no scaling
	opts := phi.Options{Cap: cap}

	// Server options
	options := server.Options{MaxBatchSize: maxBatchSize}

	// Create the store and insert the bootstrap addresses.
	firstBootstrapAddr := bootstrapAddrs[0]
	multiStore := store.New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses"), firstBootstrapAddr)
	for _, bootstrapAddr := range bootstrapAddrs {
		if err := multiStore.Insert(bootstrapAddr); err != nil {
			logger.Fatalf("cannot insert bootstrap address: %v", err)
		}
	}

	updater := updater.New(logger, bootstrapAddrs, multiStore, pollRate, timeout)
	dispatcher := dispatcher.New(logger, timeout, multiStore, opts)
	cacher := cacher.New(ctx, network, db, dispatcher, logger, cacheCap, minTTL, maxTTL, opts)
	validator := validator.New(logger, cacher, multiStore, opts)
	server := server.New(logger, port, options, validator)

	return Lightnode{
		logger,
		server,
		validator,
		cacher,
		dispatcher,
		updater,
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
