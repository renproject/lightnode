package rpc

import (
	"fmt"

	"github.com/republicprotocol/co-go"
	"github.com/renproject/lightnode/rpc/jsonrpc"
	"github.com/republicprotocol/tau"
	"github.com/sirupsen/logrus"
)

type Client struct {
	logger    logrus.FieldLogger
	queue     chan tau.Message
	rpcClient jsonrpc.Client
}

func NewClient(cap int, logger logrus.FieldLogger, numWorkers int) tau.Task {
	client := &Client{
		logger: logger,
		queue:  make(chan tau.Message, cap),
	}
	client.runWorkers(numWorkers)
	return tau.New(tau.NewIO(cap), client)
}

func (client *Client) Reduce(message tau.Message) tau.Message {
	switch message := message.(type) {
	case InvokeRPC:
		return client.invoke(message)
	default:
		panic(fmt.Errorf("unexpected message type %T", message))
	}
}

func (client *Client) invoke(message tau.Message) tau.Message {
	select {
	case client.queue <- message:
	default:
		client.logger.Errorf("dropping request: too much back pressure")
	}
	return nil
}

func (client *Client) runWorkers(n int) {
	go co.ParForAll(n, func(i int) {
		// Two phase locking is used by the workers to minimise the number of write locks that are needed when sharing
		// streams between goroutines

		for message := range client.queue {
			// todo : marshal the message according to the message type

			// get the target net.Addr

			// Send the message using the rpcClient

			// Write response to the reponder channel

			panic(message)
		}
	})
}

type InvokeRPC struct {
}

func (InvokeRPC) IsMessage() {

}
