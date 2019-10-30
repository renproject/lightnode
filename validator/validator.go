package validator

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/renproject/darknode/jsonrpc"
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

// A Validator takes as input requests and checks whether they meet some
// baseline criteria that the darknodes expect. This means that obviously
// invalid requests will not make it to the darknodes, but not all invalid
// requests will get rejected.
type Validator struct {
	logger     logrus.FieldLogger
	cacher     phi.Sender
	multiStore store.MultiAddrStore
}

// New constructs a new `Validator`.
func New(logger logrus.FieldLogger, cacher phi.Sender, multiStore store.MultiAddrStore, opts phi.Options) phi.Task {
	return phi.New(&Validator{logger, cacher, multiStore}, opts)
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
	validator.cacher.Send(msg)
}

func (validator *Validator) isValid(msg server.RequestWithResponder) *jsonrpc.Error {
	// Reject requests that don't conform to the JSON-RPC standard.
	if msg.Request.Version != "2.0" {
		err := jsonrpc.NewError(jsonrpc.ErrorCodeInvalidRequest, fmt.Sprintf("invalid jsonrpc field: expected \"2.0\", got \"%s\"", msg.Request.Version), nil)
		return &err
	}

	// Reject unsupported methods.
	method := msg.Request.Method
	if !isSupported(method) {
		err := jsonrpc.NewError(jsonrpc.ErrorCodeMethodNotFound, fmt.Sprintf("unsupported method %s", method), nil)
		return &err
	}

	// Reject requests with invalid parameters.
	if ok, msg := hasValidParams(msg.Request); !ok {
		err := jsonrpc.NewError(jsonrpc.ErrorCodeInvalidParams, fmt.Sprintf("invalid parameters in request: %s", msg), nil)
		return &err
	}

	// Reject requests with invalid Darknode IDs.
	darknodeID := msg.DarknodeID
	if darknodeID != "" {
		_, err := validator.multiStore.Get(darknodeID)
		if err != nil {
			err := jsonrpc.NewError(jsonrpc.ErrorCodeInvalidParams, fmt.Sprintf("unknown darknode id %s", darknodeID), nil)
			return &err
		}
	}

	return nil
}

func isSupported(method string) bool {
	_, supported := jsonrpc.RPCs[method]
	return supported
}

func hasValidParams(message jsonrpc.Request) (bool, error) {
	switch message.Method {
	case jsonrpc.MethodQueryBlock:
		return validQueryBlockParams(message.Params)
	case jsonrpc.MethodQueryBlocks:
		return validQueryBlocksParams(message.Params)
	case jsonrpc.MethodSubmitTx:
		return validSubmitTxParams(message.Params)
	case jsonrpc.MethodQueryTx:
		return validQueryTxParams(message.Params)
	case jsonrpc.MethodQueryNumPeers:
		return validQueryNumPeersParams(message.Params)
	case jsonrpc.MethodQueryPeers:
		return validQueryPeersParams(message.Params)
	case jsonrpc.MethodQueryEpoch:
		// TODO: At the time of writing this method is not supported by the
		// darknode. This should be implemented once it is implemented in the
		// darknode.
		return false, errors.New("method QueryEpoch is not supported")
	case jsonrpc.MethodQueryStat:
		return validQueryStatParams(message.Params)
	default:
		// TODO: Is it ok to panic at this level, or should all panics happen
		// through a logger?
		panic(fmt.Sprintf("[validator] unsupported method %s encountered which should have been rejected by the previous checks", message.Method))
	}
}

func validQueryBlockParams(params json.RawMessage) (bool, error) {
	// This parameter type has no fields, so there is nothing to check.
	return true, nil
}

func validQueryBlocksParams(params json.RawMessage) (bool, error) {
	// This parameter type has no fields, so there is nothing to check.
	return true, nil
}

func validSubmitTxParams(params json.RawMessage) (bool, error) {
	var data jsonrpc.ParamsSubmitTx
	if err := json.Unmarshal(params, &data); err != nil {
		return false, ErrInvalidParams
	}
	// TODO: Check fields. Do we want to use the entire darknode transform
	// pipeline to check validity?
	return true, nil
}

func validQueryTxParams(params json.RawMessage) (bool, error) {
	var data jsonrpc.ParamsQueryTx
	if err := json.Unmarshal(params, &data); err != nil {
		return false, ErrInvalidParams
	}
	// Currently the only field in the parameters is a hash field, which can't
	// really be checked for validity here
	return true, nil
}

func validQueryNumPeersParams(params json.RawMessage) (bool, error) {
	// This parameter type has no fields, so there is nothing to check.
	return true, nil
}

func validQueryPeersParams(params json.RawMessage) (bool, error) {
	// This parameter type has no fields, so there is nothing to check.
	return true, nil
}

func validQueryStatParams(params json.RawMessage) (bool, error) {
	// This parameter type has no fields, so there is nothing to check.
	return true, nil
}
