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

type Accept struct {
}

func (Accept) IsMessage() {
}

func NewAccept() Accept {
	return Accept{}
}

type SendMessage struct {
	jsonrpc.Request
}

func (SendMessage) IsMessage() {
}

func NewMessageAccepted(req jsonrpc.Request) SendMessage {
	return SendMessage{
		Request: req,
	}
}

type QueryPeersMessage struct {
	jsonrpc.Request
}

func (QueryPeersMessage) IsMessage() {
}

func NewQueryMessage(req jsonrpc.Request) QueryPeersMessage {
	return QueryPeersMessage{
		Request: req,
	}
}
