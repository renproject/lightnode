package resolver

import (
	"fmt"

	"github.com/renproject/lightnode/p2p"
	"github.com/renproject/lightnode/rpc"
	"github.com/republicprotocol/darknode-go/server/jsonrpc"
	"github.com/republicprotocol/renp2p-go/foundation/addr"
	"github.com/republicprotocol/tau"
	"github.com/sirupsen/logrus"
)

// Resolver is the parent task which is used to relay messages between the client, server and other sub-tasks.
type Resolver struct {
	logger *logrus.Logger
	client tau.Task
	server tau.Task
	p2p    tau.Task
	addrs  []addr.Addr
}

// newResolver returns a new Resolver.
func newResolver(logger *logrus.Logger, client, server, p2p tau.Task, addresses []string) *Resolver {
	addrs := make([]addr.Addr, len(addresses))
	for i, address := range addresses {
		addrs[i] = addr.New(address)
	}

	return &Resolver{
		logger: logger,
		client: client,
		server: server,
		p2p:    p2p,
		addrs:  addrs,
	}
}

// New returns a new Resolver task.
func New(cap int, logger *logrus.Logger, client, server, p2p tau.Task, addresses []string) tau.Task {
	resolver := newResolver(logger, client, server, p2p, addresses)
	resolver.server.Send(rpc.NewAccept())
	return tau.New(tau.NewIO(cap), resolver, client, server, p2p)
}

// Reduce implements the `tau.Task` interface.
func (resolver *Resolver) Reduce(message tau.Message) tau.Message {
	switch message := message.(type) {
	case rpc.SendMessage:
		resolver.server.Send(rpc.NewAccept())
		return resolver.handleServerMessage(message.Request)
	case rpc.QueryPeersMessage:
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

func (resolver *Resolver) handleServerMessage(request jsonrpc.Request) tau.Message {
	resolver.client.Send(rpc.InvokeRPC{
		Request:   request,
		Addresses: resolver.addrs,
	})

	return nil
}
