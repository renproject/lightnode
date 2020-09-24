package lightnode

import (
	"time"

	"github.com/renproject/aw/wire"
	"github.com/renproject/darknode/txengine/txenginebindings"
	"github.com/renproject/id"
	"github.com/renproject/lightnode/confirmer"
	"github.com/renproject/multichain"
)

// Enumerate default options.
var (
	DefaultPort              = "5000"
	DefaultCap               = 128
	DefaultMaxBatchSize      = 10
	DefaultMaxPageSize       = 10
	DefaultServerTimeout     = 15 * time.Second
	DefaultClientTimeout     = 15 * time.Second
	DefaultTTL               = 3 * time.Second
	DefaultUpdaterPollRate   = 5 * time.Minute
	DefaultConfirmerPollRate = confirmer.DefaultPollInterval
	DefaultWatcherPollRate   = 15 * time.Second
	DefaultTransactionExpiry = confirmer.DefaultExpiry
	DefaultBootstrapAddrs    = []wire.Address{}
)

// Options to configure the precise behaviour of the Lightnode.
type Options struct {
	Network           multichain.Network
	DistPubKey        *id.PubKey
	Port              string
	Cap               int
	MaxBatchSize      int
	MaxPageSize       int
	ServerTimeout     time.Duration
	ClientTimeout     time.Duration
	TTL               time.Duration
	UpdaterPollRate   time.Duration
	ConfirmerPollRate time.Duration
	WatcherPollRate   time.Duration
	TransactionExpiry time.Duration
	BootstrapAddrs    []wire.Address
	Chains            map[multichain.Chain]txenginebindings.ChainOptions
}

// DefaultOptions returns new options with default configurations that should
// work for the majority of use cases.
func DefaultOptions() Options {
	return Options{
		Port:              DefaultPort,
		Cap:               DefaultCap,
		MaxBatchSize:      DefaultMaxBatchSize,
		MaxPageSize:       DefaultMaxPageSize,
		ServerTimeout:     DefaultServerTimeout,
		ClientTimeout:     DefaultClientTimeout,
		TTL:               DefaultTTL,
		UpdaterPollRate:   DefaultUpdaterPollRate,
		ConfirmerPollRate: DefaultConfirmerPollRate,
		WatcherPollRate:   DefaultWatcherPollRate,
		TransactionExpiry: DefaultTransactionExpiry,
		BootstrapAddrs:    DefaultBootstrapAddrs,
	}
}

// WithNetwork updates the network.
func (opts Options) WithNetwork(network multichain.Network) Options {
	opts.Network = network
	return opts
}

// WithDistPubKey updates the distributed public key.
func (opts Options) WithDistPubKey(distPubKey *id.PubKey) Options {
	opts.DistPubKey = distPubKey
	return opts
}

// WithPort updates the port.
func (opts Options) WithPort(port string) Options {
	opts.Port = port
	return opts
}

// WithCap updates the capacity.
func (opts Options) WithCap(cap int) Options {
	opts.Cap = cap
	return opts
}

// WithMaxBatchSize updates the maximum batch size when submitting
// requests.
func (opts Options) WithMaxBatchSize(maxBatchSize int) Options {
	opts.MaxBatchSize = maxBatchSize
	return opts
}

// WithMaxPageSize updates the maximum page size when querying
// transactions.
func (opts Options) WithMaxPageSize(maxPageSize int) Options {
	opts.MaxPageSize = maxPageSize
	return opts
}

// WithServerTimeout updates the server timeout.
func (opts Options) WithServerTimeout(serverTimeout time.Duration) Options {
	opts.ServerTimeout = serverTimeout
	return opts
}

// WithClientTimeout updates the client timeout.
func (opts Options) WithClientTimeout(clientTimeout time.Duration) Options {
	opts.ClientTimeout = clientTimeout
	return opts
}

// WithTTL updates the time-to-live duration.
func (opts Options) WithTTL(ttl time.Duration) Options {
	opts.TTL = ttl
	return opts
}

// WithUpdaterPollRate updates the updater poll rate.
func (opts Options) WithUpdaterPollRate(updaterPollRate time.Duration) Options {
	opts.UpdaterPollRate = updaterPollRate
	return opts
}

// WithConfirmerPollRate updates the confirmer poll rate.
func (opts Options) WithConfirmerPollRate(confirmerPollRate time.Duration) Options {
	opts.ConfirmerPollRate = confirmerPollRate
	return opts
}

// WithWatcherPollRate updates the watcher poll rate.
func (opts Options) WithWatcherPollRate(watcherPollRate time.Duration) Options {
	opts.WatcherPollRate = watcherPollRate
	return opts
}

// WithTransactionExpiry updates the transaction expiry.
func (opts Options) WithTransactionExpiry(transactionExpiry time.Duration) Options {
	opts.TransactionExpiry = transactionExpiry
	return opts
}

// WithBootstrapAddrs makes an initial list of nodes known to the node. These
// nodes will be used to bootstrap into the P2P network.
func (opts Options) WithBootstrapAddrs(bootstrapAddrs []wire.Address) Options {
	opts.BootstrapAddrs = bootstrapAddrs
	return opts
}

// WithChains is used to specify the chain options for a the supported chains.
func (opts Options) WithChains(chains map[multichain.Chain]txenginebindings.ChainOptions) Options {
	opts.Chains = chains
	return opts
}
