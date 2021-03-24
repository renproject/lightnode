package v1_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1 "github.com/renproject/lightnode/compat/v1"
	v2 "github.com/renproject/lightnode/compat/v2"
	"github.com/renproject/lightnode/testutils"
)

var _ = Describe("Compat V0", func() {
	It("should convert a QueryBlockState response into a QueryState response", func() {
		mockState := testutils.MockQueryBlockStateResponse()

		raw, err := json.Marshal(mockState)
		Expect(err).ToNot(HaveOccurred())

		var resp v2.QueryBlockStateJSON
		json.Unmarshal(raw, &resp)
		Expect(err).ToNot(HaveOccurred())

		stateResponse, err := v1.QueryStateResponseFromState(
			map[string]v2.UTXOState{
				"BTC":  resp.State.BTC,
				"ZEC":  resp.State.ZEC,
				"BCH":  resp.State.BCH,
				"DGB":  resp.State.DGB,
				"DOGE": resp.State.DOGE,
			},
			map[string]v2.AccountState{
				"LUNA": resp.State.LUNA,
				"FIL":  resp.State.FIL,
			},
		)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(stateResponse.Bitcoin.Gaslimit).Should(Equal("3"))
	})
})
