package rpc

import (
	"fmt"
	"net/http"

	"github.com/renproject/lightnode/rpc/jsonrpc"
	"github.com/republicprotocol/tau"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
)

type Server struct {
	logger       logrus.FieldLogger
	jsonrpcQueue <-chan jsonrpc.Request
}

func NewServer(cap int, jsonRPCPort string, logger logrus.FieldLogger) tau.Task {
	jsonrpcQueue := make(chan jsonrpc.Request, cap)
	service := jsonrpc.NewService()
	go func() {
		handler := cors.New(cors.Options{
			AllowCredentials: true,
			AllowedMethods:   []string{"GET", "POST", "OPTIONS", "DELETE"},
			AllowedHeaders:   []string{"Authorization", "Content-Type"},
		}).Handler(service)
		logger.Infof("jsonRPC listening on 0.0.0.0:%v...", jsonRPCPort)
		if err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%v", jsonRPCPort), handler); err != nil {
			logger.Errorln("fail to serve the server,", err)
		}
	}()

	return tau.New(tau.NewIO(cap), &Server{
		logger:       logger,
		jsonrpcQueue: jsonrpcQueue,
	})
}

func (server *Server) Reduce(message tau.Message) tau.Message {
	switch message.(type) {
	case Accept:
		return server.accept()
	default:
		panic(fmt.Errorf("unexpected message type %T", message))
	}
}

func (server *Server) accept() tau.Message {
	select {
	case req := <-server.jsonrpcQueue:
		return MessageAccepted{
			Request: req,
		}

	}
	return nil
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
