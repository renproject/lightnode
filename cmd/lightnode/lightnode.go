package main

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/evalphobia/logrus_sentry"
	"github.com/getsentry/raven-go"
	"github.com/renproject/lightnode"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/sirupsen/logrus"
)

func main() {
	// Retrieve environment variables.
	port := os.Getenv("PORT")
	sentryURL := os.Getenv("SENTRY_URL")
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

	// Setup logger and attach Sentry hook.
	logger := logrus.New()
	client, err := raven.New(sentryURL)
	if err != nil {
		logger.Fatalf("cannot connect to sentry: %v", err)
	}
	hook, err := logrus_sentry.NewWithClientSentryHook(client, []logrus.Level{
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

	bootstrapAddrs := make([]peer.MultiAddr, len(addresses))
	for i := range addresses {
		multiAddr, err := peer.NewMultiAddr(addresses[i], 0, [65]byte{})
		if err != nil {
			logger.Fatalf("invalid bootstrap addresses: %v", err)
		}
		bootstrapAddrs[i] = multiAddr
	}

	// Start running Lightnode.
	done := make(chan struct{})
	node := lightnode.NewLightnode(logger, cap, workers, timeout, port, bootstrapAddrs)
	node.Run(done)
}
