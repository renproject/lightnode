package v0_test

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"

	_ "github.com/mattn/go-sqlite3"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/alicebob/miniredis/v2"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-redis/redis/v7"
	"github.com/renproject/darknode/binding"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/id"
	v0 "github.com/renproject/lightnode/compat/v0"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/lightnode/testutils"
	"github.com/renproject/multichain"
	"github.com/renproject/pack"
)

var _ = Describe("Compat V0", func() {
	init := func(params v0.ParamsSubmitTx, hasCache bool) (v0.Store, redis.Cmdable, *binding.Binding, *id.PubKey) {
		mr, err := miniredis.Run()
		if err != nil {
			panic(err)
		}

		client := redis.NewClient(&redis.Options{
			Addr: mr.Addr(),
		})

		if hasCache {
			// Cache a lookup value for the utxo so that
			// we don't have to rely on external explorers
			utxo := params.Tx.In.Get("utxo").Value.(v0.ExtBtcCompatUTXO)
			vout := utxo.VOut.Int.String()
			txHash := utxo.TxHash
			key := fmt.Sprintf("amount_%s_%s", txHash, vout)
			client.Set(key, 200000, 0)

		}

		bindingsOpts := binding.DefaultOptions().
			WithNetwork("testnet")

		bindingsOpts.WithChainOptions(multichain.Bitcoin, binding.ChainOptions{
			RPC:           pack.String("https://multichain-staging.renproject.io/testnet/bitcoind"),
			Confirmations: pack.U64(0),
		})

		bindingsOpts.WithChainOptions(multichain.BitcoinCash, binding.ChainOptions{
			RPC:           pack.String("https://multichain-staging.renproject.io/testnet/bitcoincashd"),
			Confirmations: pack.U64(0),
		})

		bindingsOpts.WithChainOptions(multichain.Zcash, binding.ChainOptions{
			RPC:           pack.String("https://multichain-staging.renproject.io/testnet/zcashd"),
			Confirmations: pack.U64(0),
		})

		bindingsOpts.WithChainOptions(multichain.Ethereum, binding.ChainOptions{
			RPC:           pack.String("https://multichain-staging.renproject.io/testnet/kovan"),
			Confirmations: pack.U64(0),
			Protocol:      pack.String("0x5045E727D9D9AcDe1F6DCae52B078EC30dC95455"),
		})

		bindings := binding.New(bindingsOpts)

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

	It("should convert a QueryState response into a QueryFees response", func() {
		shardsResponse, err := v0.QueryFeesResponseFromState(testutils.MockEngineState())

		Expect(err).ShouldNot(HaveOccurred())
		Expect(shardsResponse.Btc.Lock.Int).Should(Equal(big.NewInt(6)))
	})

	It("should convert a QueryState response into a QueryShards response", func() {
		shardsResponse, err := v0.ShardsResponseFromSystemState(testutils.MockSystemState())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(shardsResponse.Shards[0].Gateways[0].PubKey).Should(Equal("Akwn5WEMcB2Ff_E0ZOoVks9uZRvG_eFD99AysymOc5fm"))
	})

	It("should convert a v0 BTC Burn ParamsSubmitTx into an empty v1 ParamsSubmitTx", func() {
		params := testutils.MockBurnParamSubmitTxV0BTC()
		store, _, bindings, pubkey := init(params, false)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		v1, err := v0.V1TxParamsFromTx(ctx, params, bindings, pubkey, store)
		Expect(err).ShouldNot(HaveOccurred())
		hash, err := hex.DecodeString("ec5a011106c04a4019587c192409ca92faa518639569ccebd3c025c283b80fe9")
		hash32 := [32]byte{}
		copy(hash32[:], hash[:])
		Expect(v1).Should(Equal(jsonrpc.ParamsSubmitTx{
			Tx: tx.Tx{
				Selector: tx.Selector("BTC/fromEthereum"),
				Input:    pack.NewTyped("v0hash", pack.NewBytes32(hash32)),
			},
		}))
	})

	It("should convert a v0 BTC ParamsSubmitTx into a v1 ParamsSubmitTx", func() {
		params := testutils.MockParamSubmitTxV0BTC()
		store, client, bindings, pubkey := init(params, true)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		v1, err := v0.V1TxParamsFromTx(ctx, params, bindings, pubkey, store)
		Expect(err).ShouldNot(HaveOccurred())

		// should have a key for the utxo
		utxo := params.Tx.In.Get("utxo").Value.(v0.ExtBtcCompatUTXO)
		keys, err := client.Keys(utxo.TxHash.String() + "_" + utxo.VOut.Int.String()).Result()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(len(keys)).Should(Equal(1))

		// Check that redis mapped the hashes correctly
		hash := v1.Tx.Hash.String()
		storedHash, err := client.Get(keys[0]).Result()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(storedHash).Should(Equal(hash))

		// btc txhash mapping
		keys, err = client.Keys("*").Result()

		// v0 hash should have a mapping in the store
		ghash := v1.Tx.Input.Get("ghash").(pack.Bytes32)
		txid := v1.Tx.Input.Get("txid").(pack.Bytes)
		txindex := v1.Tx.Input.Get("txindex").(pack.U32)
		v0Hash := v0.MintTxHash(v1.Tx.Selector, ghash, txid, txindex)
		Expect(keys).Should(ContainElement(v0Hash.String()))

		// v1 hash should be correct
		v1Hash, err := tx.NewTxHash(v1.Tx.Version, v1.Tx.Selector, v1.Tx.Input)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(hash).To(Equal(v1Hash.String()))
	})

	It("should convert a v0 ZEC ParamsSubmitTx into a v1 ParamsSubmitTx", func() {
		params := testutils.MockParamSubmitTxV0ZEC()
		store, client, bindings, pubkey := init(params, true)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		v1, err := v0.V1TxParamsFromTx(ctx, params, bindings, pubkey, store)
		Expect(err).ShouldNot(HaveOccurred())

		// should have a key for the utxo
		utxo := params.Tx.In.Get("utxo").Value.(v0.ExtBtcCompatUTXO)
		keys, err := client.Keys(utxo.TxHash.String() + "_" + utxo.VOut.Int.String()).Result()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(len(keys)).Should(Equal(1))

		// Check that redis mapped the hashes correctly
		hash := v1.Tx.Hash.String()
		storedHash, err := client.Get(keys[0]).Result()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(storedHash).Should(Equal(hash))

		// btc txhash mapping
		keys, err = client.Keys("*").Result()

		// v0 hash should have a mapping in the store
		ghash := v1.Tx.Input.Get("ghash").(pack.Bytes32)
		txid := v1.Tx.Input.Get("txid").(pack.Bytes)
		txindex := v1.Tx.Input.Get("txindex").(pack.U32)
		v0Hash := v0.MintTxHash(v1.Tx.Selector, ghash, txid, txindex)
		Expect(keys).Should(ContainElement(v0Hash.String()))

		// v1 hash should be correct
		v1Hash, err := tx.NewTxHash(v1.Tx.Version, v1.Tx.Selector, v1.Tx.Input)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(hash).To(Equal(v1Hash.String()))
	})

	It("should convert a v0 BCH ParamsSubmitTx into a v1 ParamsSubmitTx", func() {
		params := testutils.MockParamSubmitTxV0BCH()
		store, client, bindings, pubkey := init(params, true)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		v1, err := v0.V1TxParamsFromTx(ctx, params, bindings, pubkey, store)
		Expect(err).ShouldNot(HaveOccurred())

		// should have a key for the utxo
		utxo := params.Tx.In.Get("utxo").Value.(v0.ExtBtcCompatUTXO)
		keys, err := client.Keys(utxo.TxHash.String() + "_" + utxo.VOut.Int.String()).Result()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(len(keys)).Should(Equal(1))

		// Check that redis mapped the hashes correctly
		hash := v1.Tx.Hash.String()
		storedHash, err := client.Get(keys[0]).Result()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(hash).Should(Equal(storedHash))

		// btc txhash mapping
		keys, err = client.Keys("*").Result()

		// v0 hash should have a mapping in the store
		ghash := v1.Tx.Input.Get("ghash").(pack.Bytes32)
		txid := v1.Tx.Input.Get("txid").(pack.Bytes)
		txindex := v1.Tx.Input.Get("txindex").(pack.U32)
		v0Hash := v0.MintTxHash(v1.Tx.Selector, ghash, txid, txindex)
		Expect(keys).Should(ContainElement(v0Hash.String()))

		// v1 hash should be correct
		v1Hash, err := tx.NewTxHash(v1.Tx.Version, v1.Tx.Selector, v1.Tx.Input)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(hash).To(Equal(v1Hash.String()))
	})
})
