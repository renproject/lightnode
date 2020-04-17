package http

import (
	"context"
	"net/url"

	"github.com/renproject/darknode/jsonrpc"
)

type RequestWithResponder struct {
	Context   context.Context
	ID        interface{}
	Method    string
	Params    interface{}
	Responder chan jsonrpc.Response
	Query     url.Values
}

// IsMessage implements the `phi.Message` interface.
func (RequestWithResponder) IsMessage() {}

func (req RequestWithResponder) RespondWithErr(code int, err error) {
	jsonErr := &jsonrpc.Error{Code: code, Message: err.Error(), Data: nil}
	req.Responder <- jsonrpc.NewResponse(req.ID, nil, jsonErr)
}

// NewRequestWithResponder constructs a new SubmitTx request wrapper object.
func NewRequestWithResponder(ctx context.Context, id interface{}, method string, params interface{}, query url.Values) RequestWithResponder {
	responder := make(chan jsonrpc.Response, 1)
	return RequestWithResponder{ctx, id, method, params, responder, query}
}
