package testutils

import (
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/lightnode/http"
	"github.com/renproject/phi"
)

type MockValidator struct {
	alwaysSuccess bool
}

func NewMockValidator(alwaysSuccess bool) phi.Task{
	validator := MockValidator{alwaysSuccess:alwaysSuccess}
	opts := phi.Options{Cap: 128}
	return phi.New(validator, opts)
}

func (m MockValidator) Handle(_ phi.Task, message phi.Message) {
	msg, ok := message.(http.RequestWithResponder)
	if !ok {
		panic("invalid message type")
	}
	response := jsonrpc.Response{
		Version: "2.0",
		ID:      msg.Request.ID,
	}
	if !m.alwaysSuccess{
		err := jsonrpc.NewError(jsonrpc.ErrorCodeInvalidJSON, "bad request", nil)
		response.Error = &err
	}
	msg.Responder <- response
}
