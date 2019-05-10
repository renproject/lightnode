package lightnode

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/renproject/lightnode/p2p"
	"github.com/renproject/lightnode/resolver"
	"github.com/renproject/lightnode/rpc"
	"github.com/renproject/lightnode/store"
	"github.com/republicprotocol/darknode-go/health"
	"github.com/republicprotocol/darknode-go/rpc/jsonrpc"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/republicprotocol/renp2p-go/foundation/addr"
	"github.com/republicprotocol/tau"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
)

// Lightnode defines the fields required by the server.
type Lightnode struct {
	port     string
	logger   logrus.FieldLogger
	handler  http.Handler
	resolver tau.Task
}

// New constructs a new Lightnode.
func New(logger logrus.FieldLogger, cap, workers, timeout int, version, port string, bootstrapMultiAddrs []peer.MultiAddr, pollRate time.Duration, peerCount, maxBatchSize int) *Lightnode {
	lightnode := &Lightnode{
		port:   port,
		logger: logger,
	}

	// Construct client and server.
	multiStore := store.NewCache(0)
	statsStore := store.NewCache(0)
	messageStore := store.NewCache(60)
	store := store.NewProxy(multiStore, statsStore, messageStore)
	client := rpc.NewClient(logger, store, cap, workers, time.Duration(timeout)*time.Second)
	requests := make(chan jsonrpc.Request, cap)
	jsonrpcService := jsonrpc.New(logger, requests, time.Duration(timeout)*time.Second, maxBatchSize)
	server := rpc.NewServer(logger, cap, requests)

	health := health.NewHealthCheck(version, addr.New(""))
	p2pService := p2p.New(logger, cap, time.Duration(timeout)*time.Second, store, health, bootstrapMultiAddrs, pollRate, peerCount)
	lightnode.handler = cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS", "DELETE"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		Debug:            os.Getenv("DEBUG") == "true",
	}).Handler(jsonrpcService)

	// Construct resolver.
	bootstrapAddrs := make([]addr.Addr, len(bootstrapMultiAddrs))
	for i := range bootstrapMultiAddrs {
		bootstrapAddrs[i] = bootstrapMultiAddrs[i].Addr()
	}

	lightnode.resolver = resolver.New(cap, logger, client, server, p2pService, bootstrapAddrs)

	return lightnode
}

// run starts listening for requests using a HTTP server.
func (node *Lightnode) Run(done <-chan struct{}) {
	node.logger.Infof("JSON-RPC server listening on 0.0.0.0:%v...", node.port)
	go func() {
		if err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%v", node.port), node.handler); err != nil {
			node.logger.Errorf("failed to serve: %v", err)
		}
	}()

	node.resolver.Run(done)
}
