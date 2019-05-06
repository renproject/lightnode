package resolver

import (
	"github.com/renproject/lightnode/rpc"
	"github.com/republicprotocol/renp2p-go/foundation/addr"
	"github.com/republicprotocol/tau"
	"github.com/sirupsen/logrus"
)

// Resolver is the parent task which is used to relay messages between the client, server and other sub-tasks.
type Resolver struct {
	logger         logrus.FieldLogger
	client         tau.Task
	server         tau.Task
	p2p            tau.Task
	bootstrapAddrs []addr.Addr
}

// newResolver returns a new Resolver.
func newResolver(logger logrus.FieldLogger, client, server, p2p tau.Task, bootstrapAddrs []addr.Addr) *Resolver {
	return &Resolver{
		logger:         logger,
		client:         client,
		server:         server,
		p2p:            p2p,
		bootstrapAddrs: bootstrapAddrs,
	}
}

// New returns a new Resolver task.
func New(cap int, logger logrus.FieldLogger, client, server, p2p tau.Task, bootstrapAddrs []addr.Addr) tau.Task {
	resolver := newResolver(logger, client, server, p2p, bootstrapAddrs)
	resolver.server.Send(rpc.NewAccept())
	return tau.New(tau.NewIO(cap), resolver, client, server, p2p)
}

// Reduce implements the `tau.Task` interface.
func (resolver *Resolver) Reduce(message tau.Message) tau.Message {
	switch message := message.(type) {
	case rpc.SendMessage:
		resolver.server.Send(rpc.NewAccept())
		resolver.client.Send(rpc.InvokeRPC{
			Request:   message.Request,
			Addresses: resolver.bootstrapAddrs,
		})
	case rpc.QueryMessage:
		resolver.server.Send(rpc.NewAccept())
		resolver.p2p.Send(p2p.InvokeQuery{
			Request: message.Request,
		})
	case tau.Error:
		resolver.logger.Errorln(message.Error())
	default:
		resolver.logger.Panicf("unexpected message type %T", message)
	}

	return nil
}
