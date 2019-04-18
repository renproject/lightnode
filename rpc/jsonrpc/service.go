package jsonrpc

import "net/http"

type Service struct {
}

func NewService() Service {
	return Service{}
}

func (service Service) ServeHTTP(http.ResponseWriter, *http.Request) {

}
