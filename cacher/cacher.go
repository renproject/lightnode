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

	cache map[interface{}]jsonrpc.Response
}

func New(dispatcher phi.Sender, logger logrus.FieldLogger, cap int, opts phi.Options) phi.Task {
	return phi.New(&Cacher{logger, dispatcher, make(map[interface{}]jsonrpc.Response, cap)}, opts)
}

func (cacher *Cacher) Handle(_ phi.Task, message phi.Message) {
	msg, ok := message.(server.RequestWithResponder)
	if !ok {
		cacher.logger.Panicf("[dispatcher] unexpected message type %T", message)
	}

	response, cached := cacher.get(msg.Request.ID)
	if cached {
		msg.Responder <- response
	} else {
		responder := make(chan jsonrpc.Response, 1)
		cacher.dispatcher.Send(server.RequestWithResponder{
			Request:   msg.Request,
			Responder: responder,
		})

		response := <-responder
		cacher.insert(msg.Request.ID, response)
		msg.Responder <- response
	}
}

func (cacher *Cacher) insert(id interface{}, response jsonrpc.Response) {
	cacher.cache[id] = response
	return
}

func (cacher *Cacher) get(id interface{}) (jsonrpc.Response, bool) {
	response, ok := cacher.cache[id]
	return response, ok
}
