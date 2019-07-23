package lightnode

import (
	"context"
	"time"

	"github.com/renproject/darknode/addr"
	"github.com/renproject/kv"
	"github.com/renproject/lightnode/cacher"
	"github.com/renproject/lightnode/dispatcher"
	"github.com/renproject/lightnode/server"
	"github.com/renproject/lightnode/updater"
	"github.com/renproject/lightnode/validator"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

type Lightnode struct {
	logger logrus.FieldLogger
	server *server.Server

	// Tasks
	validator  phi.Task
	cacher     phi.Task
	dispatcher phi.Task

	updater updater.Updater
}

func New(logger logrus.FieldLogger, timeout time.Duration, cap int, port string, maxBatchSize int, bootstrapAddrs addr.MultiAddresses, pollRate time.Duration) Lightnode {
	// All tasks have the same capacity, and no scaling
	opts := phi.Options{Cap: cap}

	// Server options
	options := server.Options{MaxBatchSize: maxBatchSize}

	// Store to be used by both the updater (which updates the store) and the
	// dispatcher (which reads from the store)
	multiStore := kv.NewMemDB()

	updater := updater.New(logger, bootstrapAddrs, multiStore, pollRate, timeout)
	dispatcher := dispatcher.New(logger, timeout, multiStore, opts)
	cacher := cacher.New(dispatcher, logger, opts)
	validator := validator.New(cacher, logger, opts)
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

func (lightnode Lightnode) Run(ctx context.Context) {
	go lightnode.updater.Run(ctx)
	go lightnode.validator.Run(ctx)
	go lightnode.cacher.Run(ctx)
	go lightnode.dispatcher.Run(ctx)

	lightnode.server.Run()
}
