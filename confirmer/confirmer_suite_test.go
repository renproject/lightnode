package confirmer_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestConfirmer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Confirmer Suite")
}
