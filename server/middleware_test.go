package server_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing/quick"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/server"
	. "github.com/renproject/lightnode/testutils"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Middleware", func() {

	init := func(middleware mux.MiddlewareFunc, handler http.Handler) *httptest.Server {
		router := mux.NewRouter()
		router.Handle("/", handler).Methods("GET")
		router.Use(middleware)
		httpHandler := cors.New(cors.Options{
			AllowedOrigins:   []string{"*"},
			AllowCredentials: true,
			AllowedMethods:   []string{"POST"},
		}).Handler(router)

		return httptest.NewServer(httpHandler)
	}

	Context("recovery middleware", func() {
		It("should recover panic and log it to the provided logger", func() {
			// Initialize the middleware
			logger := logrus.New()
			chanWriter, output := NewChanWriter()
			logger.SetOutput(chanWriter)
			rm := NewRecoveryMiddleware(logger)

			// Initialize the server
			server := init(rm, PanicHandler())
			defer server.Close()

			// Quick check
			test := func() bool {
				// Send a request
				resp, err := http.Get(server.URL)
				Expect(err).NotTo(HaveOccurred())

				// Expect the resp contains a error message about the panic
				msg, err := ioutil.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(strings.Contains(string(msg), "Recovered from a panic in the lightnode:"))

				// Expect the logger sends a error message about the panic
				var errLog string
				Eventually(output).Should(Receive(&errLog))
				return strings.Contains(errLog, "Recovered from a panic in the lightnode:")
			}

			Expect(quick.Check(test, nil)).NotTo(HaveOccurred())
		})
	})
})
