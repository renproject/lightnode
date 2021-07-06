package resolver_test

import (
	"net"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/resolver"

	"golang.org/x/time/rate"
)

var _ = Describe("Rate Limiter", func() {
	It("Should allow when 1 rate is set", func() {
		conf := NewRateLimitConf(
			rate.Limit(1),
			rate.Limit(1),
			time.Second,
			1,
		)
		limiter := NewRateLimiter(conf)
		allowed := limiter.Allow("unknown", net.IPv4(0, 0, 0, 0))
		Expect(allowed).To(BeTrue())
	})

	It("Should rate limit when rate is over global limit", func() {
		conf := NewRateLimitConf(
			rate.Limit(1),
			rate.Limit(100),
			time.Second,
			1,
		)
		limiter := NewRateLimiter(conf)
		allowed := limiter.Allow("unknown", net.IPv4(0, 0, 0, 0))
		Expect(allowed).To(BeTrue())
		allowed = limiter.Allow("unknown", net.IPv4(0, 0, 0, 0))
		Expect(allowed).To(BeFalse())
	})

	It("Should rate limit when rate is over ip limit", func() {
		conf := NewRateLimitConf(
			rate.Limit(100),
			rate.Limit(1),
			time.Second,
			1,
		)
		limiter := NewRateLimiter(conf)
		allowed := limiter.Allow("unknown", net.IPv4(0, 0, 0, 0))
		Expect(allowed).To(BeTrue())
		allowed = limiter.Allow("unknown", net.IPv4(0, 0, 0, 0))
		allowed = limiter.Allow("unknown", net.IPv4(0, 0, 0, 0))
		allowed = limiter.Allow("unknown", net.IPv4(0, 0, 0, 0))
		Expect(allowed).To(BeFalse())
	})

	It("Should rate limit when rate is over high ip limit", func() {
		conf := NewRateLimitConf(
			rate.Limit(1000),
			rate.Limit(100),
			time.Second,
			1,
		)
		limiter := NewRateLimiter(conf)
		allowed := limiter.Allow("unknown", net.IPv4(0, 0, 0, 0))
		Expect(allowed).To(BeTrue())
		for i := 0; i < 101; i++ {
			allowed = limiter.Allow("unknown", net.IPv4(0, 0, 0, 0))
		}
		Expect(allowed).To(BeFalse())
	})

	It("Should allow multiple ips", func() {
		conf := NewRateLimitConf(
			rate.Limit(5000),
			rate.Limit(25),
			time.Second,
			4,
		)
		ips := []net.IP{
			net.IPv4(0, 0, 0, 0),
			net.IPv4(0, 0, 0, 1),
			net.IPv4(0, 0, 0, 2),
			net.IPv4(0, 0, 0, 3),
		}
		limiter := NewRateLimiter(conf)
		for _, ip := range ips {
			iip := ip

			allowed := 0
			for i := 0; i < 30; i++ {
				if limiter.Allow("unknown", iip) {
					allowed += 1
				}
			}
			Expect(allowed).To(Equal(25))
		}
	})

	It("Should limit multiple ips with global rates of specific methods", func() {
		globalMethodRate := make(map[string]rate.Limit)
		globalMethodRate["known"] = 15
		globalMethodRate["fallback"] = 5000

		ipMethodRate := make(map[string]rate.Limit)
		ipMethodRate["known"] = 25
		ipMethodRate["fallback"] = 10

		conf := RateLimiterConf{
			Ttl:              time.Second,
			GlobalMethodRate: globalMethodRate,
			IpMethodRate:     ipMethodRate,
			MaxClients:       4,
		}
		ips := []net.IP{
			net.IPv4(0, 0, 0, 0),
			net.IPv4(0, 0, 0, 1),
			net.IPv4(0, 0, 0, 2),
			net.IPv4(0, 0, 0, 3),
		}

		type testResult struct {
			known   int
			unknown int
		}

		testChans := make([]chan testResult, 0)
		limiter := NewRateLimiter(conf)

		for _, ip := range ips {
			ichan := make(chan testResult)
			testChans = append(testChans, ichan)

			iip := ip

			mu := sync.RWMutex{}
			unknownAllowed := 0
			knownAllowed := 0
			requests := 0
			// We run these tests in parrallel in order to allow all clients
			// to be distributed between the global rate limit, instead of
			// running down the limit sequentially
			for i := 0; i < 30; i++ {
				go func() {
					mu.Lock()
					defer mu.Unlock()
					// The rate limit seems to be more granular than 1s
					// ie. if we hit it many times in a millisecond, it will deny
					// even if under the absolute rate.
					// We are however outside of the burst window; so we end up being
					// more aggressively rate limited.
					// We sleep so that we get closer to the desired test rate
					// time.Sleep(time.Second / (60 * 4))
					if limiter.Allow("unknown", iip) {
						unknownAllowed += 1
					}

					// time.Sleep(time.Second / (60 * 4))
					if limiter.Allow("known", iip) {
						knownAllowed += 1
					}

					ichan <- testResult{
						known:   knownAllowed,
						unknown: unknownAllowed,
					}
					requests += 1
					if requests == 30 {
						close(ichan)
					}
				}()
			}
		}

		globalKnown := 0
		for _, i := range testChans {
			ipUnknown := 0
			ipKnown := 0
			for res := range i {
				// limited by
				// global fallback (5000 in total)
				// ip fallback (10 per client per second)
				ipUnknown = res.unknown

				// limited by
				// global method limit (15 p/s across all clients)
				// ip method limit (25 per client per second)
				ipKnown = res.known
			}
			globalKnown += ipKnown
			// Diffcult to get exact rate when testing async
			Expect(ipUnknown).To(Equal((10)))
		}
		Expect(globalKnown).To(Equal(15))
	})

	It("Should allow multiple ips with rates of specific methods", func() {
		ipMethodRate := make(map[string]rate.Limit)
		ipMethodRate["known"] = 10
		ipMethodRate["fallback"] = 25

		globalMethodRate := make(map[string]rate.Limit)
		globalMethodRate["fallback"] = 5000

		conf := RateLimiterConf{
			Ttl:              time.Second,
			GlobalMethodRate: globalMethodRate,
			IpMethodRate:     ipMethodRate,
			MaxClients:       4,
		}
		ips := []net.IP{
			net.IPv4(0, 0, 0, 0),
			net.IPv4(0, 0, 0, 1),
			net.IPv4(0, 0, 0, 2),
			net.IPv4(0, 0, 0, 3),
		}
		limiter := NewRateLimiter(conf)
		for _, ip := range ips {
			iip := ip

			unknownAllowed := 0
			knownAllowed := 0
			for i := 0; i < 30; i++ {
				if limiter.Allow("unknown", iip) {
					unknownAllowed += 1
				}

				if limiter.Allow("known", iip) {
					knownAllowed += 1
				}
			}
			Expect(unknownAllowed).To(Equal(25))
			Expect(knownAllowed).To(Equal(10))
		}
	})

	It("Should prune ips", func() {
		conf := NewRateLimitConf(
			rate.Limit(5000),
			rate.Limit(25),
			time.Second,
			5,
		)
		ips := []net.IP{
			net.IPv4(0, 0, 0, 0),
			net.IPv4(0, 0, 0, 1),
			net.IPv4(0, 0, 0, 2),
			net.IPv4(0, 0, 0, 3),
		}
		limiter := NewRateLimiter(conf)

		for _, ip := range ips {
			iip := ip
			go func() {
				for i := 0; i < 30; i++ {
					limiter.Allow("unknown", iip)
				}
			}()
		}
		time.Sleep(3 * time.Second)
		pruned := limiter.Prune()
		Expect(pruned).To(Equal(4))
		Expect(limiter.Prune()).To(Equal(0))
	})
})
