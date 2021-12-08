package watcher

import (
	"time"

	"github.com/renproject/multichain"
	"github.com/sirupsen/logrus"
)

var (
	DefaultPollInterval              = 30 * time.Second
	DefaultNetwork                   = multichain.NetworkMainnet
	DefaultChain                     = multichain.Ethereum
	DefaultAssets                    = []multichain.Asset{}
	DefaultConfidenceInterval uint64 = 30
	DefaultMaxBlockAdvance    uint64 = 100
)

// Options to configure the precise behaviour of the watcher
type Options struct {
	Logger             logrus.FieldLogger
	Network            multichain.Network
	Chain              multichain.Chain
	Assets             []multichain.Asset
	PollInterval       time.Duration
	ConfidenceInterval uint64
	MaxBlockAdvance    uint64
}

// DefaultOptions returns new options with default configurations that should
// work for the majority of use cases.
func DefaultOptions() Options {
	logger := logrus.New()
	return Options{
		Logger:             logger,
		Network:            DefaultNetwork,
		Chain:              DefaultChain,
		Assets:             DefaultAssets,
		PollInterval:       DefaultPollInterval,
		ConfidenceInterval: DefaultConfidenceInterval,
		MaxBlockAdvance:    DefaultMaxBlockAdvance,
	}
}

func (opts Options) WithLogger(logger logrus.FieldLogger) Options {
	opts.Logger = logger
	return opts
}

func (opts Options) WithNetwork(network multichain.Network) Options {
	opts.Network = network
	return opts
}

func (opts Options) WithChain(chain multichain.Chain) Options {
	opts.Chain = chain
	return opts
}

func (opts Options) WithAssets(assets []multichain.Asset) Options {
	opts.Assets = assets
	return opts
}

func (opts Options) WithPollInterval(interval time.Duration) Options {
	opts.PollInterval = interval
	return opts
}

func (opts Options) WithConfidenceInterval(interval uint64) Options {
	opts.ConfidenceInterval = interval
	return opts
}

func (opts Options) WithMaxBlockAdvance(maxBlockAdvance uint64) Options {
	opts.MaxBlockAdvance = maxBlockAdvance
	return opts
}
