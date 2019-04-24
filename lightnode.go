package lightnode

import (
	"fmt"
	"net/http"
	"time"

	"github.com/republicprotocol/darknode-go/server/jsonrpc"
	"github.com/republicprotocol/lightnode/resolver"
	"github.com/republicprotocol/lightnode/rpc"
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

func NewLightnode(logger *logrus.Logger, cap, workers, timeout int, port string) *Lightnode {
	lightNode := &Lightnode{
		port:   port,
		logger: logger,
	}

	client := rpc.NewClient(logger, cap, workers, time.Duration(timeout)*time.Second)
	requests := make(chan jsonrpc.Request, cap)
	jsonrpcService := jsonrpc.New(logger, requests, time.Duration(timeout)*time.Second)
	server := rpc.NewServer(logger, cap, requests)
	lightNode.handler = cors.New(cors.Options{
		AllowCredentials: true,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS", "DELETE"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
	}).Handler(jsonrpcService)

	lightNode.resolver = resolver.New(cap, logger, client, server)
	return lightNode
}

func (node *Lightnode) Run(done <-chan struct{}) {
	node.logger.Infof("jsonRPC listening on 0.0.0.0:%v...", node.port)
	go func() {
		if err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%v", node.port), node.handler); err != nil {
			node.logger.Errorln("fail to serve the server,", err)
		}
	}()
	node.resolver.Run(done)
}
