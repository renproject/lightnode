package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
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

	var addrs []wire.Address
	var err error
	id := msg.Query.Get("id")
	if id != "" {
		addrs, err = dispatcher.multiAddr(id)
	} else {
		addrs, err = dispatcher.multiAddrs(msg.Method)
	}
	if err != nil {
		dispatcher.logger.Errorf("[dispatcher] sending %v request to [%v]: getting multi-address: %v", msg.Method, id, err)
		msg.RespondWithErr(jsonrpc.ErrorCodeInternal, err)
		return
	}

	// Send the request to the darknodes and pipe the response to the iterator
	ctx, cancel := context.WithCancel(msg.Context)
	responses := make(chan jsonrpc.Response, len(addrs))
	resIter := dispatcher.newResponseIter(msg.Method)

	go func() {
		phi.ParForAll(addrs, func(i int) {
			addrParts := strings.Split(addrs[i].Value, ":")
			if len(addrParts) != 2 {
				dispatcher.logger.Errorf("[dispatcher] invalid address value=%v: %v", addrs[i].Value, err)
				return
			}
			port, err := strconv.Atoi(addrParts[1])
			if err != nil {
				dispatcher.logger.Errorf("[dispatcher] invalid port=%v: %v", addrParts[1], err)
				return
			}
			addrString := fmt.Sprintf("http://%s:%v", addrParts[0], port+1)
			params, err := json.Marshal(msg.Params)
			if err != nil {
				dispatcher.logger.Errorf("[dispatcher] invalid params=%v: %v", msg.Params, err)
				return
			}
			req := jsonrpc.Request{
				Version: "2.0",
				ID:      msg.ID,
				Method:  msg.Method,
				Params:  params,
			}
			response, err := dispatcher.client.SendRequest(ctx, addrString, req, nil)
			if err != nil {
				dispatcher.logger.Errorf("[dispatcher] sending %v request: %v", msg.Method, err)
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
func (dispatcher *Dispatcher) multiAddr(darknodeID string) ([]wire.Address, error) {
	multi, err := dispatcher.multiStore.Get(darknodeID)
	if err != nil {
		return nil, err
	}
	return []wire.Address{multi}, nil
}

// multiAddrs returns the multi-addresses for the Darknodes based on the given
// method.
func (dispatcher *Dispatcher) multiAddrs(method string) ([]wire.Address, error) {
	switch method {
	case jsonrpc.MethodSubmitTx:
		return dispatcher.multiStore.RandomBootstrapAddrs(3)
	case jsonrpc.MethodQueryTx:
		return dispatcher.multiStore.BootstrapAll()
	case jsonrpc.MethodQueryStat:
		return dispatcher.multiStore.RandomAddrs(3)
	default:
		return dispatcher.multiStore.RandomBootstrapAddrs(5)
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
