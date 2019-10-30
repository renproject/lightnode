package server_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/lightnode/client"
	"github.com/renproject/lightnode/server"
	"github.com/renproject/lightnode/testutils"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

var IP4 = "0.0.0.0"
var PORT = "5000"

func initServer(ctx context.Context) <-chan phi.Message {
	logger := logrus.New()
	options := server.Options{MaxBatchSize: 2}
	inspector, messages := testutils.NewInspector(100)
	server := server.New(logger, PORT, options, inspector)

	go inspector.Run(ctx)
	go server.Run()

	waitForInit(messages)
	return messages
}

func sendRequest(request jsonrpc.Request) {
	timeout := time.Second
	url := fmt.Sprintf("http://%s:%s", IP4, PORT)
	client.SendToDarknode(url, request, timeout)
}

func waitForInit(messages <-chan phi.Message) {
	for {
		request := testutils.ValidRequest(jsonrpc.MethodSubmitTx)
		sendRequest(request)

		select {
		case <-time.After(10 * time.Millisecond):
			continue
		case <-messages:
			return
		}
	}
}

var _ = Describe("Lightnode server", func() {
	Context("When Running a server", func() {
		It("Should pass a valid message through", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			messages := initServer(ctx)
			request := testutils.ValidRequest(jsonrpc.MethodSubmitTx)

			sendRequest(request)
			select {
			case <-time.After(time.Second):
				Fail("timeout")
			case message := <-messages:
				req, ok := message.(server.RequestWithResponder)
				Expect(ok).To(BeTrue())
				Expect(req.Request).To(Equal(request))
				Expect(req.Responder).To(Not(BeNil()))

				req.Responder <- testutils.ErrorResponse(req.Request.ID)
			}
		})
	})
})
