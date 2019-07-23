package validator

import (
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

	if isValid(msg.Request) {
		validator.cacher.Send(msg)
	} else {
		// TODO: Populate response with appropriate error fields.
		msg.Responder <- jsonrpc.Response{}
	}
}

func isValid(message jsonrpc.Request) bool {
	// TODO: Implement validation logic.
	return true
}
