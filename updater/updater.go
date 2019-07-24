package updater

import (
	"context"
	"encoding/json"
	"math/rand"
	"time"

	"github.com/renproject/darknode/addr"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/p2p"
	"github.com/renproject/kv/db"
	"github.com/renproject/lightnode/client"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

type Updater struct {
	logger         logrus.FieldLogger
	bootstrapAddrs addr.MultiAddresses
	multiStore     db.Iterable
	pollRate       time.Duration
	timeout        time.Duration
}

func New(logger logrus.FieldLogger, bootstrapAddrs addr.MultiAddresses, multiStore db.Iterable, pollRate, timeout time.Duration) Updater {
	return Updater{
		logger:         logger,
		bootstrapAddrs: bootstrapAddrs,
		multiStore:     multiStore,
		pollRate:       pollRate,
		timeout:        timeout,
	}
}

func (updater *Updater) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			updater.updateMultiAddress()
			time.Sleep(updater.pollRate)
		}
	}
}

func (updater *Updater) updateMultiAddress() {
	params, err := json.Marshal(jsonrpc.ParamsQueryPeers{})
	if err != nil {
		updater.logger.Errorf("cannot marshal query peers params: %v", err)
		return
	}

	addrs := updater.GetQueryAddresses()

	phi.ParForAll(addrs, func(i int) {
		multi := addrs[i]

		// Send request to the node to retrieve its peers.
		request := jsonrpc.Request{
			Version: "2.0",
			ID:      rand.Int31(),
			Method:  jsonrpc.MethodQueryPeers,
			Params:  params,
		}
		response, err := client.SendToDarknode(client.URLFromMulti(multi), request, updater.timeout)

		// TODO: Maybe we shouldn't always delete an address when we can't
		// query it; probably put some more intelligent logic here.
		if err != nil {
			updater.logger.Warnf("cannot connect to node %v: %v", multi.String(), err)
			updater.deleteMultiAddr(multi)
			return
		}

		// Parse the response and write any multi-addresses returned by the node to the store.
		resp, ok := response.Result.(p2p.QueryPeersResponse)
		if !ok {
			updater.logger.Panicf("unexpected response type %T", response.Result)
		}
		for _, node := range resp.MultiAddresses {
			multiAddr, err := addr.NewMultiAddressFromString(node.String())
			if err != nil {
				updater.logger.Errorf("invalid QueryPeersResponse from node %v: %v", multi.String(), err)
				return
			}
			if err := updater.insertMultiAddr(multiAddr); err != nil {
				updater.logger.Errorf("cannot add multi-address to store: %v", err)
				return
			}
		}
	})
}

func (updater *Updater) GetQueryAddresses() addr.MultiAddresses {
	// TODO: Select a random subset of known addresses.
	return updater.bootstrapAddrs
}

func (updater *Updater) insertMultiAddr(addr addr.MultiAddress) error {
	return updater.multiStore.Insert(addr.String(), []byte(addr.String()))
}

func (updater *Updater) deleteMultiAddr(addr addr.MultiAddress) {
	// NOTE: The `Delete` function always returns a nil error, so we ignore it.
	_ = updater.multiStore.Delete(addr.String())
}
