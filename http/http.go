package http

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/sirupsen/logrus"
)

// NewRecoveryMiddleware returns a new RecoveryMiddleware which recovers from
// panics when processing requests and logs the error through the given logger.
func NewRecoveryMiddleware(logger logrus.FieldLogger) mux.MiddlewareFunc {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					errMsg := fmt.Sprintf("Recovered from a panic in the lightnode: %v", err)
					logger.Error(errMsg)
					jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, errMsg, nil)
					writeError(w, 0, jsonErr)
				}
			}()
			h.ServeHTTP(w, r)
		})
	}
}

// RateLimiter handles the rate limiting logic for JSON-RPC requests. Each
// different type of JSON-RPC method has an independent rate limit.
type RateLimiter struct {
	limiters map[string]*jsonrpc.RateLimiter
}

// NewRateLimiter constructs a new rate limiter.
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
