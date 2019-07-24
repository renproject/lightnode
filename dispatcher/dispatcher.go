package dispatcher

import (
	"encoding/json"
	"fmt"
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
	resIter := dispatcher.responseIterator(msg.Request.Method)

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
	// TODO: Implement method based address fetching.
	iter := dispatcher.multiStore.Iterator()
	if !iter.Next() {
		panic("[dispatcher] empty address store")
	}
	str, err := iter.Key()
	if err != nil {
		panic("[dispatcher] empty address store")
	}
	address, err := addr.NewMultiAddressFromString(str)
	if err != nil {
		panic("[dispatcher] incorrectly stored multi address")
	}
	return addr.MultiAddresses{address}
}

func (dispatcher *Dispatcher) responseIterator(method string) ResponseIterator {
	// TODO: Implement method based result iterator return values.
	return NewFirstResponseIterator()
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
