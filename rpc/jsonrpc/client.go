package jsonrpc

import (
	"net/http"

	"github.com/republicprotocol/dcc/jsonrpc"
	"github.com/republicprotocol/lightnode/store"
)

type Client struct {
	http  http.Client
	store store.KVStore
}

func NewClient() Client {
	return Client{}
}

func (client Client) Invoke(req jsonrpc.JSONRequest) (jsonrpc.JSONResponse, error) {
	panic("unimplemented")
}
