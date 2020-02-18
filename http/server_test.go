package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing/quick"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/http"
	. "github.com/renproject/lightnode/testutils"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Lightnode server", func() {

	init := func()(*Server, <-chan phi.Message) {
		options := Options{
			MaxBatchSize: 10,
			Timeout:      3 * time.Second,
		}
		sender := NewMockSender()
		server := New(logrus.New(), options, sender)
		return server, sender.Messages
	}

	Context("when initializing server options", func() {
		It("should set zero values to default", func() {
			options := Options{}
			options.SetZeroToDefault()
			Expect(options.MaxBatchSize).Should(Equal(10))
			Expect(options.Timeout).Should(Equal(15 * time.Second))
		})
	})

	Context("when running a server", func() {
		It("should pass a valid message to validator", func() {
			server, messages := init()
			httpServer := httptest.NewServer(http.HandlerFunc(server.Handle))
			defer httpServer.Close()

			test := func() bool {
				// Send a valid request to server
				request := ValidRequest(jsonrpc.MethodSubmitTx)
				respChan, err := SendRequestAsync(request, httpServer.URL)
				Expect(err).NotTo(HaveOccurred())

				// Expect the request pass all the check and is sent to validator.
				var req RequestWithResponder
				Eventually(messages).Should(Receive(&req))
				Expect(req.Request).To(Equal(request))
				Expect(req.Responder).To(Not(BeNil()))

				// Simulate a response been sent through the responder channel.
				response := ErrorResponse(req.Request.ID)
				req.Responder <- response
				var resp *jsonrpc.Response
				Eventually(respChan).Should(Receive(&resp))
				return cmp.Equal(*response.Error, *resp.Error, cmpopts.EquateEmpty())
			}

			Expect(quick.Check(test, &quick.Config{MaxCount: 50})).NotTo(HaveOccurred())
		})
	})

	Context("when sending invalid request to server", func() {
		It("should return error for invalid JSON request", func() {
			server, _ := init()
			httpServer := httptest.NewServer(http.HandlerFunc(server.Handle))
			defer httpServer.Close()

			test := func(data []byte) bool {
				response, err := http.Post(httpServer.URL, "application/json", nil)
				Expect(err).NotTo(HaveOccurred())

				var resp jsonrpc.Response
				err = json.NewDecoder(response.Body).Decode(&resp)
				Expect(err).NotTo(HaveOccurred())

				Expect(resp.Error).ShouldNot(BeNil())
				Expect(resp.Error.Code).Should(Equal(jsonrpc.ErrorCodeInvalidJSON))
				Expect(resp.Error.Message).Should(Equal("lightnode could not decode JSON request"))
				return true
			}

			Expect(quick.Check(test, nil)).NotTo(HaveOccurred())
		})

		It("should return error when exceeding max batch size", func() {
			server, _ := init()
			httpServer := httptest.NewServer(http.HandlerFunc(server.Handle))
			defer httpServer.Close()

			test := func(data []byte) bool {
				buffer := bytes.NewBuffer([]byte{})
				err := json.NewEncoder(buffer).Encode(BatchRequest(20))
				Expect(err).NotTo(HaveOccurred())

				response, err := http.Post(httpServer.URL, "application/json", buffer)
				Expect(err).NotTo(HaveOccurred())

				var resp jsonrpc.Response
				err = json.NewDecoder(response.Body).Decode(&resp)
				Expect(err).NotTo(HaveOccurred())

				Expect(resp.Error).ShouldNot(BeNil())
				Expect(resp.Error.Code).Should(Equal(ErrorCodeMaxBatchSizeExceeded))
				return strings.Contains(resp.Error.Message, "maximum batch size exceeded")
			}

			Expect(quick.Check(test, nil)).NotTo(HaveOccurred())
		})
	})
})
