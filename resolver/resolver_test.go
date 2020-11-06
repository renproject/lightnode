package resolver_test

import (
	"context"
	"database/sql"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/resolver"

	"github.com/renproject/aw/wire"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx/txutil"
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
	init := func(ctx context.Context) *Resolver {
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

		r := rand.New(rand.NewSource(GinkgoRandomSeed()))

		output := pack.NewTyped(
			"foo", pack.U64(0).Generate(r, 1).Interface().(pack.U64),
			"bar", pack.Bytes32{}.Generate(r, 1).Interface().(pack.Bytes32),
		)

		verifier := txpoolverifier.New(txengineutil.NewMockTxEngine(output))

		cacher := testutils.NewMockCacher()
		go cacher.Run(ctx)

		resolver := New(logger, cacher, multiaddrStore, verifier, database, jsonrpc.Options{})

		return resolver
	}

	cleanup := func() {
		Expect(os.Remove("./resolver_test.db")).Should(BeNil())
	}

	It("should handle all known methods", func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		resolver := init(ctx)
		defer cleanup()
		offset := pack.NewU32(1)
		responses := []jsonrpc.Response{
			resolver.QueryTx(ctx, nil, &jsonrpc.ParamsQueryTx{}, nil),
			resolver.QueryTxs(ctx, nil, &jsonrpc.ParamsQueryTxs{}, nil),
			resolver.QueryTxs(ctx, nil, &jsonrpc.ParamsQueryTxs{
				TxStatus: 0,
				Offset:   &offset,
				Limit:    &offset,
			}, nil),
			resolver.QueryBlock(ctx, nil, &jsonrpc.ParamsQueryBlock{}, nil),
			resolver.QueryBlocks(ctx, nil, &jsonrpc.ParamsQueryBlocks{}, nil),
			resolver.QueryPeers(ctx, nil, &jsonrpc.ParamsQueryPeers{}, nil),
			resolver.QueryNumPeers(ctx, nil, &jsonrpc.ParamsQueryNumPeers{}, nil),
			resolver.QueryShards(ctx, nil, &jsonrpc.ParamsQueryShards{}, nil),
			resolver.QueryStat(ctx, nil, &jsonrpc.ParamsQueryStat{}, nil),
			resolver.QueryFees(ctx, nil, &jsonrpc.ParamsQueryFees{}, nil),
			resolver.QueryConfig(ctx, nil, &jsonrpc.ParamsQueryConfig{}, nil),
			resolver.QueryState(ctx, nil, &jsonrpc.ParamsQueryState{}, nil),
		}

		// Validate responses.
		for _, response := range responses {
			Expect(response.ID).To(BeNil())
			Expect(response.Error).To(BeNil())
		}
	})

	It("should handle a request witout a specified ID", func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		resolver := init(ctx)
		defer cleanup()

		urlI, err := url.Parse("http://localhost/")
		Expect(err).ShouldNot(HaveOccurred())
		params := jsonrpc.ParamsQueryConfig{}
		req := http.Request{
			Method: http.MethodPost,
			URL:    urlI,
		}

		resp := resolver.QueryConfig(ctx, "", &params, &req)
		Expect(resp).Should(BeZero())
	})

	It("should fail when querying an unknown node", func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		resolver := init(ctx)
		defer cleanup()

		urlI, err := url.Parse("http://localhost/?id=123")
		Expect(err).ShouldNot(HaveOccurred())
		params := jsonrpc.ParamsQueryConfig{}
		req := http.Request{
			Method: http.MethodPost,
			URL:    urlI,
		}

		resp := resolver.QueryConfig(ctx, "", &params, &req)
		Expect(resp.Error.Message).Should(ContainSubstring("unknown darknode"))
	})

	It("should succeed when querying a known node", func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		resolver := init(ctx)
		defer cleanup()

		urlI, err := url.Parse("http://localhost/?id=localhost")
		Expect(err).ShouldNot(HaveOccurred())
		params := jsonrpc.ParamsQueryConfig{}
		req := http.Request{
			Method: http.MethodPost,
			URL:    urlI,
		}

		resp := resolver.QueryConfig(ctx, "", &params, &req)
		Expect(resp.Error).Should(BeZero())
	})

	It("should submit txs", func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		resolver := init(ctx)
		defer cleanup()

		r := rand.New(rand.NewSource(GinkgoRandomSeed()))

		params := jsonrpc.ParamsSubmitTx{
			Tx: txutil.RandomGoodTx(r),
		}

		resp := resolver.SubmitTx(ctx, nil, &params, nil)

		Expect(resp.Error).Should(BeZero())
	})
})
