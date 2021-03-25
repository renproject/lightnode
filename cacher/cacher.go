package cacher

import (
	"encoding/hex"
	"encoding/json"

	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/kv"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/lightnode/http"
	"github.com/renproject/pack"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/sha3"
)

// ID is a key for a cached response.
type ID [32]byte

// String returns the hex encoding of the ID.
func (id ID) String() string {
	return hex.EncodeToString(id[:])
}

// Cacher is a task responsible for caching responses for corresponding
// requests. Upon receiving a request it will check its cache to see if it has a
// cached response. If it does, it will write this immediately as a response,
// otherwise it will forward the request on to the `Dispatcher`. Once the
// `Dispatcher` has a response ready, the `Cacher` will store this response in
// its cache with a key derived from the request, and then pass the response
// along to be given to the client.
type Cacher struct {
	logger     logrus.FieldLogger
	dispatcher phi.Sender
	db         db.DB
	ttlCache   kv.Table
}

// New constructs a new `Cacher` as a `phi.Task` which can be `Run()`.
func New(dispatcher phi.Sender, logger logrus.FieldLogger, ttl kv.Table, opts phi.Options, db db.DB) phi.Task {
	return phi.New(&Cacher{
		logger:     logger,
		dispatcher: dispatcher,
		db:         db,
		ttlCache:   ttl,
	}, opts)
}

// Handle implements the `phi.Handler` interface.
func (cacher *Cacher) Handle(_ phi.Task, message phi.Message) {
	msg, ok := message.(http.RequestWithResponder)
	if !ok {
		cacher.logger.Panicf("[cacher] unexpected message type %T", message)
	}

	paramsBytes, err := json.Marshal(msg.Params)
	if err != nil {
		cacher.logger.Errorf("[cacher] cannot marshal request to json: %v", err)
		msg.RespondWithErr(jsonrpc.ErrorCodeInvalidParams, err)
		return
	}

	// Calculate the request ID.
	data := append(paramsBytes, []byte(msg.Method)...)
	reqID := sha3.Sum256(data)

	switch msg.Method {
	case jsonrpc.MethodSubmitTx:
	// case jsonrpc.MethodQueryTx:
	// We used to perform custom logic here to determine whether a
	// tx should be fetched from the db or requested from the darknode.
	// This logic has been moved to the resolver for compatability reasons
	// The cacher will only be called when the darknode itself is queried
	default:
		darknodeID := msg.Query.Get("id")
		response, cached := cacher.get(reqID, darknodeID)
		if cached {
			msg.Responder <- response
			return
		}
	}
	cacher.dispatch(reqID, msg)
}

func (cacher *Cacher) insert(reqID ID, darknodeID string, response jsonrpc.Response) {
	id := reqID.String() + darknodeID
	if err := cacher.ttlCache.Insert(id, response); err != nil {
		cacher.logger.Errorf("[cacher] cannot insert response into TTL cache: %v", err)
		return
	}
}

func (cacher *Cacher) get(reqID ID, darknodeID string) (jsonrpc.Response, bool) {
	id := reqID.String() + darknodeID

	var response jsonrpc.Response
	if err := cacher.ttlCache.Get(id, &response); err == nil {
		return response, true
	}

	return jsonrpc.Response{}, false
}

func (cacher *Cacher) dispatch(id [32]byte, msg http.RequestWithResponder) {
	responder := make(chan jsonrpc.Response, 1)
	cacher.dispatcher.Send(http.RequestWithResponder{
		Context:   msg.Context,
		ID:        msg.ID,
		Method:    msg.Method,
		Params:    msg.Params,
		Responder: responder,
		Query:     msg.Query,
	})

	go func() {
		response := <-responder
		// QueryTx has an intermediary state where it has not yet been executed
		// don't cache if we don't have output
		skipCache := func() bool {
			if msg.Method == jsonrpc.MethodQueryTx {
				raw, err := json.Marshal(response.Result)
				// no need to handle errors here as it will be handled by the resolver
				if err != nil {
					cacher.logger.Warnf("Failed to marshal queryTx response: %v", err)
					return true
				}
				var tx jsonrpc.ResponseQueryTx
				err = json.Unmarshal(raw, &tx)
				if err != nil {
					cacher.logger.Warnf("Failed to unmarshal queryTx response: %v", err)
					return true
				}

				if tx.Tx.Output.String() == pack.NewTyped().String() {
					return true
				}
			}
			return false
		}
		if !skipCache() {
			cacher.insert(id, msg.Query.Get("id"), response)
		}
		msg.Responder <- response
	}()
}
