package dispatcher

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/renproject/darknode/addr"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/lightnode/client"
	"github.com/renproject/lightnode/server"
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
	timeout    time.Duration
	multiStore store.MultiAddrStore
}

// New constructs a new `Dispatcher`.
func New(logger logrus.FieldLogger, timeout time.Duration, multiStore store.MultiAddrStore, opts phi.Options) phi.Task {
	return phi.New(
		&Dispatcher{
			logger:     logger,
			timeout:    timeout,
			multiStore: multiStore,
		},
		opts,
	)
}

// Handle implements the `phi.Handler` interface.
func (dispatcher *Dispatcher) Handle(_ phi.Task, message phi.Message) {
	msg, ok := message.(server.RequestWithResponder)
	if !ok {
		dispatcher.logger.Panicf("[dispatcher] unexpected message type %T", message)
	}

	addrs, err := dispatcher.multiAddrs(msg.Request.Method)
	if err != nil {
		dispatcher.logger.Panicf("[dispatcher] error getting multi-addresses: %v", err)
	}
	responses := make(chan jsonrpc.Response, len(addrs))
	resIter := newResponseIter(msg.Request.Method)

	go func() {
		phi.ParForAll(addrs, func(i int) {
			response, err := client.SendToDarknode(client.URLFromMulti(addrs[i]), msg.Request, dispatcher.timeout)
			if err != nil {
				errMsg := fmt.Sprintf("lightnode could not forward response to darknode: %v", err)
				err := jsonrpc.NewError(server.ErrorCodeForwardingError, errMsg, json.RawMessage{})
				response := jsonrpc.NewResponse(0, nil, &err)
				responses <- response
			} else {
				responses <- response
			}
		})
		close(responses)
	}()

	go func() {
		i := 1
		for res := range responses {
			done, response := resIter.update(res, i == len(addrs))
			if done {
				msg.Responder <- response
				return
			}
			i++
		}
	}()
}

func (dispatcher *Dispatcher) multiAddrs(method string) (addr.MultiAddresses, error) {
	// The method `Size` for a `memdb` always returns a nil error, so we ignore
	// it
	// NOTE: This is commented out for now but address selection policies used
	// in the future should make use of this number.
	// numDarknodes, _ := dispatcher.multiStore.Size()

	// TODO: The following is an initial choice of darknode selection policies,
	// which are likely to not be what we use long term. These should be
	// updated when these policies have been decided in more detail.
	switch method {
	case jsonrpc.MethodQueryBlock:
		return dispatcher.multiStore.AddrsRandom(3)
	case jsonrpc.MethodQueryBlocks:
		return dispatcher.multiStore.AddrsRandom(3)
	case jsonrpc.MethodSubmitTx:
		return dispatcher.multiStore.AddrsAll()
	case jsonrpc.MethodQueryTx:
		return dispatcher.multiStore.AddrsRandom(3)
	case jsonrpc.MethodQueryNumPeers:
		return dispatcher.multiStore.AddrsRandom(3)
	case jsonrpc.MethodQueryPeers:
		return dispatcher.multiStore.AddrsRandom(3)
	case jsonrpc.MethodQueryEpoch:
		return dispatcher.multiStore.AddrsRandom(3)
	case jsonrpc.MethodQueryStat:
		return dispatcher.multiStore.AddrsRandom(3)
	default:
		dispatcher.logger.Panicf("[dispatcher] unsupported method %s encountered which should have been rejected by the validator", method)
		panic("unreachable")
	}
}

func newResponseIter(method string) responseIterator {
	// TODO: The following is an initial choice of response aggregation
	// policies, which are likely to not be what we use long term. These should
	// be updated when these policies have been decided in more detail.
	switch method {
	case jsonrpc.MethodQueryBlock:
		return newFirstResponseIterator()
	case jsonrpc.MethodQueryBlocks:
		return newFirstResponseIterator()
	case jsonrpc.MethodSubmitTx:
		// TODO: This should instead return an iterator that will check for a
		// threshold of consistent responses.
		return newFirstResponseIterator()
	case jsonrpc.MethodQueryTx:
		return newFirstResponseIterator()
	case jsonrpc.MethodQueryNumPeers:
		return newFirstResponseIterator()
	case jsonrpc.MethodQueryPeers:
		return newFirstResponseIterator()
	case jsonrpc.MethodQueryEpoch:
		return newFirstResponseIterator()
	case jsonrpc.MethodQueryStat:
		return newFirstResponseIterator()
	default:
		panic(fmt.Sprintf("[dispatcher] unsupported method %s encountered which should have been rejected by the validator", method))
	}
}

type responseIterator interface {
	update(jsonrpc.Response, bool) (bool, jsonrpc.Response)
}

type firstResponseIterator struct{}

func newFirstResponseIterator() responseIterator {
	return firstResponseIterator{}
}

func (firstResponseIterator) update(res jsonrpc.Response, final bool) (bool, jsonrpc.Response) {
	return true, res
}
