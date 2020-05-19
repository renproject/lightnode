package testutils

import (
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"math/rand"

	"github.com/renproject/darknode/abi"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/testutil"
	"github.com/renproject/phi"
)

type MockSender struct {
	Messages chan phi.Message
}

func (m *MockSender) Send(message phi.Message) bool {
	select {
	case m.Messages <- message:
		return true
	default:
		return false
	}
}

func NewMockSender() *MockSender {
	return &MockSender{
		Messages: make(chan phi.Message, 128),
	}
}

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
func ValidRequest(method string) (id interface{}, params interface{}) {
	id = 1
	switch method {
	case jsonrpc.MethodQueryBlock:
		params = jsonrpc.ParamsQueryBlock{}
	case jsonrpc.MethodQueryBlocks:
		params = jsonrpc.ParamsQueryBlocks{}
	case jsonrpc.MethodSubmitTx:
		params = RandomSubmitTxParams()
	case jsonrpc.MethodQueryTx:
		params = jsonrpc.ParamsQueryTx{}
	case jsonrpc.MethodQueryNumPeers:
		params = jsonrpc.ParamsQueryNumPeers{}
	case jsonrpc.MethodQueryPeers:
		params = jsonrpc.ParamsQueryPeers{}
	case jsonrpc.MethodQueryShards:
		params = jsonrpc.ParamsQueryShards{}
	case jsonrpc.MethodQueryStat:
		params = jsonrpc.ParamsQueryStat{}
	default:
		log.Print("method = ", method)
		panic("invalid method")
	}
	return
}

func RandomSubmitTxParams() jsonrpc.ParamsSubmitTx {
	contract := testutil.RandomMintMethod()
	args := abi.Args{}
	for _, formal := range abi.Intrinsics[contract].In {
		arg := abi.Arg{
			Name:  formal.Name,
			Type:  formal.Type,
			Value: RandomAbiValue(formal.Type),
		}
		args.Set(arg)
	}
	return jsonrpc.ParamsSubmitTx{Tx: abi.Tx{
		Hash: testutil.RandomB32(),
		To:   contract,
		In:   args,
	}}
}

func RandomAbiValue(t abi.Type) abi.Value {
	switch t {
	case abi.TypeB32:
		return testutil.RandomB32()
	case abi.TypeU64:
		return abi.U64{Int: big.NewInt(rand.Int63())}
	case abi.ExtTypeBtcCompatUTXO:
		return testutil.RandomExtBtcCompatUTXO()
	case abi.ExtTypeEthCompatAddress:
		return testutil.RandomExtEthCompatAddress()
	case abi.ExtTypeEthCompatPayload:
		return RandomExtCompatPayload()
	default:
		panic(fmt.Sprintf("unknown type %v", t))
	}
}

func RandomExtCompatPayload() abi.Value {
	abiArg := make([]abi.B, rand.Intn(32))
	for i := range abiArg {
		abiArg[i] = testutil.RandomB()
	}
	return abi.ExtEthCompatPayload{
		ABI:   testutil.RandomB(),
		Value: testutil.RandomB(),
		Fn:    testutil.RandomB(),
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
