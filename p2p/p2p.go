package p2p

import (
	"github.com/renproject/lightnode/store"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/republicprotocol/renp2p-go/foundation/addr"
)

// P2P manages the peer-to-peer network.
type P2P interface {

	// MultiAddress returns the MultiAddr of the given REN address.
	MultiAddress (addr.Addr) (peer.MultiAddr, error)

	Tick()
}

type p2p struct {
	store store.KVStore
}

func NewP2P(store store.KVStore) P2P {
	return &p2p{
		store: store,
	}
}

func (p2p *p2p) MultiAddress ( address addr.Addr) (peer.MultiAddr, error){
	var multi peer.MultiAddr
	err := p2p.store.Read(address.String(), &multi)
	return multi, err
}



