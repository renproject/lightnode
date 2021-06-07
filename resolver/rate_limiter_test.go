package resolver_test

import (
	"net"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	. "github.com/renproject/lightnode/resolver"
)

var _ = Describe("Rate Limiter", func() {
	It("Should allow when 1 rate is set", func() {
		logger := logrus.New()
		conf := RateLimiterConf{
			GlobalRate: rate.Limit(1),
			IpRate:     rate.Limit(1),
			Ttl:        time.Second,
			MaxClients: 1,
		}
		limiter := NewRateLimiter(conf, logger)
		allowed := limiter.Allow(net.IPv4(0, 0, 0, 0))
		Expect(allowed).To(BeTrue())
	})

	It("Should rate limit when rate is over global limit", func() {
		logger := logrus.New()
		conf := RateLimiterConf{
			GlobalRate: rate.Limit(1),
			IpRate:     rate.Limit(100),
			Ttl:        time.Second,
			MaxClients: 1,
		}
		limiter := NewRateLimiter(conf, logger)
		allowed := limiter.Allow(net.IPv4(0, 0, 0, 0))
		Expect(allowed).To(BeTrue())
		allowed = limiter.Allow(net.IPv4(0, 0, 0, 0))
		Expect(allowed).To(BeFalse())
	})

	It("Should rate limit when rate is over ip limit", func() {
		logger := logrus.New()
		conf := RateLimiterConf{
			GlobalRate: rate.Limit(100),
			IpRate:     rate.Limit(1),
			Ttl:        time.Second,
			MaxClients: 1,
		}
		limiter := NewRateLimiter(conf, logger)
		allowed := limiter.Allow(net.IPv4(0, 0, 0, 0))
		Expect(allowed).To(BeTrue())
		allowed = limiter.Allow(net.IPv4(0, 0, 0, 0))
		allowed = limiter.Allow(net.IPv4(0, 0, 0, 0))
		allowed = limiter.Allow(net.IPv4(0, 0, 0, 0))
		Expect(allowed).To(BeFalse())
	})

	It("Should rate limit when rate is over high ip limit", func() {
		logger := logrus.New()
		conf := RateLimiterConf{
			GlobalRate: rate.Limit(1000),
			IpRate:     rate.Limit(100),
			Ttl:        time.Second,
			MaxClients: 1,
		}
		limiter := NewRateLimiter(conf, logger)
		allowed := limiter.Allow(net.IPv4(0, 0, 0, 0))
		Expect(allowed).To(BeTrue())
		for i := 0; i < 101; i++ {
			allowed = limiter.Allow(net.IPv4(0, 0, 0, 0))
		}
		Expect(allowed).To(BeFalse())
	})

	It("Should allow multiple ips", func() {
		logger := logrus.New()
		conf := RateLimiterConf{
			GlobalRate: rate.Limit(5000),
			IpRate:     rate.Limit(25),
			Ttl:        time.Second,
			MaxClients: 4,
		}
		ips := []net.IP{
			net.IPv4(0, 0, 0, 0),
			net.IPv4(0, 0, 0, 1),
			net.IPv4(0, 0, 0, 2),
			net.IPv4(0, 0, 0, 3),
		}
		limiter := NewRateLimiter(conf, logger)
		for _, ip := range ips {
			iip := ip
			go func() {
				allowed := 0
				for i := 0; i < 30; i++ {
					if limiter.Allow(iip) {
						allowed += 1
					}
				}
				Expect(allowed).To(Equal(26))
			}()
		}
	})

	It("Should prune ips", func() {
		logger := logrus.New()
		conf := RateLimiterConf{
			GlobalRate: rate.Limit(1000),
			IpRate:     rate.Limit(25),
			Ttl:        time.Second,
			MaxClients: 5,
		}
		ips := []net.IP{
			net.IPv4(0, 0, 0, 0),
			net.IPv4(0, 0, 0, 1),
			net.IPv4(0, 0, 0, 2),
			net.IPv4(0, 0, 0, 3),
		}
		limiter := NewRateLimiter(conf, logger)

		for _, ip := range ips {
			iip := ip
			go func() {
				for i := 0; i < 30; i++ {
					limiter.Allow(iip)
				}
			}()
		}
		time.Sleep(3 * time.Second)
		pruned := limiter.Prune()
		Expect(pruned).To(Equal(4))
		Expect(limiter.Prune()).To(Equal(0))
	})
})
