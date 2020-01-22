package lightnode_test

import (
	"context"
	"database/sql"
	"log"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/testutils"

	_ "github.com/mattn/go-sqlite3"

	"github.com/renproject/darknode"
	"github.com/renproject/darknode/addr"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/testutil"
	"github.com/renproject/kv"
	"github.com/renproject/lightnode"
	"github.com/renproject/lightnode/http"
	"github.com/renproject/lightnode/store"
	"github.com/renproject/lightnode/testutils"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Lightnode", func() {
	Context("when sending requests to lightnode", func() {
		Context("when submitting a tx", func() {
			It("todo", func() {

				Expect(true).Should(BeTrue())
			})

		})

		Context("when sending a query request to lightnode", func() {
			FIt("should forward the request to darknode", func() {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				ln, _ := New(ctx, 10, 3)
				go ln.Run(ctx)
				time.Sleep(time.Second)

				for _, method := range testutils.QueryRequests {
					req := testutils.RandomRequest(method)
					client := http.NewClient(time.Second)
					response, err := client.SendRequest(ctx, "http://0.0.0.0:8000", req, nil)
					Expect(err).NotTo(HaveOccurred())
					Expect(response.Error).Should(BeNil())
				}
			})
		})
	})
})

func initOptions(addrs addr.MultiAddresses) lightnode.Options {
	key, err := testutil.RandomEcdsaKey()
	if err != nil {
		panic(err)
	}
	return lightnode.Options{
		Network:           darknode.Devnet,
		DisPubkey:         &key.PublicKey,
		Port:              "8000",
		BtcShifterAddr:    "0x0000000000000000000000000000000000000000",
		ZecShifterAddr:    "0x0000000000000000000000000000000000000000",
		BchShifterAddr:    "0x0000000000000000000000000000000000000000",
		Cap:               0,
		MaxBatchSize:      0,
		ServerTimeout:     0,
		ClientTimeout:     0,
		TTL:               0,
		UpdaterPollRate:   0,
		ConfirmerPollRate: 0,
		BootstrapAddrs:    addrs,
	}
}

func New(ctx context.Context, good, bad int) (lightnode.Lightnode, chan jsonrpc.Request) {
	requests := make(chan jsonrpc.Request, 128)
	darknodes := InitDarknodes(good, bad, requests)

	// Assume lightnode has connected to all darknodes.
	bootstrap, err := darknodes[0].Store.AddrsRandom(good + bad)
	Expect(err).NotTo(HaveOccurred())
	opts := initOptions(bootstrap)
	opts.SetZeroToDefault()
	logger := logrus.New()
	sqlDB, err := sql.Open("sqlite3", "./test.db")
	if err != nil {
		panic(err)
	}
	return lightnode.New(ctx, opts, logger, sqlDB), requests
}

func InitDarknodes(good, bad int, requests chan jsonrpc.Request) []*MockDarknode {
	store := store.New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "multi"))
	nodes := make([]*MockDarknode, good+bad)
	for i := 0; i < good+bad; i++ {
		handler := SimpleHandler(false, requests)
		if i < good {
			handler = SimpleHandler(true, requests)
		}
		nodes[i] = NewMockDarknode(httptest.NewServer(handler), store)
	}

	return nodes
}
