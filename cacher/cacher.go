package cacher

import (
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/lightnode/server"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/sha3"
)

type ID [32]byte

type Cacher struct {
	logger     logrus.FieldLogger
	dispatcher phi.Sender

	cache map[ID]jsonrpc.Response
}

func New(dispatcher phi.Sender, logger logrus.FieldLogger, cap int, opts phi.Options) phi.Task {
	return phi.New(&Cacher{logger, dispatcher, make(map[ID]jsonrpc.Response, cap)}, opts)
}

func (cacher *Cacher) Handle(_ phi.Task, message phi.Message) {
	msg, ok := message.(server.RequestWithResponder)
	if !ok {
		cacher.logger.Panicf("[cacher] unexpected message type %T", message)
	}

	request, err := msg.Request.Params.MarshalJSON()
	{
		cacher.logger.Errorf("[cacher] cannot marshal request to json: %v", err)
	}

	reqID := hash(request)

	response, cached := cacher.get(reqID)
	if cached {
		msg.Responder <- response
	} else {
		responder := make(chan jsonrpc.Response, 1)
		cacher.dispatcher.Send(server.RequestWithResponder{
			Request:   msg.Request,
			Responder: responder,
		})

		response := <-responder
		cacher.insert(reqID, response)
		msg.Responder <- response
	}
}

func (cacher *Cacher) insert(id ID, response jsonrpc.Response) {
	cacher.cache[id] = response
	return
}

func (cacher *Cacher) get(id ID) (jsonrpc.Response, bool) {
	response, ok := cacher.cache[id]
	return response, ok
}

func hash(data []byte) ID {
	return sha3.Sum256(data)
}
