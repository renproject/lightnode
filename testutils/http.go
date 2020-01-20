package testutils

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/testutil"
)

// ChanWriter is a io.Writer which writes all messages to an output channel.
type ChanWriter struct {
	output chan string
}

// NewChanWriter returns a new ChanWriter and the output channel.
func NewChanWriter() (ChanWriter, chan string) {
	output := make(chan string, 128)
	return ChanWriter{output: output}, output
}

// ChanWriter implements the `io.Writer` interface.
func (cl ChanWriter) Write(p []byte) (n int, err error) {
	select {
	case cl.output <- string(p):
		return len(p), nil
	case <-time.After(time.Second):
		return 0, errors.New("timeout")
	}
}

func ChanMiddleware(requests chan jsonrpc.Request, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request jsonrpc.Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			response := jsonrpc.Response{
				Version: "2.0",
				ID:      -1,
				Result:  nil,
				Error: &jsonrpc.Error{
					Code:    jsonrpc.ErrorCodeInvalidJSON,
					Message: "invalid json request",
				},
			}
			if err := json.NewEncoder(w).Encode(response); err != nil {
				panic(fmt.Sprintf("fail to write response back, err = %v", err))
			}
		}
		requests <- request
		next.ServeHTTP(w, r)
	}
}

// PanicHandler returns a simple `http.HandlerFunc` which panics.
func PanicHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		panic("intentionally panic here")
	}
}

// NilHandler don't do anything with the request which causing EOF error.
func NilHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
	}
}

// OKHandler returns a non-error response if the request if of valid format.
func OKHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request jsonrpc.Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			response := jsonrpc.Response{
				Version: "2.0",
				ID:      -1,
				Result:  nil,
				Error: &jsonrpc.Error{
					Code:    jsonrpc.ErrorCodeInvalidJSON,
					Message: "invalid json request",
				},
			}
			if err := json.NewEncoder(w).Encode(response); err != nil {
				panic(fmt.Sprintf("fail to write response back, err = %v", err))
			}
		}

		response := jsonrpc.Response{
			Version: "2.0",
			ID:      request.ID,
		}
		data, err := json.Marshal(response)
		if err != nil {
			panic(err)
		}
		w.Write(data)
	}
}

// TimeoutHandler returns a simple `http.HandlerFunc` which sleeps a certain
// amount of time before responding.
func TimeoutHandler(timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(timeout)
		w.Write([]byte{})
	}
}

// SendRequestAsync send the request to the target address. It returns a chan of
// jsonrpc.Response without any blocking, actual response(or error) will be sent
// to the channel while received.
func SendRequestAsync(req jsonrpc.Request, to string) (chan *jsonrpc.Response, error) {
	client := new(http.Client)
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	r, err := http.NewRequest(http.MethodPost, to, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	respChan := make(chan *jsonrpc.Response, 1)
	go func() {
		r.Header.Set("Content-Type", "application/json")
		response, err := client.Do(r)
		if err != nil {
			respChan <- nil
			return
		}

		// Read response.
		var resp jsonrpc.Response
		if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
			respChan <- nil
			return
		}
		respChan <- &resp
	}()
	return respChan, nil
}

func RandomMethod() string {
	methods := []string{
		jsonrpc.MethodQueryBlock,
		jsonrpc.MethodQueryBlocks,
		jsonrpc.MethodSubmitTx,
		jsonrpc.MethodQueryTx,
		jsonrpc.MethodQueryNumPeers,
		jsonrpc.MethodQueryPeers,
		jsonrpc.MethodQueryEpoch,
		jsonrpc.MethodQueryStat,
	}
	return methods[rand.Intn(len(methods))]
}

func RandomRequest(method string) jsonrpc.Request {
	request := jsonrpc.Request{
		Version: "2.0",
		ID:      float64(rand.Int31()),
		Method:  method,
		Params:  nil,
	}

	var params interface{}
	switch method {
	// Blocks
	case jsonrpc.MethodQueryBlock:
		height := testutil.RandomU64()
		params = jsonrpc.ParamsQueryBlock{
			BlockHeight: &height,
		}
	case jsonrpc.MethodQueryBlocks:
		height := testutil.RandomU64()
		n := testutil.RandomU64()
		params = jsonrpc.ParamsQueryBlocks{
			BlockHeight: &height,
			N:           &n,
		}

	// Transactions
	case jsonrpc.MethodSubmitTx:
		params = jsonrpc.ParamsSubmitTx{
			Tx: testutil.RandomTransformTx().Tx,
		}
	case jsonrpc.MethodQueryTx:
		params = jsonrpc.ParamsQueryTx{
			TxHash: testutil.RandomB32(),
		}

	// Peers
	case jsonrpc.MethodQueryNumPeers:
		params = jsonrpc.ParamsQueryNumPeers{}
	case jsonrpc.MethodQueryPeers:
		params = jsonrpc.ParamsQueryPeers{}

	// Epoch
	case jsonrpc.MethodQueryEpoch:
		params = jsonrpc.ParamsQueryEpoch{
			EpochHash: testutil.RandomB32(),
		}

	// System stats
	case jsonrpc.MethodQueryStat:
		params = jsonrpc.ParamsQueryStat{}

	default:
		panic(fmt.Errorf("unexpected method=%v", method))
	}

	var err error
	request.Params, err = json.Marshal(params)
	if err != nil {
		panic(err)
	}
	return request
}

func BatchRequest(size int) []jsonrpc.Request {
	reqs := make([]jsonrpc.Request, size)
	for i := range reqs {
		reqs[i] = RandomRequest(RandomMethod())
	}
	return reqs
}
