package testutils

import (
	"fmt"

	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/lightnode/http"
	"github.com/renproject/phi"
)

type MockCacher struct {
}

func NewMockCacher() phi.Task {
	return phi.New(
		&MockCacher{},
		phi.Options{
			Cap: 128,
		},
	)
}

// Handle implements the `phi.Handler` interface.
// We respond with darknode v1 responses to test the compat endpoints
func (cacher *MockCacher) Handle(_ phi.Task, message phi.Message) {
	msg, ok := message.(http.RequestWithResponder)
	if !ok {
		panic(fmt.Errorf("unexpected message type %T", message))
	}

	switch msg.Method {
	case jsonrpc.MethodQueryBlockState:
		msg.Responder <- jsonrpc.Response{
			Version: "2.0",
			ID:      msg.ID,
			Result:  MockQueryBlockStateResponse(),
			Error:   nil,
		}
	default:
		msg.Responder <- jsonrpc.Response{}
	}

}
