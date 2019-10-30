package cacher_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestCacher(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cacher Suite")
}
