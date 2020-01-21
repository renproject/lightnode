package dispatcher_test

import (
	"context"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/testutils"

	"github.com/renproject/darknode/addr"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/kv"
	"github.com/renproject/lightnode/dispatcher"
	"github.com/renproject/lightnode/http"
	"github.com/renproject/lightnode/store"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

func initDispatcher(ctx context.Context, bootstrapAddrs addr.MultiAddresses, timeout time.Duration) phi.Sender {
	opts := phi.Options{Cap: 10}
	logger := logrus.New()
	multiStore := store.New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses"))
	for _, addr := range bootstrapAddrs {
		Expect(multiStore.Insert(addr)).Should(Succeed())
	}
	dispatcher := dispatcher.New(logger, timeout, multiStore, opts)

	go dispatcher.Run(ctx)

	return dispatcher
}

func initDarknodes(n int) []*MockDarknode {
	dns := make([]*MockDarknode, n)
	for i := 0; i < n; i++ {
		server := httptest.NewServer(SimpleHandler(true))
		dns[i] = NewMockDarknode(server)
	}
	return dns
}

var _ = Describe("Dispatcher", func() {
	Context("When running", func() {
		It("Should send valid requests to the darknodes based on their policy", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			darknodes := initDarknodes(13)
			multis := make([]addr.MultiAddress, 13)
			for i := range multis {
				multis[i] = darknodes[i].Me
				defer darknodes[i].Close()
			}
			dispatcher := initDispatcher(ctx, multis, time.Second)

			for method, _ := range jsonrpc.RPCs {
				// TODO: This method is not supported right now, but when it is
				// this case should be tested too.
				if method == jsonrpc.MethodQueryEpoch {
					continue
				}

				req := http.NewRequestWithResponder(ctx, ValidRequest(method), "")
				Expect(dispatcher.Send(req)).To(BeTrue())

				var response jsonrpc.Response
				Eventually(req.Responder).Should(Receive(&response))
				Expect(response.Error).Should(BeNil())
			}
		})
	})
})
