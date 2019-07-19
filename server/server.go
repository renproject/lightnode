package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/renproject/phi"
	"github.com/republicprotocol/darknode-go/p2p"
	"github.com/republicprotocol/darknode-go/stat"
	"github.com/republicprotocol/darknode-go/sync"
	"github.com/republicprotocol/darknode-go/txcheck"
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
	rateLimiter RateLimiter
	validator   phi.Sender
}

func NewServer(logger logrus.FieldLogger, port string) *Server {
	return &Server{
		port:   port,
		logger: logger,
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
	if !server.rateLimiter.Allow(r.RemoteAddr) {
		// TODO: Return error response.
		return
	}

	rawMessage := json.RawMessage{}
	if err := json.NewDecoder(r.Body).Decode(&rawMessage); err != nil {
		// TODO: Return error response.
	}
	// Unmarshal requests with support for batching
	reqs := []Request{}
	if err := json.Unmarshal(rawMessage, &reqs); err != nil {
		// If we fail to unmarshal the raw message into a list of JSON-RPC 2.0
		// requests, try to unmarshal the raw messgae into a single JSON-RPC 2.0
		// request
		var req Request
		if err := json.Unmarshal(rawMessage, &req); err != nil {
			// TODO: Return error response.
			return
		}
		reqs = []Request{req}
	}

	// Check that batch size does not exceed the maximum allowed batch size
	if len(reqs) > server.options.MaxBatchSize {
		// TODO: Return error response.
		return
	}

	// Handle all requests concurrently and, after all responses have been
	// received, write all responses back to the http.ResponseWriter
	responses := make([]Response, len(reqs))
	phi.ParForAll(reqs, func(i int) {
		// Each request is rate-limited independently, because all methods have
		// potentially different rate-limits
		responses[i] = server.handleRequest(ctx, r.RemoteAddr, reqs[i])
	})
}

type Request interface {
	IsRequest()
}

type SubmitTx struct {
	Message txcheck.AcceptTx
}

func (SubmitTx) IsMessage() {}
func (SubmitTx) IsRequest() {}

func NewSubmitTx(message txcheck.AcceptTx) SubmitTx {
	return SubmitTx{message}
}

type QueryTx struct {
	Message sync.QueryTx
}

func (QueryTx) IsMessage() {}
func (QueryTx) IsRequest() {}

func NewQueryTx(message sync.QueryTx) QueryTx {
	return QueryTx{message}
}

type QueryPeers struct {
	Message p2p.QueryPeers
}

func (QueryPeers) IsMessage() {}
func (QueryPeers) IsRequest() {}

func NewQueryPeers(message p2p.QueryPeers) QueryPeers {
	return QueryPeers{message}
}

type QueryNumPeers struct {
	Message p2p.QueryNumPeers
}

func (QueryNumPeers) IsMessage() {}
func (QueryNumPeers) IsRequest() {}

func NewQueryNumPeers(message p2p.QueryNumPeers) QueryNumPeers {
	return QueryNumPeers{message}
}

type QueryStat struct {
	Message stat.QueryStat
}

func (QueryStat) IsMessage() {}
func (QueryStat) IsRequest() {}

func NewQueryStat(message stat.QueryStat) QueryStat {
	return QueryStat{message}
}

type Response interface {
	IsResponse()
}

type SubmitTxResponse struct {
	Message txcheck.AcceptTxResponse
}

func (SubmitTxResponse) IsMessage()  {}
func (SubmitTxResponse) IsResponse() {}

func NewSubmitTxResponse(message txcheck.AcceptTxResponse) SubmitTxResponse {
	return SubmitTxResponse{message}
}

type QueryTxResponse struct {
	Message sync.QueryTxResponse
}

func (QueryTxResponse) IsMessage()  {}
func (QueryTxResponse) IsResponse() {}

func NewQueryTxResponse(message sync.QueryTxResponse) QueryTxResponse {
	return QueryTxResponse{message}
}

type QueryPeersResponse struct {
	Message p2p.QueryPeersResponse
}

func (QueryPeersResponse) IsMessage()  {}
func (QueryPeersResponse) IsResponse() {}

func NewQueryPeersResponse(message p2p.QueryPeersResponse) QueryPeersResponse {
	return QueryPeersResponse{message}
}

type QueryNumPeersResponse struct {
	Message p2p.QueryNumPeersResponse
}

func (QueryNumPeersResponse) IsMessage()  {}
func (QueryNumPeersResponse) IsResponse() {}

func NewQueryNumPeersResponse(message p2p.QueryNumPeersResponse) QueryNumPeersResponse {
	return QueryNumPeersResponse{message}
}

type QueryStatResponse struct {
	Message stat.QueryStatResponse
}

func (QueryStatResponse) IsMessage()  {}
func (QueryStatResponse) IsResponse() {}

func NewQueryStatResponse(message stat.QueryStatResponse) QueryStatResponse {
	return QueryStatResponse{message}
}
