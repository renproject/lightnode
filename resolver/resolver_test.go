package resolver_test

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-redis/redis/v7"
	_ "github.com/mattn/go-sqlite3"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/renproject/id"
	v0 "github.com/renproject/lightnode/compat/v0"
	. "github.com/renproject/lightnode/resolver"
	"github.com/renproject/multichain"

	"github.com/renproject/aw/wire"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx/txutil"
	"github.com/renproject/darknode/txengine/txenginebindings"
	"github.com/renproject/darknode/txengine/txengineutil"
	"github.com/renproject/darknode/txpool/txpoolverifier"
	"github.com/renproject/kv"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/lightnode/store"
	"github.com/renproject/lightnode/testutils"
	"github.com/renproject/pack"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Resolver", func() {
	init := func(ctx context.Context) (*Resolver, jsonrpc.Validator) {
		logger := logrus.New()

		table := kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses")

		bootstrap := make([]wire.Address, 1)
		bootstrap[0] = wire.Address{
			Protocol:  wire.TCP,
			Value:     "localhost",
			Nonce:     0,
			Signature: [65]byte{},
		}

		multiaddrStore := store.New(table, bootstrap)

		sqlDB, err := sql.Open("sqlite3", "./resolver_test.db")
		Expect(err).NotTo(HaveOccurred())

		database := db.New(sqlDB)
		Expect(database.Init()).Should(Succeed())

		mr, err := miniredis.Run()
		if err != nil {
			panic(err)
		}

		client := redis.NewClient(&redis.Options{
			Addr: mr.Addr(),
		})
		// Hack to ensure that the mock tx can be cast
		// prevents needing to use the bindings to find the utxo
		params := testutils.MockParamSubmitTxV0BTC()
		utxo := params.Tx.In.Get("utxo").Value.(v0.ExtBtcCompatUTXO)
		vout := utxo.VOut.Int.String()
		btcTxHash := utxo.TxHash
		key := fmt.Sprintf("amount_%s_%s", btcTxHash, vout)
		client.Set(key, 200000, 0)

		r := rand.New(rand.NewSource(GinkgoRandomSeed()))

		output := pack.NewTyped(
			"foo", pack.U64(0).Generate(r, 1).Interface().(pack.U64),
			"bar", pack.Bytes32{}.Generate(r, 1).Interface().(pack.Bytes32),
		)

		verifier := txpoolverifier.New(txengineutil.NewMockTxEngine(output))

		bindingsOpts := txenginebindings.DefaultOptions().
			WithNetwork("localnet")

		bindingsOpts.WithChainOptions(multichain.Bitcoin, txenginebindings.ChainOptions{
			RPC:           pack.String("https://multichain-staging.renproject.io/testnet/bitcoind"),
			Confirmations: pack.U64(0),
		})

		bindingsOpts.WithChainOptions(multichain.Ethereum, txenginebindings.ChainOptions{
			RPC:           pack.String("https://multichain-staging.renproject.io/testnet/geth"),
			Confirmations: pack.U64(0),
			Protocol:      pack.String("0xcF9F36668ad5b28B336B248a67268AFcF1ECbdbF"),
		})

		bindings, err := txenginebindings.New(bindingsOpts)
		if err != nil {
			logger.Panicf("bad bindings: %v", err)
		}

		cacher := testutils.NewMockCacher()
		go cacher.Run(ctx)

		compatStore := v0.NewCompatStore(database, client)

		pubkeyB, err := base64.URLEncoding.DecodeString("AiF7_2ykZmts2wzZKJ5D-J1scRM2Pm2jJ84W_K4PQaGl")
		Expect(err).ShouldNot(HaveOccurred())

		pubkey, err := crypto.DecompressPubkey(pubkeyB)
		Expect(err).ShouldNot(HaveOccurred())

		validator := NewValidator(bindings, (*id.PubKey)(pubkey), compatStore, logger)
		resolver := New(logger, cacher, multiaddrStore, verifier, database, jsonrpc.Options{}, compatStore, bindings)

		return resolver, validator
	}

	cleanup := func() {
		Expect(os.Remove("./resolver_test.db")).Should(BeNil())
	}

	It("should handle all known methods", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		resolver, _ := init(ctx)
		defer cleanup()
		offset := pack.NewU32(1)

		innerCtx, innerCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer innerCancel()

		responses := []jsonrpc.Response{
			resolver.QueryTx(innerCtx, nil, &jsonrpc.ParamsQueryTx{}, nil),
			resolver.QueryTxs(innerCtx, nil, &jsonrpc.ParamsQueryTxs{}, nil),
			resolver.QueryTxs(innerCtx, nil, &jsonrpc.ParamsQueryTxs{
				TxStatus: 0,
				Offset:   &offset,
				Limit:    &offset,
			}, nil),
			resolver.QueryBlock(innerCtx, nil, &jsonrpc.ParamsQueryBlock{}, nil),
			resolver.QueryBlocks(innerCtx, nil, &jsonrpc.ParamsQueryBlocks{}, nil),
			resolver.QueryPeers(innerCtx, nil, &jsonrpc.ParamsQueryPeers{}, nil),
			resolver.QueryNumPeers(innerCtx, nil, &jsonrpc.ParamsQueryNumPeers{}, nil),
			resolver.QueryShards(innerCtx, nil, &jsonrpc.ParamsQueryShards{}, nil),
			resolver.QueryStat(innerCtx, nil, &jsonrpc.ParamsQueryStat{}, nil),
			resolver.QueryFees(innerCtx, nil, &jsonrpc.ParamsQueryFees{}, nil),
			resolver.QueryConfig(innerCtx, nil, &jsonrpc.ParamsQueryConfig{}, nil),
			resolver.QueryState(innerCtx, nil, &jsonrpc.ParamsQueryState{}, nil),
		}

		// Validate responses.
		for _, response := range responses {
			Expect(response.ID).To(BeNil())
			Expect(response.Error).To(BeNil())
		}
	})

	It("should handle queryTx to a v0 tx", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		resolver, validator := init(ctx)
		defer cleanup()

		innerCtx, innerCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer innerCancel()

		// Submit tx to ensure that it can be queried against
		params := testutils.MockParamSubmitTxV0BTC()
		paramsJSON, err := json.Marshal(params)
		Expect(err).ShouldNot(HaveOccurred())

		// It's a bit of a pain to make this robustly calculatable, so lets use the mock
		// v0 tx's v0 hash directly
		v0HashBytes, err := base64.StdEncoding.DecodeString("npiRyatJm8KSgbwA/EqdvFclMjfsnfrVY2HkjhElEDk=")
		Expect(err).ShouldNot(HaveOccurred())
		v0Hash := [32]byte{}
		copy(v0Hash[:], v0HashBytes[:])

		req, resp := validator.ValidateRequest(innerCtx, nil, jsonrpc.Request{
			Version: "2.0",
			ID:      nil,
			Method:  jsonrpc.MethodSubmitTx,
			Params:  paramsJSON,
		})
		Expect(resp).Should(Equal(jsonrpc.Response{}))

		// Submit so that it gets persisted in db
		resp = resolver.SubmitTx(ctx, nil, (req).(*jsonrpc.ParamsSubmitTx), nil)

		submission := (req).(*jsonrpc.ParamsSubmitTx)
		Expect(submission.Tx.Hash).NotTo(Equal(pack.Bytes32{}))

		resp = resolver.QueryTx(ctx, nil, &jsonrpc.ParamsQueryTx{
			TxHash: v0Hash,
		}, nil)

		Expect(resp).ShouldNot(Equal(jsonrpc.Response{}))
	})

	It("should handle a request witout a specified ID", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		resolver, _ := init(ctx)
		defer cleanup()

		urlI, err := url.Parse("http://localhost/")
		Expect(err).ShouldNot(HaveOccurred())
		params := jsonrpc.ParamsQueryConfig{}
		req := http.Request{
			Method: http.MethodPost,
			URL:    urlI,
		}

		innerCtx, innerCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer innerCancel()

		resp := resolver.QueryConfig(innerCtx, "", &params, &req)
		Expect(resp).Should(BeZero())
	})

	It("should fail when querying an unknown node", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		resolver, _ := init(ctx)
		defer cleanup()

		urlI, err := url.Parse("http://localhost/?id=123")
		Expect(err).ShouldNot(HaveOccurred())
		params := jsonrpc.ParamsQueryConfig{}
		req := http.Request{
			Method: http.MethodPost,
			URL:    urlI,
		}

		innerCtx, innerCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer innerCancel()

		resp := resolver.QueryConfig(innerCtx, "", &params, &req)
		Expect(resp.Error.Message).Should(ContainSubstring("unknown darknode"))
	})

	It("should succeed when querying a known node", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		resolver, _ := init(ctx)
		defer cleanup()

		urlI, err := url.Parse("http://localhost/?id=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
		Expect(err).ShouldNot(HaveOccurred())
		params := jsonrpc.ParamsQueryConfig{}
		req := http.Request{
			Method: http.MethodPost,
			URL:    urlI,
		}

		innerCtx, innerCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer innerCancel()

		resp := resolver.QueryConfig(innerCtx, "", &params, &req)
		Expect(resp.Error).Should(BeZero())
	})

	It("should submit v0 burn txs", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		resolver, validator := init(ctx)
		defer cleanup()

		params := testutils.MockBurnParamSubmitTxV0BTC()
		paramsJSON, err := json.Marshal(params)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(params).ShouldNot(Equal([]byte{}))

		innerCtx, innerCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer innerCancel()

		req, resp := validator.ValidateRequest(innerCtx, nil, jsonrpc.Request{
			Version: "2.0",
			ID:      nil,
			Method:  jsonrpc.MethodSubmitTx,
			Params:  paramsJSON,
		})
		// Response will only exist for errors
		Expect(resp).Should(Equal(jsonrpc.Response{}))
		Expect((req).(*jsonrpc.ParamsSubmitTx).Tx.Hash).ShouldNot(BeEmpty())

		resp = resolver.SubmitTx(ctx, nil, (req).(*jsonrpc.ParamsSubmitTx), nil)
		Expect(resp.Error).Should(BeZero())
	})

	It("should submit txs", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		resolver, _ := init(ctx)
		defer cleanup()

		r := rand.New(rand.NewSource(GinkgoRandomSeed()))

		params := jsonrpc.ParamsSubmitTx{
			Tx: txutil.RandomGoodTx(r),
		}

		innerCtx, innerCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer innerCancel()

		resp := resolver.SubmitTx(innerCtx, nil, &params, nil)

		Expect(resp.Error).Should(BeZero())
	})
})
