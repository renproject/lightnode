package dispatcher_test

import (
	"context"
	"fmt"
	"net/url"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/testutils"

	"github.com/renproject/aw/wire"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/jsonrpc/jsonrpcresolver"
	"github.com/renproject/kv"
	"github.com/renproject/lightnode/dispatcher"
	"github.com/renproject/lightnode/http"
	"github.com/renproject/lightnode/store"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

func initDispatcher(ctx context.Context, bootstrapAddrs []wire.Address, timeout time.Duration) phi.Sender {
	opts := phi.Options{Cap: 10}
	logger := logrus.New()
	table := kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses")
	multiStore := store.New(table, bootstrapAddrs)
	dispatcher := dispatcher.New(logger, timeout, multiStore, opts)

	go dispatcher.Run(ctx)

	return dispatcher
}

func initDarknodes(ctx context.Context, n int) []*MockDarknode {
	dns := make([]*MockDarknode, n)
	store := store.New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "multi"), nil)
	for i := 0; i < n; i++ {
		server := jsonrpc.NewServer(jsonrpc.DefaultOptions(), jsonrpcresolver.OkResponder(), jsonrpc.NewValidator())
		url := fmt.Sprintf("0.0.0.0:%v", 3333+i)
		go server.Listen(ctx, url)

		dns[i] = NewMockDarknode(url, store)
	}
	return dns
}

var _ = Describe("Dispatcher", func() {
	Context("When running", func() {
		It("Should send valid requests to the darknodes based on their policy", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			darknodes := initDarknodes(ctx, 13)
			multis := make([]wire.Address, 13)
			for i := range multis {
				multis[i] = darknodes[i].Me
			}
			dispatcher := initDispatcher(ctx, multis, time.Second)

			for method := range jsonrpc.RPCs {
				id, params := ValidRequest(method)
				req := http.NewRequestWithResponder(ctx, id, method, params, url.Values{})
				Expect(dispatcher.Send(req)).To(BeTrue())

				var response jsonrpc.Response
				Eventually(req.Responder).Should(Receive(&response))
				Expect(response.Error).Should(BeNil())
			}
		})
	})
})
