package v1_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1 "github.com/renproject/lightnode/compat/v1"
	"github.com/renproject/lightnode/testutils"
)

var _ = Describe("Compat V0", func() {
	It("should convert a QueryBlockState response into a QueryState response", func() {
		stateResponse, err := v1.QueryStateResponseFromState(testutils.MockEngineState())

		Expect(err).ShouldNot(HaveOccurred())
		Expect(stateResponse.Bitcoin.Gaslimit).Should(Equal("3"))
	})
})
