package v0_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/renproject/darknode/txengine/txenginebindings"
	"github.com/renproject/id"
	v0 "github.com/renproject/lightnode/compat/v0"
	"github.com/renproject/lightnode/testutils"
	"github.com/renproject/multichain"
	"github.com/renproject/pack"
)

var _ = Describe("Compat V0", func() {
	It("should convert a QueryState response into a QueryShards response", func() {

		shardsResponse, err := v0.ShardsResponseFromState(testutils.MockQueryStateResponse())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(shardsResponse.Shards[0].Gateways[0].PubKey).Should(Equal("Akwn5WEMcB2Ff_E0ZOoVks9uZRvG_eFD99AysymOc5fm"))
	})

	It("should convert a v0 ParamsSubmitTx into a v1 ParamsSubmitTx", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		params := testutils.MockParamSubmitTxVo()

		bindingsOpts := txenginebindings.DefaultOptions().
			WithNetwork("localnet")

		bindingsOpts.WithChainOptions(multichain.Bitcoin, txenginebindings.ChainOptions{
			RPC:           pack.String("https://multichain-staging.renproject.io/testnet/bitcoind"),
			Confirmations: pack.U64(0),
		})

		bindingsOpts.WithChainOptions(multichain.Ethereum, txenginebindings.ChainOptions{
			RPC:           pack.String("https://multichain-staging.renproject.io/testnet/geth"),
			Confirmations: pack.U64(0),
			Protocol:      pack.String("0x1CAD87e16b56815d6a0b4Cd91A6639eae86Fc53A"),
		})

		bindings, err := txenginebindings.New(bindingsOpts)
		privKey := id.NewPrivKey()
		v1, err := v0.V1TxParamsFromTx(ctx, params, bindings, privKey.PubKey())
		Expect(err).ShouldNot(HaveOccurred())
		hash := v1.Tx.Hash.String()
		Expect(hash).Should(Equal("FkS4YttbHfuxaUaoHFAe-FZ3lodPa4C-Al9BONurjmQ__"))
	})
})
