package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/renproject/aw/wire"
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
	bootstraps []wire.Address
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
		updater.logger.Errorf("[updater] cannot marshal query peers params: %v", err)
		return
	}
	addrs, err := updater.multiStore.BootstrapAll()
	if err != nil {
		updater.logger.Errorf("[updater] cannot get query addresses: %v", err)
		return
	}

	// Collect all peers connected to Bootstrap nodes.
	phi.ParForAll(addrs, func(i int) {
		multi := addrs[i]

		// Send request to the node to retrieve its peers.
		request := jsonrpc.Request{
			Version: "2.0",
			ID:      rand.Int31(),
			Method:  jsonrpc.MethodQueryPeers,
			Params:  params,
		}

		addrParts := strings.Split(addrs[i].Value, ":")
		if len(addrParts) != 2 {
			updater.logger.Errorf("[updater] invalid address value=%v", addrs[i].Value)
			return
		}
		port, err := strconv.Atoi(addrParts[1])
		if err != nil {
			updater.logger.Errorf("[updater] invalid port=%v", addrParts[1])
			return
		}
		addrString := fmt.Sprintf("http://%s:%v", addrParts[0], port+1)
		response, err := updater.client.SendRequest(queryCtx, addrString, request, nil)
		if err != nil {
			updater.logger.Warnf("[updater] cannot connect to node %v: %v", multi.String(), err)
			if !updater.isBootstrap(multi) {
				if err := updater.multiStore.Delete(multi); err != nil {
					updater.logger.Warnf("[updater] cannot delete multi address from db : %v", err)
				}
			}
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
		for _, peer := range resp.Peers {
			addr, err := wire.DecodeString(peer)
			if err != nil {
				updater.logger.Errorf("[updater] failed to decode multi-address: %v", err)
				continue
			}
			if err := updater.multiStore.Insert(addr); err != nil {
				updater.logger.Errorf("[updater] failed to add multi-address to store: %v", err)
				return
			}
		}
	})

	// Print how many nodes we have connected to.
	size, err := updater.multiStore.Size()
	if err != nil {
		updater.logger.Errorf("cannot get query addresses: %v", err)
		return
	}
	updater.logger.Infof("connected to %v nodes", size)
}

func (updater Updater) isBootstrap(addr wire.Address) bool {
	for i := range updater.bootstraps {
		if updater.bootstraps[i].Value == addr.Value {
			return true
		}
	}

	return false
}
