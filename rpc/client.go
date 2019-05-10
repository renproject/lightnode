// Package rpc defines the client and server `tau.Task`s. The client starts background workers which handle sending
// requests to the Darknodes using the `Client` defined in jsonrpc/client.go. If the number of requests exceeds the
// number of background workers, they will remain on the channel until the buffer is exceeded (in which case the request
// is dropped).
package rpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"time"

	jrpc "github.com/renproject/lightnode/rpc/jsonrpc"
	"github.com/renproject/lightnode/store"
	"github.com/republicprotocol/co-go"
	"github.com/republicprotocol/darknode-go/rpc/jsonrpc"
	"github.com/republicprotocol/renp2p-go/foundation/addr"
	"github.com/republicprotocol/tau"
	"github.com/sirupsen/logrus"
)

var (
	// ErrNotEnoughResultsReturned is returned if we do not get enough successful responses from the Darknodes.
	ErrNotEnoughResultsReturned = errors.New("not enough results returned")

	// ErrTooManyRequests is returned when there is too much back pressure.
	ErrTooManyRequests = errors.New("dropping request: too much back pressure")
)

// Client is used to send RPC requests.
type Client struct {
	logger  logrus.FieldLogger
	store   store.Proxy
	queue   chan RPCCall
	timeout time.Duration
}

// NewClient returns a new Client task.
func NewClient(logger logrus.FieldLogger, store store.Proxy, cap, numWorkers int, timeout time.Duration) tau.Task {
	client := &Client{
		logger:  logger,
		store:   store,
		queue:   make(chan RPCCall, cap),
		timeout: timeout,
	}

	// Start running the background workers.
	go client.runWorkers(numWorkers)
	return tau.New(tau.NewIO(cap), client)
}

// Reduce implements the `tau.Task` interface.
func (client *Client) Reduce(message tau.Message) tau.Message {
	switch message := message.(type) {
	case InvokeRPC:
		return client.invoke(message)
	default:
		client.logger.Panicf("unexpected message type %T", message)
	}
	return nil
}

// invoke sends the given message to the target addresses. If the queue is full, the message will be dropped.
func (client *Client) invoke(message InvokeRPC) tau.Message {
	switch request := message.Request.(type) {
	case jsonrpc.SendMessageRequest:
		return client.handleMessage(request, jsonrpc.MethodSendMessage, message.Addresses)
	case jsonrpc.ReceiveMessageRequest:
		return client.handleMessage(request, jsonrpc.MethodReceiveMessage, message.Addresses)
	default:
		client.logger.Panicf("unexpected message type %T", request)
	}
	return nil
}

// runWorkers starts running a given number of workers. They each try to read from the request queue and send the
// request.
func (client *Client) runWorkers(n int) {
	co.ParForAll(n, func(i int) {
		// For each RPC call inside the queue.
		for call := range client.queue {
			// Construct a new JSON client and send the request.
			jsonClient := jrpc.NewClient(client.logger, client.timeout)
			response, err := jsonClient.Call(call.URL, call.Request)
			if err != nil {
				call.Responder <- nil
				client.logger.Warnf("cannot send message to %v: %v", call.URL, err)
				continue
			}

			// Unmarshal the response and write it to the responder channel. We assume the responder channel always has
			// a buffer size of 1.
			switch call.Request.Method {
			case jsonrpc.MethodSendMessage:
				var resp jsonrpc.SendMessageResponse
				if response.Error != nil {
					resp.Error = fmt.Errorf("received response error: %v", response.Error)
				} else if err := json.Unmarshal([]byte(response.Result), &resp); err != nil {
					call.Responder <- nil
					client.logger.Errorf("cannot unmarshal SendMessageResponse from Darknode: %v", err)
					continue
				}
				call.Responder <- resp
			case jsonrpc.MethodReceiveMessage:
				var resp jsonrpc.ReceiveMessageResponse
				if response.Error != nil {
					resp.Error = fmt.Errorf("%v", response.Error.Message)
				} else if err := json.Unmarshal([]byte(response.Result), &resp); err != nil {
					call.Responder <- nil
					client.logger.Errorf("cannot unmarshal ReceiveMessageResponse from Darknode: %v", err)
					continue
				}
				call.Responder <- resp
			}
		}
	})
}

func (client *Client) handleMessage(request jsonrpc.Request, method string, addresses []addr.Addr) tau.Message {
	data, err := json.Marshal(request)
	if err != nil {
		client.logger.Errorf("failed to marshal the SendMessageRequest: %v", err)
		return nil
	}
	results := make(chan jsonrpc.Response, len(addresses))
	tick := time.Tick(client.timeout)

	// We construct a JSON-RPC request for each target darknode and send the request to the queue.
	for _, address := range addresses {
		if err := client.handleRequest(method, data, address, results); err != nil {
			client.logger.Warnf("cannot send request to the worker, %v", err)
			continue
		}
	}

	switch method {
	case jsonrpc.MethodSendMessage:
		go client.handleSendMessageResults(request, tick, results, len(addresses))
	case jsonrpc.MethodReceiveMessage:
		go client.handleReceiveMessageResults(request, tick, results, len(addresses))
	default:
		client.logger.Panicf("unexpected request method %T", method)
	}

	return nil
}

// handleRequest sending the constructed request to the queue.
func (client *Client) handleRequest(method string, data []byte, address addr.Addr, results chan jsonrpc.Response) error {
	// Get multi-address of the darknode from store.
	multi, err := client.store.MultiAddress(address)
	if err != nil {
		return err
	}
	// We assume the JSON-RPC port = gRPC port + 1
	netAddr := multi.ResolveTCPAddr().(*net.TCPAddr)
	netAddr.Port += 1
	call := RPCCall{
		Request: jsonrpc.JSONRequest{
			JSONRPC: "2.0",
			Method:  method,
			Params:  data,
			ID:      rand.Int31(),
		},
		URL:       "http://" + netAddr.String(),
		Responder: results,
	}

	// Write each request to the queue.
	select {
	case client.queue <- call:
		return nil
	default:
		return ErrTooManyRequests
	}
}

// handleSendMessageResults reads and validates the responses from the workers. It will write a response to the user
// when we get enough results from the Darknodes. If it does not receive sufficient responses before a `tick` message is
// received, it writes an error.
func (client *Client) handleSendMessageResults(request jsonrpc.Request, tick <-chan time.Time, results chan jsonrpc.Response, total int) {
	messageIDs := map[string]int{}
	counter := 0
	req, ok := request.(jsonrpc.SendMessageRequest)
	if !ok {
		client.logger.Panicf("invalid request type: expected jsonrpc.SendMessageRequest, got %T", request)
	}

Loop:
	for {
		select {
		case <-tick:
			// The Darknodes failed to provide a response within the given time.
			break Loop
		case result := <-results:
			counter++
			if result == nil {
				if counter == total {
					// All of the Darknodes have returned a nil message.
					break Loop
				}
				continue
			}
			res := result.(jsonrpc.SendMessageResponse)
			if res.Error == nil && res.Ok {
				messageIDs[res.MessageID]++
			}

			// If the majority of the Darknodes return the same message, write it to the user.
			for id, num := range messageIDs {
				if num >= (total+1)*2/3 {
					req.Responder <- jsonrpc.SendMessageResponse{
						MessageID: id,
						Ok:        true,
					}
					return
				}
			}

			if counter == total {
				// We have not received a consistent response from the Darknodes.
				break Loop
			}
		}
	}

	// Write an error back if we do not get enough results from the Darknodes.
	req.Responder <- jsonrpc.SendMessageResponse{
		Error: ErrNotEnoughResultsReturned,
	}
}

// handleSendMessageResults reads responses from the workers and writes the first non-nil response to the user. If it
// does not receive a response before a `tick` message is received, it writes an error.
func (client *Client) handleReceiveMessageResults(request jsonrpc.Request, tick <-chan time.Time, results chan jsonrpc.Response, total int) {
	counter := 0
	req, ok := request.(jsonrpc.ReceiveMessageRequest)
	if !ok {
		client.logger.Panicf("invalid request type: expected jsonrpc.ReceiveMessageRequest, got %T", request)
	}

Loop:
	for {
		select {
		case <-tick:
			// The Darknodes failed to provide a response within the given time.
			break Loop
		case result := <-results:
			counter++
			if result == nil {
				if counter == total {
					// All of the Darknodes returned a nil message.
					break Loop
				}
				continue
			}

			// TODO: We currently forward the first non-error response we receive. In the future we may want to do some basic
			// validation here to ensure it is consistent with the responses returned by the other Darknodes.
			req.Responder <- result.(jsonrpc.ReceiveMessageResponse)
			return
		}
	}

	// Write an error back if we do not get any results back from the Darknodes.
	req.Responder <- jsonrpc.ReceiveMessageResponse{
		Error: jsonrpc.ErrResultNotAvailable,
	}
}

// InvokeRPC is tau.Message contains a `jsonrpc.Request` and a list of target Darknode addresses. The client task sends
// the request to the given addresses and returns the results to the responder channel in the request.
type InvokeRPC struct {
	Request   jsonrpc.Request
	Addresses []addr.Addr
}

// IsMessage implements the `tau.Message` interface.
func (InvokeRPC) IsMessage() {
}

// RPCCall contains the information `jsonrpc.Client` requires in order to send the request. The response will be written
// to the responder channel which is assumed to have a buffer size of 1.
type RPCCall struct {
	Request   jsonrpc.JSONRequest
	URL       string
	Responder chan<- jsonrpc.Response
}
