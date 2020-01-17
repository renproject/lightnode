package main

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	"github.com/evalphobia/logrus_sentry"
	"github.com/renproject/darknode"
	"github.com/renproject/darknode/addr"
	"github.com/renproject/lightnode"
	"github.com/sirupsen/logrus"
)

func main() {

	// Seed random number generator.
	rand.Seed(time.Now().UnixNano())

	// Parse Lightnode options from environment variables
	name := os.Getenv("HEROKU_APP_NAME")
	options := lightnode.Options{
		Network:           parseNetwork(name),
		Port:              os.Getenv("PORT"),
		Cap:               parseInt("CAP"),
		MaxBatchSize:      parseInt("MAX_BATCH_SIZE"),
		Timeout:           parseTime("TIMEOUT"),
		TTL:               parseTime("TTL"),
		UpdaterPollRate:   parseTime("UPDATER_POLL_RATE"),
		ConfirmerPollRate: parseTime("CONFIRMER_POLL_RATE"),
		BootstrapAddrs:    parseAddresses(),
	}

	// Setup logger and attach Sentry hook.
	logger := logrus.New()
	sentryURL := os.Getenv("SENTRY_URL")
	if options.Network != darknode.Devnet {
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

	// Initialize the database
	sqlDB, err := sql.Open(os.Getenv("DATABASE_DRIVER"), os.Getenv("DATABASE_URL"))
	if err != nil {
		logger.Fatalf("failed to connect to psql db: %v", err)
	}
	defer sqlDB.Close()

	// Start running Lightnode.
	ctx := context.Background()
	node := lightnode.New(ctx, options, logger, sqlDB)
	node.Run(ctx)
}

func parseNetwork(appName string) darknode.Network {
	if strings.Contains(appName, "devnet") {
		return darknode.Devnet
	}
	if strings.Contains(appName, "testnet") {
		return darknode.Testnet
	}
	if strings.Contains(appName, "chaosnet") {
		return darknode.Chaosnet
	}
	if strings.Contains(appName, "localnet") {
		return darknode.Localnet
	}
	panic("unsupported network")
}

func parseInt(name string) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil {
		return 0
	}
	return value
}

func parseTime(name string) time.Duration {
	duration, err := strconv.Atoi(os.Getenv(name))
	if err != nil {
		return 0 * time.Second
	}
	return time.Duration(duration) * time.Second
}

func parseAddresses() addr.MultiAddresses {
	addrs := strings.Split(os.Getenv("ADDRESSES"), ",")
	multis := make([]addr.MultiAddress, len(addrs))
	for i := range multis {
		multi, err := addr.NewMultiAddressFromString(addrs[i])
		if err != nil {
			panic(fmt.Sprintf("invalid bootstrap address : fail to parse from string `%v`", addrs[i]))
		}
		multis[i] = multi
	}
	return multis
}
