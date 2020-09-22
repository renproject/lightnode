package resolver

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"net/http"
	"net/url"

	"github.com/renproject/darknode/abi"
	"github.com/renproject/darknode/consensus/txcheck/transform"
	"github.com/renproject/darknode/jsonrpc"
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

func New(logger logrus.FieldLogger, cacher phi.Task, multiStore store.MultiAddrStore, key ecdsa.PublicKey, bc transform.Blockchain, db db.DB, serverOptions jsonrpc.Options) *Resolver {
	requests := make(chan lhttp.RequestWithResponder, 128)
	txChecker := newTxChecker(logger, requests, key, bc, db)
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
	if params.Tags != nil && len(*params.Tags) > resolver.serverOptions.MaxTags {
		jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInvalidParams, fmt.Sprintf("maximum number of tags is %d", resolver.serverOptions.MaxTags), nil)
		return jsonrpc.NewResponse(id, nil, &jsonErr)
	}
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

func (resolver *Resolver) QueryTxs(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryTxs, req *http.Request) jsonrpc.Response {
	var tag abi.B32
	if params.Tags != nil && len(*params.Tags) > 0 {
		if len(*params.Tags) > resolver.serverOptions.MaxTags {
			jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInvalidParams, fmt.Sprintf("maximum number of tags supported is %d", resolver.serverOptions.MaxTags), nil)
			return jsonrpc.NewResponse(id, nil, &jsonErr)
		}

		// Currently we only support a maximum of one tag, but this can be
		// extended in the future.
		tag = (*params.Tags)[0]
	}

	var page uint64
	if params.Page != nil {
		page = params.Page.Int.Uint64()
	}

	var pageSize uint64
	if params.PageSize == nil || params.PageSize.Int.Uint64() > uint64(resolver.serverOptions.MaxPageSize) {
		pageSize = uint64(resolver.serverOptions.MaxPageSize)
	} else {
		pageSize = params.PageSize.Int.Uint64()
	}

	// Fetch the matching transactions from the database.
	txs, err := resolver.db.Txs(tag, page, pageSize)
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
			if _, err := resolver.multiStore.Address(darknodeID); err != nil {
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
