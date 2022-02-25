package confirmer

import (
	"time"

	"github.com/sirupsen/logrus"
)

// Enumerate default options.
var (
	DefaultConfirmationPeriod = 3 * 24 * time.Hour // we want to check txs within the last 3 days
	DefaultPollInterval       = 30 * time.Second
	DefaultExpiry             = 6 * 30 * 24 * time.Hour // 6 month expiry
)

// Options to configure the precise behaviour of the confirmer.
type Options struct {
	Logger             logrus.FieldLogger
	ConfirmationPeriod time.Duration
	PollInterval       time.Duration
	Expiry             time.Duration
}

// DefaultOptions returns new options with default configurations that should
// work for the majority of use cases.
func DefaultOptions() Options {
	return Options{
		Logger:             logrus.New(),
		ConfirmationPeriod: DefaultConfirmationPeriod,
		PollInterval:       DefaultPollInterval,
		Expiry:             DefaultExpiry,
	}
}

// WithLogger returns new options with the given logger.
func (opts Options) WithLogger(logger logrus.FieldLogger) Options {
	opts.Logger = logger
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

// WithConfirmationPeriod returns new options with the given confirmation period.
func (opts Options) WithConfirmationPeriod(cp time.Duration) Options {
	opts.ConfirmationPeriod = cp
	return opts
}
