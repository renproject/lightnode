package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
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

	// Initialize address map for updating/deleting
	addrsToUpdate := []addr.MultiAddress{}
	idsToDelete := []string{}

	// Query 50 random addresses from store
	mu := new(sync.Mutex)
	addrsToDecide := map[string]map[string]struct{}{}
	randAddrs := updater.multiStore.RandomAddresses(50)
	phi.ParForAll(randAddrs, func(i int) {
		multi := randAddrs[i]

		// Send request to the node to retrieve its peers.
		params, err := json.Marshal(jsonrpc.ParamsQueryPeers{})
		if err != nil {
			updater.logger.Errorf("cannot marshal query peers params: %v", err)
			return
		}
		request := jsonrpc.Request{
			Version: "2.0",
			ID:      rand.Int31(),
			Method:  jsonrpc.MethodQueryPeers,
			Params:  params,
		}

		address := fmt.Sprintf("http://%s:%v", randAddrs[i].IP4(), randAddrs[i].Port()+1)
		response, err := updater.client.SendRequest(queryCtx, address, request, nil)
		if err != nil {
			mu.Lock()
			defer mu.Unlock()
			idsToDelete = append(idsToDelete, multi.ID().String())
			return
		}

		// Parse the response
		raw, err := json.Marshal(response.Result)
		if err != nil {
			updater.logger.Errorf("[updater] error marshaling queryPeers result: %v", err)
			return
		}
		var resp jsonrpc.ResponseQueryPeers
		if err := json.Unmarshal(raw, &resp); err != nil {
			updater.logger.Warnf("[updater] cannot unmarshal queryPeers result from %v: %v", multi.String(), err)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		for _, peer := range resp.Peers {
			multiAddr, err := addr.NewMultiAddressFromString(peer)
			if err != nil {
				continue
			}
			addrsForSameID := addrsToDecide[multiAddr.ID().String()]
			if addrsForSameID == nil {
				addrsForSameID = map[string]struct{}{}
			}
			addrsForSameID[peer] = struct{}{}
			addrsToDecide[multiAddr.ID().String()] = addrsForSameID
		}
	})

	for _, multis := range addrsToDecide{
		// If we only get one multiAddress for the id, we simply add that to our store
		// It would take a long time to ping each of them to check if they're online.
		if len(multis) == 1 {
			for peer := range multis{
				multi, err := addr.NewMultiAddressFromString(peer)
				if err != nil {
					return
				}
				addrsToUpdate = append(addrsToUpdate, multi)

				// Remove it from the store so that only ids with more than 1 multiAddress
				// are stored in `addrsToDecide`
				delete(addrsToDecide, multi.ID().String())
			}
		}
	}

	// Ping different multiAddress of the same id to see which is actually online
	phi.ParForAll(addrsToDecide, func(key string) {
		multis := addrsToDecide[key]
		pingCtx, pingCancel := context.WithTimeout(queryCtx, time.Second)
		defer pingCancel()

		phi.ForAll(multis, func(key string) {
			multi, err := addr.NewMultiAddressFromString(key)
			if err != nil {
				return
			}

			// Send request to the node to retrieve its peers.
			request := jsonrpc.Request{
				Version: "2.0",
				ID:      1,
				Method:  jsonrpc.MethodQueryStat,
				Params:  json.RawMessage("{}"),
			}

			address := fmt.Sprintf("http://%s:%v", multi.IP4(), multi.Port()+1)
			response, err := updater.client.SendRequest(pingCtx, address, request, nil)
			if err != nil {
				return
			}
			if response.Error != nil {
				return
			}

			pingCancel()
			mu.Lock()
			defer mu.Unlock()
			addrsToUpdate = append(addrsToUpdate, multi)
		})
	})

	// Update store with new addresses
	if err := updater.multiStore.Insert(addrsToUpdate); err != nil {
		updater.logger.Errorf("cannot update new addresses: %v", err)
		return
	}

	// Delete non-responsive addresses
	if err := updater.multiStore.Delete(idsToDelete); err != nil {
		updater.logger.Errorf("cannot delete non-responsive addresses: %v", err)
		return
	}

	// Print how many nodes we have connected to.
	size := updater.multiStore.Size()
	updater.logger.Infof("connected to %v nodes", size)
}

func (updater Updater) isBootstrap(addr addr.MultiAddress) bool {
	for i := range updater.bootstrap {
		if updater.bootstrap[i].ID().Equal(addr.ID()) {
			return true
		}
	}

	return false
}
