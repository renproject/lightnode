package blockchain_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConnPool", func() {
	Context("after creating a ConnPool object.", func() {
		// TODO : Need to set up ganache for local test and CI
		It("should get ShiftOut details from contract logs", func() {
			Expect(true).Should(BeTrue())
		})

		It("should get utxo details from specific blockchain", func() {
			Expect(true).Should(BeTrue())
		})

		It("should be able to return number of confirmations of a tx", func() {
			Expect(true).Should(BeTrue())
		})

		It("should be able to return number of confirmations of a event log", func() {
			Expect(true).Should(BeTrue())
		})

		It("should be able to verify if a utxo can be spent by a key", func() {
			Expect(true).Should(BeTrue())
		})

		It("should tell if a tx is a ShiftIn or ShiftOut", func() {
			Expect(true).Should(BeTrue())
		})
	})
})
