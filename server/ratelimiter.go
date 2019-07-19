package server

type RateLimiter struct{}

func (rl *RateLimiter) Allow(addr string) bool {
	return true
}
