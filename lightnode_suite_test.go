package lightnode_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLightnode(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Lightnode Suite")
}
