package lightnode

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/renproject/lightnode/p2p"
	"github.com/renproject/lightnode/resolver"
	"github.com/renproject/lightnode/rpc"
	"github.com/republicprotocol/darknode-go/health"
	"github.com/republicprotocol/darknode-go/rpc/jsonrpc"
	storeAdapter "github.com/republicprotocol/renp2p-go/adapter/store"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/republicprotocol/renp2p-go/foundation/addr"
	"github.com/republicprotocol/store"
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
	timeoutSeconds := time.Duration(timeout) * time.Second
	lightnode := &Lightnode{
		port:   port,
		logger: logger,
	}

	// Construct client task.
	multiStore := storeAdapter.NewMultiAddrStore(store.NewIterableCache(0))
	statsStore := store.NewIterableCache(0)
	proxyStore := p2p.NewProxy(multiStore, statsStore)
	client := rpc.NewClient(logger, multiStore, cap, workers, timeoutSeconds)

	// Construct the json-rpc server handler and server task.
	requests := make(chan jsonrpc.Request, cap)
	jsonrpcService := jsonrpc.New(logger, requests, timeoutSeconds, maxBatchSize)
	server := rpc.NewServer(logger, cap, requests)
	lightnode.handler = cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS", "DELETE"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		Debug:            os.Getenv("DEBUG") == "true",
	}).Handler(jsonrpcService)

	// Construct the p2p service
	health := health.NewHealthCheck(version, addr.New(""))
	p2pService := p2p.New(logger, cap, peerCount, timeoutSeconds, pollRate, proxyStore, health, bootstrapMultiAddrs)

	// Construct resolver.
	bootstrapAddrs := make([]addr.Addr, len(bootstrapMultiAddrs))
	for i := range bootstrapMultiAddrs {
		bootstrapAddrs[i] = bootstrapMultiAddrs[i].Addr()
	}
	lightnode.resolver = resolver.New(cap, logger, client, server, p2pService, bootstrapAddrs)

	return lightnode
}

// Run starts listening for requests using a HTTP server.
func (node *Lightnode) Run(done <-chan struct{}) {
	node.logger.Infof("JSON-RPC server listening on 0.0.0.0:%v...", node.port)
	go http.ListenAndServe(fmt.Sprintf("0.0.0.0:%v", node.port), node.handler)
	node.resolver.Run(done)
}
