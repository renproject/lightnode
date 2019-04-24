package p2p

import (
	"fmt"

	"github.com/renproject/lightnode/store"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/republicprotocol/renp2p-go/foundation/addr"
	"github.com/republicprotocol/tau"
)

type P2P struct {
	store store.KVStore
}

func NewP2P(store store.KVStore) *P2P {
	return &P2P{
		store: store,
	}
}

func (p2p *P2P) Reduce(message tau.Message) tau.Message {
	switch message := message.(type) {
	case QueryAddressRequest:
		return p2p.handleQueryAddress(message)
	default:
		panic(fmt.Errorf("unexpected message type %T", message))
	}
}

func (p2p *P2P) handleQueryAddress(message QueryAddressRequest) tau.Message {
	var multi peer.MultiAddr
	err := p2p.store.Read(message.Addr.String(), &multi)
	if err != nil {
		return tau.NewError(err)
	}
	return QueryAddressResponse{
		Multi: multi,
	}
}

func New(cap int, store store.KVStore) tau.Task {
	p2p := NewP2P(store)
	return tau.New(tau.NewIO(cap), p2p)
}

type QueryAddressRequest struct {
	// todo : decorator pattern
	Addr addr.Addr
}

func (message QueryAddressRequest) IsMessage() {

}

type QueryAddressResponse struct {
	Multi peer.MultiAddr
}

func (message QueryAddressResponse) IsMessage() {
}
