package cacher

import (
	lru "github.com/hashicorp/golang-lru"
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

	cache *lru.Cache
}

func New(dispatcher phi.Sender, logger logrus.FieldLogger, cap int, opts phi.Options) phi.Task {
	cache, err := lru.New(cap)
	if err != nil {
		logger.Panicf("[cacher] cannot create LRU cache: %v", err)
	}
	return phi.New(&Cacher{logger, dispatcher, cache}, opts)
}

func (cacher *Cacher) Handle(_ phi.Task, message phi.Message) {
	msg, ok := message.(server.RequestWithResponder)
	if !ok {
		cacher.logger.Panicf("[cacher] unexpected message type %T", message)
	}

	request, err := msg.Request.Params.MarshalJSON()
	if err != nil {
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
	cacher.cache.Add(id, response)
}

func (cacher *Cacher) get(id ID) (jsonrpc.Response, bool) {
	if response, ok := cacher.cache.Get(id); ok {
		return response.(jsonrpc.Response), ok
	}
	return jsonrpc.Response{}, false
}

func hash(data []byte) ID {
	return sha3.Sum256(data)
}
