package resolver

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/txpool"
	"github.com/renproject/lightnode/db"
	lhttp "github.com/renproject/lightnode/http"
	"github.com/renproject/lightnode/store"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

type Resolver struct {
	logger            logrus.FieldLogger
	txCheckerRequests chan lhttp.RequestWithResponder
	multiStore        store.MultiAddrStore
	cacher            phi.Task
	db                db.DB
	serverOptions     jsonrpc.Options
}

func New(logger logrus.FieldLogger, cacher phi.Task, multiStore store.MultiAddrStore, verifier txpool.Verifier, db db.DB, serverOptions jsonrpc.Options) *Resolver {
	requests := make(chan lhttp.RequestWithResponder, 128)
	txChecker := newTxChecker(logger, requests, verifier, db)
	go txChecker.Run()

	return &Resolver{
		logger:            logger,
		txCheckerRequests: requests,
		multiStore:        multiStore,
		cacher:            cacher,
		db:                db,
		serverOptions:     serverOptions,
	}
}

func (resolver *Resolver) QueryBlock(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryBlock, req *http.Request) jsonrpc.Response {
	return resolver.handleMessage(ctx, id, jsonrpc.MethodQueryBlock, *params, req, false)
}

func (resolver *Resolver) QueryBlocks(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryBlocks, req *http.Request) jsonrpc.Response {
	return resolver.handleMessage(ctx, id, jsonrpc.MethodQueryBlocks, *params, req, false)
}

func (resolver *Resolver) SubmitTx(ctx context.Context, id interface{}, params *jsonrpc.ParamsSubmitTx, req *http.Request) jsonrpc.Response {
	return resolver.handleMessage(ctx, id, jsonrpc.MethodSubmitTx, *params, req, true)
}

func (resolver *Resolver) QueryTx(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryTx, req *http.Request) jsonrpc.Response {
	return resolver.handleMessage(ctx, id, jsonrpc.MethodQueryTx, *params, req, false)
}

func (resolver *Resolver) QueryPeers(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryPeers, req *http.Request) jsonrpc.Response {
	return resolver.handleMessage(ctx, id, jsonrpc.MethodQueryPeers, *params, req, false)
}

func (resolver *Resolver) QueryNumPeers(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryNumPeers, req *http.Request) jsonrpc.Response {
	return resolver.handleMessage(ctx, id, jsonrpc.MethodQueryNumPeers, *params, req, false)
}

func (resolver *Resolver) QueryShards(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryShards, req *http.Request) jsonrpc.Response {
	return resolver.handleMessage(ctx, id, jsonrpc.MethodQueryShards, *params, req, false)
}

func (resolver *Resolver) QueryStat(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryStat, req *http.Request) jsonrpc.Response {
	return resolver.handleMessage(ctx, id, jsonrpc.MethodQueryStat, *params, req, false)
}

func (resolver *Resolver) QueryFees(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryFees, req *http.Request) jsonrpc.Response {
	return resolver.handleMessage(ctx, id, jsonrpc.MethodQueryFees, *params, req, false)
}

func (resolver *Resolver) QueryConfig(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryConfig, req *http.Request) jsonrpc.Response {
	return resolver.handleMessage(ctx, id, jsonrpc.MethodQueryConfig, *params, req, false)
}

func (resolver *Resolver) QueryState(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryState, req *http.Request) jsonrpc.Response {
	return resolver.handleMessage(ctx, id, jsonrpc.MethodQueryState, *params, req, false)
}

func (resolver *Resolver) QueryTxs(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryTxs, req *http.Request) jsonrpc.Response {
	var offset int
	if params.Offset == nil {
		// If the offset is nil, set it to 0.
		offset = 0
	} else {
		offset = int(*params.Offset)
	}

	var limit int
	if params.Limit == nil {
		// If the limit is nil, set it to 8.
		limit = 8
	} else {
		limit = int(*params.Limit)
	}

	// Fetch the matching transactions from the database.
	txs, err := resolver.db.Txs(offset, limit)
	if err != nil {
		jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, fmt.Sprintf("failed to fetch txs: %v", err), nil)
		return jsonrpc.NewResponse(id, nil, &jsonErr)
	}

	return jsonrpc.NewResponse(id, jsonrpc.ResponseQueryTxs{Txs: txs}, nil)
}

func (resolver *Resolver) handleMessage(ctx context.Context, id interface{}, method string, params interface{}, r *http.Request, isSubmitTx bool) jsonrpc.Response {
	query := url.Values{}
	if r != nil {
		query = r.URL.Query()
		darknodeID := query.Get("id")
		if darknodeID != "" {
			if _, err := resolver.multiStore.Get(darknodeID); err != nil {
				jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInvalidParams, fmt.Sprintf("unknown darknode id %s", darknodeID), nil)
				return jsonrpc.NewResponse(id, nil, &jsonErr)
			}
		}
	}

	reqWithResponder := lhttp.NewRequestWithResponder(ctx, id, method, params, query)
	if isSubmitTx {
		resolver.txCheckerRequests <- reqWithResponder
	} else {
		if ok := resolver.cacher.Send(reqWithResponder); !ok {
			resolver.logger.Error("failed to send request to cacher, too much back pressure")
			jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "too much back pressure", nil)
			return jsonrpc.NewResponse(id, nil, &jsonErr)
		}
	}

	select {
	case <-ctx.Done():
		resolver.logger.Error("timeout when waiting for response: %v", ctx.Err())
		jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "request timed out", nil)
		return jsonrpc.NewResponse(id, nil, &jsonErr)
	case res := <-reqWithResponder.Responder:
		return res
	}
}
