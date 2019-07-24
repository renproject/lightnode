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

var _ = Describe("Lightnode server", func() {
	IP4 := "0.0.0.0"
	PORT := "5000"

	initServer := func(ctx context.Context) <-chan phi.Message {
		logger := logrus.New()
		options := server.Options{MaxBatchSize: 2}
		inspector, messages := testutils.NewInspector(100)
		server := server.New(logger, PORT, options, inspector)

		go inspector.Run(ctx)
		go server.Run()

		return messages
	}

	sendRequest := func(request jsonrpc.Request) {
		timeout := time.Second
		url := fmt.Sprintf("http://%s:%s", IP4, PORT)
		fmt.Printf("sending request to %s\n", url)
		client.SendToDarknode(url, request, timeout)
	}

	Context("When Running a server", func() {
		It("Should pass a valid message through", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			messages := initServer(ctx)
			request := jsonrpc.Request{
				Version: "2.0",
				Method:  jsonrpc.MethodSubmitTx,
			}

			time.Sleep(50 * time.Millisecond)
			sendRequest(request)
			select {
			case <-time.After(time.Second):
				Fail("timeout")
			case message := <-messages:
				req, ok := message.(server.RequestWithResponder)
				Expect(ok).To(BeTrue())
				Expect(req.Request).To(Equal(request))
				Expect(req.Responder).To(Not(BeNil()))
			}
		})
	})
})
