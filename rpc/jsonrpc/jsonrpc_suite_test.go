package jsonrpc_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestJsonrpc(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Jsonrpc Suite")
}
