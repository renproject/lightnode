package ratelimiter_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRatelimiter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ratelimiter Suite")
}
