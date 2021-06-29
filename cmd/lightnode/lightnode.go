package main

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
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
	"github.com/renproject/darknode/binding"
	"github.com/renproject/darknode/engine"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/id"
	"github.com/renproject/lightnode"
	"github.com/renproject/lightnode/http"
	"github.com/renproject/multichain"
	"github.com/renproject/pack"
	"github.com/renproject/surge"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
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

	ctx := context.Background()

	// Fetch and apply the first successfully exposed config from bootstrap nodes
	conf, err := getConfigFromBootstrap(ctx, logger, options.BootstrapAddrs)
	if err != nil {
		logger.Fatalf("failed to fetch config from any bootstrap node")
	}

	options.Whitelist = conf.Whitelist

	for chain, chainOpt := range options.Chains {
		chainOpt.Confirmations = conf.Confirmations[chain]
		chainOpt.MaxConfirmations = conf.MaxConfirmations[chain]
		options.Chains[chain] = chainOpt
	}

	// Fetch block state from first bootstrap node and use the public key
	state, err := fetchBlockState(context.Background(), addrToUrl(options.BootstrapAddrs[0], logger), logger, time.Minute)
	if err != nil {
		logger.Fatalf("failed to fetch block state from bootstrap node")
	}
	pub, err := parsePubkey(state)
	if err != nil {
		logger.Fatalf("failed to parse public key from block state")
	}
	options = options.WithDistPubKey(&pub)

	// Run Lightnode.
	node := lightnode.New(options, ctx, logger, sqlDB, client.Conn())
	node.Run(ctx)
}

func getConfigFromBootstrap(ctx context.Context, logger logrus.FieldLogger, addrs []wire.Address) (jsonrpc.ResponseQueryConfig, error) {
	for i, addr := range addrs {
		conf, err := fetchConfig(ctx, addrToUrl(addr, logger), logger, time.Minute)
		if i == len(addrs)-1 && err != nil {
			return conf, err
		}

		if err == nil {
			return conf, nil
		}
	}
	return jsonrpc.ResponseQueryConfig{}, fmt.Errorf("Could not load config from darknodes")
}

func addrToUrl(addr wire.Address, logger logrus.FieldLogger) string {
	addrParts := strings.Split(addr.Value, ":")
	if len(addrParts) != 2 {
		logger.Errorf("[config] invalid address value=%v", addr.Value)
		return ""
	}
	port, err := strconv.Atoi(addrParts[1])
	if err != nil {
		logger.Errorf("[config] invalid port=%v", addr)
		return ""
	}
	return fmt.Sprintf("http://%s:%v", addrParts[0], port+1)
}

func fetchConfig(ctx context.Context, url string, logger logrus.FieldLogger, timeout time.Duration) (jsonrpc.ResponseQueryConfig, error) {
	var resp jsonrpc.ResponseQueryConfig
	params, err := json.Marshal(jsonrpc.ParamsQueryConfig{})
	if err != nil {
		logger.Errorf("[config] cannot marshal query config params: %v", err)
		return resp, err
	}
	client := http.NewClient(timeout)

	request := jsonrpc.Request{
		Version: "2.0",
		ID:      rand.Int31(),
		Method:  jsonrpc.MethodQueryConfig,
		Params:  params,
	}

	response, err := client.SendRequest(ctx, url, request, nil)
	if err != nil {
		logger.Errorf("[config] error calling queryConfig: %v", err)
		return resp, err
	}

	raw, err := json.Marshal(response.Result)
	if err != nil {
		logger.Errorf("[config] error marshaling queryConfig result: %v", err)
		return resp, err
	}

	if err := json.Unmarshal(raw, &resp); err != nil {
		logger.Warnf("[config] cannot unmarshal queryConfig result from %v: %v", url, err)
		return resp, err
	}

	return resp, nil
}

func fetchBlockState(ctx context.Context, url string, logger logrus.FieldLogger, timeout time.Duration) (jsonrpc.ResponseQueryBlockState, error) {
	var resp jsonrpc.ResponseQueryBlockState
	params, err := json.Marshal(jsonrpc.ParamsQueryBlockState{})
	if err != nil {
		logger.Errorf("[config] cannot marshal query block state params: %v", err)
		return resp, err
	}
	client := http.NewClient(timeout)

	request := jsonrpc.Request{
		Version: "2.0",
		ID:      rand.Int31(),
		Method:  jsonrpc.MethodQueryBlockState,
		Params:  params,
	}

	response, err := client.SendRequest(ctx, url, request, nil)
	if err != nil {
		logger.Errorf("[config] error calling queryConfig: %v", err)
		return resp, err
	}

	raw, err := json.Marshal(response.Result)
	if err != nil {
		logger.Errorf("[config] error marshaling queryConfig result: %v", err)
		return resp, err
	}

	if err := json.Unmarshal(raw, &resp); err != nil {
		logger.Warnf("[config] cannot unmarshal queryConfig result from %v: %v", url, err)
		return resp, err
	}

	return resp, nil
}

func parsePubkey(response jsonrpc.ResponseQueryBlockState) (id.PubKey, error) {
	systemContract := response.State.Get("System")
	if systemContract == nil {
		return id.PubKey{}, fmt.Errorf("system contract is nil")
	}

	var state engine.SystemState
	if err := pack.Decode(&state, systemContract); err != nil {
		return id.PubKey{}, err
	}
	if len(state.Shards.Primary) < 1 {
		return id.PubKey{}, fmt.Errorf("nil primary shard")
	}
	shard := state.Shards.Primary[0]
	var pub id.PubKey
	if err := surge.FromBinary(&pub, shard.PubKey); err != nil {
		return id.PubKey{}, err
	}
	return pub, nil
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
		WithNetwork(parseNetwork("HEROKU_APP_NAME"))

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
	if os.Getenv("MAX_GATEWAY_COUNT") != "" {
		options = options.WithMaxGatewayCount(parseInt("MAX_GATEWAY_COUNT"))
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
	if os.Getenv("WATCHER_MAX_BLOCK_ADVANCE") != "" {
		options = options.WithWatcherMaxBlockAdvance(uint64(parseInt("WATCHER_MAX_BLOCK_ADVANCE")))
	}
	if os.Getenv("WATCHER_CONFIDENCE_INTERVAL") != "" {
		options = options.WithWatcherMaxBlockAdvance(uint64(parseInt("WATCHER_CONFIDENCE_INTERVAL")))
	}
	if os.Getenv("EXPIRY") != "" {
		options = options.WithTransactionExpiry(parseTime("EXPIRY"))
	}
	if os.Getenv("ADDRESSES") != "" {
		options = options.WithBootstrapAddrs(parseAddresses("ADDRESSES"))
	}

	if os.Getenv("LIMITER_TTL") != "" {
		options = options.WithLimiterTTL(parseTime("LIMITER_TTL"))
	}
	if os.Getenv("LIMITER_IP_RATE") != "" {
		options = options.WithLimiterIPRates(parseRates("LIMITER_IP_RATE"))
	}
	if os.Getenv("LIMITER_GLOBAL_RATE") != "" {
		options = options.WithLimiterGlobalRates(parseRates("LIMITER_GLOBAL_RATE"))
	}

	chains := map[multichain.Chain]binding.ChainOptions{}
	if os.Getenv("RPC_AVALANCHE") != "" {
		chains[multichain.Avalanche] = binding.ChainOptions{
			RPC:      pack.String(os.Getenv("RPC_AVALANCHE")),
			Protocol: pack.String(os.Getenv("GATEWAY_AVALANCHE")),
		}
	}
	if os.Getenv("RPC_BINANCE") != "" {
		chains[multichain.BinanceSmartChain] = binding.ChainOptions{
			RPC:      pack.String(os.Getenv("RPC_BINANCE")),
			Protocol: pack.String(os.Getenv("GATEWAY_BINANCE")),
		}
	}
	if os.Getenv("RPC_BITCOIN") != "" {
		chains[multichain.Bitcoin] = binding.ChainOptions{
			RPC: pack.String(os.Getenv("RPC_BITCOIN")),
		}
	}
	if os.Getenv("RPC_BITCOIN_CASH") != "" {
		chains[multichain.BitcoinCash] = binding.ChainOptions{
			RPC: pack.String(os.Getenv("RPC_BITCOIN_CASH")),
		}
	}
	if os.Getenv("RPC_DIGIBYTE") != "" {
		chains[multichain.DigiByte] = binding.ChainOptions{
			RPC: pack.String(os.Getenv("RPC_DIGIBYTE")),
		}
	}
	if os.Getenv("RPC_DOGECOIN") != "" {
		chains[multichain.Dogecoin] = binding.ChainOptions{
			RPC: pack.String(os.Getenv("RPC_DOGECOIN")),
		}
	}
	if os.Getenv("RPC_ETHEREUM") != "" {
		chains[multichain.Ethereum] = binding.ChainOptions{
			RPC:      pack.String(os.Getenv("RPC_ETHEREUM")),
			Protocol: pack.String(os.Getenv("GATEWAY_ETHEREUM")),
		}
	}
	if os.Getenv("RPC_FANTOM") != "" {
		chains[multichain.Fantom] = binding.ChainOptions{
			RPC:      pack.String(os.Getenv("RPC_FANTOM")),
			Protocol: pack.String(os.Getenv("GATEWAY_FANTOM")),
		}
	}
	if os.Getenv("RPC_FILECOIN") != "" {
		chains[multichain.Filecoin] = binding.ChainOptions{
			RPC: pack.String(os.Getenv("RPC_FILECOIN")),
			Extras: map[pack.String]pack.String{
				"authToken": pack.String(os.Getenv("EXTRAS_FILECOIN_AUTH")),
			},
		}
	}
	if os.Getenv("RPC_POLYGON") != "" {
		chains[multichain.Polygon] = binding.ChainOptions{
			RPC:      pack.String(os.Getenv("RPC_POLYGON")),
			Protocol: pack.String(os.Getenv("GATEWAY_POLYGON")),
		}
	}
	if os.Getenv("RPC_SOLANA") != "" {
		chains[multichain.Solana] = binding.ChainOptions{
			RPC:      pack.String(os.Getenv("RPC_SOLANA")),
			Protocol: pack.String(os.Getenv("GATEWAY_SOLANA")),
		}
	}
	if os.Getenv("RPC_TERRA") != "" {
		chains[multichain.Terra] = binding.ChainOptions{
			RPC: pack.String(os.Getenv("RPC_TERRA")),
		}
	}
	if os.Getenv("RPC_ZCASH") != "" {
		chains[multichain.Zcash] = binding.ChainOptions{
			RPC: pack.String(os.Getenv("RPC_ZCASH")),
		}
	}
	options = options.WithChains(chains)

	return options
}

func parseNetwork(name string) multichain.Network {
	appName := os.Getenv(name)
	if strings.Contains(appName, "devnet") {
		return multichain.NetworkDevnet
	}
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

func parseRates(name string) map[string]rate.Limit {
	rateStrings := strings.Split(os.Getenv(name), ",")
	rates := make(map[string]rate.Limit)
	for i := range rateStrings {
		methodRate := strings.Split(rateStrings[i], ":")
		if len(methodRate) != 2 {
			panic(fmt.Sprintf("invalid rate pair %v", rateStrings[i]))
		}
		parsedRate, err := strconv.Atoi(methodRate[1])
		if err != nil {
			panic(fmt.Sprintf("invalid rate pair %v: %v", rateStrings[i], err))
		}
		rates[methodRate[0]] = rate.Limit(parsedRate)
	}
	return rates
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

func parseWhitelist(name string) []tx.Selector {
	whitelistStrings := strings.Split(os.Getenv(name), ",")
	whitelist := make([]tx.Selector, len(whitelistStrings))
	for i := range whitelist {
		whitelist[i] = tx.Selector(whitelistStrings[i])
	}
	return whitelist
}
