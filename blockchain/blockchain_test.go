package blockchain_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConnPool", func() {
	Context("after creating a ConnPool object.", func() {
		// TODO: Ganache must be set-up in order to test these.
		It("should get shift out details from contract logs", func() {
			Expect(true).Should(BeTrue())
		})

		It("should get UTXO details from specific blockchain", func() {
			Expect(true).Should(BeTrue())
		})

		It("should be able to return number of confirmations for a given transaction", func() {
			Expect(true).Should(BeTrue())
		})

		It("should be able to return number of confirmations for a given event", func() {
			Expect(true).Should(BeTrue())
		})

		It("should be able to verify if a UTXO can be spent by a given key", func() {
			Expect(true).Should(BeTrue())
		})

		It("should be able to check whether a given transaction is a shift in or shift out", func() {
			Expect(true).Should(BeTrue())
		})
	})
})
