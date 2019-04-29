package p2p

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"

	"github.com/renproject/lightnode/rpc"
	jrpc "github.com/renproject/lightnode/rpc/jsonrpc"
	"github.com/renproject/lightnode/store"
	"github.com/republicprotocol/co-go"
	"github.com/republicprotocol/darknode-go/server/jsonrpc"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/republicprotocol/tau"
	"github.com/sirupsen/logrus"
)

// P2P handles the peer-to-peer network of nodes.
type P2P struct {
	timeout        time.Duration
	bootstrapAddrs []peer.MultiAddr
	logger         *logrus.Logger
	store          store.KVStore
}

func New(logger *logrus.Logger, cap int, timeout time.Duration, store store.KVStore, bootstrapAddrs []string) tau.Task {
	addrs := make([]peer.MultiAddr, len(bootstrapAddrs))
	for i, addr := range bootstrapAddrs {
		multiAddr, err := peer.NewMultiAddr(addr, 0, [65]byte{})
		if err != nil {
			logger.Fatal("invalid bootstrap addresses", err)
		}
		addrs[i] = multiAddr
	}
	p2p := &P2P{
		timeout:        timeout,
		logger:         logger,
		store:          store,
		bootstrapAddrs: addrs,
	}
	return tau.New(tau.NewIO(cap), p2p)
}

func (p2p *P2P) Reduce(message tau.Message) tau.Message {
	switch message := message.(type) {
	case Tick:
		return p2p.handleTick(message)
	case rpc.QueryPeersMessage:
		return p2p.handleQuery(message)
	default:
		panic(fmt.Errorf("unexpected message type %T", message))
	}
}

func (p2p *P2P) handleTick(message tau.Message) tau.Message {
	// TODO: FIX THE VERSION AND ID.
	request := jsonrpc.JSONRequest{
		JSONRPC: "2.0",
		Version: "0.1",
		Method:  jsonrpc.MethodQueryPeers,
		ID:      rand.Int31(),
	}

	go func() {
		co.ParForAll(p2p.bootstrapAddrs, func(i int) {
			multi := p2p.bootstrapAddrs[i]
			client := jrpc.NewClient(p2p.timeout)
			addr := multi.ResolveTCPAddr().(*net.TCPAddr)
			addr.Port = 18515
			response, err := client.Call(fmt.Sprintf("http://%v", addr.String()), request)
			if err != nil {
				p2p.logger.Errorf("bootstrap node = %v is offline, err = %v", multi.Addr().String(), err)
				if err := p2p.store.Delete(multi.Addr().String()); err != nil {
					p2p.logger.Errorf("fail to delete entry from KVStore, %v", err)
				}
				return
			}
			if err := p2p.store.Write(multi.Addr().String(), multi); err != nil {
				p2p.logger.Errorf("fail to add new entry to KVStore, %v", err)
				return
			}
			if response.Error != nil {
				// todo : handle error
			}

			var result jsonrpc.QueryPeersResponse
			if err := json.Unmarshal(response.Result, &result); err != nil {
				p2p.logger.Errorf("invalid QueryPeersResponse from node %v, %v", multi.Addr().String(), err)
				return
			}
			for _, node := range result.Peers {
				multiAddr, err := peer.NewMultiAddr(node, 0, [65]byte{})
				if err != nil {
					p2p.logger.Errorf("invalid QueryPeersResponse from node %v, %v", multi.Addr().String(), err)
					return
				}
				if err := p2p.store.Write(multiAddr.Addr().String(), multiAddr); err != nil {
					p2p.logger.Errorf("fail to add new entry to KVStore, %v", err)
					return
				}
			}
		})
		p2p.logger.Infof("connecting to %v darknodes", p2p.store.Entries())
	}()

	return nil
}

func (p2p *P2P) handleQuery(message rpc.QueryPeersMessage) tau.Message {
	var response jsonrpc.Response
	var responder chan<- jsonrpc.Response
	switch request := message.Request.(type) {
	case jsonrpc.QueryPeersRequest:
		iter := p2p.store.Iterator()
		addresses := make([]string, 0, p2p.store.Entries())
		for iter.Next() {
			var value peer.MultiAddr
			_, err := iter.KV(value)
			if err != nil {
				p2p.logger.Error("cannot read key-value from the multiAddr store iterator.")
			}
			addresses = append(addresses, value.Value())
		}

		indexes := rand.Perm(len(addresses))
		randAddress := make([]string, 5)
		for i := range randAddress {
			randAddress[i] = addresses[indexes[i]]
		}
		response = jsonrpc.QueryPeersResponse{
			Peers: randAddress,
		}
		responder = request.Responder
	case jsonrpc.QueryNumPeersRequest:
		entries := p2p.store.Entries()
		response = jsonrpc.QueryNumPeersResponse{
			NumPeers: entries,
		}
		responder = request.Responder
	default:
		panic("unknown request type to p2p task")
	}

	select {
	case responder <- response:
		return nil
	case <-time.After(time.Second):
		p2p.logger.Debug("fail to write response to responder channel")
		return nil
	}
}

type Tick struct {
}

func (message Tick) IsMessage() {
}
