package validator

import (
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/lightnode/blockchain"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/lightnode/http"
	"github.com/renproject/lightnode/store"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

var (
	// ErrInvalidParams is returned when the parameters for a request do not
	// match those defined in the specification.
	ErrInvalidParams = errors.New("parameters object does not match method")
)

// A Validator takes as input txs and checks whether they meet some
// baseline criteria that the darknodes expect. This means that obviously
// invalid txs will not make it to the darknodes, but not all invalid
// txs will get rejected.
type Validator struct {
	logger     logrus.FieldLogger
	cacher     phi.Sender
	multiStore store.MultiAddrStore
	requests   chan http.RequestWithResponder
	connPool   blockchain.ConnPool
}

// New constructs a new `Validator`.
func New(logger logrus.FieldLogger, cacher phi.Sender, multiStore store.MultiAddrStore, opts phi.Options, key ecdsa.PublicKey, connPool blockchain.ConnPool, db db.DB) phi.Task {
	requests := make(chan http.RequestWithResponder, 128)
	txChecker := newTxChecker(logger, requests, key, connPool, db)
	go txChecker.Run()

	return phi.New(&Validator{
		logger:     logger,
		cacher:     cacher,
		multiStore: multiStore,
		requests:   requests,
	}, opts)
}

// Handle implements the `phi.Handler` interface.
func (validator *Validator) Handle(_ phi.Task, message phi.Message) {
	msg, ok := message.(http.RequestWithResponder)
	if !ok {
		validator.logger.Panicf("[validator] unexpected message type %T", message)
	}

	if err := validator.isValid(msg); err != nil {
		msg.Responder <- jsonrpc.NewResponse(msg.Request.ID, nil, err)
		return
	}

	// The SubmitTx method does not need to be sent to the cacher as the
	// txchecker writes the response directly.
	if msg.Request.Method != jsonrpc.MethodSubmitTx {
		validator.cacher.Send(msg)
	}
}

// isValid does basic verification of the message.
func (validator *Validator) isValid(msg http.RequestWithResponder) *jsonrpc.Error {
	// Reject requests that don't conform to the JSON-RPC standard.
	if msg.Request.Version != "2.0" {
		errMsg := fmt.Sprintf(`invalid jsonrpc field: expected "2.0", got "%s"`, msg.Request.Version)
		return &jsonrpc.Error{Code: jsonrpc.ErrorCodeInvalidRequest, Message: errMsg, Data: nil}
	}

	// Reject unsupported methods.
	method := msg.Request.Method
	if _, ok := jsonrpc.RPCs[method]; !ok {
		errMsg := fmt.Sprintf("unsupported method %s", method)
		return &jsonrpc.Error{Code: jsonrpc.ErrorCodeMethodNotFound, Message: errMsg, Data: nil}
	}

	// Reject requests with invalid Darknode IDs.
	darknodeID := msg.DarknodeID
	if darknodeID != "" {
		if _, err := validator.multiStore.Get(darknodeID); err != nil {
			errMsg := fmt.Sprintf("unknown darknode id %s", darknodeID)
			return &jsonrpc.Error{Code: jsonrpc.ErrorCodeInvalidParams, Message: errMsg, Data: nil}
		}
	}

	if msg.Request.Method == jsonrpc.MethodSubmitTx {
		// Send to txchecker.
		validator.requests <- msg
	} else {
		if ok, msg := validator.hasValidParams(msg); !ok {
			errMsg := fmt.Sprintf("invalid parameters in request: %s", msg)
			return &jsonrpc.Error{Code: jsonrpc.ErrorCodeInvalidParams, Message: errMsg, Data: nil}
		}
	}

	return nil
}

// hasValidParams checks if the request has valid params depending on its method
func (validator *Validator) hasValidParams(message http.RequestWithResponder) (bool, error) {
	switch message.Request.Method {
	// These methods don't require any parameters or are handled prior to this
	// function call.
	case jsonrpc.MethodSubmitTx, jsonrpc.MethodQueryBlock, jsonrpc.MethodQueryBlocks, jsonrpc.MethodQueryNumPeers, jsonrpc.MethodQueryPeers, jsonrpc.MethodQueryStat:
	case jsonrpc.MethodQueryTx:
		var queryTx jsonrpc.ParamsQueryTx
		if err := json.Unmarshal(message.Request.Params, &queryTx); err != nil {
			return false, ErrInvalidParams
		}
		return true, nil
	case jsonrpc.MethodQueryEpoch:
		// TODO: At the time of writing this method is not supported by the
		// darknode. This should be implemented once it is implemented in the
		// darknode.
		return false, errors.New("method QueryEpoch is not supported")
	default:
		panic(fmt.Sprintf("[validator] unsupported method %s encountered which should have been rejected by the previous checks", message.Request.Method))
	}
	return true, nil
}
