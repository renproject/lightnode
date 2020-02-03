package cacher

import (
	"database/sql"
	"encoding/hex"
	"encoding/json"

	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/kv"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/lightnode/http"
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
// requests. Upon receiving a request (in the current architecture this request
// comes from the `Validator`) it will check its cache to see if it has a
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
	// ttlCache := kv.NewTTLCache(ctx, kv.NewMemDB(kv.JSONCodec), "responses", ttl)
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
	params, err := msg.Request.Params.MarshalJSON()
	if err != nil {
		cacher.logger.Errorf("[cacher] cannot marshal request to json: %v", err)
	}

	// Calculate the hash as the request ID
	data := append(params, []byte(msg.Request.Method)...)
	reqID := sha3.Sum256(data)

	switch msg.Request.Method {
	case jsonrpc.MethodSubmitTx:
	case jsonrpc.MethodQueryTx:
		// Get tx status from our db.
		req := jsonrpc.ParamsQueryTx{}
		if err := json.Unmarshal(params, &req); err != nil {
			cacher.logger.Errorf("[cacher] cannot unmarshal request request to json: %v", err)
			return
		}
		confirmed, err := cacher.db.Confirmed(req.TxHash)
		if err != nil {
			// Forward the request to darknode if we don't have the tx in db.
			if err == sql.ErrNoRows {
				break
			}
			cacher.logger.Errorf("[cacher] cannot get tx status from db: %v", err)
			return
		}

		// when tx hasn't reached enough confirmation, respond with confirming status.
		if !confirmed {
			cacher.respondWithConfirmingStatus(req, msg)
			return
		}
	default:
		response, cached := cacher.get(reqID, msg.DarknodeID)
		if cached {
			msg.Responder <- response
			return
		}
	}
	cacher.dispatch(reqID, msg)
}

// insert the response into the ttl table.
func (cacher *Cacher) insert(reqID ID, darknodeID string, response jsonrpc.Response) {
	id := reqID.String() + darknodeID
	if err := cacher.ttlCache.Insert(id, response); err != nil {
		cacher.logger.Errorf("[cacher] could not insert response into TTL cache: %v", err)
	}
}

// get the cached response from the ttl table, the returned bool indicates
// whether there's a cached response can be found from the ttl table.
func (cacher *Cacher) get(reqID ID, darknodeID string) (jsonrpc.Response, bool) {
	id := reqID.String() + darknodeID

	var response jsonrpc.Response
	if err := cacher.ttlCache.Get(id, &response); err == nil {
		return response, true
	}

	return jsonrpc.Response{}, false
}

// dispatch sends the request to darknode and return the response.
func (cacher *Cacher) dispatch(id [32]byte, msg http.RequestWithResponder) {
	responder := make(chan jsonrpc.Response, 1)
	cacher.dispatcher.Send(http.RequestWithResponder{
		Context:    msg.Context,
		Request:    msg.Request,
		Responder:  responder,
		DarknodeID: msg.DarknodeID,
	})

	// TODO: What do we do when a second request comes in that is already
	// being fetched at the moment? Currently it will also send it to the
	// dispatcher, which is probably not ideal.
	go func() {
		response := <-responder
		// TODO: Consider thread safety of insertion.
		cacher.insert(id, msg.DarknodeID, response)
		msg.Responder <- response
	}()
}

// respondWithConfirmingStatus returns a confirming status without forwarding
// the request to darknode.
// FIXME : RETURNS AND ERROR IF AN INTERNAL ERROR HAPPENS, AND RESPOND TO THE USER
// WITH A PREDEFINED ERROR.
func (cacher *Cacher) respondWithConfirmingStatus(req jsonrpc.ParamsQueryTx, msg http.RequestWithResponder) {
	tx, err := cacher.db.ShiftIn(req.TxHash)
	if err != nil {
		if err == sql.ErrNoRows {
			tx, err = cacher.db.ShiftOut(req.TxHash)
			if err != nil {
				cacher.logger.Errorf("[cacher] cannot get tx from db: %v", err)
				return
			}
		} else {
			cacher.logger.Errorf("[cacher] cannot get tx from db: %v", err)
			return
		}
	}
	msg.Responder <- jsonrpc.Response{
		Version: "2.0",
		ID:      msg.Request.ID,
		Result: jsonrpc.ResponseQueryTx{
			Tx:       tx,
			TxStatus: "confirming",
		},
	}
}
