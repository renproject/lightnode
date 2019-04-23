package jsonrpc

import (
	"github.com/republicprotocol/dcc/jsonrpc"
	"github.com/republicprotocol/lightnode/store"
)

type Client struct {
	store.KVStore
}

func NewClient() Client {
	return Client{}
}

// TODO: Pass list of addresses to Invoke()
func (client Client) Invoke(request jsonrpc.JSONRequest) (jsonrpc.JSONResponse, error) {
	/* var params []jsonrpc.Request
	if err := json.Unmarshal(*request.Params, params); err != nil {
		return jsonrpc.JSONResponse{}, err
	}

	for _, param := range params {
		switch req := param.(type) {
		case jsonrpc.SendMessageRequest:
			address := ""
			client.Post()
		}
	} */

	return jsonrpc.JSONResponse{}, nil
}
