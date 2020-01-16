package validator

import (
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/lightnode/blockchain"
	"github.com/renproject/lightnode/server"
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
	requests   chan server.RequestWithResponder
	connPool   blockchain.ConnPool
}

// New constructs a new `Validator`.
func New(logger logrus.FieldLogger, cacher phi.Sender, multiStore store.MultiAddrStore, opts phi.Options, key ecdsa.PublicKey, connPool blockchain.ConnPool) phi.Task {
	requests := make(chan server.RequestWithResponder, 128)
	txChecker := txChecker{
		logger:    logger,
		requests:  requests,
		disPubkey: key,
		connPool:  connPool,
	}
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
	msg, ok := message.(server.RequestWithResponder)
	if !ok {
		validator.logger.Panicf("[validator] unexpected message type %T", message)
	}

	if err := validator.isValid(msg); err != nil {
		msg.Responder <- jsonrpc.NewResponse(msg.Request.ID, nil, err)
		return
	}
	if msg.Request.Method != jsonrpc.MethodSubmitTx {
		validator.cacher.Send(msg)
	}
}

// isValid does basic verification of the message.
func (validator *Validator) isValid(msg server.RequestWithResponder) *jsonrpc.Error {
	// Reject requests that don't conform to the JSON-RPC standard.
	if msg.Request.Version != "2.0" {
		errMsg := fmt.Sprintf(`invalid jsonrpc field: expected "2.0", got "%s"`, msg.Request.Version)
		return &jsonrpc.Error{jsonrpc.ErrorCodeInvalidRequest, errMsg, nil}
	}

	// Reject unsupported methods.
	method := msg.Request.Method
	if _, ok := jsonrpc.RPCs[method]; !ok {
		errMsg := fmt.Sprintf("unsupported method %s", method)
		return &jsonrpc.Error{jsonrpc.ErrorCodeMethodNotFound, errMsg, nil}
	}

	// Reject requests with invalid Darknode IDs.
	darknodeID := msg.DarknodeID
	if darknodeID != "" {
		if _, err := validator.multiStore.Get(darknodeID); err != nil {
			errMsg := fmt.Sprintf("unknown darknode id %s", darknodeID)
			return &jsonrpc.Error{jsonrpc.ErrorCodeInvalidParams, errMsg, nil}
		}
	}

	// Reject requests with invalid parameters.
	if ok, msg := validator.hasValidParams(msg); !ok {
		errMsg := fmt.Sprintf("invalid parameters in request: %s", msg)
		return &jsonrpc.Error{jsonrpc.ErrorCodeInvalidParams, errMsg, nil}
	}

	return nil
}

// hasValidParams checks if the request has valid params depending on its method
func (validator *Validator) hasValidParams(message server.RequestWithResponder) (bool, error) {
	switch message.Request.Method {
	// These methods don't require any parameters
	case jsonrpc.MethodQueryBlock, jsonrpc.MethodQueryBlocks, jsonrpc.MethodQueryNumPeers, jsonrpc.MethodQueryPeers, jsonrpc.MethodQueryStat:
	case jsonrpc.MethodSubmitTx:
		validator.requests <- message
	case jsonrpc.MethodQueryTx:
		return validQueryTxParams(message.Request.Params)
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

// validate the params of the `QueryTx` request.
func validQueryTxParams(params json.RawMessage) (bool, error) {
	var data jsonrpc.ParamsQueryTx
	if err := json.Unmarshal(params, &data); err != nil {
		return false, ErrInvalidParams
	}
	return true, nil
}
