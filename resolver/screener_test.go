package resolver_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/resolver"

	"github.com/renproject/pack"
)

var _ = Describe("Screening sanction addresses", func() {
	Context("when sending a list of addresses to the API", func() {
		It("should return whether the addresses are sanctioned", func() {
			screener := NewScreener("")

			// Test a sanctioned address
			addr1 := pack.String("149w62rY42aZBox8fGcmqNsXUzSStKeq8C")
			ok, err := screener.IsSanctioned(addr1)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).Should(BeTrue())

			// Test a normal address
			addr2 := pack.String("0xEAF4a99DEA6fdc1e84996a2e61830222766D8303")
			ok, err = screener.IsSanctioned(addr2)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).Should(BeFalse())
		})
	})
})
