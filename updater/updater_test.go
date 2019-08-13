package updater_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/renproject/darknode/addr"
	"github.com/renproject/kv"
	"github.com/renproject/lightnode/store"
	"github.com/renproject/lightnode/testutils"
	"github.com/renproject/lightnode/updater"
	"github.com/sirupsen/logrus"
)

func initUpdater(ctx context.Context, bootstrapAddrs addr.MultiAddresses, pollRate, timeout time.Duration) store.MultiAddrStore {
	logger := logrus.New()
	multiStore := store.New(kv.NewMemDB())
	for _, addr := range bootstrapAddrs {
		multiStore.Insert(addr)
	}
	updater := updater.New(logger, bootstrapAddrs, multiStore, pollRate, timeout)

	go updater.Run(ctx)

	return multiStore
}

func initDNs(n int) {
	dns := make([]testutils.MockDarknode, n)
	for i := 0; i < n; i++ {
		neighbour := testutils.NewMultiFromIPAndPort("0.0.0.0", 5000+((2*(i+1))%(2*n)))
		dns[i] = testutils.NewMockDarknode(5000+2*i+1, addr.MultiAddresses{neighbour})
		go dns[i].Run()
	}
}

var _ = Describe("Updater", func() {
	Context("When running", func() {
		It("Should periodically query the darknodes", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			numDNs := 6
			numBootstrap := numDNs / 2
			pollRate, timeout := 100*time.Millisecond, time.Second
			bootstrapAddrs := make(addr.MultiAddresses, numBootstrap)

			for i := 0; i < numBootstrap; i++ {
				bootstrapAddrs[i] = testutils.NewMultiFromIPAndPort("0.0.0.0", 5000+4*i)
			}

			initDNs(numDNs)
			multiStore := initUpdater(ctx, bootstrapAddrs, pollRate, timeout)

			Eventually(func() int { size, err := multiStore.Size(); Expect(err).ShouldNot(HaveOccurred()); return size }).Should(Equal(6))
		})
	})
})
