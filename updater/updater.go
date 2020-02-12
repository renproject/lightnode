package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/renproject/darknode/addr"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/lightnode/http"
	"github.com/renproject/lightnode/store"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

// An Updater is a task responsible for querying the darknodes periodically to
// know which darknodes are in the network. It does this by requesting the
// peers of a random subset of the already known darknodes and adding any new
// darknodes to a store. This store is shared by the `Dispatcher`, which needs
// to know about the darknodes in the network.
type Updater struct {
	bootstrap  addr.MultiAddresses
	logger     logrus.FieldLogger
	multiStore store.MultiAddrStore
	client     http.Client
	pollRate   time.Duration
}

// New constructs a new `Updater`. If the given store of multi addresses is
// empty, then the constructed `Updater` will be useless since it will not know
// any darknodes to query. Therefore the given store must contain some number
// of bootstrap addresses.
func New(logger logrus.FieldLogger, multiStore store.MultiAddrStore, pollRate, timeout time.Duration) Updater {
	return Updater{
		logger:     logger,
		multiStore: multiStore,
		pollRate:   pollRate,
		client:     http.NewClient(timeout),
	}
}

// Run starts the `Updater` making requests to the darknodes and updating its
// store. This function is blocking.
func (updater *Updater) Run(ctx context.Context) {
	ticker := time.NewTicker(updater.pollRate)
	defer ticker.Stop()

	updater.updateMultiAddress(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			updater.updateMultiAddress(ctx)
		}
	}
}

func (updater *Updater) updateMultiAddress(ctx context.Context) {
	queryCtx, cancel := context.WithTimeout(ctx, updater.pollRate)
	defer cancel()

	params, err := json.Marshal(jsonrpc.ParamsQueryPeers{})
	if err != nil {
		updater.logger.Errorf("cannot marshal query peers params: %v", err)
		return
	}
	addrs, err := updater.multiStore.RandomAddrs(3)
	if err != nil {
		updater.logger.Errorf("cannot get query addresses: %v", err)
		return
	}

	phi.ParForAll(addrs, func(i int) {
		multi := addrs[i]

		// Send request to the node to retrieve its peers.
		request := jsonrpc.Request{
			Version: "2.0",
			ID:      rand.Int31(),
			Method:  jsonrpc.MethodQueryPeers,
			Params:  params,
		}

		address := fmt.Sprintf("http://%s:%v", addrs[i].IP4(), addrs[i].Port()+1)
		response, err := updater.client.SendRequest(queryCtx, address, request, nil)
		if err != nil {
			updater.logger.Warnf("[updater] cannot connect to node %v: %v", multi.String(), err)
			if err := updater.multiStore.Delete(multi); err != nil {
				updater.logger.Warnf("[updater] cannot delete multi address from db : %v", err)
			}
			return
		}
		// Parse the response and write any multi-addresses returned by the node to the store.
		raw, err := json.Marshal(response.Result)
		if err != nil {
			updater.logger.Errorf("[updater] error marshaling and already unmarshaled result: %v", err)
			return
		}
		var resp jsonrpc.ResponseQueryPeers
		err = json.Unmarshal(raw, &resp)
		if err != nil {
			updater.logger.Warnf("[updater] cannot connect to node %v: %v", multi.String(), err)
			if !updater.isBootstrap(multi) {
				updater.multiStore.Delete(multi)
			}
			return
		}
		for _, peer := range resp.Peers {
			multiAddr, err := addr.NewMultiAddressFromString(peer)
			if err != nil {
				updater.logger.Errorf("[updater] failed to decode multi-address: %v", err)
				continue
			}
			if err := updater.multiStore.Insert(multiAddr); err != nil {
				updater.logger.Errorf("[updater] failed to add multi-address to store: %v", err)
				return
			}
		}
	})
}

func (updater Updater) isBootstrap(addr addr.MultiAddress) bool {
	for i := range updater.bootstrap {
		if updater.bootstrap[i].ID().Equal(addr.ID()) {
			return true
		}
	}

	return false
}
