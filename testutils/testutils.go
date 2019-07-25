package testutils

import (
	"encoding/json"

	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/phi"
)

// An Inspector is a mock task that will simply write all of its received
// messages out on to a channel for inspection.
type Inspector struct {
	messages chan phi.Message
}

// NewInspector constructs a new `Inspector` task.
func NewInspector(cap int) (phi.Task, <-chan phi.Message) {
	opts := phi.Options{Cap: cap}
	messages := make(chan phi.Message, opts.Cap)
	inspector := Inspector{messages}
	return phi.New(&inspector, opts), messages
}

// Handle implements the `phi.Handler` interface.
func (inspector *Inspector) Handle(_ phi.Task, message phi.Message) {
	inspector.messages <- message
}

// ValidRequest constructs a basic but valid `jsonrpc.Request` of the given
// method.
func ValidRequest(method string) jsonrpc.Request {
	switch method {
	case jsonrpc.MethodQueryBlock:
		return jsonrpc.Request{
			Version: "2.0",
			ID:      float64(1),
			Method:  method,
			Params:  json.RawMessage{},
		}
	case jsonrpc.MethodQueryBlocks:
		return jsonrpc.Request{
			Version: "2.0",
			ID:      float64(1),
			Method:  method,
			Params:  json.RawMessage{},
		}
	case jsonrpc.MethodSubmitTx:
		// TODO: Add fields to params struct.
		rawMsg, err := json.Marshal(jsonrpc.ParamsSubmitTx{})
		if err != nil {
			panic("marshalling error")
		}
		return jsonrpc.Request{
			Version: "2.0",
			ID:      float64(1),
			Method:  method,
			Params:  rawMsg,
		}
	case jsonrpc.MethodQueryTx:
		// TODO: Add fields to params struct.
		rawMsg, err := json.Marshal(jsonrpc.ParamsQueryTx{})
		if err != nil {
			panic("marshalling error")
		}
		return jsonrpc.Request{
			Version: "2.0",
			ID:      float64(1),
			Method:  method,
			Params:  rawMsg,
		}
	case jsonrpc.MethodQueryNumPeers:
		return jsonrpc.Request{
			Version: "2.0",
			ID:      float64(1),
			Method:  method,
			Params:  json.RawMessage{},
		}
	case jsonrpc.MethodQueryPeers:
		return jsonrpc.Request{
			Version: "2.0",
			ID:      float64(1),
			Method:  method,
			Params:  json.RawMessage{},
		}
	case jsonrpc.MethodQueryEpoch:
		panic("unsupported method")
	case jsonrpc.MethodQueryStat:
		return jsonrpc.Request{
			Version: "2.0",
			ID:      float64(1),
			Method:  method,
			Params:  json.RawMessage{},
		}
	default:
		panic("invalid method")
	}
}

// ErrorResponse constructs a basic valid `jsonrpc.Response` that contains a
// simple error message.
func ErrorResponse(id interface{}) jsonrpc.Response {
	err := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "test error message", json.RawMessage([]byte("{}")))
	return jsonrpc.Response{
		Version: "2.0",
		ID:      id,
		Error:   &err,
	}
}
