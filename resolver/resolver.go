package resolver

import (
	"github.com/republicprotocol/tau"
)

type Resolver struct {
	client   tau.Task
	server   tau.Task
	p2p      tau.Task
	sharding tau.Task
}

func NewResolver() *Resolver {
	panic("unimplemented")
}

func (resolver *Resolver) Reduce(message tau.Message) tau.Message {
	panic("unimplemented")
}

func New(cap int, client, server, p2p, sharding tau.Task) tau.Task {
	resolver := NewResolver()
	return tau.New(tau.NewIO(cap), resolver, client, server, p2p, sharding)
}
