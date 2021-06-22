package testutils

import (
	"fmt"

	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/lightnode/http"
	"github.com/renproject/phi"
)

type MockDispatcher struct {
	Fail bool
}

func NewMockDispatcher(fail bool) phi.Task {
	return phi.New(
		&MockDispatcher{Fail: fail},
		phi.Options{
			Cap: 128,
		},
	)
}

// Handle implements the `phi.Handler` interface.
func (dispatcher *MockDispatcher) Handle(_ phi.Task, message phi.Message) {
	msg, ok := message.(http.RequestWithResponder)
	if !ok {
		panic(fmt.Errorf("unexpected message type %T", message))
	}

	if dispatcher.Fail {
		msg.RespondWithErr(1, fmt.Errorf("set to fail"))
	}

	msg.Responder <- jsonrpc.Response{}
}
