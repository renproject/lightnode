package server

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/sirupsen/logrus"
)

// NewRecoveryMiddleware returns a new RecoveryMiddleware which recover panic
// from processing the request and log the error through the provided logger.
func NewRecoveryMiddleware(logger logrus.FieldLogger) mux.MiddlewareFunc {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					errMsg := fmt.Sprintf("Recovered from a panic in the lightnode: %v", err)
					logger.Error(errMsg)
					jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, errMsg, nil)
					writeError(w, 0, jsonErr)
				}
			}()
			h.ServeHTTP(w, r)
		})
	}
}
