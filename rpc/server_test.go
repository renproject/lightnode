package rpc_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/rpc"

	"github.com/republicprotocol/darknode-go/server/jsonrpc"
	"github.com/sirupsen/logrus"
)

var _ = Describe("rpc server task", func() {
	Context("when receiving a new request", func() {
		It("should forward the SendMessageRequest to its parent task", func() {
			// Initialise the server.
			logger := logrus.New()
			inputs := make(chan jsonrpc.Request)
			server := NewServer(logger, 128, inputs)
			done := make(chan struct{})
			defer close(done)
			go server.Run(done)

			// Simulate the server receiving an Accept message.
			server.IO().InputWriter() <- Accept{}
			reqIn := jsonrpc.SendMessageRequest{
				Nonce:   1,
				To:      "to",
				Payload: jsonrpc.Payload{},
			}
			inputs <- reqIn

			// Simulate reading messages as the parent task and validating the message.
			request := <-server.IO().OutputReader()
			message, ok := request.(SendMessage)
			Expect(ok).Should(BeTrue())
			reqOut, ok := message.Request.(jsonrpc.SendMessageRequest)
			Expect(ok).Should(BeTrue())
			Expect(reqIn.To).To(Equal(reqOut.To))
			Expect(reqIn.Nonce).To(Equal(reqOut.Nonce))
		})

		It("should forward the ReceiveMessageRequest to its parent task", func() {
			// Initialise the server.
			logger := logrus.New()
			inputs := make(chan jsonrpc.Request)
			server := NewServer(logger, 128, inputs)
			done := make(chan struct{})
			defer close(done)
			go server.Run(done)

			// Simulate the server receiving an Accept message.
			server.IO().InputWriter() <- Accept{}
			reqIn := jsonrpc.ReceiveMessageRequest{
				MessageID: "1234567890",
			}
			inputs <- reqIn

			// Simulate reading messages as the parent task and validating the message.
			request := <-server.IO().OutputReader()
			message, ok := request.(SendMessage)
			Expect(ok).Should(BeTrue())
			reqOut, ok := message.Request.(jsonrpc.ReceiveMessageRequest)
			Expect(ok).Should(BeTrue())
			Expect(reqIn.MessageID).To(Equal(reqOut.MessageID))
		})

		It("should forward the QueryPeersRequest to it's parent task", func() {
			// Initialise the server.
			logger := logrus.New()
			inputs := make(chan jsonrpc.Request)
			server := NewServer(logger, 128, inputs)
			done := make(chan struct{})
			defer close(done)
			go server.Run(done)

			// Simulate the server receiving an Accept message.
			server.IO().InputWriter() <- Accept{}
			req := jsonrpc.QueryPeersRequest{}
			inputs <- req

			// Simulate reading messages as the parent task and validating the message.
			request := <-server.IO().OutputReader()
			message, ok := request.(QueryMessage)
			Expect(ok).Should(BeTrue())
			_, ok = message.Request.(jsonrpc.QueryPeersRequest)
			Expect(ok).Should(BeTrue())
		})

		It("should forward the QueryNumPeersRequest to it's parent task", func() {
			// Initialise the server.
			logger := logrus.New()
			inputs := make(chan jsonrpc.Request)
			server := NewServer(logger, 128, inputs)
			done := make(chan struct{})
			defer close(done)
			go server.Run(done)

			// Simulate the server receiving an Accept message.
			server.IO().InputWriter() <- Accept{}
			req := jsonrpc.QueryNumPeersRequest{}
			inputs <- req

			// Simulate reading messages as the parent task and validating the message.
			request := <-server.IO().OutputReader()
			message, ok := request.(QueryMessage)
			Expect(ok).Should(BeTrue())
			_, ok = message.Request.(jsonrpc.QueryNumPeersRequest)
			Expect(ok).Should(BeTrue())
		})
	})
})
