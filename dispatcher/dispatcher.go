package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/renproject/darknode/addr"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/lightnode/http"
	"github.com/renproject/lightnode/store"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

// A Dispatcher is a task that is responsible for taking a request, sending it
// to a subset of the darknodes, waiting for the corresponding results, and the
// finally aggregating the results into a single result to be returned to the
// client of the lightnode. The addresses of known darknodes are stored in a
// store that is shared by the `Updater`, which will periodically update the
// store so that the addresses of the known darkndoes are kept up to date.
type Dispatcher struct {
	logger     logrus.FieldLogger
	client     http.Client
	multiStore store.MultiAddrStore
}

// New constructs a new `Dispatcher`.
func New(logger logrus.FieldLogger, timeout time.Duration, multiStore store.MultiAddrStore, opts phi.Options) phi.Task {
	return phi.New(
		&Dispatcher{
			logger:     logger,
			client:     http.NewClient(timeout),
			multiStore: multiStore,
		},
		opts,
	)
}

// Handle implements the `phi.Handler` interface.
func (dispatcher *Dispatcher) Handle(_ phi.Task, message phi.Message) {
	msg, ok := message.(http.RequestWithResponder)
	if !ok {
		dispatcher.logger.Panicf("[dispatcher] unexpected message type %T", message)
	}

	var addrs addr.MultiAddresses
	var err error
	id := msg.Query.Get("id")
	if id != "" {
		addrs, err = dispatcher.multiAddr(id)
	} else {
		addrs, err = dispatcher.multiAddrs(msg.Method)
	}
	if err != nil {
		dispatcher.logger.Errorf("[dispatcher] fail to send %v message to [%v], error getting multi-address: %v", msg.Method, id, err)
		msg.RespondWithErr(jsonrpc.ErrorCodeInternal, err)
		return
	}

	// Send the request to the darknodes and pipe the response to the iterator
	ctx, cancel := context.WithCancel(msg.Context)
	responses := make(chan jsonrpc.Response, len(addrs))
	resIter := dispatcher.newResponseIter(msg.Method)

	go func() {
		phi.ParForAll(addrs, func(i int) {
			address := fmt.Sprintf("http://%s:%v", addrs[i].IP4(), addrs[i].Port()+1)
			params, err := json.Marshal(msg.Params)
			if err != nil {
				return
			}
			req := jsonrpc.Request{
				Version: "2.0",
				ID:      msg.ID,
				Method:  msg.Method,
				Params:  params,
			}
			response, err := dispatcher.client.SendRequest(ctx, address, req, nil)
			if err != nil {
				return
			}
			responses <- response
		})
		close(responses)
	}()

	go func() {
		msg.Responder <- resIter.Collect(msg.ID, cancel, responses)
	}()
}

// multiAddrs returns the multi-address for the given Darknode ID.
func (dispatcher *Dispatcher) multiAddr(darknodeID string) (addr.MultiAddresses, error) {
	multi, err := dispatcher.multiStore.Address(darknodeID)
	if err != nil {
		return nil, err
	}
	return addr.MultiAddresses{multi}, nil
}

// multiAddrs returns the multi-addresses for the Darknodes based on the given
// method.
func (dispatcher *Dispatcher) multiAddrs(method string) (addr.MultiAddresses, error) {
	switch method {
	case jsonrpc.MethodSubmitTx:
		return dispatcher.multiStore.RandomAddresses(3, true)
	case jsonrpc.MethodQueryTx:
		return dispatcher.multiStore.BootstrapAddresses(), nil
	case jsonrpc.MethodQueryStat:
		return dispatcher.multiStore.RandomAddresses(3, false)
	default:
		return dispatcher.multiStore.RandomAddresses(5, true)
	}
}

// newResponseIter returns the iterator type for the given method.
func (dispatcher *Dispatcher) newResponseIter(method string) Iterator {
	switch method {
	case jsonrpc.MethodQueryTx:
		return NewMajorityResponseIterator(dispatcher.logger)
	default:
		return NewFirstResponseIterator()
	}
}
