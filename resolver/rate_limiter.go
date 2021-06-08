package resolver

import (
	"net"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type RateLimiterConf struct {
	GlobalMethodRate map[string]rate.Limit
	IpMethodRate     map[string]rate.Limit
	Ttl              time.Duration
	MaxClients       int
}

const (
	LimiterDefaultGlobalRate = rate.Limit(1000)
	LimiterDefaultIPRate     = rate.Limit(10)
	LimiterDefaultTTL        = time.Minute
	LimiterDefaultMaxClients = 1000
)

func DefaultRateLimitConf() RateLimiterConf {
	return RateLimiterConf{
		GlobalMethodRate: map[string]rate.Limit{"fallback": LimiterDefaultGlobalRate},
		IpMethodRate:     map[string]rate.Limit{"fallback": LimiterDefaultIPRate},
		Ttl:              time.Minute,
		MaxClients:       1000,
	}
}

func NewRateLimitConf(global rate.Limit, ip rate.Limit, ttl time.Duration, maxClients int) RateLimiterConf {
	return RateLimiterConf{
		GlobalMethodRate: map[string]rate.Limit{"fallback": global},
		IpMethodRate:     map[string]rate.Limit{"fallback": ip},
		Ttl:              ttl,
		MaxClients:       maxClients,
	}
}

type LightnodeRateLimiter struct {
	mu   sync.RWMutex
	conf RateLimiterConf

	// Per method global limit
	// will use "fallback" if method is not configured
	globalLimit map[string]*rate.Limiter

	// Per method, per ip limit
	// will use "fallback" if method is not configured
	ipLimiters map[string]map[string]*rate.Limiter
	ipLastSeen map[string]time.Time
	maxClients int
	ttl        time.Duration
}

func NewRateLimiter(conf RateLimiterConf) LightnodeRateLimiter {
	if conf.IpMethodRate == nil {
		conf.IpMethodRate = make(map[string]rate.Limit)
	}

	globalLimits := make(map[string]*rate.Limiter)
	for method, r := range conf.GlobalMethodRate {
		globalLimits[method] = rate.NewLimiter(r, int(r))
	}

	return LightnodeRateLimiter{
		conf:        conf,
		globalLimit: globalLimits,
		ipLimiters:  make(map[string]map[string]*rate.Limiter),
		ipLastSeen:  make(map[string]time.Time),
		maxClients:  conf.MaxClients,
		ttl:         conf.Ttl,
	}
}

// Checks if the ip has an available limit, and increment if so
// Returns true if below limit, false otherwise
func (limiter *LightnodeRateLimiter) Allow(method string, ip net.IP) bool {
	limiter.mu.Lock()

	// We prune when we are tracking too many ips
	if len(limiter.ipLimiters) > limiter.maxClients {
		limiter.mu.Unlock()
		limiter.Prune()
		return false
	}
	defer limiter.mu.Unlock()

	globalMethod := method
	// if we have a per-method limit set
	_, ok := limiter.conf.GlobalMethodRate[method]
	if !ok {
		globalMethod = "fallback"
	}
	if !limiter.globalLimit[globalMethod].Allow() {
		return false
	}

	// if we have a per-method limit set
	methodLimit, ok := limiter.conf.IpMethodRate[method]
	if !ok {
		method = "fallback"
		methodLimit = limiter.conf.IpMethodRate[method]
	}
	limit, ok := limiter.ipLimiters[method][ip.String()]
	limiter.ipLastSeen[ip.String()] = time.Now()

	if !ok {
		if limiter.ipLimiters[method] == nil {
			limiter.ipLimiters[method] = make(map[string]*rate.Limiter)
		}
		il := rate.NewLimiter(methodLimit, int(methodLimit))
		limiter.ipLimiters[method][ip.String()] = il
		return il.Allow()
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
