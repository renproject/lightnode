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
	"github.com/republicprotocol/darknode-go/server/jsonrpc"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/republicprotocol/renp2p-go/foundation/addr"
	"github.com/republicprotocol/tau"
	"github.com/sirupsen/logrus"
)

var (
	// ErrNotEnoughResultsReturned is returned if we do not get enough successful responses from Darknodes.
	ErrNotEnoughResultsReturned = errors.New("not enough results returned")

	// ErrNoResultReceived is returned when the response we receive from each Darknode is nil.
	ErrNoResultReceived = errors.New("no result received")
)

// Client is used to send RPC requests.
type Client struct {
	logger  logrus.FieldLogger
	store   store.KVStore
	queue   chan RPCCall
	timeout time.Duration
}

// NewClient returns a new Client task.
func NewClient(logger logrus.FieldLogger, cap, numWorkers int, timeout time.Duration, store store.KVStore) tau.Task {
	client := &Client{
		logger:  logger,
		store:   store,
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
		client.logger.Panicf("unexpected message type %T", message)
	}
	return nil
}

// invoke sends the given message to the target addresses. If the queue is full, the message will be dropped.
func (client *Client) invoke(message InvokeRPC) tau.Message {
	switch request := message.Request.(type) {
	case jsonrpc.SendMessageRequest:
		return client.handleSendMessageRequest(request, jsonrpc.MethodSendMessage, message.Addresses)
	case jsonrpc.ReceiveMessageRequest:
		return client.handleReceiveMessageRequest(request, jsonrpc.MethodReceiveMessage, message.Addresses)
	case jsonrpc.QueryStatsRequest:
		return client.handleQueryStatsRequest(request, jsonrpc.MethodQueryStats)
	default:
		client.logger.Panicf("unexpected message type %T", request)
	}
	return nil
}

// runWorkers starts running a given number of workers. They each try to read from the request queue and send the
// request.
func (client *Client) runWorkers(n int) {
	go co.ForAll(n, func(i int) {
		// For each RPC call inside the queue.
		for call := range client.queue {
			// Construct a new JSON client and send the request.
			jsonClient := jrpc.NewClient(client.timeout)
			response, err := jsonClient.Call(call.URL, call.Request)
			if err != nil {
				close(call.Responder)
				client.logger.Warnf("cannot send message to %v: %v", call.URL, err)
				continue
			}

			// Unmarshal the response and write it to the responder channel.
			switch call.Request.Method {
			case jsonrpc.MethodSendMessage:
				var resp jsonrpc.SendMessageResponse
				if response.Error != nil {
					resp.Error = fmt.Errorf("received response error: %v", response.Error)
				} else if err := json.Unmarshal([]byte(response.Result), &resp); err != nil {
					close(call.Responder)
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
				if response.Error != nil {
					resp.Error = fmt.Errorf("%v", response.Error.Message)
				} else if err := json.Unmarshal([]byte(response.Result), &resp); err != nil {
					close(call.Responder)
					client.logger.Errorf("cannot unmarshal ReceiveMessageResponse from Darknode: %v", err)
					continue
				}

				select {
				case call.Responder <- resp:
				case <-time.After(client.timeout):
					client.logger.Errorf("cannot write response to the responder channel")
				}
			case jsonrpc.MethodQueryStats:
				var resp jsonrpc.QueryStatsResponse
				if response.Error != nil {
					resp.Error = fmt.Errorf("%v", response.Error.Message)
				} else if err := json.Unmarshal([]byte(response.Result), &resp); err != nil {
					close(call.Responder)
					client.logger.Errorf("cannot unmarshal QueryStatsResponse from Darknode: %v", err)
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
func (client *Client) handleRequest(request interface{}, method string, addresses []addr.Addr) []interface{} {
	results := make([]interface{}, len(addresses))

	// Loop through the provided addresses.
	co.ParForAll(addresses, func(i int) {
		address := addresses[i]
		responder := make(chan jsonrpc.Response, 1)
		data, err := json.Marshal(request)
		if err != nil {
			client.logger.Errorf("failed to marshal the SendMessageRequest: %v", err)
			return
		}

		// Get multi-address of the darknode from store.
		var multi peer.MultiAddr
		if err := client.store.Read(address.String(), &multi); err != nil {
			client.logger.Warnf("cannot read multi-address of %v from the store: %v", address.String(), err)
			return
		}
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

func (client *Client) handleSendMessageRequest(request jsonrpc.SendMessageRequest, method string, addresses []addr.Addr) tau.Message {
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

	// Write the response returned by most darknode to the user
	for id, num := range messageIDs {
		if num >= (len(addresses)+1)*2/3 {
			request.Responder <- jsonrpc.SendMessageResponse{
				MessageID: id,
				Ok:        true,
			}

			return nil
		}
	}

	// Write error back if we don't get enough results from darknodes.
	request.Responder <- jsonrpc.SendMessageResponse{
		Error: ErrNotEnoughResultsReturned,
	}

	return nil
}

func (client *Client) handleReceiveMessageRequest(request jsonrpc.ReceiveMessageRequest, method string, addresses []addr.Addr) tau.Message {
	results := client.handleRequest(request, method, addresses)

	// TODO: We currently forward the first non-error response we receive. In future we may want to do some basic
	// validation here to ensure it is consistent with the responses returned by the other Darknodes.
	for _, result := range results {
		if result == nil {
			continue
		}
		res := result.(jsonrpc.ReceiveMessageResponse)
		select {
		case request.Responder <- res:
		case <-time.After(client.timeout):
			client.logger.Errorf("cannot write response to the responder channel")
		}

		return nil
	}

	// If all result are bad, return an error to the sender.
	request.Responder <- jsonrpc.ReceiveMessageResponse{
		Error: ErrNotEnoughResultsReturned,
	}

	return nil
}

func (client *Client) handleQueryStatsRequest(request jsonrpc.QueryStatsRequest, method string) tau.Message {
	if request.DarknodeID == "" {
		// TODO: We likely want to return the Lightnode stats if the request does not contain a Darknode ID.
		request.Responder <- jsonrpc.QueryStatsResponse{
			Error: errors.New("missing darknode ID"),
		}
	}

	addresses := []addr.Addr{addr.New(request.DarknodeID)}
	results := client.handleRequest(request, method, addresses)

	if len(results) == 0 || results[0] == nil {
		request.Responder <- jsonrpc.QueryStatsResponse{
			Error: ErrNoResultReceived,
		}
	} else {
		result := results[0].(jsonrpc.QueryStatsResponse)
		request.Responder <- result
	}

	return nil
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
