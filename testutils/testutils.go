package testutils

import (
	"encoding/json"

	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/phi"
)

type Inspector struct {
	messages chan phi.Message
}

func NewInspector(cap int) (phi.Task, <-chan phi.Message) {
	opts := phi.Options{Cap: cap}
	messages := make(chan phi.Message, opts.Cap)
	inspector := Inspector{messages}
	return phi.New(&inspector, opts), messages
}

func (inspector *Inspector) Handle(_ phi.Task, message phi.Message) {
	inspector.messages <- message
}

func ValidRequest(method string) jsonrpc.Request {
	switch method {
	case jsonrpc.MethodQueryBlock:
		return jsonrpc.Request{
			Version: "2.0",
			ID:      1,
			Method:  method,
			Params:  json.RawMessage{},
		}
	case jsonrpc.MethodQueryBlocks:
		return jsonrpc.Request{
			Version: "2.0",
			ID:      1,
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
			ID:      1,
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
			ID:      1,
			Method:  method,
			Params:  rawMsg,
		}
	case jsonrpc.MethodQueryNumPeers:
		return jsonrpc.Request{
			Version: "2.0",
			ID:      1,
			Method:  method,
			Params:  json.RawMessage{},
		}
	case jsonrpc.MethodQueryPeers:
		return jsonrpc.Request{
			Version: "2.0",
			ID:      1,
			Method:  method,
			Params:  json.RawMessage{},
		}
	case jsonrpc.MethodQueryEpoch:
		panic("unsupported method")
	case jsonrpc.MethodQueryStat:
		return jsonrpc.Request{
			Version: "2.0",
			ID:      1,
			Method:  method,
			Params:  json.RawMessage{},
		}
	default:
		panic("invalid method")
	}
}
