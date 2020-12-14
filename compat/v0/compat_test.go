package v0_test

import (
	"context"
	"encoding/base64"

	"github.com/alicebob/miniredis"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-redis/redis/v7"
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
		mr, err := miniredis.Run()
		if err != nil {
			panic(err)
		}

		client := redis.NewClient(&redis.Options{
			Addr: mr.Addr(),
		})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		params := testutils.MockParamSubmitTxVo()

		bindingsOpts := txenginebindings.DefaultOptions().
			WithNetwork("testnet")

		bindingsOpts.WithChainOptions(multichain.Bitcoin, txenginebindings.ChainOptions{
			RPC:           pack.String("https://multichain-staging.renproject.io/testnet/bitcoind"),
			Confirmations: pack.U64(0),
		})

		bindingsOpts.WithChainOptions(multichain.Ethereum, txenginebindings.ChainOptions{
			RPC:           pack.String("https://kovan.infura.io/v3/fa2051f87efb4c48ba36d607a271da49"),
			Confirmations: pack.U64(0),
			Protocol:      pack.String("0x557e211EC5fc9a6737d2C6b7a1aDe3e0C11A8D5D"),
		})

		bindings, err := txenginebindings.New(bindingsOpts)
		Expect(err).ShouldNot(HaveOccurred())

		pubkeyB, err := base64.URLEncoding.DecodeString("AiF7_2ykZmts2wzZKJ5D-J1scRM2Pm2jJ84W_K4PQaGl")
		Expect(err).ShouldNot(HaveOccurred())

		pubkey, err := crypto.DecompressPubkey(pubkeyB)
		Expect(err).ShouldNot(HaveOccurred())

		v1, err := v0.V1TxParamsFromTx(ctx, params, bindings, (*id.PubKey)(pubkey), client)
		Expect(err).ShouldNot(HaveOccurred())
		// Check that redis mapped the hashes correctly
		hash := v1.Tx.Hash.String()
		keys, err := client.Keys("*").Result()
		Expect(err).ShouldNot(HaveOccurred())

		//Expect(keys[0]).Should(Equal("/6B+uY79GyznFcupz8VR918f+utk8wI/zAnsYI3OPnI="))
		Expect(keys[0]).Should(Equal("fC8FhISFgwCkDCw5SumejYhdXZAavG/2ucX+kGyOifE="))
		storedHash, err := client.Get(keys[0]).Bytes()
		Expect(err).ShouldNot(HaveOccurred())

		storedHash32 := [32]byte{}
		copy(storedHash32[:], storedHash)
		Expect(pack.Bytes32(storedHash32)).Should(Equal(v1.Tx.Hash))

		Expect(hash).Should(Equal("SJyIgJBmSrfeMbwRW1rPvav90R632Ie79AtJ5Io4INc"))
	})
})
