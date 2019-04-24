package resolver

import (
	"fmt"

	"github.com/republicprotocol/darknode-go/server"
	"github.com/republicprotocol/darknode-go/server/jsonrpc"
	"github.com/renproject/lightnode/rpc"
	"github.com/republicprotocol/tau"
	"github.com/sirupsen/logrus"
)

// Resolver is the parent task which is used to relay messages between the client, server and other sub-tasks.
type Resolver struct {
	logger    *logrus.Logger
	client    tau.Task
	server    tau.Task
	addresses []string
}

// newResolver returns a new Resolver.
func newResolver(logger *logrus.Logger, client, server tau.Task, addresses []string) *Resolver {
	return &Resolver{
		logger:    logger,
		client:    client,
		server:    server,
		addresses: addresses,
	}
}

// New returns a new Resolver task.
func New(cap int, logger *logrus.Logger, client, server tau.Task, addresses []string) tau.Task {
	resolver := newResolver(logger, client, server, addresses)
	resolver.server.Send(rpc.NewAccept())
	return tau.New(tau.NewIO(cap), resolver, client, server)
}

// Reduce implements the `tau.Task` interface.
func (resolver *Resolver) Reduce(message tau.Message) tau.Message {
	switch message := message.(type) {
	case rpc.MessageAccepted:
		resolver.server.Send(server.NewAccept())
		return resolver.handleServerMessage(message.Request)
	case tau.Error:
		resolver.logger.Errorln(message.Error())
		return nil
	default:
		panic(fmt.Errorf("unexpected message type %T", message))
	}
}

func (resolver *Resolver) handleServerMessage(request jsonrpc.Request) tau.Message {
	resolver.client.Send(rpc.InvokeRPC{
		Request:   request,
		Addresses: resolver.addresses,
	})

	return nil
}
