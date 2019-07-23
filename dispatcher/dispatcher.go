package dispatcher

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/renproject/lightnode/server"
	"github.com/renproject/phi"
	"github.com/republicprotocol/co-go"
	"github.com/republicprotocol/darknode-go/addr"
	"github.com/republicprotocol/darknode-go/jsonrpc"
	"github.com/sirupsen/logrus"
)

type Dispatcher struct {
	logger  logrus.FieldLogger
	addrs   addr.MultiAddresses
	timeout time.Duration
}

func New(logger logrus.FieldLogger, timeout time.Duration, opts phi.Options) phi.Task {
	return phi.New(
		&Dispatcher{
			logger:  logger,
			addrs:   addr.MultiAddresses{},
			timeout: timeout,
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
		co.ParForAll(addrs, func(i int) {
			response, err := dispatcher.sendToDarknode(addrs[i], msg.Request)
			if err != nil {
				// TODO: Return more appropriate error message.
				responses <- jsonrpc.Response{}
			} else {
				responses <- response
			}
		})
		close(responses)
	}()

	i := 1
	for res := range responses {
		done, response := resIter.update(res, i == len(addrs))
		if done {
			msg.Responder <- response
			return
		}
		i++
	}

	// TODO: Return more appropriate error response.
	msg.Responder <- jsonrpc.Response{}
}

func (dispatcher *Dispatcher) multiAddrs(method string) addr.MultiAddresses {
	// TODO: Implement method based address fetching.
	return addr.MultiAddresses{dispatcher.addrs[0]}
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

func (dispatcher *Dispatcher) sendToDarknode(addr addr.MultiAddress, req jsonrpc.Request) (jsonrpc.Response, error) {
	httpClient := new(http.Client)
	httpClient.Timeout = dispatcher.timeout

	// FIXME: This will give the wrong port, we need to instead use the jsonrpc
	// port.
	netAddr := addr.NetworkAddress()
	url := "http://" + netAddr.String()

	// Construct HTTP request.
	body, err := json.Marshal(req)
	if err != nil {
		return jsonrpc.Response{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), httpClient.Timeout)
	defer cancel()
	r, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return jsonrpc.Response{}, err
	}
	r = r.WithContext(ctx)
	r.Header.Set("Content-Type", "application/json")

	// Read response.
	response, err := httpClient.Do(r)
	if err != nil {
		return jsonrpc.Response{}, err
	}

	var resp jsonrpc.Response
	err = json.NewDecoder(response.Body).Decode(&resp)
	return resp, err
}
