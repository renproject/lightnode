package updater_test

import (
	"context"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/testutils"

	"github.com/renproject/darknode/addr"
	"github.com/renproject/kv"
	"github.com/renproject/lightnode/store"
	"github.com/renproject/lightnode/updater"
	"github.com/sirupsen/logrus"
)

func initUpdater(ctx context.Context, bootstrapAddrs addr.MultiAddresses, pollRate, timeout time.Duration) store.MultiAddrStore {
	logger := logrus.New()
	multiStore := store.New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses"))
	for _, addr := range bootstrapAddrs {
		multiStore.Insert(addr)
	}
	updater := updater.New(logger, bootstrapAddrs, multiStore, pollRate, timeout)

	go updater.Run(ctx)

	return multiStore
}

func initDarknodes(n int) []*MockDarknode {
	dns := make([]*MockDarknode, n)
	store := store.New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses"))
	for i := 0; i < n; i++ {
		server := httptest.NewServer(RandomAddressHandler(store))
		dns[i] = NewMockDarknode(server)
		Expect(store.Insert(dns[i].Me)).Should(Succeed())
	}
	return dns
}

var _ = Describe("Updater", func() {
	Context("When running", func() {
		It("Should periodically query the darknodes", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			darknodes := initDarknodes(13)
			multis := make([]addr.MultiAddress, 13)
			for i := range multis {
				multis[i] = darknodes[i].Me
				defer darknodes[i].Close()
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
