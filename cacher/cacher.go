package cacher

import (
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/renproject/darknode/abi"
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
		msg.RespondWithErr(jsonrpc.ErrorCodeInternal, err)
		return
	}

	// Calculate the request ID.
	data := append(params, []byte(msg.Request.Method)...)
	reqID := sha3.Sum256(data)

	switch msg.Request.Method {
	case jsonrpc.MethodSubmitTx:
	case jsonrpc.MethodQueryTx:
		// Retrieve transaction status from the database.
		req := jsonrpc.ParamsQueryTx{}
		if err := json.Unmarshal(params, &req); err != nil {
			cacher.logger.Errorf("[cacher] cannot unmarshal request request from json: %v", err)
			msg.RespondWithErr(jsonrpc.ErrorCodeInternal, err)
			return
		}
		confirmed, err := cacher.db.Confirmed(req.TxHash)
		if err != nil {
			// Send the request to the Darknodes if we do not have it in our
			// database.
			if err == sql.ErrNoRows {
				break
			}
			cacher.logger.Errorf("[cacher] cannot get tx status from db: %v", err)
			msg.RespondWithErr(jsonrpc.ErrorCodeInternal, err)
			return
		}

		// If the transaction has not reached sufficient confirmations (i.e. the
		// Darknodes do not yet know about the transaction), respond with a
		// custom confirming status.
		if !confirmed {
			tx, err := cacher.tx(req)
			if err == nil {
				msg.Responder <- jsonrpc.Response{
					Version: "2.0",
					ID:      msg.Request.ID,
					Result: jsonrpc.ResponseQueryTx{
						Tx:       tx,
						TxStatus: "confirming",
					},
				}
				return
			}
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
		Context:    msg.Context,
		Request:    msg.Request,
		Responder:  responder,
		DarknodeID: msg.DarknodeID,
	})

	go func() {
		response := <-responder
		cacher.insert(id, msg.DarknodeID, response)
		msg.Responder <- response
	}()
}

func (cacher *Cacher) tx(req jsonrpc.ParamsQueryTx) (abi.Tx, error) {
	// Fetch the transaction if it is a shift in.
	tx, err := cacher.db.ShiftIn(req.TxHash)
	if err != nil {
		if err == sql.ErrNoRows {
			// Check if the transaction is a shift out.
			tx, err = cacher.db.ShiftOut(req.TxHash)
			if err != nil {
				return abi.Tx{}, fmt.Errorf("[cacher] cannot get tx from db: %v", err)
			}
		} else {
			return abi.Tx{}, fmt.Errorf("[cacher] cannot get tx from db: %v", err)
		}
	}
	return tx, nil
}
