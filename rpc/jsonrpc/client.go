package jsonrpc

import "net/http"

type Client struct {
	http.Client
}

func (client Client) Invoke() {
	panic("unimplemented")
}
