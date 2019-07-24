package validator

import (
	"encoding/json"
	"fmt"

	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/p2p"
	"github.com/renproject/darknode/stat"
	"github.com/renproject/darknode/sync"
	"github.com/renproject/darknode/txcheck"
	"github.com/renproject/lightnode/server"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

type Validator struct {
	logger logrus.FieldLogger
	cacher phi.Sender
}

func New(cacher phi.Sender, logger logrus.FieldLogger, opts phi.Options) phi.Task {
	return phi.New(&Validator{logger, cacher}, opts)
}

func (validator *Validator) Handle(_ phi.Task, message phi.Message) {
	msg, ok := message.(server.RequestWithResponder)
	if !ok {
		validator.logger.Panicf("[validator] unexpected message type %T", message)
	}

	if err := isValid(msg.Request); err != nil {
		msg.Responder <- jsonrpc.NewResponse(msg.Request.ID, nil, err)
		return
	}
	validator.cacher.Send(msg)
}

func isValid(message jsonrpc.Request) *jsonrpc.Error {
	// Reject requests that don't conform to the JSON-RPC standard
	if message.Version != "2.0" {
		err := jsonrpc.NewError(jsonrpc.ErrorCodeInvalidRequest, fmt.Sprintf("invalid jsonrpc field: expected \"2.0\", got \"%s\"", message.Version), json.RawMessage{})
		return &err
	}

	// Reject unsupported methods
	method := message.Method
	if !isSupported(method) {
		err := jsonrpc.NewError(jsonrpc.ErrorCodeMethodNotFound, fmt.Sprintf("unsupported method %s", method), json.RawMessage{})
		return &err
	}

	// Reject requests with invalid parameters
	if ok, msg := hasValidParams(message); !ok {
		err := jsonrpc.NewError(server.ErrorCodeInvalidParams, fmt.Sprintf("invalid parameters in request: %s", msg), json.RawMessage{})
		return &err
	}

	return nil
}

func isSupported(method string) bool {
	_, supported := jsonrpc.RPCs[method]
	return supported
}

func hasValidParams(message jsonrpc.Request) (bool, string) {
	switch message.Method {
	case jsonrpc.MethodQueryBlock:
		var params sync.QueryBlock
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return false, "parameters object does not match method"
		}
		return validQueryBlockParams(params)
	case jsonrpc.MethodQueryBlocks:
		var params sync.QueryBlocks
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return false, "parameters object does not match method"
		}
		return validQueryBlocksParams(params)
	case jsonrpc.MethodSubmitTx:
		var params txcheck.AcceptTx
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return false, "parameters object does not match method"
		}
		return validSubmitTxParams(params)
	case jsonrpc.MethodQueryTx:
		var params sync.QueryTx
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return false, "parameters object does not match method"
		}
		return validQueryTxParams(params)
	case jsonrpc.MethodQueryNumPeers:
		var params p2p.QueryNumPeers
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return false, "parameters object does not match method"
		}
		return validQueryNumPeersParams(params)
	case jsonrpc.MethodQueryPeers:
		var params p2p.QueryPeers
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return false, "parameters object does not match method"
		}
		return validQueryPeersParams(params)
	case jsonrpc.MethodQueryEpoch:
		// TODO: At the time of writing this method is not supported by the
		// darknode. This should be implemented once it is implemented in the
		// darknode.
		return false, "method QueryEpoch is not supported"
	case jsonrpc.MethodQueryStat:
		var params stat.QueryStat
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return false, "parameters object does not match method"
		}
		return validQueryStatParams(params)
	default:
		// TODO: Is it ok to panic at this level, or should all panics happen
		// through a logger?
		panic(fmt.Sprintf("[validator] unsupported method %s encountered which should have been rejected by the previous checks", message.Method))
	}
}

func validQueryBlockParams(params sync.QueryBlock) (bool, string) {
	// This parameter type has no fields, so there is nothing to check.
	return true, ""
}

func validQueryBlocksParams(params sync.QueryBlocks) (bool, string) {
	// This parameter type has no fields, so there is nothing to check.
	return true, ""
}

func validSubmitTxParams(params txcheck.AcceptTx) (bool, string) {
	// TODO: Check fields.
	return true, ""
}

func validQueryTxParams(params sync.QueryTx) (bool, string) {
	// Currently the only field in the parameters is a hash field, which can't
	// really be checked for validity here
	return true, ""
}

func validQueryNumPeersParams(params p2p.QueryNumPeers) (bool, string) {
	// This parameter type has no fields, so there is nothing to check.
	return true, ""
}

func validQueryPeersParams(params p2p.QueryPeers) (bool, string) {
	// This parameter type has no fields, so there is nothing to check.
	return true, ""
}

func validQueryStatParams(params stat.QueryStat) (bool, string) {
	// This parameter type has no fields, so there is nothing to check.
	return true, ""
}
