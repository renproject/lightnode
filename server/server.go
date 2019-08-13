package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/lightnode/server/ratelimiter"
	"github.com/renproject/phi"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
)

var (
	// ErrorCodeMaxBatchSizeExceeded is an implemementation specific error code
	// that indicates that the maximum batch size has been exceeded.
	ErrorCodeMaxBatchSizeExceeded = -32001

	// ErrorCodeRateLimitExceeded is an implementation specific error code that
	// indicates that the client has been rate limited.
	ErrorCodeRateLimitExceeded = -32002

	// ErrorCodeForwardingError is an implementation specific error code that
	// indicates that a http error occurred when forwarding a request to a
	// darknode.
	ErrorCodeForwardingError = -32003
)

// Options are used when constructing a `Server`.
type Options struct {
	// Maximum JSON-RPC batch size that will be accepted.
	MaxBatchSize int
}

// Server defines the HTTP server for the lightnode.
type Server struct {
	port        string
	logger      logrus.FieldLogger
	options     Options
	rateLimiter ratelimiter.RateLimiter
	validator   phi.Sender
}

// New constructs a new `Server` with the given options.
func New(logger logrus.FieldLogger, port string, options Options, validator phi.Sender) *Server {
	rateLimiter := ratelimiter.New()
	return &Server{
		port,
		logger,
		options,
		rateLimiter,
		validator,
	}
}

// Run starts the `Server` listening on its port. This function is blocking.
func (server *Server) Run() {
	r := mux.NewRouter()
	r.HandleFunc("/health", server.healthCheck).Methods("GET")
	r.HandleFunc("/", server.handleFunc).Methods("POST")
	r.Use(recoveryHandler)

	httpHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
		AllowedMethods:   []string{"POST"},
	}).Handler(r)

	// Start running the server.
	server.logger.Infof("lightnode listening on 0.0.0.0:%v...", server.port)
	http.ListenAndServe(fmt.Sprintf(":%s", server.port), httpHandler)
}

func (server *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	return
}

func (server *Server) handleFunc(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	darknodeID := v.Get("id")

	rawMessage := json.RawMessage{}
	if err := json.NewDecoder(r.Body).Decode(&rawMessage); err != nil {
		err := jsonrpc.NewError(jsonrpc.ErrorCodeInvalidJSON, "lightnode could not decode JSON request", nil)
		response := jsonrpc.NewResponse(0, nil, &err)
		writeResponses(w, []jsonrpc.Response{response})
		return
	}
	// Unmarshal requests with support for batching
	reqs := []jsonrpc.Request{}
	if err := json.Unmarshal(rawMessage, &reqs); err != nil {
		// If we fail to unmarshal the raw message into a list of JSON-RPC 2.0
		// requests, try to unmarshal the raw messgae into a single JSON-RPC 2.0
		// request
		var req jsonrpc.Request
		if err := json.Unmarshal(rawMessage, &req); err != nil {
			err := jsonrpc.NewError(jsonrpc.ErrorCodeInvalidJSON, "lightnode could not parse JSON request", nil)
			response := jsonrpc.NewResponse(0, nil, &err)
			writeResponses(w, []jsonrpc.Response{response})
			return
		}
		reqs = []jsonrpc.Request{req}
	}

	// Check that batch size does not exceed the maximum allowed batch size
	batchSize := len(reqs)
	if batchSize > server.options.MaxBatchSize {
		errMsg := fmt.Sprintf("maximum batch size exceeded: maximum is %v but got %v", server.options.MaxBatchSize, batchSize)
		err := jsonrpc.NewError(ErrorCodeMaxBatchSizeExceeded, errMsg, nil)
		response := jsonrpc.NewResponse(0, nil, &err)
		writeResponses(w, []jsonrpc.Response{response})
		return
	}

	// Handle all requests concurrently and, after all responses have been
	// received, write all responses back to the http.ResponseWriter
	responses := make([]jsonrpc.Response, len(reqs))
	phi.ParForAll(reqs, func(i int) {
		method := reqs[i].Method

		// Ensure method exists prior to checking rate limit.
		_, ok := jsonrpc.RPCs[method]
		if !ok {
			err := jsonrpc.NewError(jsonrpc.ErrorCodeMethodNotFound, "unsupported method", nil)
			response := jsonrpc.NewResponse(reqs[i].ID, nil, &err)
			responses[i] = response
			return
		}

		if !server.rateLimiter.Allow(method, r.RemoteAddr) {
			err := jsonrpc.NewError(ErrorCodeRateLimitExceeded, "rate limit exceeded", nil)
			response := jsonrpc.NewResponse(reqs[i].ID, nil, &err)
			responses[i] = response
			return
		}

		reqWithResponder := NewRequestWithResponder(reqs[i], darknodeID)
		server.validator.Send(reqWithResponder)
		responses[i] = <-reqWithResponder.Responder
	})

	if err := writeResponses(w, responses); err != nil {
		server.logger.Errorf("error writing http response: %v", err)
	}
}

func writeResponses(w http.ResponseWriter, responses []jsonrpc.Response) error {
	w.Header().Set("Content-Type", "application/json")
	if len(responses) == 1 {
		// We use the `writeError` function to determine the relevant status code
		// as we do not want to return a `http.StatusOK`.
		if responses[0].Error != nil {
			return writeError(w, responses[0].ID, *responses[0].Error)
		}
		return json.NewEncoder(w).Encode(responses[0])
	}
	return json.NewEncoder(w).Encode(responses)
}

func writeError(w http.ResponseWriter, id interface{}, err jsonrpc.Error) error {
	var statusCode int
	switch err.Code {
	case jsonrpc.ErrorCodeInvalidJSON, jsonrpc.ErrorCodeInvalidParams, jsonrpc.ErrorCodeInvalidRequest:
		statusCode = http.StatusBadRequest
	case jsonrpc.ErrorCodeMethodNotFound, jsonrpc.ErrorCodeResultNotFound:
		statusCode = http.StatusNotFound
	default:
		statusCode = http.StatusInternalServerError
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	return json.NewEncoder(w).Encode(jsonrpc.NewResponse(id, nil, &err))
}

func recoveryHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, fmt.Sprintf("recovered from a panic in the lightnode: %v", err), nil)
				writeError(w, 0, jsonErr)
			}
		}()
		h.ServeHTTP(w, r)
	})
}

// RequestWithResponder wraps a `jsonrpc.Request` with a responder channel that
// the response will be written to.
type RequestWithResponder struct {
	Request    jsonrpc.Request
	Responder  chan jsonrpc.Response
	DarknodeID string
}

// IsMessage implements the `phi.Message` interface.
func (RequestWithResponder) IsMessage() {}

// NewRequestWithResponder constructs a new request wrapper object.
func NewRequestWithResponder(req jsonrpc.Request, darknodeAddr string) RequestWithResponder {
	responder := make(chan jsonrpc.Response, 1)
	return RequestWithResponder{req, responder, darknodeAddr}
}
