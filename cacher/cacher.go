package cacher

import (
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/kv"
	"github.com/renproject/lightnode/server"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/sha3"
)

// ID is a key for a cached response.
type ID [32]byte

func (id ID) String() string {
	return string(id[:32])
}

// Cacher is a task responsible for caching responses for corresponding
// requests. Upon receiving a request (in the current architecture this request
// comes from the `Validator`) it will check its cache to see if it has a
// cached response. If it does, it will write this immediately as a repsonse,
// otherwise it will forward the request on to the `Dispatcher`. Once the
// `Dispatcher` has a response ready, the `Cacher` will store this response in
// its cache with a key derived from the request, and then pass the repsonse
// along to be given to the client. Currently, idempotent requests are stored
// in a LRU cache, and non-idempotent requests are stored in a TTL cache.
type Cacher struct {
	logger     logrus.FieldLogger
	dispatcher phi.Sender

	// TODO: Should these two caches be encapsulated into a single object?
	cache    *lru.Cache
	ttlCache kv.Iterable
}

// New constructs a new `Cacher` as a `phi.Task` which can be `Run()`.
func New(dispatcher phi.Sender, logger logrus.FieldLogger, cap int, ttl time.Duration, opts phi.Options) phi.Task {
	cache, err := lru.New(cap)
	if err != nil {
		logger.Panicf("[cacher] cannot create LRU cache: %v", err)
	}
	ttlCache, err := kv.NewTTLCache(kv.NewJSON(kv.NewMemDB()), ttl)
	if err != nil {
		logger.Panicf("[cacher] cannot create TTL cache: %v", err)
	}
	return phi.New(&Cacher{logger, dispatcher, cache, ttlCache}, opts)
}

// Handle implements the `phi.Handler` interface.
func (cacher *Cacher) Handle(_ phi.Task, message phi.Message) {
	msg, ok := message.(server.RequestWithResponder)
	if !ok {
		cacher.logger.Panicf("[cacher] unexpected message type %T", message)
	}

	params, err := msg.Request.Params.MarshalJSON()
	if err != nil {
		cacher.logger.Errorf("[cacher] cannot marshal request to json: %v", err)
	}

	data := append(params, []byte(msg.Request.Method)...)
	reqID := hash(data)

	response, cached := cacher.get(reqID)
	if cached {
		msg.Responder <- response
	} else {
		responder := make(chan jsonrpc.Response, 1)
		cacher.dispatcher.Send(server.RequestWithResponder{
			Request:   msg.Request,
			Responder: responder,
		})

		// TODO: What do we do when a second request comes in that is already
		// being fetched at the moment? Currently it will also send it to the
		// dispatcher, which is probably not ideal.
		go func() {
			response := <-responder
			cacher.insert(reqID, response, msg.Request.Method)
			msg.Responder <- response
		}()
	}
}

func (cacher *Cacher) insert(id ID, response jsonrpc.Response, method string) {
	// It is assumed at this point that the method is valid, so we can safely
	// avoid the case of undefined methods.
	if method != jsonrpc.MethodSubmitTx {
		if err := cacher.ttlCache.Insert(id.String(), response); err != nil {
			cacher.logger.Panicf("[cacher] could not insert response into TTL cache: %v", err)
		}
	} else {
		cacher.cache.Add(id, response)
	}
}

func (cacher *Cacher) get(id ID) (jsonrpc.Response, bool) {
	if response, ok := cacher.cache.Get(id); ok {
		return response.(jsonrpc.Response), true
	}

	var response jsonrpc.Response
	if err := cacher.ttlCache.Get(id.String(), &response); err == nil {
		return response, true
	}

	return jsonrpc.Response{}, false
}

func hash(data []byte) ID {
	return sha3.Sum256(data)
}
