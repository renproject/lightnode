package testutils

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/testutil"
	"github.com/renproject/lightnode/server"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
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

// PanicHandler returns a simple `http.HandlerFunc` which panics.
func PanicHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		panic("intentionally panic here")
	}
}

func InitServer(ctx context.Context, port string) <-chan phi.Message {
	logger := logrus.New()
	inspector, messages := NewInspector(100)
	go inspector.Run(ctx)

	options := server.Options{
		Port:         port,
		MaxBatchSize: 10,
		Timeout:      3 * time.Second,
	}
	server := server.New(logger, options, inspector)
	go server.Listen(ctx)
	time.Sleep(time.Second)

	return messages
}

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

func RandomRequest(method string) jsonrpc.Request {
	request := jsonrpc.Request{
		Version: "2.0",
		ID:      rand.Uint64(),
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
