package validator

import (
	"encoding/json"
	"fmt"

	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/lightnode/server"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

type Validator struct {
	logger logrus.FieldLogger
	cacher phi.Sender
}

func New(cacher phi.Sender, logger logrus.FieldLogger, opts phi.Options) phi.Task {
	return phi.New(&Validator{logger, cacher}, opts)
}

func (validator *Validator) Handle(_ phi.Task, message phi.Message) {
	msg, ok := message.(server.RequestWithResponder)
	if !ok {
		validator.logger.Panicf("[validator] unexpected message type %T", message)
	}

	if err := isValid(msg.Request); err != nil {
		msg.Responder <- jsonrpc.NewResponse(msg.Request.ID, nil, err)
		return
	}
	validator.cacher.Send(msg)
}

func isValid(message jsonrpc.Request) *jsonrpc.Error {
	// Reject unsupported methods
	method := message.Method
	if !isSupported(method) {
		err := jsonrpc.NewError(jsonrpc.ErrorCodeMethodNotFound, fmt.Sprintf("unsupported method %s", method), json.RawMessage{})
		return &err
	}

	// TODO: Implement more validation logic.
	return nil
}

func isSupported(method string) bool {
	_, supported := jsonrpc.RPCs[method]
	return supported
}
