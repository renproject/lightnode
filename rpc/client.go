package rpc

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/republicprotocol/co-go"
	"github.com/republicprotocol/darknode-go/server/jsonrpc"
	jrpc "github.com/republicprotocol/lightnode/rpc/jsonrpc"
	"github.com/republicprotocol/tau"
	"github.com/sirupsen/logrus"
)

// Client is able to send RPC request.
type Client struct {
	logger  logrus.FieldLogger
	queue   chan InvokeRPC
	timeout time.Duration
}

// NewClient returns a new Client.
func NewClient(logger logrus.FieldLogger, cap, numWorkers int, timeout time.Duration) tau.Task {
	client := &Client{
		logger:  logger,
		queue:   make(chan InvokeRPC, cap),
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

// invoke sends the message to the target address. If all workers are busy, the message will be dropped.
func (client *Client) invoke(message InvokeRPC) tau.Message {
	select {
	case client.queue <- message:
	default:
		client.logger.Errorf("dropping request: too much back pressure")
	}
	return nil
}

// runWorkers starts running given amount of workers. They all trying to read from the request queue and sending the
// request.
func (client *Client) runWorkers(n int) {
	go co.ParForAll(n, func(i int) {
		for message := range client.queue {
			var err error
			var method string
			var params []byte
			var responder chan<- jsonrpc.Response

			switch request := message.Request.(type) {
			case jsonrpc.SendMessageRequest:
				method = jsonrpc.MethodSendMessage
				responder = request.Responder
				params, err = json.Marshal(request)
			case jsonrpc.ReceiveMessageRequest:
				method = jsonrpc.MethodReceiveMessage
				responder = request.Responder
				params, err = json.Marshal(request)
			default:
				panic("unknown request type")
			}
			if err != nil {
				client.logger.Errorf("cannot send message to %v, %v", message.Url, err)
				return
			}

			request := jsonrpc.JSONRequest{
				JSONRPC: "2.0",
				Method:  method,
				Params:  params,
				ID:      nil, // fixme : what do we do with ID
			}

			jsonClient := jrpc.NewClient(client.timeout)
			response, err := jsonClient.Call(message.Url, request)
			if err != nil {
				client.logger.Errorf("cannot send message to %v, %v", message.Url, err)
				return
			}

			switch message.Request.(type) {
			case jsonrpc.SendMessageRequest:
				var resp jsonrpc.SendMessageResponse
				if err := json.Unmarshal([]byte(response.Result), &resp); err != nil {
					client.logger.Errorf("cannot unmarshal result from darknode, %v", err)
					return
				}
				select {
				case responder <- resp:
				case <-time.After(client.timeout):
					client.logger.Errorf("cannot write response to the responder channel")
				}
			case jsonrpc.ReceiveMessageRequest:
				var resp jsonrpc.ReceiveMessageResponse
				if err := json.Unmarshal([]byte(response.Result), &resp); err != nil {
					client.logger.Errorf("cannot unmarshal result from darknode, %v", err)
					return
				}
				select {
				case responder <- resp:
				case <-time.After(client.timeout):
					client.logger.Errorf("cannot write response to the responder channel")
				}
			}
		}
	})
}

type InvokeRPC struct {
	Request jsonrpc.Request
	Url     string
}

func (InvokeRPC) IsMessage() {
}
