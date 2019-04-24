package resolver

import (
	"fmt"

	"github.com/republicprotocol/darknode-go/server"
	"github.com/republicprotocol/darknode-go/server/jsonrpc"
	"github.com/republicprotocol/lightnode/rpc"
	"github.com/republicprotocol/tau"
	"github.com/sirupsen/logrus"
)

type Resolver struct {
	logger *logrus.Logger
	client tau.Task
	server tau.Task
}

func newResolver(logger *logrus.Logger, client, server tau.Task) *Resolver {
	return &Resolver{
		logger: logger,
		client: client,
		server: server,
	}
}

func New(cap int, logger *logrus.Logger, client, server tau.Task) tau.Task {
	resolver := newResolver(logger, client, server)
	resolver.server.Send(rpc.NewAccept())
	return tau.New(tau.NewIO(cap), resolver, client, server)
}

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
	// FIXME:
	addresses := []string{
		"",
	}

	resolver.client.Send(rpc.InvokeRPC{
		Request:   request,
		Addresses: addresses,
	})

	return nil
}
