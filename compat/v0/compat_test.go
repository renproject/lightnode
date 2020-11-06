package v0_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v0 "github.com/renproject/lightnode/compat/v0"
	"github.com/renproject/lightnode/testutils"
)

var _ = Describe("Compat V0", func() {
	It("should convert a QueryState response into a QueryShards response", func() {

		shardsResponse, err := v0.ShardsFromState(testutils.MockQueryStateResponse())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(shardsResponse.Shards[0].Gateways[0].PubKey).Should(Equal("Akwn5WEMcB2Ff_E0ZOoVks9uZRvG_eFD99AysymOc5fm"))
	})
})
