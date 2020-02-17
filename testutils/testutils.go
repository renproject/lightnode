package testutils

import (
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"

	"github.com/renproject/darknode/abi"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/testutil"
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

func jsonrpcResponse(id interface{}, result interface{}, err *jsonrpc.Error) jsonrpc.Response {
	return jsonrpc.Response{
		Version: "2.0",
		ID:      id,
		Result:  result,
		Error:   err,
	}
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
		return RandomSubmitTx()
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

func RandomSubmitTx() jsonrpc.Request {
	contract := testutil.RandomMintMethod()
	args := abi.Args{}
	for _, formal := range abi.Intrinsics[contract].In {
		arg := abi.Arg{
			Name:  formal.Name,
			Type:  formal.Type,
			Value: RandomAbiValue(formal.Type),
		}
		args.Append(arg)
	}
	submitTx := jsonrpc.ParamsSubmitTx{Tx: abi.Tx{
		Hash: testutil.RandomB32(),
		To:   contract,
		In:   args,
	}}
	rawMsg, err := json.Marshal(submitTx)
	if err != nil {
		panic(err)
	}
	return jsonrpc.Request{
		Version: "2.0",
		ID:      float64(1),
		Method:  jsonrpc.MethodSubmitTx,
		Params:  rawMsg,
	}
}

func RandomAbiValue(t abi.Type) abi.Value {
	switch t {
	case abi.TypeB32:
		return testutil.RandomB32()
	case abi.TypeU64:
		return abi.U64{Int: big.NewInt(rand.Int63())}
	case abi.ExtTypeBtcCompatUTXO:
		return RandomUtxo()
	case abi.ExtTypeEthCompatAddress:
		return testutil.RandomExtEthCompatAddress()
	default:
		panic(fmt.Sprintf("unknown type %v", t))
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
