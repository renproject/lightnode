package jsonrpc

type Request interface {
	IsRequest()
}
