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
		return NewMessageAccepted(req)
	}
}

type Accept struct {
}

func (Accept) IsMessage() {
}

func NewAccept() Accept {
	return Accept{}
}

type MessageAccepted struct {
	jsonrpc.Request
}

func (MessageAccepted) IsMessage() {
}

func NewMessageAccepted(req jsonrpc.Request) MessageAccepted {
	return MessageAccepted{
		Request: req,
	}
}
