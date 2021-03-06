package updater_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/jsonrpc/jsonrpcresolver"
	. "github.com/renproject/lightnode/testutils"

	"github.com/renproject/aw/wire"
	"github.com/renproject/kv"
	"github.com/renproject/lightnode/store"
	"github.com/renproject/lightnode/updater"
	"github.com/sirupsen/logrus"
)

func initUpdater(ctx context.Context, bootstrapAddrs []wire.Address, pollRate, timeout time.Duration) store.MultiAddrStore {
	logger := logrus.New()
	multiStore := store.New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses"), bootstrapAddrs)
	for _, addr := range bootstrapAddrs {
		multiStore.Insert(addr)
	}
	updater := updater.New(logger, multiStore, pollRate, timeout)

	go updater.Run(ctx)

	return multiStore
}

func initDarknodes(ctx context.Context, n int) []*MockDarknode {
	dns := make([]*MockDarknode, n)
	store := store.New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses"), nil)
	for i := 0; i < n; i++ {
		server := jsonrpc.NewServer(jsonrpc.DefaultOptions(), &jsonrpcresolver.Callbacks{
			QueryPeersHandler: func(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryPeers, r *http.Request) jsonrpc.Response {
				addrs, err := store.RandomAddrs(5)
				if err != nil {
					panic(err)
				}
				addrsString := make([]string, len(addrs))
				for i := range addrsString {
					addrsString[i] = addrs[i].String()
				}
				queryRes := jsonrpc.ResponseQueryPeers{
					Peers: addrsString,
				}
				return jsonrpc.Response{
					Version: "2.0",
					ID:      0,
					Result:  queryRes,
				}
			},
		}, jsonrpc.NewValidator())
		url := fmt.Sprintf("0.0.0.0:%v", 4444+i)
		go server.Listen(ctx, url)

		dns[i] = NewMockDarknode(url, store)
	}
	return dns
}

var _ = Describe("Updater", func() {
	Context("When running", func() {
		It("Should periodically query the darknodes", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			darknodes := initDarknodes(ctx, 13)
			multis := make([]wire.Address, 13)
			for i := range multis {
				multis[i] = darknodes[i].Me
			}
			updater := initUpdater(ctx, multis[:4], 100*time.Millisecond, time.Second)
			Eventually(func() int {
				size, err := updater.Size()
				Expect(err).ShouldNot(HaveOccurred())
				return size
			}, 5*time.Second).Should(Equal(13))
		})
	})
})
