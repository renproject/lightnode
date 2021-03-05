package updater_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http/httptest"
	"time"

	_ "github.com/lib/pq"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/testutils"

	"github.com/renproject/darknode/addr"
	"github.com/renproject/lightnode/store"
	"github.com/renproject/lightnode/updater"
	"github.com/sirupsen/logrus"
)

func initUpdater(ctx context.Context, bootstrapAddrs addr.MultiAddresses, pollRate, timeout time.Duration) store.MultiAddrStore {
	logger := logrus.New()
	multiStore := NewStore(bootstrapAddrs)
	updater := updater.New(logger, multiStore, pollRate, timeout)

	go updater.Run(ctx)

	return multiStore
}

func initDarknodes(n int) []*MockDarknode {
	dns := make([]*MockDarknode, n)
	store := NewStore(nil)
	for i := 0; i < n; i++ {
		server := httptest.NewServer(RandomAddressHandler(store))
		dns[i] = NewMockDarknode(server, store)
	}
	return dns
}

var _ = Describe("Updater", func() {
	Context("When running", func() {
		FIt("should periodically query the darknodes", func() {
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
				size := updater.Size()
				return size
			}, 5*time.Second).Should(Equal(13))
		})
	})

	Context("when running against existing network", func() {
		It("should connect to most of the nodes", func() {
			addrStrs := []string{
				"/ip4/35.180.200.106/tcp/18514/ren/8MGaGCjCjrJMjp7kMrkKzxtmLpbX8q",
				"/ip4/18.221.96.210/tcp/18514/ren/8MKcWsSD8asdBzsGrFh7jShGL9QJR3",
				"/ip4/3.23.0.5/tcp/18514/ren/8MGC6fgodDrs3x97e8cACQ691Fme5Z",
				"/ip4/18.231.47.73/tcp/18514/ren/8MGDQn1LrKBbiqWU9q4Re3xSsbDAKS",
				"/ip4/15.223.97.146/tcp/18514/ren/8MJDZ8Dsg4jytEZxnmY4dQcmUqkUYN",
				"/ip4/3.13.16.10/tcp/18514/ren/8MJiy8CU3HvTWTTCAuhDEjHTe5dvib",
				"/ip4/13.238.180.76/tcp/18514/ren/8MK929isSkURtgwjZNsG31HNZYEyfx",
			}
			addrs := make([]addr.MultiAddress, len(addrStrs))
			for i, str := range addrStrs {
				multiAddr, err := addr.NewMultiAddressFromString(str)
				Expect(err).NotTo(HaveOccurred())
				addrs[i] = multiAddr
			}

			logger := logrus.New()
			source := "postgresql://postgres:postgres@localhost:5432/postgres?sslmode=disable"
			db, err := sql.Open("postgres", source)
			Expect(err).NotTo(HaveOccurred())
			store, err := store.New(db, addrs)
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				script := fmt.Sprintf("DROP TABLE %v;", "addresses")
				_, err := db.Exec(script)
				Expect(err).NotTo(HaveOccurred())
			}()

			updater := updater.New(logger, store, 10*time.Second, 5*time.Second)
			go updater.Run(context.Background())

			time.Sleep(30 * time.Second)

			size := store.Size()
			Expect(size).Should(BeNumerically(">", 1000))
		})
	})
})
