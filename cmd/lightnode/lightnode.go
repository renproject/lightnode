package main

import (
	"context"
	"database/sql"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/evalphobia/logrus_sentry"
	_ "github.com/lib/pq"
	"github.com/renproject/darknode"
	"github.com/renproject/darknode/addr"
	"github.com/renproject/lightnode"
	"github.com/renproject/lightnode/db"
	"github.com/sirupsen/logrus"
)

func main() {
	// Retrieve environment variables.
	port := os.Getenv("PORT")
	name := os.Getenv("HEROKU_APP_NAME")
	sentryURL := os.Getenv("SENTRY_URL")
	cap, err := strconv.Atoi(os.Getenv("CAP"))
	if err != nil {
		cap = 128
	}
	cacheCap, err := strconv.Atoi(os.Getenv("CACHE_CAP"))
	if err != nil {
		cacheCap = 128
	}
	maxBatchSize, err := strconv.Atoi(os.Getenv("MAX_BATCH_SIZE"))
	if err != nil {
		maxBatchSize = 10
	}
	// Specified in seconds
	var timeout time.Duration
	timeoutInt, err := strconv.Atoi(os.Getenv("TIMEOUT"))
	if err != nil {
		timeout = time.Minute
	} else {
		timeout = time.Duration(timeoutInt) * time.Second
	}
	// Specified in seconds
	var ttl time.Duration
	ttlInt, err := strconv.Atoi(os.Getenv("TTL"))
	if err != nil {
		ttl = 3 * time.Second
	} else {
		ttl = time.Duration(ttlInt) * time.Second
	}
	// Specified in seconds
	var pollRate time.Duration
	pollRateInt, err := strconv.Atoi(os.Getenv("POLL_RATE"))
	if err != nil {
		pollRate = 5 * time.Minute
	} else {
		pollRate = time.Duration(pollRateInt) * time.Second
	}
	addresses := strings.Split(os.Getenv("ADDRESSES"), ",")

	// Seed random number generator.
	rand.Seed(time.Now().UnixNano())

	// Setup logger and attach Sentry hook.
	logger := logrus.New()
	if !strings.Contains(name, "devnet") {
		tags := map[string]string{
			"name": name,
		}

		hook, err := logrus_sentry.NewWithTagsSentryHook(sentryURL, tags, []logrus.Level{
			logrus.PanicLevel,
			logrus.FatalLevel,
			logrus.ErrorLevel,
		})
		if err != nil {
			logger.Fatalf("cannot create a sentry hook: %v", err)
		}
		hook.Timeout = 500 * time.Millisecond
		logger.AddHook(hook)
	}

	bootstrapMultiAddrs := make(addr.MultiAddresses, len(addresses))
	for i := range addresses {
		multiAddr, err := addr.NewMultiAddressFromString(addresses[i])
		if err != nil {
			logger.Fatalf("invalid bootstrap addresses: %v", err)
		}
		bootstrapMultiAddrs[i] = multiAddr
	}

	psqlDB, err := sql.Open("postgres", os.Getenv("POSTGRES_CONN"))
	if err != nil {
		logger.Fatalf("failed to connect to psql db: %v", err)
	}
	db := db.NewSQLDB(psqlDB)
	net := parseNetwork(name)

	// Start running Lightnode.
	ctx := context.Background()
	node := lightnode.New(ctx, net, db, logger, cap, cacheCap, maxBatchSize, timeout, ttl, pollRate, port, bootstrapMultiAddrs)
	node.Run(ctx)
}

func parseNetwork(name string) darknode.Network {
	switch strings.ToLower(strings.Split(name, "-")[1]) {
	case "devnet":
		return darknode.Devnet
	case "testnet":
		return darknode.Testnet
	case "mainnet":
		return darknode.Mainnet
	default:
		panic("unsupported network")
	}
}
