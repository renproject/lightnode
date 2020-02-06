package dispatcher

import (
	"context"
	"fmt"
	"log"
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
	if msg.DarknodeID != "" {
		addrs, err = dispatcher.multiAddr(msg.Request.Method, msg.DarknodeID)
	} else {
		addrs, err = dispatcher.multiAddrs(msg.Request.Method)
	}
	if err != nil {
		dispatcher.logger.Panicf("[dispatcher] error getting multi-address: %v", err)
		return
	}

	// Send the request to the darknodes and pipe the response to the iterator
	ctx, cancel := context.WithCancel(msg.Context)
	responses := make(chan jsonrpc.Response, len(addrs))
	resIter := dispatcher.newResponseIter(msg.Request.Method)

	var retryOptions *http.RetryOptions
	if msg.Request.Method == jsonrpc.MethodSubmitTx {
		retryOptions = &http.DefaultRetryOptions
	}

	go func() {
		phi.ParForAll(addrs, func(i int) {
			addr := fmt.Sprintf("http://%s:%v", addrs[i].IP4(), addrs[i].Port()+1)
			response, err := dispatcher.client.SendRequest(ctx, addr, msg.Request, retryOptions)
			if err != nil {
				errMsg := fmt.Errorf("lightnode could not forward request to darknode: %v", err)
				jsonErr := jsonrpc.NewError(http.ErrorCodeForwardingError, errMsg.Error(), nil)
				response = jsonrpc.NewResponse(msg.Request.ID, nil, &jsonErr)
			}
			responses <- response
			if msg.Request.Method == jsonrpc.MethodSubmitTx {
				if err != nil || response.Error != nil {
					log.Printf("😿 fail to send to darknode = %v, err = %v, jsonErr = %v", addr, err, response.Error)
				} else {
					log.Printf("✅ successfully send request to darknode = %v", addr)
				}
			}
		})
		close(responses)
	}()

	go func() {
		msg.Responder <- resIter.Collect(msg.Request.ID, cancel, responses)
	}()
}

// multiAddrs returns the multi-address for the given Darknode ID.
func (dispatcher *Dispatcher) multiAddr(method string, darknodeID string) (addr.MultiAddresses, error) {
	multi, err := dispatcher.multiStore.Get(darknodeID)
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
		return dispatcher.multiStore.RandomBootstrapAddrs(2)
	case jsonrpc.MethodQueryTx:
		return dispatcher.multiStore.BootstrapAll()
	case jsonrpc.MethodQueryStat:
		return dispatcher.multiStore.RandomAddrs(3)
	default:
		return dispatcher.multiStore.RandomBootstrapAddrs(3)
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
