package server

import (
	"github.com/renproject/darknode/jsonrpc"
)

// A RateLimiter handles the rate limiting logic for JSON-RPC requests. Each
// different type of JSON-RPC method has an independent rate limit.
type RateLimiter struct {
	limiters map[string]*jsonrpc.RateLimiter
}

// New constructs a new `RateLimiter`.
func NewRateLimiter() RateLimiter {
	limiters := map[string]*jsonrpc.RateLimiter{}

	// TODO: Currently this uses the same rate limits as the darknode, but
	// since the lightnode sends requests to many darknodes, these rate limits
	// should be different (but still dependent on the darknode limits).
	for method, rpc := range jsonrpc.RPCs {
		limiters[method] = rpc.RateLimiter
	}

	return RateLimiter{limiters}
}

// Allow updates and checks the rate limiting for a given IP address and
// JSON-RPC method. A return value of false indicates that the rate limit has
// been exceeded. It will also return false if the method is not supported
// (i.e. unsupported methods have rate limits of 0/s).
func (rl *RateLimiter) Allow(method, addr string) bool {
	limiter, ok := rl.limiters[method]
	if !ok {
		return false
	}
	return limiter.IPAddressLimiter(addr).Allow()
}
