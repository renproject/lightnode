package confirmer

import (
	"time"

	"github.com/sirupsen/logrus"
)

// Enumerate default options.
var (
	DefaultPollInterval = 30 * time.Second
	DefaultExpiry       = 14 * 24 * time.Hour
)

// Options to configure the precise behaviour of the confirmer.
type Options struct {
	Logger       logrus.FieldLogger
	PollInterval time.Duration
	Expiry       time.Duration
}

// DefaultOptions returns new options with default configurations that should
// work for the majority of use cases.
func DefaultOptions() Options {
	return Options{
		Logger:       logrus.New(),
		PollInterval: DefaultPollInterval,
		Expiry:       DefaultExpiry,
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
