package updater

import (
	"context"
	"encoding/json"
	"math/rand"
	"time"

	"github.com/renproject/darknode/addr"
	"github.com/renproject/darknode/jsonrpc"
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
	logger         logrus.FieldLogger
	bootstrapAddrs addr.MultiAddresses
	multiStore     store.MultiAddrStore
	pollRate       time.Duration
	timeout        time.Duration
}

// New constructs a new `Updater`. If the given store of multi addresses is
// empty, then the constructed `Updater` will be useless since it will not know
// any darknodes to query. Therefore the given store must contain some number
// of bootstrap addresses.
func New(logger logrus.FieldLogger, bootstrapAddrs addr.MultiAddresses, multiStore store.MultiAddrStore, pollRate, timeout time.Duration) Updater {
	return Updater{
		logger:         logger,
		bootstrapAddrs: bootstrapAddrs,
		multiStore:     multiStore,
		pollRate:       pollRate,
		timeout:        timeout,
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

	addrs, err := updater.getQueryAddresses()
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
		response, err := client.SendToDarknode(client.URLFromMulti(multi), request, updater.timeout)
		if err != nil {
			updater.logger.Warnf("[updater] cannot connect to node %v: %v", multi.String(), err)

			// Delete address if it is not a Boostrap node and we do not receive
			// a response.
			isBootstrap := false
			for _, bootstrapAddr := range updater.bootstrapAddrs {
				if multi.String() == bootstrapAddr.String() {
					isBootstrap = true
				}
			}

			if !isBootstrap {
				updater.multiStore.Delete(multi)
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
			updater.logger.Errorf("[updater] could not unmarshal into expected result type: %v", err)
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

func (updater *Updater) getQueryAddresses() (addr.MultiAddresses, error) {
	// TODO: Should this be a constant number of random addresses always? If
	// so, is this the right constant?
	return updater.multiStore.AddrsRandom(3)
}
