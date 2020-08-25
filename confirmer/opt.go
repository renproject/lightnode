package confirmer

import (
	"time"

	"github.com/renproject/lightnode"
	"github.com/renproject/multichain"
	"github.com/sirupsen/logrus"
)

// Enumerate default options.
var (
	DefaultMinConfirmations = map[multichain.Chain]uint64{}
	DefaultPollInterval     = lightnode.DefaultConfirmerPollRate
	DefaultExpiry           = lightnode.DefaultTransactionExpiry
)

// Options to configure the precise behaviour of the confirmer.
type Options struct {
	Logger           logrus.FieldLogger
	MinConfirmations map[multichain.Chain]uint64
	PollInterval     time.Duration
	Expiry           time.Duration
}

// DefaultOptions returns new options with default configurations that should
// work for the majority of use cases.
func DefaultOptions() Options {
	return Options{
		Logger:           logrus.New(),
		MinConfirmations: DefaultMinConfirmations,
		PollInterval:     DefaultPollInterval,
		Expiry:           DefaultExpiry,
	}
}

// WithLogger returns new options with the given logger.
func (opts Options) WithLogger(logger logrus.FieldLogger) Options {
	opts.Logger = logger
	return opts
}

// WithMinConfirmations returns new options with the given minimum
// confirmations.
func (opts Options) WithMinConfirmations(minConfirmations map[multichain.Chain]uint64) Options {
	opts.MinConfirmations = minConfirmations
	return opts
}

// WithPollInterval returns new options with the given poll interval.
func (opts Options) WithPollInterval(pollInterval time.Duration) Options {
	opts.PollInterval = pollInterval
	return opts
}

// WithExpiry returns new options with the given transaction expiry.
func (opts Options) WithExpiry(expiry time.Duration) Options {
	opts.Expiry = expiry
	return opts
}
