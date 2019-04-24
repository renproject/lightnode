package shard

import "github.com/republicprotocol/renp2p-go/foundation/addr"

// FIXME:
type Sharding interface {

	// FIXME:
	Shard(to string) ([]addr.Addr, error)
}

type sharding struct {
}

func NewSharding(config []addr.Addr) Sharding {
	return sharding{}
}
