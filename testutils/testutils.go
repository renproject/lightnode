package testutils

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/renproject/darknode/engine"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx/txutil"
	v0 "github.com/renproject/lightnode/compat/v0"
	"github.com/renproject/lightnode/resolver"
	"github.com/renproject/multichain"
	"github.com/renproject/multichain/api/utxo"
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
	case jsonrpc.MethodQueryBlockState:
		params = jsonrpc.ParamsQueryBlockState{}
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

func MockSystemState() engine.SystemState {
	pubkeyBytes, err := base64.URLEncoding.DecodeString("Akwn5WEMcB2Ff_E0ZOoVks9uZRvG_eFD99AysymOc5fm")
	if err != nil {
		panic(fmt.Sprintf("failed to encode pubk %v", err))
	}
	return engine.SystemState{
		Epoch: engine.SystemStateEpoch{
			Hash:      pack.Bytes32{},
			Number:    0,
			NumNodes:  0,
			Timestamp: 0,
		},
		Nodes: []engine.SystemStateNode{},
		Shards: engine.SystemStateShards{
			Primary: []engine.SystemStateShardsShard{{
				Shard:  pack.Bytes32{},
				PubKey: pubkeyBytes,
			}},
			Secondary: []engine.SystemStateShardsShard{},
			Tertiary:  []engine.SystemStateShardsShard{},
		},
	}
}

func MockQueryBlockStateResponse() jsonrpc.ResponseQueryBlockState {

	bitcoinState, err := pack.Encode(MockEngineState()["BTC"])
	if err != nil {
		panic(fmt.Sprintf("failed to encode bitcoinState:  %v", err))
	}
	bitcoinStateS := bitcoinState.(pack.Struct)

	terraState, err := pack.Encode(MockEngineState()["LUNA"])
	if err != nil {
		panic(fmt.Sprintf("failed to encode terraState:  %v", err))
	}
	terraStateS := terraState.(pack.Struct)

	systemStateP, err := pack.Encode(MockSystemState())
	if err != nil {
		panic(fmt.Sprintf("failed to encode system state:  %v", err))
	}

	var state pack.Typed
	state = append(state, pack.NewStructField("System", systemStateP))
	state = append(state, pack.NewStructField(string(multichain.BTC), bitcoinStateS))
	state = append(state, pack.NewStructField(string(multichain.BCH), bitcoinStateS))
	state = append(state, pack.NewStructField(string(multichain.ZEC), bitcoinStateS))
	state = append(state, pack.NewStructField(string(multichain.LUNA), terraStateS))

	return jsonrpc.ResponseQueryBlockState{
		State: state,
	}
}

func MockEngineState() map[string]engine.XState {
	pkBytes, err := base64.RawURLEncoding.DecodeString("Akwn5WEMcB2Ff_E0ZOoVks9uZRvG_eFD99AysymOc5fm")
	if err != nil {
		panic(fmt.Sprintf("failed to decode pub key: %v", err))
	}

	bitcoinShardState := engine.XStateShardUTXO{
		Outpoint: utxo.Outpoint{
			Hash:  []byte{0},
			Index: 0,
		},
		Value:        pack.NewU256FromU8(0),
		PubKeyScript: pkBytes,
	}
	bitcoinShardStateP, err := pack.Encode(bitcoinShardState)
	if err != nil {
		panic(fmt.Sprintf("failed to encode utxo shard: %v", err))
	}

	accountShardState := engine.XStateShardAccount{
		Nonce:   pack.NewU256FromU64(0),
		Gnonces: []engine.XStateShardGnonce{},
	}
	accountShardStateP, err := pack.Encode(accountShardState)
	if err != nil {
		panic(fmt.Sprintf("failed to encode account shard: %v", err))
	}

	utxoState := engine.XState{
		LatestHeight:  pack.NewU256FromU64(0),
		GasCap:        pack.NewU256FromU64(2),
		GasLimit:      pack.NewU256FromU64(3),
		GasPrice:      pack.NewU256FromU64(0),
		MinimumAmount: pack.NewU256FromU64(0),
		DustAmount:    pack.NewU256FromU64(0),
		Shards: []engine.XStateShard{
			{
				Shard:  pack.Bytes32{},
				PubKey: pkBytes,
				Queue:  []engine.XStateShardQueueItem{},
				State:  bitcoinShardStateP,
			},
		},
		Fees: engine.XStateFees{
			Unassigned: pack.NewU256FromU64(0),
			Epochs:     []engine.XStateFeesEpoch{},
			Nodes:      []engine.XStateFeesNode{},
		},
	}
	accountState := engine.XState{
		LatestHeight:  pack.NewU256FromU64(0),
		GasCap:        pack.NewU256FromU64(2),
		GasLimit:      pack.NewU256FromU64(3),
		GasPrice:      pack.NewU256FromU64(0),
		MinimumAmount: pack.NewU256FromU64(0),
		DustAmount:    pack.NewU256FromU64(0),
		Shards: []engine.XStateShard{
			{
				Shard:  pack.Bytes32{},
				PubKey: pkBytes,
				Queue:  []engine.XStateShardQueueItem{},
				State:  accountShardStateP,
			},
		},
		Fees: engine.XStateFees{
			Unassigned: pack.NewU256FromU64(0),
			Epochs:     []engine.XStateFeesEpoch{},
			Nodes:      []engine.XStateFeesNode{},
		},
	}

	return map[string]engine.XState{
		"BTC":  utxoState,
		"BCH":  utxoState,
		"ZEC":  utxoState,
		"DGB":  utxoState,
		"DOGE": utxoState,
		"FIL":  accountState,
		"LUNA": accountState,
	}
}

func MockParamsSubmitGatewayFil() resolver.ParamsSubmitGateway {
	jsonStr := `{"gateway":"t1syl7g6fypnv2ykixojpfjaxdpoqpmqgpodaojsa","tx":{"selector":"FIL/toSolana","version":"1","in":{"t":{"struct":[{"payload":"bytes"},{"phash":"bytes32"},{"to":"string"},{"nonce":"bytes32"},{"nhash":"bytes32"},{"gpubkey":"bytes"},{"ghash":"bytes32"}]},"v":{"ghash":"6hXCi-_3bJ6Qh6wo18xg8U0rYg4aeqKsMh384rIRIeM","gpubkey":"Aw3WX32ykguyKZEuP0IT3RUOX5csm3PpvnFNhEVhrDVc","nhash":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","nonce":"ICAgICAgICAgICAgICAgICAgICAgICAgICAgIDQ5OTY","payload":"","phash":"xdJGAYb3IzySfn2y3McDwOUAtlPKgic7e_rYBF2FpHA","to":"CCUr6NyGmj3MLVwN6XoxRm2RDtaZUqxS6djctfMyFszr"}}}}`

	var params resolver.ParamsSubmitGateway

	err := json.Unmarshal([]byte(jsonStr), &params)

	if err != nil {
		panic(fmt.Sprintf("Failed to unmarshal params %v", err))
	}
	return params
}

func MockParamSubmitTxV0ZEC() v0.ParamsSubmitTx {
	jsonStr := `{"tx":{"to":"ZEC0Zec2Eth","in":[{"name":"p","type":"ext_ethCompatPayload","value":{"abi":"W3siY29uc3RhbnQiOmZhbHNlLCJpbnB1dHMiOlt7InR5cGUiOiJzdHJpbmciLCJuYW1lIjoiX3N5bWJvbCJ9LHsidHlwZSI6ImFkZHJlc3MiLCJuYW1lIjoiX2FkZHJlc3MifSx7Im5hbWUiOiJfYW1vdW50IiwidHlwZSI6InVpbnQyNTYifSx7Im5hbWUiOiJfbkhhc2giLCJ0eXBlIjoiYnl0ZXMzMiJ9LHsibmFtZSI6Il9zaWciLCJ0eXBlIjoiYnl0ZXMifV0sIm91dHB1dHMiOltdLCJwYXlhYmxlIjp0cnVlLCJzdGF0ZU11dGFiaWxpdHkiOiJwYXlhYmxlIiwidHlwZSI6ImZ1bmN0aW9uIiwibmFtZSI6Im1pbnQifV0=","value":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAADqiy/w1/VGr66uF3EwZzY1fe+kNAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAADWkVDAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","fn":"bWludA=="}},{"name":"token","type":"ext_ethCompatAddress","value":"6f35D542f3E0886281fb6152010fb52aC6B931F6"},{"name":"to","type":"ext_ethCompatAddress","value":"6fA045D176CE69Fdf9837242E8A72e81c2750E64"},{"name":"n","type":"b32","value":"w6chJGKc0alaCkrLxdeAqubqyhfNGv+FD2Zh2eU4lRM="},{"name":"utxo","type":"ext_btcCompatUTXO","value":{"txHash":"eK3tRXHMrxw1SXOSGVJAWIpfTTy4Cr1g6wMWBATt3UM=","vOut":"0"}}]},"tags":[]}`

	var params v0.ParamsSubmitTx
	err := json.Unmarshal([]byte(jsonStr), &params)
	if err != nil {
		fmt.Printf("Failed to unmarshal params %v", jsonStr)
	}
	return params
}

func MockParamSubmitTxV0BTC() v0.ParamsSubmitTx {
	jsonStr := `{"tx":{"to":"BTC0Btc2Eth","in":[{"name":"p","type":"ext_ethCompatPayload","value":{"abi":"W3siY29uc3RhbnQiOmZhbHNlLCJpbnB1dHMiOlt7InR5cGUiOiJzdHJpbmciLCJuYW1lIjoiX3N5bWJvbCJ9LHsidHlwZSI6ImFkZHJlc3MiLCJuYW1lIjoiX2FkZHJlc3MifSx7Im5hbWUiOiJfYW1vdW50IiwidHlwZSI6InVpbnQyNTYifSx7Im5hbWUiOiJfbkhhc2giLCJ0eXBlIjoiYnl0ZXMzMiJ9LHsibmFtZSI6Il9zaWciLCJ0eXBlIjoiYnl0ZXMifV0sIm91dHB1dHMiOltdLCJwYXlhYmxlIjp0cnVlLCJzdGF0ZU11dGFiaWxpdHkiOiJwYXlhYmxlIiwidHlwZSI6ImZ1bmN0aW9uIiwibmFtZSI6Im1pbnQifV0=","value":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAADqiy/w1/VGr66uF3EwZzY1fe+kNAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAADQlRDAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","fn":"bWludA=="}},{"name":"token","type":"ext_ethCompatAddress","value":"581347fc652f9FCdbCA8372A4f65404C4154e93b"},{"name":"to","type":"ext_ethCompatAddress","value":"6fA045D176CE69Fdf9837242E8A72e81c2750E64"},{"name":"n","type":"b32","value":"D7NxsMHBvIxWLFebaK86BhxfFf9srpj+u67GZV/fKPs="},{"name":"utxo","type":"ext_btcCompatUTXO","value":{"txHash":"mArPrCPk9+zMT6h9s0aUKuJ1zV4S5X1zXqZObPL0wMM=","vOut":"0"}}]},"tags":[]}`

	var params v0.ParamsSubmitTx
	err := json.Unmarshal([]byte(jsonStr), &params)
	if err != nil {
		fmt.Printf("Failed to unmarshal params %v", jsonStr)
	}
	return params
}

func MockBurnParamSubmitTxV0BTC() v0.ParamsSubmitTx {
	jsonStr := `{"tx":{"to":"BTC0Eth2Btc","in":[{"name":"ref","type":"u64","value":"2482"}]},"tags":[]}`

	var params v0.ParamsSubmitTx
	err := json.Unmarshal([]byte(jsonStr), &params)
	if err != nil {
		fmt.Printf("Failed to unmarshal params %v", jsonStr)
	}
	return params
}

func MockParamSubmitTxV0BCH() v0.ParamsSubmitTx {
	jsonStr := `{"tx":{"to":"BCH0Bch2Eth","in":[{"name":"p","type":"ext_ethCompatPayload","value":{"abi":"W3siY29uc3RhbnQiOmZhbHNlLCJpbnB1dHMiOlt7InR5cGUiOiJzdHJpbmciLCJuYW1lIjoiX3N5bWJvbCJ9LHsidHlwZSI6ImFkZHJlc3MiLCJuYW1lIjoiX2FkZHJlc3MifSx7Im5hbWUiOiJfYW1vdW50IiwidHlwZSI6InVpbnQyNTYifSx7Im5hbWUiOiJfbkhhc2giLCJ0eXBlIjoiYnl0ZXMzMiJ9LHsibmFtZSI6Il9zaWciLCJ0eXBlIjoiYnl0ZXMifV0sIm91dHB1dHMiOltdLCJwYXlhYmxlIjp0cnVlLCJzdGF0ZU11dGFiaWxpdHkiOiJwYXlhYmxlIiwidHlwZSI6ImZ1bmN0aW9uIiwibmFtZSI6Im1pbnQifV0=","value":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAADqiy/w1/VGr66uF3EwZzY1fe+kNAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAADQkNIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","fn":"bWludA=="}},{"name":"token","type":"ext_ethCompatAddress","value":"148234809A551c131951bD01640494eecB905b08"},{"name":"to","type":"ext_ethCompatAddress","value":"6fA045D176CE69Fdf9837242E8A72e81c2750E64"},{"name":"n","type":"b32","value":"PO6UI3V84YBYp9MiJGvi6SyUzxXOHugTaqiQYFuTxNo="},{"name":"utxo","type":"ext_btcCompatUTXO","value":{"txHash":"xt4W7r/K0xZh6awkn7PgJqpvS/mI23exobLJnmgeFfA=","vOut":"0"}}]},"tags":[]}`

	var params v0.ParamsSubmitTx
	err := json.Unmarshal([]byte(jsonStr), &params)
	if err != nil {
		fmt.Printf("Failed to unmarshal params %v", jsonStr)
	}
	return params
}

func MockQueryTxResponse() jsonrpc.ResponseQueryTx {
	jsonStr := `{"tx":{"hash":"hOyESX5qOdO2atRZt1T18YLP7vtctSpRpP-yTEVHaO0","version":"1","selector":"LUNA/toEthereum","in":{"t":{"struct":[{"txid":"bytes"},{"txindex":"u32"},{"amount":"u256"},{"payload":"bytes"},{"phash":"bytes32"},{"to":"string"},{"nonce":"bytes32"},{"nhash":"bytes32"},{"gpubkey":"bytes"},{"ghash":"bytes32"}]},"v":{"amount":"2000000","ghash":"STHYl6MD1O7sdsbD30ez2eMfOiVyHKKckgZdrej_rrA","gpubkey":"AryOfNtQ4P_VKlT6-YTWrI_l7mhW04pfis2b0z_Jx9UN","nhash":"b53RK1thjnj_aIIwrI6XhA6PDhq1vdKQEWz4yocGK_g","nonce":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","payload":"","phash":"xdJGAYb3IzySfn2y3McDwOUAtlPKgic7e_rYBF2FpHA","to":"0xa0df350d2637096571F7A701CBc1C5fdE30dF76A","txid":"aT2vuwEIqE6ACHVx4RxcT__hbQ0r_eHv_r3uxl92yz0","txindex":"0"}},"out":{"t":{"struct":[{"hash":"bytes32"},{"amount":"u256"},{"fees":"u256"},{"sighash":"bytes32"},{"sig":"bytes65"},{"txid":"bytes"},{"txindex":"u32"},{"revert":"string"}]},"v":{"amount":"1698300","fees":"301700","hash":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","revert":"","sig":"hsLcCyisnEvfE0OKzNDsWxdecXaekVapQNlpZ7pIlw1Q-ZxsJ0SU--L30g-JFzHbZ2fPAy4r_KycQji240wAKgA","sighash":"0vuEp5oKnlLUanQFXnKbN6xpjXqhEBPU82JacxxgbhE","txid":"","txindex":"0"}}},"txStatus":"done"}`
	var params jsonrpc.ResponseQueryTx
	err := json.Unmarshal([]byte(jsonStr), &params)
	if err != nil {
		fmt.Printf("Failed to unmarshal params %v", jsonStr)
	}
	return params
}
