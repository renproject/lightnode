package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	_ "github.com/mattn/go-sqlite3"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v7"
	"github.com/renproject/darknode"
	"github.com/renproject/lightnode"
	"github.com/renproject/multichain"
	"github.com/sirupsen/logrus"
)

func main() {

	// Parse flags.
	flagPort := flag.String("port", "", "Port for lightnode RPC")
	// FIXME : IDEALLY LIGHTNODE SHOULDN'T HAVE ACCESS TO DARKNODE CONFIG, BUT
	// FIXME : GETTING THOSE INFOS FROM QUERYING DARKNODES.
	flagConfig := flag.String("config", "", "Config file path for the shard darknode")
	flagOut := flag.String("out", "", "Output directory for all the files")

	flag.Parse()

	if *flagPort == ""{
		panic("Please provide the port number using --port")
	}
	var config darknode.Options
	configFile, err := os.Open(*flagConfig)
	if err != nil {
		panic(err)
	}
	if err := json.NewDecoder(configFile).Decode(&config); err != nil {
		panic(err)
	}
	configFile.Close()
	if *flagOut == "" {
		panic("Please provide the output directory using --out")
	}

	// Wait for an interrupt or terminate signal from the OS, and then cancel
	// the running context.
	ctx, cancel := context.WithCancel(context.Background())
	wg := new(sync.WaitGroup)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signals
		cancel()
	}()

	options := lightnode.DefaultOptions().
		WithNetwork(multichain.NetworkLocalnet).
		WithDistPubKey(config.PrivKey.PubKey()).
		WithPort(*flagPort).
		WithBootstrapAddrs(config.Peers).
		WithChains(config.Chains).
		WithWhitelist(config.Whitelist).
		WithWatcherConfidenceInterval(0)

	db := initSQLITE(*flagOut)
	wg.Add(1)
	go func() {
		defer wg.Done()
		<- ctx.Done()
		db.Close()
	}()

	redisClient, redisServer := initRedis()
	wg.Add(1)
	go func() {
		defer wg.Done()
		<- ctx.Done()
		redisServer.Close()
	}()

	node := lightnode.New(options, ctx, logrus.New(), db, redisClient)
	go node.Run(ctx)
	<- ctx.Done()
	wg.Wait()
}

func initSQLITE(dir string ) *sql.DB {
	if err := os.MkdirAll(dir, 0766); err != nil {
		panic(err)
	}
	dir = filepath.Join(dir, "db")
	sqlDB, err := sql.Open("sqlite3", dir)
	if err != nil {
		panic(err)
	}
	_, err = sqlDB.Exec("PRAGMA foreign_keys = ON;")
	if err != nil {
		panic(err)
	}
	return sqlDB
}

func initRedis() (*redis.Client, *miniredis.Miniredis) {
	server, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	client := redis.NewClient(&redis.Options{
		Addr: server.Addr(),
	})

	return client, server
}