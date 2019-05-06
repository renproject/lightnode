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
	"github.com/republicprotocol/darknode-go/server/jsonrpc"
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
func New(logger logrus.FieldLogger, cap, workers, timeout int, port string, bootstrapMultiAddrs []peer.MultiAddr, pollRate time.Duration, multiAddrCount int) *Lightnode {
	lightnode := &Lightnode{
		port:   port,
		logger: logger,
	}

	// Construct client and server.
	multiStore := store.NewCache(0)
	statsStore := store.NewCache(0)
	store := store.NewProxy(multiStore, statsStore)
	client := rpc.NewClient(logger, store, cap, workers, time.Duration(timeout)*time.Second)
	requests := make(chan jsonrpc.Request, cap)
	jsonrpcService := jsonrpc.New(logger, requests, time.Duration(timeout)*time.Second)
	server := rpc.NewServer(logger, cap, requests)

	p2pService := p2p.New(logger, cap, time.Duration(timeout)*time.Second, store, bootstrapMultiAddrs, pollRate, multiAddrCount)
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

// Run starts listening for requests using a HTTP server.
func (node *Lightnode) Run(done <-chan struct{}) {
	node.logger.Infof("JSON-RPC server listening on 0.0.0.0:%v...", node.port)
	go func() {
		if err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%v", node.port), node.handler); err != nil {
			node.logger.Errorf("failed to serve: %v", err)
		}
	}()

	node.resolver.Run(done)
}
