package v0_test

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"

	_ "github.com/mattn/go-sqlite3"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/alicebob/miniredis"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-redis/redis/v7"
	"github.com/renproject/darknode/txengine/txenginebindings"
	"github.com/renproject/id"
	v0 "github.com/renproject/lightnode/compat/v0"
	"github.com/renproject/lightnode/db"
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
		params := testutils.MockParamSubmitTxV0()

		utxo := params.Tx.In.Get("utxo").Value.(v0.ExtBtcCompatUTXO)
		vout := utxo.VOut.Int.String()
		btcTxHash := utxo.TxHash
		key := fmt.Sprintf("amount_%s_%s", btcTxHash, vout)
		fmt.Println(key)
		client.Set(key, 200000, 0)

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

		sqlDB, err := sql.Open("sqlite3", "./test.db")
		database := db.New(sqlDB)
		store := v0.NewCompatStore(database, client)

		v1, err := v0.V1TxParamsFromTx(ctx, params, bindings, (*id.PubKey)(pubkey), store)
		Expect(err).ShouldNot(HaveOccurred())
		// Check that redis mapped the hashes correctly
		hash := v1.Tx.Hash.String()
		// btc txhash mapping
		keys, err := client.Keys("*").Result()

		// should have a key for the utxo
		keys, err = client.Keys(utxo.TxHash.String() + "_" + utxo.VOut.Int.String()).Result()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(len(keys)).Should(Equal(1))

		storedHash, err := client.Get(keys[0]).Result()
		Expect(err).ShouldNot(HaveOccurred())

		Expect(storedHash).Should(Equal(v1.Tx.Hash.String()))

		Expect(hash).Should(Equal("J7-sw5tPd_HzC8IPbsjEq_cqUjG_1pRgEp0gjC-kiL8"))
	})
})
