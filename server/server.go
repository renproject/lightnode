package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/renproject/darknode/jsonrpc"
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
	rateLimiter *jsonrpc.RateLimiter
	validator   phi.Sender
}

func New(logger logrus.FieldLogger, port string, options Options, validator phi.Sender) *Server {
	rateLimiter := jsonrpc.NewRateLimiter(1.0, 1)
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
	// TODO: Rate limit request types individually.
	if !server.rateLimiter.IPAddressLimiter(r.RemoteAddr).Allow() {
		// TODO: Return error response.
		return
	}

	rawMessage := json.RawMessage{}
	if err := json.NewDecoder(r.Body).Decode(&rawMessage); err != nil {
		// TODO: Return error response.
	}
	// Unmarshal requests with support for batching
	reqs := []jsonrpc.Request{}
	if err := json.Unmarshal(rawMessage, &reqs); err != nil {
		// If we fail to unmarshal the raw message into a list of JSON-RPC 2.0
		// requests, try to unmarshal the raw messgae into a single JSON-RPC 2.0
		// request
		var req jsonrpc.Request
		if err := json.Unmarshal(rawMessage, &req); err != nil {
			// TODO: Return error response.
			return
		}
		reqs = []jsonrpc.Request{req}
	}

	// Check that batch size does not exceed the maximum allowed batch size
	if len(reqs) > server.options.MaxBatchSize {
		// TODO: Return error response.
		return
	}

	// Handle all requests concurrently and, after all responses have been
	// received, write all responses back to the http.ResponseWriter
	responses := make([]jsonrpc.Response, len(reqs))
	phi.ParForAll(reqs, func(i int) {
		reqWithResponder, responder := NewRequestWithResponder(reqs[i])
		server.validator.Send(reqWithResponder)
		responses[i] = <-responder
	})
	w.Header().Set("Content-Type", "application/json")
	if len(responses) == 1 {
		if err := json.NewEncoder(w).Encode(responses[0]); err != nil {
			server.logger.Errorf("error writing http response: %v", err)
		}
	}
	if err := json.NewEncoder(w).Encode(responses); err != nil {
		server.logger.Errorf("error writing http response: %v", err)
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

func NewRequestWithResponder(req jsonrpc.Request) (RequestWithResponder, chan jsonrpc.Response) {
	responder := make(chan jsonrpc.Response, 1)
	return RequestWithResponder{req, responder}, responder
}
