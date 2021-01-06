package v0_test

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"os"

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
	init := func(params v0.ParamsSubmitTx) (v0.Store, redis.Cmdable, *txenginebindings.Bindings, *id.PubKey) {
		mr, err := miniredis.Run()
		if err != nil {
			panic(err)
		}

		client := redis.NewClient(&redis.Options{
			Addr: mr.Addr(),
		})

		// Cache a lookup value for the utxo so that
		// we don't have to rely on external explorers
		utxo := params.Tx.In.Get("utxo").Value.(v0.ExtBtcCompatUTXO)
		vout := utxo.VOut.Int.String()
		txHash := utxo.TxHash
		key := fmt.Sprintf("amount_%s_%s", txHash, vout)
		client.Set(key, 200000, 0)

		bindingsOpts := txenginebindings.DefaultOptions().
			WithNetwork("testnet")

		bindingsOpts.WithChainOptions(multichain.Bitcoin, txenginebindings.ChainOptions{
			RPC:           pack.String("https://multichain-staging.renproject.io/testnet/bitcoind"),
			Confirmations: pack.U64(0),
		})

		bindingsOpts.WithChainOptions(multichain.BitcoinCash, txenginebindings.ChainOptions{
			RPC:           pack.String("https://multichain-staging.renproject.io/testnet/bitcoincashd"),
			Confirmations: pack.U64(0),
		})

		bindingsOpts.WithChainOptions(multichain.Ethereum, txenginebindings.ChainOptions{
			RPC:           pack.String("https://multichain-staging.renproject.io/testnet/geth"),
			Confirmations: pack.U64(0),
			Protocol:      pack.String("0xcF9F36668ad5b28B336B248a67268AFcF1ECbdbF"),
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

		return store, client, bindings, (*id.PubKey)(pubkey)
	}

	BeforeSuite(func() {
		os.Remove("./test.db")
	})

	AfterSuite(func() {
		os.Remove("./test.db")
	})

	It("should convert a QueryState response into a QueryShards response", func() {

		shardsResponse, err := v0.ShardsResponseFromState(testutils.MockQueryStateResponse())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(shardsResponse.Shards[0].Gateways[0].PubKey).Should(Equal("Akwn5WEMcB2Ff_E0ZOoVks9uZRvG_eFD99AysymOc5fm"))
	})

	It("should convert a v0 BTC ParamsSubmitTx into a v1 ParamsSubmitTx", func() {
		params := testutils.MockParamSubmitTxV0BTC()
		store, client, bindings, pubkey := init(params)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		v1, err := v0.V1TxParamsFromTx(ctx, params, bindings, pubkey, store)
		Expect(err).ShouldNot(HaveOccurred())
		// Check that redis mapped the hashes correctly
		hash := v1.Tx.Hash.String()
		// btc txhash mapping
		keys, err := client.Keys("*").Result()

		// should have a key for the utxo
		utxo := params.Tx.In.Get("utxo").Value.(v0.ExtBtcCompatUTXO)
		keys, err = client.Keys(utxo.TxHash.String() + "_" + utxo.VOut.Int.String()).Result()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(len(keys)).Should(Equal(1))

		storedHash, err := client.Get(keys[0]).Result()
		Expect(err).ShouldNot(HaveOccurred())

		Expect(storedHash).Should(Equal(hash))

		// btc txhash mapping
		keys, err = client.Keys("*").Result()

		// v0 hash should have a mapping in the store
		Expect(keys).Should(ContainElement("npiRyatJm8KSgbwA/EqdvFclMjfsnfrVY2HkjhElEDk="))

		// v1 hash should be correct
		Expect(hash).Should(Equal("YlkYzfTTCcptfS4bYdxnrXXNMv-C_6Y1UzWwi_wOrGI"))
	})

	It("should convert a v0 BCH ParamsSubmitTx into a v1 ParamsSubmitTx", func() {
		params := testutils.MockParamSubmitTxV0BCH()
		store, client, bindings, pubkey := init(params)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		v1, err := v0.V1TxParamsFromTx(ctx, params, bindings, pubkey, store)
		Expect(err).ShouldNot(HaveOccurred())
		// should have a key for the utxo
		utxo := params.Tx.In.Get("utxo").Value.(v0.ExtBtcCompatUTXO)
		keys, err := client.Keys(utxo.TxHash.String() + "_" + utxo.VOut.Int.String()).Result()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(len(keys)).Should(Equal(1))

		storedHash, err := client.Get(keys[0]).Result()
		Expect(err).ShouldNot(HaveOccurred())

		// btc txhash mapping
		keys, err = client.Keys("*").Result()

		// v0 hash should have a mapping in the store
		Expect(keys).Should(ContainElement("pEXm6Sae81WZxvzyqS8VAoLBAK3Df5r6FENl5BegewI="))

		// Check that redis mapped the hashes correctly
		hash := v1.Tx.Hash.String()
		Expect(hash).Should(Equal(storedHash))

	})
})
