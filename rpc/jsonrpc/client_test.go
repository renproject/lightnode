package jsonrpc_test

import (
	"github.com/republicprotocol/dcc/jsonrpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/republicprotocol/lightnode/rpc/jsonrpc"
)

var _ = Describe("JSON-RPC Client", func() {
	var client Client

	BeforeEach(func() {
		// Set up a client.
		client = NewClient()
	})

	Context("when sending valid requests", func() {
		It("should return a response", func() {
			// Construct request.
			request := jsonrpc.JSONRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  jsonrpc.MethodSendMessage,
			}
			response, err := client.Invoke(request)
			Expect(err).ToNot(HaveOccurred())

			// TODO: Validate response
			Expect(response.ID).To(Equal(1))
			Expect(response.Error).To(BeNil())
		})
	})
})
