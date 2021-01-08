package testutils

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx/txutil"
	v0 "github.com/renproject/lightnode/compat/v0"
	"github.com/renproject/multichain"
	"github.com/renproject/pack"
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
	case jsonrpc.MethodQueryTxs:
		params = jsonrpc.ParamsQueryTxs{}
	case jsonrpc.MethodQueryNumPeers:
		params = jsonrpc.ParamsQueryNumPeers{}
	case jsonrpc.MethodQueryPeers:
		params = jsonrpc.ParamsQueryPeers{}
	case jsonrpc.MethodQueryShards:
		params = jsonrpc.ParamsQueryShards{}
	case jsonrpc.MethodQueryStat:
		params = jsonrpc.ParamsQueryStat{}
	case jsonrpc.MethodQueryFees:
		params = jsonrpc.ParamsQueryFees{}
	case jsonrpc.MethodQueryConfig:
		params = jsonrpc.ParamsQueryConfig{}
	case jsonrpc.MethodQueryState:
		params = jsonrpc.ParamsQueryState{}
	default:
		panic("invalid method")
	}
	return
}

func RandomSubmitTxParams() jsonrpc.ParamsSubmitTx {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return jsonrpc.ParamsSubmitTx{Tx: txutil.RandomTx(r)}
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

func MockQueryStateResponse() jsonrpc.ResponseQueryState {
	bitcoinState := pack.NewStruct(
		"pubKey", pack.String("Akwn5WEMcB2Ff_E0ZOoVks9uZRvG_eFD99AysymOc5fm"),
	)

	return jsonrpc.ResponseQueryState{
		State: map[multichain.Chain]pack.Struct{
			multichain.Bitcoin: bitcoinState,
		},
	}
}

func MockParamSubmitTxV0ZEC() v0.ParamsSubmitTx {
	jsonStr := `{"tx":{"to":"ZEC0Zec2Eth","in":[{"name":"p","type":"ext_ethCompatPayload","value":{"abi":"W3siY29uc3RhbnQiOmZhbHNlLCJpbnB1dHMiOlt7InR5cGUiOiJzdHJpbmciLCJuYW1lIjoiX3N5bWJvbCJ9LHsidHlwZSI6ImFkZHJlc3MiLCJuYW1lIjoiX2FkZHJlc3MifSx7Im5hbWUiOiJfYW1vdW50IiwidHlwZSI6InVpbnQyNTYifSx7Im5hbWUiOiJfbkhhc2giLCJ0eXBlIjoiYnl0ZXMzMiJ9LHsibmFtZSI6Il9zaWciLCJ0eXBlIjoiYnl0ZXMifV0sIm91dHB1dHMiOltdLCJwYXlhYmxlIjp0cnVlLCJzdGF0ZU11dGFiaWxpdHkiOiJwYXlhYmxlIiwidHlwZSI6ImZ1bmN0aW9uIiwibmFtZSI6Im1pbnQifV0=","value":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAADqiy/w1/VGr66uF3EwZzY1fe+kNAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAADWkVDAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","fn":"bWludA=="}},{"name":"token","type":"ext_ethCompatAddress","value":"B0b458DeEa6DC99E683B63dAc3a6Ee5Fc1B6f493"},{"name":"to","type":"ext_ethCompatAddress","value":"6fA045D176CE69Fdf9837242E8A72e81c2750E64"},{"name":"n","type":"b32","value":"w6chJGKc0alaCkrLxdeAqubqyhfNGv+FD2Zh2eU4lRM="},{"name":"utxo","type":"ext_btcCompatUTXO","value":{"txHash":"eK3tRXHMrxw1SXOSGVJAWIpfTTy4Cr1g6wMWBATt3UM=","vOut":"0"}}]},"tags":[]}`

	var params v0.ParamsSubmitTx
	err := json.Unmarshal([]byte(jsonStr), &params)
	if err != nil {
		fmt.Printf("Failed to unmarshal params %v", jsonStr)
	}
	return params
}

func MockParamSubmitTxV0BTC() v0.ParamsSubmitTx {
	jsonStr := `{"tx":{"to":"BTC0Btc2Eth","in":[{"name":"p","type":"ext_ethCompatPayload","value":{"abi":"W3siY29uc3RhbnQiOmZhbHNlLCJpbnB1dHMiOlt7InR5cGUiOiJzdHJpbmciLCJuYW1lIjoiX3N5bWJvbCJ9LHsidHlwZSI6ImFkZHJlc3MiLCJuYW1lIjoiX2FkZHJlc3MifSx7Im5hbWUiOiJfYW1vdW50IiwidHlwZSI6InVpbnQyNTYifSx7Im5hbWUiOiJfbkhhc2giLCJ0eXBlIjoiYnl0ZXMzMiJ9LHsibmFtZSI6Il9zaWciLCJ0eXBlIjoiYnl0ZXMifV0sIm91dHB1dHMiOltdLCJwYXlhYmxlIjp0cnVlLCJzdGF0ZU11dGFiaWxpdHkiOiJwYXlhYmxlIiwidHlwZSI6ImZ1bmN0aW9uIiwibmFtZSI6Im1pbnQifV0=","value":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAADqiy/w1/VGr66uF3EwZzY1fe+kNAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAADQlRDAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","fn":"bWludA=="}},{"name":"token","type":"ext_ethCompatAddress","value":"48d7442B9BB36FEe26a81E1b634D1c4f75BAe4Ad"},{"name":"to","type":"ext_ethCompatAddress","value":"6fA045D176CE69Fdf9837242E8A72e81c2750E64"},{"name":"n","type":"b32","value":"D7NxsMHBvIxWLFebaK86BhxfFf9srpj+u67GZV/fKPs="},{"name":"utxo","type":"ext_btcCompatUTXO","value":{"txHash":"mArPrCPk9+zMT6h9s0aUKuJ1zV4S5X1zXqZObPL0wMM=","vOut":"0"}}]},"tags":[]}`

	var params v0.ParamsSubmitTx
	err := json.Unmarshal([]byte(jsonStr), &params)
	if err != nil {
		fmt.Printf("Failed to unmarshal params %v", jsonStr)
	}
	return params
}

func MockParamSubmitTxV0BCH() v0.ParamsSubmitTx {
	jsonStr := `{"tx":{"to":"BCH0Bch2Eth","in":[{"name":"p","type":"ext_ethCompatPayload","value":{"abi":"W3siY29uc3RhbnQiOmZhbHNlLCJpbnB1dHMiOlt7InR5cGUiOiJzdHJpbmciLCJuYW1lIjoiX3N5bWJvbCJ9LHsidHlwZSI6ImFkZHJlc3MiLCJuYW1lIjoiX2FkZHJlc3MifSx7Im5hbWUiOiJfYW1vdW50IiwidHlwZSI6InVpbnQyNTYifSx7Im5hbWUiOiJfbkhhc2giLCJ0eXBlIjoiYnl0ZXMzMiJ9LHsibmFtZSI6Il9zaWciLCJ0eXBlIjoiYnl0ZXMifV0sIm91dHB1dHMiOltdLCJwYXlhYmxlIjp0cnVlLCJzdGF0ZU11dGFiaWxpdHkiOiJwYXlhYmxlIiwidHlwZSI6ImZ1bmN0aW9uIiwibmFtZSI6Im1pbnQifV0=","value":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAADqiy/w1/VGr66uF3EwZzY1fe+kNAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAADQkNIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","fn":"bWludA=="}},{"name":"token","type":"ext_ethCompatAddress","value":"DD35d74c8EF6981Eb8b01F8F74358Cf667B20Abe"},{"name":"to","type":"ext_ethCompatAddress","value":"6fA045D176CE69Fdf9837242E8A72e81c2750E64"},{"name":"n","type":"b32","value":"PO6UI3V84YBYp9MiJGvi6SyUzxXOHugTaqiQYFuTxNo="},{"name":"utxo","type":"ext_btcCompatUTXO","value":{"txHash":"xt4W7r/K0xZh6awkn7PgJqpvS/mI23exobLJnmgeFfA=","vOut":"0"}}]},"tags":[]}`

	var params v0.ParamsSubmitTx
	err := json.Unmarshal([]byte(jsonStr), &params)
	if err != nil {
		fmt.Printf("Failed to unmarshal params %v", jsonStr)
	}
	return params
}
