package lightnode

import (
	"time"

	"github.com/renproject/aw/wire"
	"github.com/renproject/id"
)

// Enumerate default options.
var (
	DefaultCap               = 128
	DefaultMaxBatchSize      = 10
	DefaultMaxPageSize       = 10
	DefaultServerTimeout     = 15 * time.Second
	DefaultClientTimeout     = time.Minute
	DefaultTTL               = 3 * time.Second
	DefaultUpdaterPollRate   = 5 * time.Minute
	DefaultConfirmerPollRate = 30 * time.Second
	DefaultWatcherPollRate   = 15 * time.Second
	DefaultSubmitterPollRate = 15 * time.Second
	DefaultTransactionExpiry = 14 * 24 * time.Hour
)

// Options to configure the precise behaviour of the Lightnode.
type Options struct {
	Network           string
	Key               *id.PrivKey
	DistPubKey        *id.PubKey
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
	TransactionExpiry time.Duration
	BootstrapAddrs    []wire.Address
}

// DefaultOptions returns new options with default configurations that should
// work for the majority of use cases.
func DefaultOptions() Options {
	return Options{
		Cap:               DefaultCap,
		MaxBatchSize:      DefaultMaxBatchSize,
		MaxPageSize:       DefaultMaxPageSize,
		ServerTimeout:     DefaultServerTimeout,
		ClientTimeout:     DefaultClientTimeout,
		TTL:               DefaultTTL,
		UpdaterPollRate:   DefaultUpdaterPollRate,
		ConfirmerPollRate: DefaultConfirmerPollRate,
		WatcherPollRate:   DefaultWatcherPollRate,
		SubmitterPollRate: DefaultSubmitterPollRate,
		TransactionExpiry: DefaultTransactionExpiry,
	}
}

// WithNetwork returns new options with the given network.
func (opts Options) WithNetwork(network string) Options {
	opts.Network = network
	return opts
}

// WithKey returns new options with the given key.
func (opts Options) WithKey(key *id.PrivKey) Options {
	opts.Key = key
	return opts
}

// WithDistPubKey returns new options with the given distributed public key.
func (opts Options) WithDistPubKey(distPubKey *id.PubKey) Options {
	opts.DistPubKey = distPubKey
	return opts
}

// WithPort returns new options with the given port.
func (opts Options) WithPort(port string) Options {
	opts.Port = port
	return opts
}

// WithProtocolAddr returns new options with the given protocol address.
func (opts Options) WithProtocolAddr(protocolAddr string) Options {
	opts.ProtocolAddr = protocolAddr
	return opts
}

// WithCap returns new options with the given capacity.
func (opts Options) WithCap(cap int) Options {
	opts.Cap = cap
	return opts
}

// WithMaxBatchSize returns new options with the given maximum batch sizee.
func (opts Options) WithMaxBatchSize(maxBatchSize int) Options {
	opts.MaxBatchSize = maxBatchSize
	return opts
}

// WithMaxPageSize returns new options with the given maximum page size.
func (opts Options) WithMaxPageSize(maxPageSize int) Options {
	opts.MaxPageSize = maxPageSize
	return opts
}

// WithServerTimeout returns new options with the given server timeout.
func (opts Options) WithServerTimeout(serverTimeout time.Duration) Options {
	opts.ServerTimeout = serverTimeout
	return opts
}

// WithClientTimeout returns new options with the given client timeout.
func (opts Options) WithClientTimeout(clientTimeout time.Duration) Options {
	opts.ClientTimeout = clientTimeout
	return opts
}

// WithTTL returns new options with the given time-to-live duration.
func (opts Options) WithTTL(ttl time.Duration) Options {
	opts.TTL = ttl
	return opts
}

// WithUpdaterPollRate returns new options with the given updater poll rate.
func (opts Options) WithUpdaterPollRate(updaterPollRate time.Duration) Options {
	opts.UpdaterPollRate = updaterPollRate
	return opts
}

// WithConfirmerPollRate returns new options with the given confirmer poll rate.
func (opts Options) WithConfirmerPollRate(confirmerPollRate time.Duration) Options {
	opts.ConfirmerPollRate = confirmerPollRate
	return opts
}

// WithWatcherPollRate returns new options with the given watcher poll rate.
func (opts Options) WithWatcherPollRate(watcherPollRate time.Duration) Options {
	opts.WatcherPollRate = watcherPollRate
	return opts
}

// WithSubmitterPollRate returns new options with the given submitter poll rate.
func (opts Options) WithSubmitterPollRate(submitterPollRate time.Duration) Options {
	opts.SubmitterPollRate = submitterPollRate
	return opts
}

// WithTransactionExpiry returns new options with the given transaction expiry.
func (opts Options) WithTransactionExpiry(transactionExpiry time.Duration) Options {
	opts.TransactionExpiry = transactionExpiry
	return opts
}

// WithBootstrapAddrs returns new options with the given bootstrap addresses.
func (opts Options) WithBootstrapAddrs(bootstrapAddrs []wire.Address) Options {
	opts.BootstrapAddrs = bootstrapAddrs
	return opts
}
