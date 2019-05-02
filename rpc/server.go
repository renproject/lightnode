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
			return NewSendMessage(req)
		default:
			panic("unknown request type")
		}
	}
}

// Accept messages are sent by the parent task indicating they are ready to accept a new request from the server.
type Accept struct {
}

// IsMessage implements the `tau.Message` interface.
func (Accept) IsMessage() {
}

// NewAccept returns a new Accept message.
func NewAccept() Accept {
	return Accept{}
}

// SendMessage is created and propagated by the server to its parent when receiving a SendMessage or ReceiveMessage
// request. These messages get forwarded to Client tasks by the resolver as these requests need to interact with the
// Darknodes using JSON-RPC.
type SendMessage struct {
	jsonrpc.Request
}

// IsMessage implements the `tau.Message` interface.
func (SendMessage) IsMessage() {
}

// NewSendMessage returns a new `SendMessage` with the given request.
func NewSendMessage(req jsonrpc.Request) SendMessage {
	return SendMessage{
		Request: req,
	}
}

// QueryMessage is created and propagated by the server to its parent when receiving a QueryPeers or QueryNumPeers
// request. These messages get forwarded to the P2P task which queries the store and writes the response in the
// responder channel.
type QueryMessage struct {
	jsonrpc.Request
}

// IsMessage implements the `tau.Message` interface.
func (QueryMessage) IsMessage() {
}

// NewQueryMessage returns a new `QueryMessage` with the given request.
func NewQueryMessage(req jsonrpc.Request) QueryMessage {
	return QueryMessage{
		Request: req,
	}
}
