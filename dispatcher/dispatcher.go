package dispatcher

import (
	"context"
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

	// Get the darknode multiAddresses where the request will be forwarded to.
	addrs, err := dispatcher.multiAddrs(msg.Request.Method, msg.DarknodeID)
	if err != nil {
		dispatcher.logger.Panicf("[dispatcher] error getting multi-address: %v", err)
		return
	}

	// Send the request to the darknodes and pipe the response to the iterator
	ctx, cancel := context.WithCancel(msg.Context)
	responses := make(chan jsonrpc.Response, len(addrs))
	resIter := dispatcher.newResponseIter(msg.Request.Method)

	go func() {
		phi.ParForAll(addrs, func(i int) {
			addr := fmt.Sprintf("http://%s:%v", addrs[i].IP4(), addrs[i].Port()+1)
			response, err := dispatcher.client.SendRequest(ctx, addr, msg.Request, &http.DefaultRetryOptions)
			if err != nil {
				errMsg := fmt.Errorf("lightnode could not forward request to darknode: %v", err)
				jsonErr := jsonrpc.NewError(http.ErrorCodeForwardingError, errMsg.Error(), nil)
				response = jsonrpc.NewResponse(msg.Request.ID, nil, &jsonErr)
			}
			responses <- response
		})
	}()

	go func() {
		msg.Responder <- resIter.Collect(msg.Request.ID, cancel, responses)
	}()
}

// multiAddrs returns multiAddresses of the darknodes we want to forward the
// request to according to the `method` and `darknodeID`.
func (dispatcher *Dispatcher) multiAddrs(method string, darknodeID string) (addr.MultiAddresses, error) {
	if darknodeID != "" {
		multi, err := dispatcher.multiStore.Get(darknodeID)
		if err != nil {
			return nil, err
		}
		return addr.MultiAddresses{multi}, nil
	}

	// TODO: The following is an initial choice of darknode selection policies,
	// which are likely to not be what we use long term. These should be
	// updated when these policies have been decided in more detail.
	switch method {
	case jsonrpc.MethodSubmitTx:
		// TODO: Eventually, we would want a more sophisticated way of sending
		// these messages.
		return dispatcher.multiStore.AddrsRandom(2)
	default:
		return dispatcher.multiStore.AddrsRandom(3)
	}
}

// newResponseIter returns the iterator to use depending on the given method.
func (dispatcher *Dispatcher) newResponseIter(method string) Iterator {
	// TODO: The following is an initial choice of response aggregation
	// policies, which are likely to not be what we use long term. These should
	// be updated when these policies have been decided in more detail.
	switch method {
	case jsonrpc.MethodQueryBlock, jsonrpc.MethodQueryBlocks, jsonrpc.MethodQueryTx:
		return NewMajorityResponseIterator(dispatcher.logger)
	default:
		return NewFirstResponseIterator()
	}
}
