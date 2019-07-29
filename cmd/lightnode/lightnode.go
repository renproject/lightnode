package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/evalphobia/logrus_sentry"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
)

func main() {
	// Setup logger and attach Sentry hook.
	logger := logrus.New()
	hook, err := logrus_sentry.NewSentryHook(os.Getenv("SENTRY_DSN"), []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
	})
	if err != nil {
		logger.Fatalf("cannot create a sentry hook: %v", err)
	}
	hook.Timeout = 500 * time.Millisecond
	logger.AddHook(hook)

	p := &proxy{
		url:    os.Getenv("DARKNODE_URL"),
		logger: logger,
	}
	port := os.Getenv("PORT")

	r := mux.NewRouter()
	r.HandleFunc("/", p.handler()).Methods("POST")
	r.Use(p.recoveryHandler)
	handler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
		AllowedMethods:   []string{"POST"},
	}).Handler(r)
	log.Printf("Listening on port %v...", port)
	http.ListenAndServe(fmt.Sprintf(":%v", port), handler)
}

type proxy struct {
	url    string
	logger logrus.FieldLogger
}

// Error defines a JSON error object that is compatible with the JSON-RPC 2.0
// specification. See https://www.jsonrpc.org/specification for more
// information.
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (proxy *proxy) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp, err := http.Post(proxy.url, "application/json", r.Body)
		if err != nil {
			proxy.writeError(w, r, http.StatusInternalServerError, Error{Code: -32097, Message: fmt.Sprintf("failed to talk to the darknode at %s: %v", proxy.url, err)})
			return
		}
		w.WriteHeader(resp.StatusCode)
		if _, err := io.Copy(w, resp.Body); err != nil {
			proxy.writeError(w, r, http.StatusInternalServerError, Error{Code: -32098, Message: fmt.Sprintf("failed to copy the response from the darknode at %s: %v", proxy.url, err)})
			return
		}
	}
}

func (proxy *proxy) writeError(w http.ResponseWriter, r *http.Request, statusCode int, err Error) {
	if statusCode >= 500 {
		proxy.logger.Errorf("failed to call %s with error %v", r.URL.String(), err)
	} else if statusCode >= 400 {
		proxy.logger.Warningf("failed to call %s with error %v", r.URL.String(), err)
	}
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(err); err != nil {
		proxy.logger.Errorf("failed to send an error back: %v", r.URL.String(), err)
	}
}

func (proxy *proxy) recoveryHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				proxy.writeError(
					w,
					r,
					http.StatusInternalServerError,
					Error{
						Code:    -32099,
						Message: fmt.Sprintf("recovered from a panic in the lightnode: %v", err),
					},
				)
			}
		}()
		h.ServeHTTP(w, r)
	})
}
