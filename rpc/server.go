package rpc

import (
	"fmt"

	"github.com/republicprotocol/darknode-go/server/jsonrpc"
	"github.com/republicprotocol/tau"
	"github.com/sirupsen/logrus"
)

// Server is used to write new RPC requests.
type Server struct {
	logger       logrus.FieldLogger
	jsonRPCQueue <-chan jsonrpc.Request
}

// NewServer returns a new Server.
func NewServer(logger logrus.FieldLogger, cap int, jsonRPCQueue <-chan jsonrpc.Request) tau.Task {
	return tau.New(tau.NewIO(cap), &Server{
		logger:       logger,
		jsonRPCQueue: jsonRPCQueue,
	})
}

// Reduce implements the `tau.Task` interface.
func (server *Server) Reduce(message tau.Message) tau.Message {
	switch message.(type) {
	case Accept:
		return server.accept()
	default:
		panic(fmt.Errorf("unexpected message type %T", message))
	}
}

// accept writes new requests back to the parent.
func (server *Server) accept() tau.Message {
	select {
	case req := <-server.jsonRPCQueue:
		switch req.(type) {
		case jsonrpc.QueryPeersRequest, jsonrpc.QueryNumPeersRequest:
			return NewQueryMessage(req)
		case jsonrpc.SendMessageRequest, jsonrpc.ReceiveMessageRequest:
			return NewMessageAccepted(req)
		default:
			panic("unknown request type")
		}
	}
}

// Accept usually sent by the parent task indicating they are ready to accept a new request from the server. Server will
// wait until receiving a new request and propagate the request to its parent task for processing.
type Accept struct {
}

// IsMessage implements the `tau.Message` interface.
func (Accept) IsMessage() {
}

// NewAccept returns a new Accept message.
func NewAccept() Accept {
	return Accept{}
}

// SendMessage is created and propagate by the server to its parent when receiving a SendMessage or ReceiveMessage
// request. Eventually the message will be allocated to a Client task by the resolver as these request need to make a
// JSON-RPC request to darknodes.
type SendMessage struct {
	jsonrpc.Request
}

// IsMessage implements the `tau.Message` interface.
func (SendMessage) IsMessage() {
}

// NewMessageAccepted returns a SendMessage with given request.
func NewMessageAccepted(req jsonrpc.Request) SendMessage {
	return SendMessage{
		Request: req,
	}
}

// SendMessage is created and propagate by the server to its parent when receiving a QueryPeers or QueryNumPeers
// request. The p2p task will query the store and write the response in the responder channel.
type QueryPeersMessage struct {
	jsonrpc.Request
}

// IsMessage implements the `tau.Message` interface.
func (QueryPeersMessage) IsMessage() {
}

// NewQueryMessage returns a new `QueryPeersMessage` with given request.
func NewQueryMessage(req jsonrpc.Request) QueryPeersMessage {
	return QueryPeersMessage{
		Request: req,
	}
}
