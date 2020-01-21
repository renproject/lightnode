package lightnode_test

// import (
// 	"context"
// 	"database/sql"
// 	"time"
//
// 	. "github.com/onsi/ginkgo"
// 	. "github.com/onsi/gomega"
//
// 	_ "github.com/mattn/go-sqlite3"
//
// 	"github.com/google/go-cmp/cmp"
// 	"github.com/renproject/darknode"
// 	"github.com/renproject/darknode/addr"
// 	"github.com/renproject/darknode/jsonrpc"
// 	"github.com/renproject/darknode/testutil"
// 	"github.com/renproject/lightnode"
// 	"github.com/renproject/lightnode/testutils"
// 	"github.com/sirupsen/logrus"
// )
//
// var _ = Describe("Lightnode", func() {
// 	Context("when sending requests to lightnode", func() {
// 		Context("when submitting a tx", func() {
// 			It("todo", func() {
//
// 				Expect(true).Should(BeTrue())
// 			})
//
// 		})
//
// 		Context("when sending a query request to lightnode", func() {
// 			FIt("should forward the request to darknode", func() {
// 				ctx, cancel := context.WithCancel(context.Background())
// 				defer cancel()
//
// 				ln, requests := New(ctx)
// 				go ln.Run(ctx)
//
// 				for _, method := range testutils.QueryRequests {
// 					req := testutils.RandomRequest(method)
// 					_, err := client.SendToDarknode("http://0.0.0.0:8000", req, 5*time.Second)
// 					Expect(err).NotTo(HaveOccurred())
//
// 					var dnReq jsonrpc.Request
// 					Eventually(requests).Should(Receive(&dnReq))
// 					Expect(cmp.Equal(req, dnReq))
// 				}
// 			})
// 		})
// 	})
// })
//
// func options(addrs addr.MultiAddresses) lightnode.Options {
// 	key, err := testutil.RandomEcdsaKey()
// 	if err != nil {
// 		panic(err)
// 	}
// 	return lightnode.Options{
// 		Network:           darknode.Devnet,
// 		DisPubkey:         &key.PublicKey,
// 		Port:              "8000",
// 		Cap:               0,
// 		MaxBatchSize:      0,
// 		ServerTimeout:     0,
// 		Timeout:           0,
// 		TTL:               0,
// 		UpdaterPollRate:   0,
// 		ConfirmerPollRate: 0,
// 		BootstrapAddrs:    addrs,
// 	}
// }
//
// func New(ctx context.Context) (lightnode.Lightnode, chan jsonrpc.Request) {
// 	requests := make(chan jsonrpc.Request, 128)
// 	darknode := testutils.NewMockDarknode(7000, requests)
// 	go darknode.Run(ctx)
// 	opts := options(addr.MultiAddresses{darknode.Me})
// 	opts.SetZeroToDefault()
// 	logger := logrus.New()
// 	sqlDB, err := sql.Open("sqlite3", "./test.db")
// 	if err != nil {
// 		panic(err)
// 	}
// 	return lightnode.New(ctx, opts, logger, sqlDB), requests
// }
