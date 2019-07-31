package client_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/renproject/darknode/addr"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/lightnode/client"
	"github.com/renproject/lightnode/testutils"
)

func initDN() {
	dn := testutils.NewMockDarknode(5000, addr.MultiAddresses{})

	init := dn.Run()
	<-init
}

var _ = Describe("Client", func() {
	Context("When sending valid requests", func() {
		It("Should receive valid responses", func() {
			initDN()

			req := testutils.ValidRequest(jsonrpc.MethodQueryPeers)
			_, err := client.SendToDarknode("http://0.0.0.0:5000", req, time.Second)

			Expect(err).To(BeNil())
		})
	})
})
