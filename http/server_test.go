package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
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

// Address and Port of the server used in the testing.
const (
	Port = "5000"
)

var _ = Describe("Lightnode server", func() {

	init := func(ctx context.Context, port string) <-chan phi.Message {
		logger := logrus.New()
		inspector, messages := NewInspector(128)
		go inspector.Run(ctx)

		options := Options{
			Port:         port,
			MaxBatchSize: 10,
			Timeout:      3 * time.Second,
		}
		server := New(logger, options, inspector)
		go server.Listen(ctx)
		time.Sleep(time.Second)

		return messages
	}

	Context("when initializing server options", func() {
		It("should panic if the port field is not set.", func() {
			options := Options{
				Port: "",
			}
			Expect(func() {
				options.SetZeroToDefault()
			}).Should(Panic())
		})

		It("should set zero values to default", func() {
			options := Options{
				Port: "12345",
			}
			options.SetZeroToDefault()
			Expect(options.MaxBatchSize).Should(Equal(10))
			Expect(options.Timeout).Should(Equal(15 * time.Second))
		})
	})

	Context("when running a server", func() {
		It("should return health status from the `/health` endpoint", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			_ = init(ctx, Port)

			response, err := http.Get(fmt.Sprintf("http://0.0.0.0:%v/health", Port))
			Expect(err).NotTo(HaveOccurred())
			data, err := ioutil.ReadAll(response.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(data)).Should(BeZero())
		})

		It("should pass a valid message to validator", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			messages := init(ctx, Port)

			test := func() bool {
				// Send a valid request to server
				request := ValidRequest(jsonrpc.MethodSubmitTx)
				url := fmt.Sprintf("http://0.0.0.0:%s", Port)
				respChan, err := SendRequestAsync(request, url)
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

			Expect(quick.Check(test, nil)).NotTo(HaveOccurred())
		})
	})

	Context("when sending invalid request to server", func() {
		It("should return error for invalid JSON request", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			init(ctx, Port) // ignore messages sent to validator

			test := func(data []byte) bool {
				url := fmt.Sprintf("http://0.0.0.0:%s", Port)
				response, err := http.Post(url, "application/json", nil)
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
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			init(ctx, Port) // ignore messages sent to validator

			test := func(data []byte) bool {
				url := fmt.Sprintf("http://0.0.0.0:%s", Port)
				buffer := bytes.NewBuffer([]byte{})
				err := json.NewEncoder(buffer).Encode(BatchRequest(20))
				Expect(err).NotTo(HaveOccurred())

				response, err := http.Post(url, "application/json", buffer)
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
