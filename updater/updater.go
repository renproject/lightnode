package updater

import (
	"context"
	"encoding/json"
	"math/rand"
	"time"

	"github.com/renproject/darknode/addr"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/p2p"
	"github.com/renproject/lightnode/client"
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
	logger     logrus.FieldLogger
	multiStore store.MultiAddrStore
	pollRate   time.Duration
	timeout    time.Duration
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
		timeout:    timeout,
	}
}

// Run starts the `Updater` making requests to the darknodes and updating its
// store. This function is blocking.
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

	addrs := updater.getQueryAddresses()

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
			updater.multiStore.Delete(multi)
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
			if err := updater.multiStore.Insert(multiAddr); err != nil {
				updater.logger.Errorf("cannot add multi-address to store: %v", err)
				return
			}
		}
	})
}

func (updater *Updater) getQueryAddresses() addr.MultiAddresses {
	// TODO: Should this be a constant number of random addresses always? If
	// so, is this the right constant?
	return updater.multiStore.AddrsRandom(3)
}
