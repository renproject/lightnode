// Package rpc defines the client and server `tau.Task`s. The client starts background workers which handle sending
// requests to the Darknodes using the `Client` defined in jsonrpc/client.go. If the number of requests exceeds the
// number of background workers, they will remain on the channel until the buffer is exceeded (in which case the request
// is dropped).
package rpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	jrpc "github.com/renproject/lightnode/rpc/jsonrpc"
	"github.com/republicprotocol/co-go"
	"github.com/republicprotocol/darknode-go/server/jsonrpc"
	"github.com/republicprotocol/tau"
	"github.com/sirupsen/logrus"
)

// ErrNotEnoughResultsReturned is returned if we do not get enough successful responses from Darknodes.
var ErrNotEnoughResultsReturned = errors.New("not enough results returned")

// Client is used to send RPC requests.
type Client struct {
	logger  logrus.FieldLogger
	queue   chan RPCCall
	timeout time.Duration
}

// NewClient returns a new Client.
func NewClient(logger logrus.FieldLogger, cap, numWorkers int, timeout time.Duration) tau.Task {
	client := &Client{
		logger:  logger,
		queue:   make(chan RPCCall, cap),
		timeout: timeout,
	}
	client.runWorkers(numWorkers)
	return tau.New(tau.NewIO(cap), client)
}

// Reduce implements the `tau.Task` interface.
func (client *Client) Reduce(message tau.Message) tau.Message {
	switch message := message.(type) {
	case InvokeRPC:
		return client.invoke(message)
	default:
		panic(fmt.Errorf("unexpected message type %T", message))
	}
}

// invoke sends the given message to the target addresses. If the queue is full, the message will be dropped.
func (client *Client) invoke(message InvokeRPC) tau.Message {
	switch request := message.Request.(type) {
	case jsonrpc.SendMessageRequest:
		return client.handleSendMessageRequest(request, jsonrpc.MethodSendMessage, message.Addresses)
	case jsonrpc.ReceiveMessageRequest:
		return client.handleReceiveMessageRequest(request, jsonrpc.MethodReceiveMessage, message.Addresses)
	default:
		panic("unknown message type")
	}
}

// runWorkers starts running a given number of workers. They each try to read from the request queue and send the
// request.
func (client *Client) runWorkers(n int) {
	go co.ParForAll(n, func(i int) {
		// For each RPC call inside the queue.
		for call := range client.queue {
			// Construct a new JSON client and send the request.
			jsonClient := jrpc.NewClient(client.timeout)
			response, err := jsonClient.Call(call.URL, call.Request)
			if err != nil {
				close(call.Responder)
				client.logger.Errorf("cannot send message to %v, %v", call.URL, err)
				continue
			}

			// Unmarshal the response and write it to the responder channel.
			switch call.Request.Method {
			case jsonrpc.MethodSendMessage:
				var resp jsonrpc.SendMessageResponse
				if err := json.Unmarshal([]byte(response.Result), &resp); err != nil {
					client.logger.Errorf("cannot unmarshal SendMessageResponse from Darknode: %v", err)
					continue
				}
				select {
				case call.Responder <- resp:
				case <-time.After(client.timeout):
					client.logger.Errorf("cannot write response to the responder channel")
				}
			case jsonrpc.MethodReceiveMessage:
				var resp jsonrpc.ReceiveMessageResponse
				if err := json.Unmarshal([]byte(response.Result), &resp); err != nil {
					client.logger.Errorf("cannot unmarshal ReceiveMessageResponse from Darknode: %v", err)
					continue
				}
				select {
				case call.Responder <- resp:
				case <-time.After(client.timeout):
					client.logger.Errorf("cannot write response to the responder channel")
				}
			}
		}
	})
}

// handleRequest handles writing the request for each Darknode to the queue and reading each of the responses.
func (client *Client) handleRequest(request interface{}, method string, addresses []string) []interface{} {
	results := make([]interface{}, len(addresses))

	// Loop through the provided addresses.
	co.ParForAll(addresses, func(i int) {
		address := addresses[i]
		responder := make(chan jsonrpc.Response)
		data, err := json.Marshal(request)
		if err != nil {
			client.logger.Errorf("failed to marshal the SendMessageRequest: %v", err)
			return
		}
		call := RPCCall{
			Request: jsonrpc.JSONRequest{
				JSONRPC: "2.0",
				Method:  method,
				Params:  data,
			},
			URL:       address,
			Responder: responder,
		}

		// Write each request to the queue.
		select {
		case client.queue <- call:
		default:
			client.logger.Errorf("dropping request: too much back pressure")
		}

		// Read the response from the responder channel.
		select {
		case response := <-responder:
			results[i] = response
		case <-time.After(client.timeout):
			client.logger.Errorf("timeout")
		}
	})

	return results
}

func (client *Client) handleSendMessageRequest(request jsonrpc.SendMessageRequest, method string, addresses []string) tau.Message {
	results := client.handleRequest(request, method, addresses)

	// Check if the majority of the Darknodes return the same result and if so write the response back to the parent
	// task.
	messageIDs := map[string]int{}
	for _, result := range results {
		if result == nil {
			continue
		}
		res := result.(jsonrpc.SendMessageResponse)
		if res.Error == nil && res.Ok {
			messageIDs[res.MessageID]++
		}
	}

	for id, num := range messageIDs {
		if num >= (len(addresses)+1)*2/3 {
			select {
			case request.Responder <- jsonrpc.SendMessageResponse{
				MessageID: id,
				Ok:        true,
			}:
			case <-time.After(client.timeout):
				client.logger.Errorf("cannot write response to the responder channel")
			}

			return nil
		}
	}

	return tau.NewError(ErrNotEnoughResultsReturned)
}

func (client *Client) handleReceiveMessageRequest(request jsonrpc.ReceiveMessageRequest, method string, addresses []string) tau.Message {
	results := client.handleRequest(request, method, addresses)

	// TODO: Fix this logic.
	for _, result := range results {
		if result == nil {
			continue
		}
		res := result.(jsonrpc.ReceiveMessageResponse)
		if res.Err() == nil {
			select {
			case request.Responder <- res:
			case <-time.After(client.timeout):
				client.logger.Errorf("cannot write response to the responder channel")
			}

			return nil
		}
	}

	return tau.NewError(ErrNotEnoughResultsReturned)
}

type InvokeRPC struct {
	Request   jsonrpc.Request
	Addresses []string
}

func (InvokeRPC) IsMessage() {
}

type RPCCall struct {
	Request   jsonrpc.JSONRequest
	URL       string
	Responder chan<- jsonrpc.Response
}
