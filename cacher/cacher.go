package cacher

import (
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/lightnode/server"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

type Cacher struct {
	logger     logrus.FieldLogger
	dispatcher phi.Sender
}

func New(dispatcher phi.Sender, logger logrus.FieldLogger, opts phi.Options) phi.Task {
	return phi.New(&Cacher{logger, dispatcher}, opts)
}

func (cacher *Cacher) Handle(_ phi.Task, message phi.Message) {
	msg, ok := message.(server.RequestWithResponder)
	if !ok {
		cacher.logger.Panicf("[dispatcher] unexpected message type %T", message)
	}

	response, cached := cacher.get(msg.Request)
	if cached {
		msg.Responder <- response
	} else {
		cacher.dispatcher.Send(msg)
	}
}

func (cacher *Cacher) get(message jsonrpc.Request) (jsonrpc.Response, bool) {
	// TODO: Implement caching.
	return jsonrpc.Response{}, false
}
