package testutils

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/renproject/darknode/addr"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/p2p"
	"github.com/renproject/phi"
	"github.com/rs/cors"
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

// A MockDarknode simulates a darknode by providing basic responses to incoming
// requests.
type MockDarknode struct {
	port  int
	peers addr.MultiAddresses
}

// NewMockDarknode constructs a new `MockDarknode` that will (when `Run()`)
// listen on the given port.
func NewMockDarknode(port int, peers addr.MultiAddresses) MockDarknode {
	return MockDarknode{port, peers}
}

// Run starts the `MockDarknode` listening on its port. This function call is
// blocking.
func (dn MockDarknode) Run() {
	r := mux.NewRouter()
	r.HandleFunc("/", dn.handleFunc)

	httpHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
		AllowedMethods:   []string{"POST"},
	}).Handler(r)

	// Start running the server.
	http.ListenAndServe(fmt.Sprintf(":%v", dn.port), httpHandler)
}

func (dn *MockDarknode) handleFunc(w http.ResponseWriter, r *http.Request) {
	rawMessage := json.RawMessage{}
	if err := json.NewDecoder(r.Body).Decode(&rawMessage); err != nil {
		panic("[mock darknode] could not decode JSON request")
	}

	var req jsonrpc.Request
	if err := json.Unmarshal(rawMessage, &req); err != nil {
		panic("[mock darknode] could not parse JSON request")
	}

	res := dn.response(req)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(res); err != nil {
		panic(fmt.Sprintf("[mock darknode] error writing http response: %v", err))
	}
}

func (dn *MockDarknode) response(req jsonrpc.Request) jsonrpc.Response {
	switch req.Method {
	case jsonrpc.MethodQueryBlock:
		// TODO: Send a more appropriate response.
		return ErrorResponse(req.ID)
	case jsonrpc.MethodQueryBlocks:
		// TODO: Send a more appropriate response.
		return ErrorResponse(req.ID)
	case jsonrpc.MethodSubmitTx:
		// TODO: Send a more appropriate response.
		return ErrorResponse(req.ID)
	case jsonrpc.MethodQueryTx:
		// TODO: Send a more appropriate response.
		return ErrorResponse(req.ID)
	case jsonrpc.MethodQueryNumPeers:
		result := p2p.QueryNumPeersResponse{Err: nil, NumPeers: len(dn.peers)}
		return jsonrpcResponse(req.ID, result, nil)
	case jsonrpc.MethodQueryPeers:
		result := p2p.QueryPeersResponse{Err: nil, MultiAddresses: dn.peers}
		return jsonrpcResponse(req.ID, result, nil)
	case jsonrpc.MethodQueryEpoch:
		// TODO: Implement once this method is supported by the darknodes.
		panic("[mock darknode] querying epochs not yet supported")
	case jsonrpc.MethodQueryStat:
		// TODO: Send a more appropriate response.
		return ErrorResponse(req.ID)
	default:
		panic(fmt.Sprintf("[mock darknode] unsupported method %s", req.Method))
	}
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

// NewMultiFromIPAndPort creates a new multi address with the given ip addres
// and port, where the REN address is constant and valid.
func NewMultiFromIPAndPort(ip string, port int) addr.MultiAddress {
	address := addr.FromBase58("8MJcAFBHuYBeJp7zP1rXYMPTeJoYjs")
	value := fmt.Sprintf("/ren/%s/ip4/%v/tcp/%v", address, ip, port)
	multi, err := addr.NewMultiAddressFromString(value)
	if err != nil {
		panic("could not create multi address")
	}
	return multi
}
