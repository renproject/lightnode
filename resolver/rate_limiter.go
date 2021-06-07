package resolver

import (
	"net"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type RateLimiterConf struct {
	GlobalRate rate.Limit
	IpRate     rate.Limit
	Ttl        time.Duration
	MaxClients int
}

func DefaultRateLimit() RateLimiterConf {
	return RateLimiterConf{
		GlobalRate: 10000,
		IpRate:     10,
		Ttl:        time.Minute,
		MaxClients: 1000,
	}
}

type LightnodeRateLimiter struct {
	mu   sync.RWMutex
	conf RateLimiterConf

	globalLimit *rate.Limiter
	ipLimiters  map[string]*rate.Limiter
	ipLastSeen  map[string]time.Time
	maxClients  int
	ttl         time.Duration
}

func NewRateLimiter(conf RateLimiterConf) LightnodeRateLimiter {
	return LightnodeRateLimiter{
		conf:        conf,
		globalLimit: rate.NewLimiter(conf.GlobalRate, int(conf.GlobalRate)),
		ipLimiters:  make(map[string]*rate.Limiter),
		ipLastSeen:  make(map[string]time.Time),
		maxClients:  conf.MaxClients,
		ttl:         conf.Ttl,
	}
}

// Checks if the ip has an available limit, and increment if so
// Returns true if below limit, false otherwise
func (limiter *LightnodeRateLimiter) Allow(ip net.IP) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	// We prune when we are tracking too many ips
	// FIXME: do we want to return false here?
	if len(limiter.ipLimiters) > limiter.maxClients {
		limiter.Prune()
	}

	if !limiter.globalLimit.Allow() {
		return false
	}

	limit, ok := limiter.ipLimiters[ip.String()]
	limiter.ipLastSeen[ip.String()] = time.Now()

	if !ok {
		limiter.ipLimiters[ip.String()] = rate.NewLimiter(limiter.conf.IpRate, int(limiter.conf.IpRate))
		return true
	}

	return limit.Allow()
}

// Prune IP-addresses that have not been seen for a while.
func (limiter *LightnodeRateLimiter) Prune() int {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	pruned := 0
	for ip, ipLastSeen := range limiter.ipLastSeen {
		if time.Now().Sub(ipLastSeen) > limiter.ttl {
			delete(limiter.ipLimiters, ip)
			delete(limiter.ipLastSeen, ip)
			pruned += 1
		}
	}
	return pruned
}
