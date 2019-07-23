package ratelimiter

import (
	"github.com/renproject/darknode/jsonrpc"
)

type RateLimiter struct {
	limiters map[string]*jsonrpc.RateLimiter
}

func New() RateLimiter {
	limiters := map[string]*jsonrpc.RateLimiter{}

	// TODO: Currently this uses the same rate limits as the darknode, but
	// since the lightnode sends requests to many darknodes, these rate limits
	// should be different (but still dependent on the darknode limits).
	for method, rpc := range jsonrpc.RPCs {
		limiters[method] = rpc.RateLimiter
	}

	return RateLimiter{limiters}
}

func (rl *RateLimiter) Allow(method, addr string) bool {
	limiter, ok := rl.limiters[method]
	if !ok {
		// NOTE: This return value hides the fact that the method is not
		// supported. The fact that the method is not supported should be
		// checked elsewhere and suitable indication that this is the case
		// should be provided.
		return false
	}
	return limiter.IPAddressLimiter(addr).Allow()
}
