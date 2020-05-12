package main

import (
	"context"
	"crypto/ecdsa"
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
	"github.com/renproject/darknode"
	"github.com/renproject/darknode/addr"
	"github.com/renproject/lightnode"
	"github.com/sirupsen/logrus"
)

func main() {
	// Seed random number generator.
	rand.Seed(time.Now().UnixNano())

	// Parse Lightnode options from environment variables.
	name := os.Getenv("HEROKU_APP_NAME")
	options := lightnode.Options{
		Network:           parseNetwork(name),
		Key:               parsePriKey(),
		DisPubkey:         parsePubKey(),
		Port:              os.Getenv("PORT"),
		ProtocolAddr:      os.Getenv("PROTOCOL_ADDRESS"),
		Cap:               parseInt("CAP"),
		MaxBatchSize:      parseInt("MAX_BATCH_SIZE"),
		ServerTimeout:     parseTime("SERVER_TIMEOUT"),
		ClientTimeout:     parseTime("CLIENT_TIMEOUT"),
		TTL:               parseTime("TTL"),
		UpdaterPollRate:   parseTime("UPDATER_POLL_RATE"),
		ConfirmerPollRate: parseTime("CONFIRMER_POLL_RATE"),
		BootstrapAddrs:    parseAddresses(),
	}

	// Initialise logger and attach Sentry hook.
	logger := initLogger(options.Network)

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
	node := lightnode.New(ctx, options, logger, sqlDB, client)
	node.Run(ctx)
}

func initLogger(network darknode.Network) logrus.FieldLogger {
	logger := logrus.New()
	sentryURL := os.Getenv("SENTRY_URL")
	name := os.Getenv("HEROKU_APP_NAME")
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

func parsePriKey() *ecdsa.PrivateKey {
	keyBytes, err := hex.DecodeString(os.Getenv("PRI_KEY"))
	if err != nil {
		panic(fmt.Sprintf("invalid private key string from the env variable, err = %v", err))
	}
	key, err := crypto.ToECDSA(keyBytes)
	if err != nil {
		panic(fmt.Sprintf("invalid private key for lightnode account, err = %v", err))
	}
	return key
}

func parsePubKey() *ecdsa.PublicKey {
	keyBytes, err := hex.DecodeString(os.Getenv("PUB_KEY"))
	if err != nil {
		panic(fmt.Sprintf("invalid public key string from the env variable, err = %v", err))
	}
	key, err := crypto.DecompressPubkey(keyBytes)
	if err != nil {
		panic(fmt.Sprintf("invalid distribute public key, err = %v", err))
	}
	return key
}
