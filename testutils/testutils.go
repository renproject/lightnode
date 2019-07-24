package testutils

import (
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/phi"
)

type Inspector struct {
	messages chan phi.Message
}

func NewInspector(cap int) (phi.Task, <-chan phi.Message) {
	opts := phi.Options{Cap: cap}
	messages := make(chan phi.Message, opts.Cap)
	inspector := Inspector{messages}
	return phi.New(&inspector, opts), messages
}

func (inspector *Inspector) Handle(_ phi.Task, message phi.Message) {
	inspector.messages <- message
}

func ValidRequest(method string) jsonrpc.Request {
	return jsonrpc.Request{
		Version: "2.0",
		Method:  method,
	}
}
