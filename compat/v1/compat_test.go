package v1_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	"encoding/json"

	"github.com/renproject/darknode/engine"
	v1 "github.com/renproject/lightnode/compat/v1"
	"github.com/renproject/lightnode/testutils"
)

var _ = Describe("Compat V0", func() {
	It("should convert a QueryBlockState response into a QueryState response", func() {
		stateResponse, err := v1.QueryStateResponseFromState(testutils.MockBindings(logrus.New(), 0), testutils.MockEngineState())

		Expect(err).ShouldNot(HaveOccurred())
		Expect(stateResponse.State.Bitcoin.Gaslimit).Should(Equal("3"))
	})

	It("should omit empty revert reasons from a queryTxResponse", func() {
		output := engine.LockMintBurnReleaseOutput{
			Revert: "some reason",
		}
		txResponse := v1.TxOutputFromV2QueryTxOutput(output)

		b, err := json.Marshal(txResponse)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(fmt.Sprintf("%v", string(b))).ShouldNot(ContainSubstring("\"revert\":\"some reason\""))
	})
})
