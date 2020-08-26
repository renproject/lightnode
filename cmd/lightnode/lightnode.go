package main

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/evalphobia/logrus_sentry"
	"github.com/go-redis/redis/v7"
	"github.com/renproject/aw/wire"
	"github.com/renproject/darknode"
	"github.com/renproject/id"
	"github.com/renproject/lightnode"
	"github.com/sirupsen/logrus"
)

func main() {
	// Seed random number generator.
	rand.Seed(time.Now().UnixNano())

	// Parse Lightnode options from environment variables.
	name := os.Getenv("HEROKU_APP_NAME")
	options := lightnode.DefaultOptions().
		WithNetwork(parseNetwork(name)).
		WithDistPubKey(parsePubKey())

	if os.Getenv("PORT") != "" {
		options = options.WithPort(os.Getenv("PORT"))
	}
	if os.Getenv("CAP") != "" {
		options = options.WithCap(parseInt("CAP"))
	}
	if os.Getenv("MAX_BATCH_SIZE") != "" {
		options = options.WithMaxBatchSize(parseInt("MAX_BATCH_SIZE"))
	}
	if os.Getenv("SERVER_TIMEOUT") != "" {
		options = options.WithServerTimeout(parseTime("SERVER_TIMEOUT"))
	}
	if os.Getenv("CLIENT_TIMEOUT") != "" {
		options = options.WithClientTimeout(parseTime("CLIENT_TIMEOUT"))
	}
	if os.Getenv("TTL") != "" {
		options = options.WithTTL(parseTime("TTL"))
	}
	if os.Getenv("UPDATER_POLL_RATE") != "" {
		options = options.WithUpdaterPollRate(parseTime("UPDATER_POLL_RATE"))
	}
	if os.Getenv("CONFIRMER_POLL_RATE") != "" {
		options = options.WithConfirmerPollRate(parseTime("CONFIRMER_POLL_RATE"))
	}
	if os.Getenv("ADDRESSES") != "" {
		options = options.WithBootstrapAddrs(parseAddresses())
	}
	// TODO: WithRPCs, WithGateways, WithConfirmations

	// Initialise logger and attach Sentry hook.
	logger := initLogger(name, options.Network)

	// Initialise the database.
	driver, dbURL := os.Getenv("DATABASE_DRIVER"), os.Getenv("DATABASE_URL")
	sqlDB, err := sql.Open(driver, dbURL)
	if err != nil {
		logger.Fatalf("failed to connect to %v db: %v", driver, err)
	}
	defer sqlDB.Close()

	// Initialise Redis client.
	client := initRedis()
	defer client.Close()

	// Run Lightnode.
	ctx := context.Background()
	node := lightnode.New(options, ctx, logger, sqlDB, client)
	node.Run(ctx)
}

func initLogger(name, network string) logrus.FieldLogger {
	logger := logrus.New()
	sentryURL := os.Getenv("SENTRY_URL")
	if network != darknode.Devnet {
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
	return logger
}

func initRedis() *redis.Client {
	redisURLString := os.Getenv("REDIS_URL")
	redisURL, err := url.Parse(redisURLString)
	if err != nil {
		panic(fmt.Sprintf("failed to parse redis URL %v: %v", redisURLString, err))
	}
	redisPassword, _ := redisURL.User.Password()
	return redis.NewClient(&redis.Options{
		Addr:     redisURL.Host,
		Password: redisPassword,
		DB:       0, // Use default DB.
	})
}

func parseNetwork(appName string) string {
	if strings.Contains(appName, "devnet") {
		return darknode.Devnet
	}
	if strings.Contains(appName, "testnet") {
		return darknode.Testnet
	}
	if strings.Contains(appName, "mainnet") {
		return darknode.Mainnet
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

func parseAddresses() []wire.Address {
	addrStrings := strings.Split(os.Getenv("ADDRESSES"), ",")
	addrs := make([]wire.Address, len(addrStrings))
	for i := range addrs {
		addr, err := wire.DecodeString(addrStrings[i])
		if err != nil {
			panic(fmt.Sprintf("invalid bootstrap address %v: %v", addrStrings[i], err))
		}
		addrs[i] = addr
	}
	return addrs
}

func parsePubKey() *id.PubKey {
	pubKeyString := os.Getenv("PUB_KEY")
	keyBytes, err := hex.DecodeString(pubKeyString)
	if err != nil {
		panic(fmt.Sprintf("invalid distributed public key %v: %v", pubKeyString, err))
	}
	key, err := crypto.DecompressPubkey(keyBytes)
	if err != nil {
		panic(fmt.Sprintf("invalid distributed public key %v: %v", pubKeyString, err))
	}
	return (*id.PubKey)(key)
}
