package dispatcher

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/renproject/darknode/addr"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/kv/db"
	"github.com/renproject/lightnode/client"
	"github.com/renproject/lightnode/server"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

type Dispatcher struct {
	logger     logrus.FieldLogger
	timeout    time.Duration
	multiStore db.Iterable
}

func New(logger logrus.FieldLogger, timeout time.Duration, multiStore db.Iterable, opts phi.Options) phi.Task {
	return phi.New(
		&Dispatcher{
			logger:     logger,
			timeout:    timeout,
			multiStore: multiStore,
		},
		opts,
	)
}

func (dispatcher *Dispatcher) Handle(_ phi.Task, message phi.Message) {
	msg, ok := message.(server.RequestWithResponder)
	if !ok {
		dispatcher.logger.Panicf("[dispatcher] unexpected message type %T", message)
	}

	addrs := dispatcher.multiAddrs(msg.Request.Method)
	responses := make(chan jsonrpc.Response, len(addrs))
	resIter := responseIterator(msg.Request.Method)

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

func (dispatcher *Dispatcher) multiAddrs(method string) addr.MultiAddresses {
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
		return dispatcher.AddrsRandom(3)
	case jsonrpc.MethodQueryBlocks:
		return dispatcher.AddrsRandom(3)
	case jsonrpc.MethodSubmitTx:
		return dispatcher.AddrsAll()
	case jsonrpc.MethodQueryTx:
		return dispatcher.AddrsRandom(3)
	case jsonrpc.MethodQueryNumPeers:
		return dispatcher.AddrsRandom(3)
	case jsonrpc.MethodQueryPeers:
		return dispatcher.AddrsRandom(3)
	case jsonrpc.MethodQueryEpoch:
		return dispatcher.AddrsRandom(3)
	case jsonrpc.MethodQueryStat:
		return dispatcher.AddrsRandom(3)
	default:
		dispatcher.logger.Panicf("[dispatcher] unsupported method %s encountered which should have been rejected by the validator", method)
		panic("unreachable")
	}
}

func (dispatcher *Dispatcher) AddrsAll() addr.MultiAddresses {
	addrs := addr.MultiAddresses{}
	for iter := dispatcher.multiStore.Iterator(); iter.Next(); {
		str, err := iter.Key()
		if err != nil {
			panic("[dispatcher] iterator invariant violated")
		}
		address, err := addr.NewMultiAddressFromString(str)
		if err != nil {
			panic("[dispatcher] incorrectly stored multi address")
		}
		addrs = append(addrs, address)
	}

	return addrs
}

func (dispatcher *Dispatcher) AddrsRandom(n int) addr.MultiAddresses {
	addrs := dispatcher.AddrsAll()

	rand.Shuffle(len(addrs), func(i, j int) {
		addrs[i], addrs[j] = addrs[j], addrs[i]
	})

	if len(addrs) < n {
		return addrs
	}
	return addrs[:n]
}

func responseIterator(method string) ResponseIterator {
	// TODO: The following is an initial choice of response aggregation
	// policies, which are likely to not be what we use long term. These should
	// be updated when these policies have been decided in more detail.
	switch method {
	case jsonrpc.MethodQueryBlock:
		return NewFirstResponseIterator()
	case jsonrpc.MethodQueryBlocks:
		return NewFirstResponseIterator()
	case jsonrpc.MethodSubmitTx:
		// TODO: This should instead return an iterator that will check for a
		// threshold of consistent responses.
		return NewFirstResponseIterator()
	case jsonrpc.MethodQueryTx:
		return NewFirstResponseIterator()
	case jsonrpc.MethodQueryNumPeers:
		return NewFirstResponseIterator()
	case jsonrpc.MethodQueryPeers:
		return NewFirstResponseIterator()
	case jsonrpc.MethodQueryEpoch:
		return NewFirstResponseIterator()
	case jsonrpc.MethodQueryStat:
		return NewFirstResponseIterator()
	default:
		panic(fmt.Sprintf("[dispatcher] unsupported method %s encountered which should have been rejected by the validator", method))
	}
}

type ResponseIterator interface {
	update(jsonrpc.Response, bool) (bool, jsonrpc.Response)
}

type FirstResponseIterator struct{}

func NewFirstResponseIterator() ResponseIterator {
	return FirstResponseIterator{}
}

func (FirstResponseIterator) update(res jsonrpc.Response, final bool) (bool, jsonrpc.Response) {
	return true, res
}
