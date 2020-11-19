package v0_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v0 "github.com/renproject/lightnode/compat/v0"
	"github.com/renproject/lightnode/testutils"
)

var _ = Describe("Compat V0", func() {
	It("should convert a QueryState response into a QueryShards response", func() {

		shardsResponse, err := v0.ShardsResponseFromState(testutils.MockQueryStateResponse())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(shardsResponse.Shards[0].Gateways[0].PubKey).Should(Equal("Akwn5WEMcB2Ff_E0ZOoVks9uZRvG_eFD99AysymOc5fm"))
	})

	It("should convert a v0 ParamsSubmitTx into a v1 ParamsSubmitTx", func() {
		params := testutils.MockParamSubmitTxVo()

		v1, err := v0.V1TxParamsFromTx(params)
		Expect(err).ShouldNot(HaveOccurred())
		hash := v1.Tx.Hash.String()
		Expect(hash).Should(Equal("VQaqF8ZagLv0RCxoAPia-fcJcwzWLWgP6vzBSCHJqN4"))
	})
})

// v0 tx query params
// {"id":1,"jsonrpc":"2.0","method":"ren_queryTx","params":{"txHash":"W7mHS0KD3ytwoXEuoKWHbWiQM+54peV3adm3PE/jAW0="}}

// v0 tx submit params
// {"id":1,"jsonrpc":"2.0","method":"ren_submitTx","params":{"tx":{"to":"BTC0Btc2Eth","in":[{"name":"p","type":"ext_ethCompatPayload","value":{"abi":"W3siY29uc3RhbnQiOmZhbHNlLCJpbnB1dHMiOlt7InR5cGUiOiJzdHJpbmciLCJuYW1lIjoiX3N5bWJvbCJ9LHsidHlwZSI6ImFkZHJlc3MiLCJuYW1lIjoiX2FkZHJlc3MifSx7Im5hbWUiOiJfYW1vdW50IiwidHlwZSI6InVpbnQyNTYifSx7Im5hbWUiOiJfbkhhc2giLCJ0eXBlIjoiYnl0ZXMzMiJ9LHsibmFtZSI6Il9zaWciLCJ0eXBlIjoiYnl0ZXMifV0sIm91dHB1dHMiOltdLCJwYXlhYmxlIjp0cnVlLCJzdGF0ZU11dGFiaWxpdHkiOiJwYXlhYmxlIiwidHlwZSI6ImZ1bmN0aW9uIiwibmFtZSI6Im1pbnQifV0=","value":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAADqiy/w1/VGr66uF3EwZzY1fe+kNAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAADQlRDAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","fn":"bWludA=="}},{"name":"token","type":"ext_ethCompatAddress","value":"0A9ADD98C076448CBcFAcf5E457DA12ddbEF4A8f"},{"name":"to","type":"ext_ethCompatAddress","value":"7DDFA2e5435027f6e13Ca8Db2f32ebd5551158Bb"},{"name":"n","type":"b32","value":"UL02xN5g613wuVxDCRDN0ynj5IVUyY0ehBgecccHLzw="},{"name":"utxo","type":"ext_btcCompatUTXO","value":{"txHash":"7AuVKdtoEOEpvhkUecFvt39ggsk/QYr0talTTGSPB4A=","vOut":"0"}}]},"tags":[]}}
