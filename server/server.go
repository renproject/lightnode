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

type Options struct {
	MaxBatchSize int
}

type Server struct {
	port        string
	logger      logrus.FieldLogger
	options     Options
	rateLimiter ratelimiter.RateLimiter
	validator   phi.Sender
}

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

func (server *Server) Run() {
	r := mux.NewRouter()
	r.HandleFunc("/", server.handleFunc)

	httpHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
		AllowedMethods:   []string{"POST"},
	}).Handler(r)

	// Start running the server.
	server.logger.Infof("lightnode listening on 0.0.0.0:%v...", server.port)
	http.ListenAndServe(fmt.Sprintf(":%s", server.port), httpHandler)
}

func (server *Server) handleFunc(w http.ResponseWriter, r *http.Request) {
	rawMessage := json.RawMessage{}
	if err := json.NewDecoder(r.Body).Decode(&rawMessage); err != nil {
		err := jsonrpc.NewError(jsonrpc.ErrorCodeInvalidJSON, "lightnode could not decode JSON request", json.RawMessage{})
		response := jsonrpc.NewResponse(0, nil, &err)
		server.writeResponses(w, []jsonrpc.Response{response})
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
			err := jsonrpc.NewError(jsonrpc.ErrorCodeInvalidJSON, "lightnode could not parse JSON request", json.RawMessage{})
			response := jsonrpc.NewResponse(0, nil, &err)
			server.writeResponses(w, []jsonrpc.Response{response})
			return
		}
		reqs = []jsonrpc.Request{req}
	}

	// Check that batch size does not exceed the maximum allowed batch size
	batchSize := len(reqs)
	if batchSize > server.options.MaxBatchSize {
		// TODO: Return error response.
		err := jsonrpc.NewError(ErrorCodeMaxBatchSizeExceeded, fmt.Sprintf("maximum batch size exceeded: maximum is %v but got %v", server.options.MaxBatchSize, batchSize), json.RawMessage{})
		response := jsonrpc.NewResponse(0, nil, &err)
		server.writeResponses(w, []jsonrpc.Response{response})
		return
	}

	// Handle all requests concurrently and, after all responses have been
	// received, write all responses back to the http.ResponseWriter
	responses := make([]jsonrpc.Response, len(reqs))
	phi.ParForAll(reqs, func(i int) {
		method := reqs[i].Method
		if !server.rateLimiter.Allow(method, r.RemoteAddr) {
			// TODO: Return error response.
			return
		}

		reqWithResponder := NewRequestWithResponder(reqs[i])
		server.validator.Send(reqWithResponder)
		responses[i] = <-reqWithResponder.Responder
	})

	server.writeResponses(w, responses)
}

func (server *Server) writeResponses(w http.ResponseWriter, responses []jsonrpc.Response) {
	w.Header().Set("Content-Type", "application/json")
	if len(responses) == 1 {
		if err := json.NewEncoder(w).Encode(responses[0]); err != nil {
			server.logger.Errorf("error writing http response: %v", err)
			return
		}
	}
	if err := json.NewEncoder(w).Encode(responses); err != nil {
		server.logger.Errorf("error writing http response: %v", err)
		return
	}
}

type Request interface {
	IsRequest()
}

type RequestWithResponder struct {
	Request   jsonrpc.Request
	Responder chan jsonrpc.Response
}

func (RequestWithResponder) IsMessage() {}
func (RequestWithResponder) IsRequest() {}

func NewRequestWithResponder(req jsonrpc.Request) RequestWithResponder {
	responder := make(chan jsonrpc.Response, 1)
	return RequestWithResponder{req, responder}
}
