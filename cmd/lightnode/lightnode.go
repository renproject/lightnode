package main

import (
	"fmt"
	"io/ioutil"
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

	p := proxy{
		url:    os.Getenv("DARKNODE_URL"),
		logger: logger,
	}
	port := os.Getenv("PORT")

	r := mux.NewRouter()
	r.HandleFunc("/", p.handler()).Methods("POST")
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

func (proxy *proxy) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := http.Post(proxy.url, "application/json", r.Body)
		if err != nil {
			proxy.writeError(w, r, resp.StatusCode, err)
			return
		}
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			proxy.writeError(w, r, resp.StatusCode, err)
			return
		}
		w.WriteHeader(resp.StatusCode)
		w.Write(data)
	}
}

func (proxy *proxy) writeError(w http.ResponseWriter, r *http.Request, statusCode int, err error) {
	if statusCode >= 500 {
		proxy.logger.Errorf("failed to call %s with error %v", r.URL.String(), err)
	}
	if statusCode >= 400 {
		proxy.logger.Warningf("failed to call %s with error %v", r.URL.String(), err)
	}
	http.Error(w, fmt.Sprintf("{ \"error\": \"%s\" }", err), statusCode)
}
