package v0_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestCompat(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Compat Suite")
}
