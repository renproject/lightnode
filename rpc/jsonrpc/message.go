package jsonrpc

import "github.com/republicprotocol/darknode-go/server/jsonrpc"

type Requests []jsonrpc.Request

func (requests Requests) IsRequest() {

}
