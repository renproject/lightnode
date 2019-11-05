package dispatcher_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/renproject/darknode/addr"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/kv"
	"github.com/renproject/lightnode/dispatcher"
	"github.com/renproject/lightnode/server"
	"github.com/renproject/lightnode/store"
	"github.com/renproject/lightnode/testutils"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

func initDispatcher(ctx context.Context, bootstrapAddrs addr.MultiAddresses, timeout time.Duration) phi.Sender {
	opts := phi.Options{Cap: 10}
	logger := logrus.New()
	multiStore := store.New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses"), bootstrapAddrs[0])
	for _, addr := range bootstrapAddrs {
		multiStore.Insert(addr)
	}
	dispatcher := dispatcher.New(logger, timeout, multiStore, opts)

	go dispatcher.Run(ctx)

	return dispatcher
}

func initDNs(n int) {
	dns := make([]testutils.MockDarknode, n)
	for i := 0; i < n; i++ {
		neighbour := testutils.NewMultiFromIPAndPort("0.0.0.0", 5000+((2*(i+1))%(2*n)))
		dns[i] = testutils.NewMockDarknode(5000+2*i+1, addr.MultiAddresses{neighbour})
		go dns[i].Run()
	}
}

var _ = Describe("Dispatcher", func() {
	Context("When running", func() {
		It("Should send valid requests to the darknodes based on their policy", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			numDNs := 6
			timeout := time.Second
			bootstrapAddrs := make(addr.MultiAddresses, numDNs)

			for i := 0; i < numDNs; i++ {
				bootstrapAddrs[i] = testutils.NewMultiFromIPAndPort("0.0.0.0", 5000+4*i)
			}

			initDNs(numDNs)
			dispatcher := initDispatcher(ctx, bootstrapAddrs, timeout)

			for method, _ := range jsonrpc.RPCs {
				// TODO: This method is not supported right now, but when it is
				// this case should be tested too.
				if method == jsonrpc.MethodQueryEpoch {
					continue
				}

				req := server.NewRequestWithResponder(testutils.ValidRequest(method), "")
				ok := dispatcher.Send(req)
				Expect(ok).To(BeTrue())

				Eventually(req.Responder, timeout*2).Should(Receive())
			}
		})
	})
})
