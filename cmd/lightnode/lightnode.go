package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/evalphobia/logrus_sentry"
	"github.com/getsentry/raven-go"
	"github.com/renproject/lightnode"
	"github.com/sirupsen/logrus"
)

func main() {
	// Getting params from environment variables.
	port := os.Getenv("PORT")
	sentryToken := os.Getenv("SENTRY_TOKEN")
	cap, err := strconv.Atoi(os.Getenv("CAP"))
	if err != nil {
		cap = 128
	}
	workers, err := strconv.Atoi(os.Getenv("WORKERS"))
	if err != nil {
		workers = 16
	}
	timeout, err := strconv.Atoi(os.Getenv("TIMEOUT"))
	if err != nil {
		timeout = 60
	}
	addresses := strings.Split(os.Getenv("ADDRESSES"), ",")

	// Setup the logger and hook it to sentry
	logger := logrus.New()
	client, err := raven.New(fmt.Sprintf("https://%v@sentry.io/1286737", sentryToken))
	if err != nil {
		logrus.Fatalf("cannot connect to sentry: %v", err)
	}
	hook, err := logrus_sentry.NewWithClientSentryHook(client, []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
	})
	if err != nil {
		logrus.Fatalf("cannot create a sentry hook: %v", err)
	}
	logger.AddHook(hook)

	// Start running the lightnode
	done := make(chan struct{})
	node := lightnode.NewLightnode(logger, cap, workers, timeout, port, addresses)
	node.Run(done)
}