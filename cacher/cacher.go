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
// requests. Upon receiving a request it will check its cache to see if it has a
// cached response. If it does, it will write this immediately as a response,
// otherwise it will forward the request on to the `Dispatcher`. Once the
// `Dispatcher` has a response ready, the `Cacher` will store this response in
// its cache with a key derived from the request, and then pass the response
// along to be given to the client.
type Cacher struct {
	logger        logrus.FieldLogger
	dispatcher    phi.Sender
	db            db.DB
	ttlCache      kv.Table
	serverOptions jsonrpc.Options
}

// New constructs a new `Cacher` as a `phi.Task` which can be `Run()`.
func New(dispatcher phi.Sender, logger logrus.FieldLogger, ttl kv.Table, opts phi.Options, db db.DB, serverOptions jsonrpc.Options) phi.Task {
	return phi.New(&Cacher{
		logger:        logger,
		dispatcher:    dispatcher,
		db:            db,
		ttlCache:      ttl,
		serverOptions: serverOptions,
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
	case jsonrpc.MethodQueryTx:
		params := msg.Params.(jsonrpc.ParamsQueryTx)

		// Retrieve transaction status from the database.
		status, err := cacher.db.TxStatus(params.TxHash)
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
		if status != db.TxStatusConfirmed {
			tx, err := cacher.db.Tx(params.TxHash, true)
			if err == nil {
				msg.Responder <- jsonrpc.Response{
					Version: "2.0",
					ID:      msg.ID,
					Result: jsonrpc.ResponseQueryTx{
						Tx:       tx,
						TxStatus: "confirming",
					},
				}
				return
			}
		}
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

	// We handle this request directly without forwarding it to the Darknodes as
	// they do not support QueryTxs.
	if msg.Method == jsonrpc.MethodQueryTxs {
		response, err := cacher.handleQueryTxs(msg)
		if err != nil {
			msg.RespondWithErr(jsonrpc.ErrorCodeInternal, err)
			return
		}
		cacher.insert(id, msg.Query.Get("id"), response)
		msg.Responder <- response
	} else {
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
			cacher.insert(id, msg.Query.Get("id"), response)
			msg.Responder <- response
		}()
	}
}

func (cacher *Cacher) handleQueryTxs(msg http.RequestWithResponder) (jsonrpc.Response, error) {
	params := msg.Params.(jsonrpc.ParamsQueryTxs)

	var tag string
	if params.Tags != nil && len(*params.Tags) > 0 {
		// Currently we only support a maximum of one tag, but this can be
		// extended in the future.
		tag = hex.EncodeToString((*params.Tags)[0][:])
	}
	var page uint64
	if params.Page != nil {
		page = params.Page.Int.Uint64()
	}
	var pageSize uint64
	if params.PageSize != nil {
		pageSize = params.PageSize.Int.Uint64()
	} else {
		pageSize = uint64(cacher.serverOptions.MaxPageSize)
	}
	txs, err := cacher.db.Txs(tag, page, pageSize)
	if err != nil {
		return jsonrpc.Response{}, err
	}

	return jsonrpc.Response{
		Version: "2.0",
		ID:      msg.ID,
		Result: jsonrpc.ResponseQueryTxs{
			Txs: txs,
		},
	}, nil
}
