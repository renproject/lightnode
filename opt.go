package lightnode

import (
	"time"

	"github.com/renproject/aw/wire"
	"github.com/renproject/darknode/binding"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/id"
	"github.com/renproject/lightnode/confirmer"
	"github.com/renproject/lightnode/resolver"
	"github.com/renproject/multichain"
	"golang.org/x/time/rate"
)

// Enumerate default options.
var (
	DefaultPort                      = "5000"
	DefaultCap                       = 128
	DefaultMaxBatchSize              = 10
	DefaultMaxPageSize               = 10
	DefaultServerTimeout             = 15 * time.Second
	DefaultClientTimeout             = 15 * time.Second
	DefaultTTL                       = 3 * time.Second
	DefaultUpdaterPollRate           = 5 * time.Minute
	DefaultConfirmerPollRate         = confirmer.DefaultPollInterval
	DefaultWatcherPollRate           = 15 * time.Second
	DefaultWatcherMaxBlockAdvance    = uint64(1000)
	DefaultWatcherConfidenceInterval = uint64(6)
	DefaultTransactionExpiry         = confirmer.DefaultExpiry
	DefaultBootstrapAddrs            = []wire.Address{}
	DefaultLimiterIPRates            = map[string]rate.Limit{"fallback": resolver.LimiterDefaultIPRate}
	DefaultLimiterGlobalRates        = map[string]rate.Limit{"fallback": resolver.LimiterDefaultGlobalRate}
	DefaultLimiterTTL                = resolver.LimiterDefaultTTL
	DefaultLimiterMaxClients         = resolver.LimiterDefaultMaxClients
)

// Options to configure the precise behaviour of the Lightnode.
type Options struct {
	Network                   multichain.Network
	DistPubKey                *id.PubKey
	Port                      string
	Cap                       int
	MaxBatchSize              int
	MaxPageSize               int
	ServerTimeout             time.Duration
	ClientTimeout             time.Duration
	TTL                       time.Duration
	UpdaterPollRate           time.Duration
	ConfirmerPollRate         time.Duration
	WatcherPollRate           time.Duration
	WatcherMaxBlockAdvance    uint64
	WatcherConfidenceInterval uint64
	TransactionExpiry         time.Duration
	BootstrapAddrs            []wire.Address
	Chains                    map[multichain.Chain]binding.ChainOptions
	Whitelist                 []tx.Selector
	LimiterGlobalRates        map[string]rate.Limit
	LimiterIPRates            map[string]rate.Limit
	LimiterTTL                time.Duration
	LimiterMaxClients         int
}

// DefaultOptions returns new options with default configurations that should
// work for the majority of use cases.
func DefaultOptions() Options {
	return Options{
		Port:                      DefaultPort,
		Cap:                       DefaultCap,
		BootstrapAddrs:            DefaultBootstrapAddrs,
		MaxBatchSize:              DefaultMaxBatchSize,
		MaxPageSize:               DefaultMaxPageSize,
		ServerTimeout:             DefaultServerTimeout,
		ClientTimeout:             DefaultClientTimeout,
		TTL:                       DefaultTTL,
		UpdaterPollRate:           DefaultUpdaterPollRate,
		ConfirmerPollRate:         DefaultConfirmerPollRate,
		WatcherPollRate:           DefaultWatcherPollRate,
		WatcherMaxBlockAdvance:    DefaultWatcherMaxBlockAdvance,
		WatcherConfidenceInterval: DefaultWatcherConfidenceInterval,
		TransactionExpiry:         DefaultTransactionExpiry,
		LimiterTTL:                DefaultLimiterTTL,
		LimiterGlobalRates:        DefaultLimiterGlobalRates,
		LimiterIPRates:            DefaultLimiterIPRates,
		LimiterMaxClients:         DefaultLimiterMaxClients,
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

// WithWatcherPollRate updates the watcher poll rate.
func (opts Options) WithWatcherMaxBlockAdvance(watcherMaxBlockAdvance uint64) Options {
	opts.WatcherMaxBlockAdvance = watcherMaxBlockAdvance
	return opts
}

// WithWatcherPollRate updates the watcher poll rate.
func (opts Options) WithWatcherConfidenceInterval(watcherConfidenceInterval uint64) Options {
	opts.WatcherConfidenceInterval = watcherConfidenceInterval
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
func (opts Options) WithChains(chains map[multichain.Chain]binding.ChainOptions) Options {
	opts.Chains = chains
	return opts
}

// WithWhitelist is used to whitelist certain selectors inside the Darknode.
func (opts Options) WithWhitelist(whitelist []tx.Selector) Options {
	opts.Whitelist = whitelist
	return opts
}

// WithLimiterGlobalRate is used to set global rate limits for specific methods
func (opts Options) WithLimiterGlobalRates(rates map[string]rate.Limit) Options {
	opts.LimiterGlobalRates = rates
	return opts
}

// WithLimiterIpRate is used to set per-ip rate limits for specific methods
func (opts Options) WithLimiterIPRates(rates map[string]rate.Limit) Options {
	opts.LimiterIPRates = rates
	return opts
}

// WithLimiterTTL used to whitelist certain selectors inside the Darknode.
func (opts Options) WithLimiterTTL(ttl time.Duration) Options {
	opts.LimiterTTL = ttl
	return opts
}

// WithLimiterMaxClients used to whitelist certain selectors inside the Darknode.
func (opts Options) WithLimiterMaxClients(maxClients int) Options {
	opts.LimiterMaxClients = maxClients
	return opts
}
