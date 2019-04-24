package jsonrpc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/republicprotocol/darknode-go/server/jsonrpc"
	"github.com/sirupsen/logrus"
)

// A Service handles JSON-RPC 2.0 requests from external clients. It is used when clients want to interact with RenVM.
// It implements the http.Handler interface and is meant to be attached to a http.Server.
type Service struct {
	timeout time.Duration
	logger  logrus.FieldLogger
	queue   chan jsonrpc.Request
}

// NewService returns a new Service that can be attached to a http.Server.
func NewService(logger logrus.FieldLogger, cap int, timeout time.Duration) Service {
	return Service{
		timeout: timeout,
		logger:  logger,
		queue:   make(chan jsonrpc.Request, cap),
	}
}

// RequestQueue returns the queue which the service task can read from.
func (service Service) RequestQueue() <-chan jsonrpc.Request {
	return service.queue
}

// ServeHTTP implements the http.Handler interface.
func (service Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	req := jsonrpc.JSONRequest{}

	// Validate JSON text
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err := writeError(w, jsonrpc.ErrorCodeInvalidJSON, "bad request", nil); err != nil {
			service.logger.Errorf("error writing response: %v", err)
		}
		return
	}

	// Validate JSON RPC version
	if req.JSONRPC != "2.0" {
		if err := writeError(w, jsonrpc.ErrorCodeInvalidRequest, fmt.Sprintf("invalid jsonrpc version: %v", req.JSONRPC), nil); err != nil {
			service.logger.Errorf("error writing response: %v", err)
		}
		return
	}

	// Validate ID
	switch req.ID.(type) {
	case float32, float64, string:
	default:
		if err := writeError(w, jsonrpc.ErrorCodeInvalidRequest, fmt.Sprintf("invalid jsonrpc id: %v", req.ID), nil); err != nil {
			service.logger.Errorf("error writing response: %v", err)
		}
		return
	}

	// Validate method
	if req.Method == "" {
		if err := writeError(w, jsonrpc.ErrorCodeInvalidRequest, fmt.Sprintf("invalid jsonrpc method: %v", req.Method), nil); err != nil {
			service.logger.Errorf("error writing response: %v", err)
		}
		return
	}

	// todo : Validate parameters

	// Construct request
	var request jsonrpc.Request
	var err error
	responder := make(chan jsonrpc.Response, 1)

	switch req.Method {
	case jsonrpc.MethodSendMessage:
		request, err = service.parseSendMessageRequest(req.Params)
	case jsonrpc.MethodReceiveMessage:
		request, err = jsonrpc.NewReceiveMessageRequest(req.Params, responder)
	default:
		if err := writeError(w, jsonrpc.ErrorCodeMethodNotFound, fmt.Sprintf("invalid jsonrpc method: %v", req.Method), nil); err != nil {
			service.logger.Errorf("error writing response: %v", err)
		}
		return
	}
	if err != nil {
		if err := writeError(w, jsonrpc.ErrorCodeInvalidParams, fmt.Sprintf("invalid jsonrpc params: %v", err), nil); err != nil {
			service.logger.Errorf("error writing response: %v", err)
		}
		return
	}

	// Handle request
	res, err := service.handleRequest(request, responder, req)
	if err != nil {
		if err := writeError(w, jsonrpc.ErrorCodeInternal, err.Error(), nil); err != nil {
			service.logger.Errorf("error writing response: %v", err)
		}
		return
	}

	// Write result
	if err := writeResult(w, req.ID, res); err != nil {
		service.logger.Errorf("error writing response: %v", err)
	}
}

func (service *Service) parseSendMessageRequest(message json.RawMessage) (Requests, error) {
	requests := make(Requests, 0)
	err := json.Unmarshal(message, &requests)
	return requests, err
}

func (service *Service) handleRequest(request jsonrpc.Request, responder <-chan jsonrpc.Response, req jsonrpc.JSONRequest) (jsonrpc.Response, error) {
	timeout := time.After(service.timeout)
	select {
	case service.queue <- request:
		select {
		case res := <-responder:
			if res.Err() != nil {
				return nil, res.Err()
			}
			return res, nil
		case <-timeout:
			return nil, jsonrpc.ErrTimeout
		}
	case <-timeout:
		return nil, jsonrpc.ErrTimeout
	}
}

func writeResult(w http.ResponseWriter, id interface{}, result jsonrpc.Response) error {
	data, err  := json.Marshal(result)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(jsonrpc.JSONResponse{
		JSONRPC: "2.0",
		Result:  data,
		ID:      id,
	})
}

func writeError(w http.ResponseWriter, code int, message string, data interface{}) error {
	errData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(jsonrpc.JSONResponse{
		JSONRPC: "2.0",
		Error: &jsonrpc.JSONError{
			Code:    code,
			Message: message,
			Data:    errData,
		},
	})
}
