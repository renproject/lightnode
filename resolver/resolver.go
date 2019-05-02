package resolver

import (
	"fmt"

	"github.com/renproject/lightnode/p2p"
	"github.com/renproject/lightnode/rpc"
	"github.com/republicprotocol/renp2p-go/foundation/addr"
	"github.com/republicprotocol/tau"
	"github.com/sirupsen/logrus"
)

// Resolver is the parent task which is used to relay messages between the client, server and other sub-tasks.
type Resolver struct {
	logger    logrus.FieldLogger
	client    tau.Task
	server    tau.Task
	p2p       tau.Task
	addresses []addr.Addr
}

// newResolver returns a new Resolver.
func newResolver(logger logrus.FieldLogger, client, server, p2p tau.Task, addresses []addr.Addr) *Resolver {
	return &Resolver{
		logger:    logger,
		client:    client,
		server:    server,
		p2p:       p2p,
		addresses: addresses,
	}
}

// New returns a new Resolver task.
func New(cap int, logger logrus.FieldLogger, client, server, p2p tau.Task, addresses []addr.Addr) tau.Task {
	resolver := newResolver(logger, client, server, p2p, addresses)
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
			Addresses: resolver.addresses,
		})
	case rpc.QueryMessage:
		resolver.server.Send(rpc.NewAccept())
		resolver.p2p.Send(message)
	case tau.Error:
		resolver.logger.Errorln(message.Error())
	case p2p.Tick:
		resolver.p2p.Send(message)
	default:
		panic(fmt.Errorf("unexpected message type %T", message))
	}

	return nil
}
