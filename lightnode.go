package lightnode

import (
	"fmt"
	"net/http"
	"time"

	"github.com/renproject/lightnode/p2p"
	"github.com/renproject/lightnode/resolver"
	"github.com/renproject/lightnode/rpc"
	"github.com/renproject/lightnode/store"
	"github.com/republicprotocol/darknode-go/server/jsonrpc"
	"github.com/republicprotocol/tau"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
)

type Lightnode struct {
	port     string
	logger   *logrus.Logger
	handler  http.Handler
	resolver tau.Task
}

func NewLightnode(logger *logrus.Logger, cap, workers, timeout int, port string, addresses []string) *Lightnode {
	lightnode := &Lightnode{
		port:   port,
		logger: logger,
	}

	// Construct client and server.
	addrStore := store.NewCache(0)
	client := rpc.NewClient(logger, cap, workers, time.Duration(timeout)*time.Second, addrStore)
	requests := make(chan jsonrpc.Request, cap)
	jsonrpcService := jsonrpc.New(logger, requests, time.Duration(timeout)*time.Second)
	server := rpc.NewServer(logger, cap, requests)
	p2pService := p2p.New(logger, cap, time.Duration(timeout)*time.Second, addrStore, addresses)
	lightnode.handler = cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS", "DELETE"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		Debug:            true,
	}).Handler(jsonrpcService)

	// Construct resolver.
	lightnode.resolver = resolver.New(cap, logger, client, server, p2pService, addresses)

	return lightnode
}

func (node *Lightnode) Run(done <-chan struct{}) {
	node.logger.Infof("JSON-RPC server listening on 0.0.0.0:%v...", node.port)
	go func() {
		if err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%v", node.port), node.handler); err != nil {
			node.logger.Errorf("failed to serve: %v", err)
		}
	}()

	go node.resolver.Run(done)
	node.resolver.Send(p2p.Tick{})

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			node.logger.Debug("updating darknode multi addresses")
			node.resolver.Send(p2p.Tick{})
		}
	}
}
