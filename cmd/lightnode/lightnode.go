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
	"github.com/renproject/darknode/txengine/txenginebindings"
	"github.com/renproject/id"
	"github.com/renproject/lightnode"
	"github.com/renproject/multichain"
	"github.com/renproject/pack"
	"github.com/sirupsen/logrus"
)

func main() {
	// Seed random number generator.
	rand.Seed(time.Now().UnixNano())

	// Parse Lightnode options from environment variables.
	options := parseOptions()

	// Initialise logger and attach Sentry hook.
	logger := initLogger(os.Getenv("HEROKU_APP_NAME"), options.Network)

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

func initLogger(name string, network multichain.Network) logrus.FieldLogger {
	logger := logrus.New()
	sentryURL := os.Getenv("SENTRY_URL")
	if network != multichain.NetworkLocalnet {
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

func parseOptions() lightnode.Options {
	options := lightnode.DefaultOptions().
		WithNetwork(parseNetwork("HEROKU_APP_NAME")).
		WithDistPubKey(parsePubKey("PUB_KEY"))

	// We only want to override the default options if the environment variable
	// has been specified.
	if os.Getenv("PORT") != "" {
		options = options.WithPort(os.Getenv("PORT"))
	}
	if os.Getenv("CAP") != "" {
		options = options.WithCap(parseInt("CAP"))
	}
	if os.Getenv("MAX_BATCH_SIZE") != "" {
		options = options.WithMaxBatchSize(parseInt("MAX_BATCH_SIZE"))
	}
	if os.Getenv("MAX_PAGE_SIZE") != "" {
		options = options.WithMaxBatchSize(parseInt("MAX_PAGE_SIZE"))
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
	if os.Getenv("WATCHER_POLL_RATE") != "" {
		options = options.WithWatcherPollRate(parseTime("WATCHER_POLL_RATE"))
	}
	if os.Getenv("EXPIRY") != "" {
		options = options.WithTransactionExpiry(parseTime("EXPIRY"))
	}
	if os.Getenv("ADDRESSES") != "" {
		options = options.WithBootstrapAddrs(parseAddresses("ADDRESSES"))
	}

	chains := map[multichain.Chain]txenginebindings.ChainOptions{}
	if os.Getenv("RPC_BINANCE") != "" {
		chains[multichain.BinanceSmartChain] = txenginebindings.ChainOptions{
			RPC:           pack.String(os.Getenv("RPC_BINANCE")),
			Confirmations: pack.U64(parseInt(os.Getenv("CONFIRMATIONS_BINANCE"))),
			Protocol:      pack.String(os.Getenv("GATEWAY_BINANCE")),
		}
	}
	if os.Getenv("RPC_BITCOIN") != "" {
		chains[multichain.Bitcoin] = txenginebindings.ChainOptions{
			RPC:           pack.String(os.Getenv("RPC_BITCOIN")),
			Confirmations: pack.U64(parseInt(os.Getenv("CONFIRMATIONS_BITCOIN"))),
		}
	}
	if os.Getenv("RPC_DIGIBYTE") != "" {
		chains[multichain.DigiByte] = txenginebindings.ChainOptions{
			RPC:           pack.String(os.Getenv("RPC_DIGIBYTE")),
			Confirmations: pack.U64(parseInt(os.Getenv("CONFIRMATIONS_DIGIBYTE"))),
		}
	}
	if os.Getenv("RPC_ETHEREUM") != "" {
		chains[multichain.Ethereum] = txenginebindings.ChainOptions{
			RPC:           pack.String(os.Getenv("RPC_ETHEREUM")),
			Confirmations: pack.U64(parseInt(os.Getenv("CONFIRMATIONS_ETHEREUM"))),
			Protocol:      pack.String(os.Getenv("GATEWAY_ETHEREUM")),
		}
	}
	options = options.WithChains(chains)

	return options
}

func parseNetwork(name string) multichain.Network {
	appName := os.Getenv(name)
	if strings.Contains(appName, "testnet") {
		return multichain.NetworkTestnet
	}
	if strings.Contains(appName, "mainnet") {
		return multichain.NetworkMainnet
	}
	return multichain.NetworkLocalnet
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

func parseAddresses(name string) []wire.Address {
	addrStrings := strings.Split(os.Getenv(name), ",")
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

func parsePubKey(name string) *id.PubKey {
	pubKeyString := os.Getenv(name)
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
