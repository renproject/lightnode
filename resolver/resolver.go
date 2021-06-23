package resolver

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/renproject/darknode/binding"
	"github.com/renproject/darknode/engine"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx"
	v0 "github.com/renproject/lightnode/compat/v0"
	v1 "github.com/renproject/lightnode/compat/v1"
	"github.com/renproject/lightnode/db"
	lhttp "github.com/renproject/lightnode/http"
	"github.com/renproject/lightnode/store"
	"github.com/renproject/pack"
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
	compatStore       v0.CompatStore
	bindings          binding.Bindings
}

func New(logger logrus.FieldLogger, cacher phi.Task, multiStore store.MultiAddrStore, db db.DB,
	serverOptions jsonrpc.Options, compatStore v0.CompatStore, bindings binding.Bindings, verifier Verifier) *Resolver {
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
		compatStore:       compatStore,
		bindings:          bindings,
	}
}

func (resolver *Resolver) QueryBlock(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryBlock, req *http.Request) jsonrpc.Response {
	return resolver.handleMessage(ctx, id, jsonrpc.MethodQueryBlock, *params, req, false)
}

func (resolver *Resolver) QueryBlocks(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryBlocks, req *http.Request) jsonrpc.Response {
	return resolver.handleMessage(ctx, id, jsonrpc.MethodQueryBlocks, *params, req, false)
}

func (resolver *Resolver) SubmitTx(ctx context.Context, id interface{}, params *jsonrpc.ParamsSubmitTx, req *http.Request) jsonrpc.Response {
	// As such, we will just respond with the v0 hash so that renjs-v1 can continue as normal, but
	// we won't actually submit to the darknodes
	emptyParams := jsonrpc.ParamsSubmitTx{}
	if params.Tx.Hash == emptyParams.Tx.Hash {
		hash, ok := params.Tx.Input.Get("v0hash").(pack.Bytes32)
		if !ok {
			jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "missing v0hash", nil)
			return jsonrpc.NewResponse(id, nil, &jsonErr)
		}

		return jsonrpc.NewResponse(id, v0.ResponseSubmitTx{Tx: v0.Tx{Hash: v0.B32(hash)}}, nil)
	}
	response := resolver.handleMessage(ctx, id, jsonrpc.MethodSubmitTx, *params, req, true)
	if params.Tx.Version != tx.Version0 {
		return response
	}
	if response.Error != nil {
		return response
	}

	v0tx, err := v0.TxFromV1Tx(params.Tx, false, resolver.bindings)
	if err != nil {
		resolver.logger.Errorf("[responder] cannot convert v1 tx to v0, %v", err)
		jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "fail to convert v1 tx to v0", nil)
		return jsonrpc.NewResponse(id, nil, &jsonErr)
	}

	return jsonrpc.Response{
		Version: response.Version,
		ID:      response.ID,
		Result: struct {
			Tx interface{} `json:"tx"`
		}{v0tx},
	}
}

const (
	MethodQueryTxByTxid = "ren_queryTxByTxid"
)

type ParamsQueryTxByTxid struct {
	Txid pack.Bytes
}

func (resolver *Resolver) Fallback(ctx context.Context, id interface{}, method string, params interface{}, req *http.Request) jsonrpc.Response {
	switch method {
	case MethodQueryTxByTxid:
		var parsedParams ParamsQueryTxByTxid
		err := json.Unmarshal(params.(json.RawMessage), &parsedParams)
		if err != nil {
			return jsonrpc.NewResponse(id, nil, &jsonrpc.Error{
				Code:    jsonrpc.ErrorCodeInvalidParams,
				Message: fmt.Sprintf("invalid params: %v", err),
			})
		}
		return resolver.QueryTxByTxid(ctx, id, &parsedParams, req)
	}
	return jsonrpc.NewResponse(id, nil, nil)
}

// Custom rpc for fetching transactions by txid
func (resolver *Resolver) QueryTxByTxid(ctx context.Context, id interface{}, params *ParamsQueryTxByTxid, req *http.Request) jsonrpc.Response {
	txs, err := resolver.db.TxsByTxid(params.Txid)
	if err != nil {
		resolver.logger.Errorf("[responder] cannot get txs for txid: %v :%v", params.Txid, err)
		jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "failed to query txid", nil)
		return jsonrpc.NewResponse(id, nil, &jsonErr)
	}

	return jsonrpc.NewResponse(id, jsonrpc.ResponseQueryTxs{Txs: txs}, nil)
}

// QueryTx either returns a locally cached result for confirming txs,
// or forwards and caches the request to the darknodes
// It will also detect if a tx is a v1 or v0 tx, and cast the response
// accordingly
func (resolver *Resolver) QueryTx(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryTx, req *http.Request) jsonrpc.Response {
	v0tx := false

	v0txhash := [32]byte{}
	copy(v0txhash[:], params.TxHash[:])

	// check if tx is v0 or v1 due to its presence in the mapping store
	// We have to encode as non-url safe because that's the format v0 uses
	txhash, err := resolver.compatStore.GetV1HashFromHash(v0txhash)
	if err != v0.ErrNotFound {
		if err != nil {
			resolver.logger.Errorf("[responder] cannot get v0-v1 tx mapping from store: %v", err)
			jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "failed to read tx mapping from store", nil)
			return jsonrpc.NewResponse(id, nil, &jsonErr)
		}

		resolver.logger.Debugf("[responder] found v0 tx mapping - v1: %s", txhash)
		params.TxHash = [32]byte(txhash)
		v0tx = true
	}

	// Retrieve transaction status from the database.
	status, err := resolver.db.TxStatus(params.TxHash)
	if err != nil {
		// Send the request to the Darknodes if we do not have it in our
		// database.
		if err != sql.ErrNoRows {
			resolver.logger.Errorf("[responder] cannot get tx status from db: %v", err)
			// some error handling
			jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "failed to read tx from db", nil)
			return jsonrpc.NewResponse(id, nil, &jsonErr)
		}
	}

	// If the transaction has not reached sufficient confirmations (i.e. the
	// Darknodes do not yet know about the transaction), respond with a
	// custom confirming status.
	if status != db.TxStatusConfirmed {
		transaction, err := resolver.db.Tx(params.TxHash)
		if err == nil {
			if v0tx {
				// we need to respond with the v0txhash to keep renjs consistent
				v0tx, err := v0.TxFromV1Tx(transaction, false, resolver.bindings)
				if err != nil {

				}
				return jsonrpc.NewResponse(
					id,
					v0.ResponseQueryTx{
						Tx:       v0tx,
						TxStatus: tx.StatusConfirming.String(),
					},
					nil,
				)
			} else {
				return jsonrpc.NewResponse(
					id,
					jsonrpc.ResponseQueryTx{
						Tx:       transaction,
						TxStatus: tx.StatusConfirming,
					},
					nil,
				)
			}
		}
	}

	query := url.Values{}
	if req != nil {
		query = req.URL.Query()
	}

	reqWithResponder := lhttp.NewRequestWithResponder(ctx, id, jsonrpc.MethodQueryTx, params, query)
	if ok := resolver.cacher.Send(reqWithResponder); !ok {
		resolver.logger.Error("failed to send request to cacher, too much back pressure")
		jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "too much back pressure", nil)
		return jsonrpc.NewResponse(id, nil, &jsonErr)
	}

	select {
	case <-ctx.Done():
		resolver.logger.Error("timeout when waiting for response: %v", ctx.Err())
		jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "request timed out", nil)
		return jsonrpc.NewResponse(id, nil, &jsonErr)

	case res := <-reqWithResponder.Responder:
		if res.Error != nil {
			return jsonrpc.NewResponse(id, nil, res.Error)
		}

		raw, err := json.Marshal(res.Result)
		if err != nil {
			resolver.logger.Errorf("[resolver] error marshaling queryTx result: %v", err)
			return res
		}

		if raw == nil {
			resolver.logger.Warnf("[resolver] empty response for hash %s", params.TxHash)
			return res
		}

		var resp jsonrpc.ResponseQueryTx
		if err := json.Unmarshal(raw, &resp); err != nil {
			resolver.logger.Warnf("[resolver] cannot unmarshal queryState result from %v", err)
			return res
		}

		if resp.Tx.Hash != params.TxHash {
			resolver.logger.Warnf("[resolver] darknode query response (%s) does not match lightnode hash request (%s)", resp.Tx.Hash, params.TxHash)
			return res
		}

		if !resp.Tx.Selector.IsIntrinsic() && resp.Tx.Output.String() == pack.NewTyped().String() {
			// Transaction is still being processed
			resp.TxStatus = tx.StatusExecuting
		}

		if v0tx {
			v0tx, err := v0.TxFromV1Tx(resp.Tx, true, resolver.bindings)
			if err != nil {
				resolver.logger.Errorf("[resolver] error casting tx from v1 to v0: %v", err)
				jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "failed to cast v1 to v0 tx", nil)
				return jsonrpc.NewResponse(id, nil, &jsonErr)
			}

			return jsonrpc.NewResponse(id, v0.ResponseQueryTx{Tx: v0tx, TxStatus: resp.TxStatus.String()}, nil)
		} else {
			return jsonrpc.NewResponse(id, resp, nil)
		}
	}
}

func (resolver *Resolver) QueryPeers(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryPeers, req *http.Request) jsonrpc.Response {
	return resolver.handleMessage(ctx, id, jsonrpc.MethodQueryPeers, *params, req, false)
}

func (resolver *Resolver) QueryNumPeers(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryNumPeers, req *http.Request) jsonrpc.Response {
	return resolver.handleMessage(ctx, id, jsonrpc.MethodQueryNumPeers, *params, req, false)
}

func (resolver *Resolver) QueryShards(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryShards, req *http.Request) jsonrpc.Response {
	// This is required for compatibility with renjs v1

	reqWithResponder := lhttp.NewRequestWithResponder(ctx, id, jsonrpc.MethodQueryBlockState, params, nil)
	if ok := resolver.cacher.Send(reqWithResponder); !ok {
		resolver.logger.Error("failed to send request to cacher, too much back pressure")
		jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "too much back pressure", nil)
		return jsonrpc.NewResponse(id, nil, &jsonErr)
	}

	select {
	case <-ctx.Done():
		resolver.logger.Error("timeout when waiting for response: %v", ctx.Err())
		jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "request timed out", nil)
		return jsonrpc.NewResponse(id, nil, &jsonErr)
	case response := <-reqWithResponder.Responder:
		raw, err := json.Marshal(response.Result)
		if err != nil {
			resolver.logger.Errorf("[resolver] error marshaling queryBlockState result: %v", err)
			jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "failed compatibility conversion", nil)
			return jsonrpc.NewResponse(id, nil, &jsonErr)
		}
		var resp jsonrpc.ResponseQueryBlockState
		if err := json.Unmarshal(raw, &resp); err != nil {
			resolver.logger.Errorf("[resolver] cannot unmarshal queryBlockState result from %v", err)
			jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "failed compatibility conversion", nil)
			return jsonrpc.NewResponse(id, nil, &jsonErr)
		}
		var system engine.SystemState

		if err := pack.Decode(&system, resp.State.Get("System")); err != nil {
			resolver.logger.Errorf("[resolver] cannot decode system state result from %v", err)
			jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "failed compatibility conversion", nil)
			return jsonrpc.NewResponse(id, nil, &jsonErr)
		}

		shards, err := v0.ShardsResponseFromSystemState(system)

		if err != nil {
			resolver.logger.Error("failed to cast to QueryShards: %v", err)
			jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "failed compatibility conversion", nil)
			return jsonrpc.NewResponse(id, nil, &jsonErr)
		}

		return jsonrpc.NewResponse(id, shards, nil)
	}
}

func (resolver *Resolver) QueryStat(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryStat, req *http.Request) jsonrpc.Response {
	return resolver.handleMessage(ctx, id, jsonrpc.MethodQueryStat, *params, req, false)
}

func (resolver *Resolver) QueryFees(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryFees, req *http.Request) jsonrpc.Response {
	// This is required for compatibility with renjs v1

	reqWithResponder := lhttp.NewRequestWithResponder(ctx, id, jsonrpc.MethodQueryBlockState, params, nil)
	if ok := resolver.cacher.Send(reqWithResponder); !ok {
		resolver.logger.Error("failed to send request to cacher, too much back pressure")
		jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "too much back pressure", nil)
		return jsonrpc.NewResponse(id, nil, &jsonErr)
	}

	select {
	case <-ctx.Done():
		resolver.logger.Error("timeout when waiting for response: %v", ctx.Err())
		jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "request timed out", nil)
		return jsonrpc.NewResponse(id, nil, &jsonErr)
	case response := <-reqWithResponder.Responder:
		raw, err := json.Marshal(response.Result)
		if err != nil {
			resolver.logger.Errorf("[resolver] error marshaling queryBlockState result: %v", err)
			jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "failed to marshal darknode queryBlockState for legacy assets", nil)
			return jsonrpc.NewResponse(id, nil, &jsonErr)
		}

		var resp jsonrpc.ResponseQueryBlockState
		if err := json.Unmarshal(raw, &resp); err != nil {
			resolver.logger.Errorf("[resolver] cannot unmarshal queryBlockState result for v2 Assets: %v", err)
			jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "failed unmarshal darknode queryBlockState", nil)
			return jsonrpc.NewResponse(id, nil, &jsonErr)
		}

		legacyAssets := []string{
			"BTC",
			"ZEC",
			"BCH",
		}
		legacyAssetState := map[string]engine.XState{}
		for _, v := range legacyAssets {
			val := resp.State.Get(v)
			if val == nil {
				continue
			}
			var state engine.XState
			if err := pack.Decode(&state, val); err != nil {
				resolver.logger.Errorf("[resolver] cannot decode pack value for %v: %v", v, err)
				jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "failed to decode block state", nil)
				return jsonrpc.NewResponse(id, nil, &jsonErr)
			}

			legacyAssetState[v] = state
		}

		fees, err := v0.QueryFeesResponseFromState(legacyAssetState)

		if err != nil {
			resolver.logger.Error("failed to cast to QueryFees: %v", err)
			jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "failed compatibility conversion", nil)
			return jsonrpc.NewResponse(id, nil, &jsonErr)
		}

		return jsonrpc.NewResponse(id, fees, nil)
	}
}

func (resolver *Resolver) QueryConfig(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryConfig, req *http.Request) jsonrpc.Response {
	return resolver.handleMessage(ctx, id, jsonrpc.MethodQueryConfig, *params, req, false)
}

func (resolver *Resolver) QueryState(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryState, req *http.Request) jsonrpc.Response {
	// This is required for compatibility with renjs v1

	reqWithResponder := lhttp.NewRequestWithResponder(ctx, id, jsonrpc.MethodQueryBlockState, params, nil)
	if ok := resolver.cacher.Send(reqWithResponder); !ok {
		resolver.logger.Error("failed to send request to cacher, too much back pressure")
		jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "too much back pressure", nil)
		return jsonrpc.NewResponse(id, nil, &jsonErr)
	}

	select {
	case <-ctx.Done():
		resolver.logger.Error("timeout when waiting for response: %v", ctx.Err())
		jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "request timed out", nil)
		return jsonrpc.NewResponse(id, nil, &jsonErr)
	case response := <-reqWithResponder.Responder:
		raw, err := json.Marshal(response.Result)
		if err != nil {
			resolver.logger.Errorf("[resolver] error marshaling queryBlockState result: %v", err)
			jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "failed marshal darknode queryBlockState", nil)
			return jsonrpc.NewResponse(id, nil, &jsonErr)
		}

		var resp jsonrpc.ResponseQueryBlockState
		if err := json.Unmarshal(raw, &resp); err != nil {
			resolver.logger.Errorf("[resolver] cannot unmarshal queryBlockState result for v2 Assets: %v", err)
			jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "failed unmarshal darknode queryBlockState", nil)
			return jsonrpc.NewResponse(id, nil, &jsonErr)
		}

		v2Assets := []string{
			"BTC",
			"ZEC",
			"BCH",
			"DGB",
			"DOGE",
			"LUNA",
			"FIL",
		}
		v2AssetState := map[string]engine.XState{}
		for _, v := range v2Assets {
			val := resp.State.Get(v)
			if val == nil {
				continue
			}
			var state engine.XState
			if err := pack.Decode(&state, val); err != nil {
				resolver.logger.Errorf("[resolver] cannot decode pack value for %v: %v", v, err)
				jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "failed to decode block state", nil)
				return jsonrpc.NewResponse(id, nil, &jsonErr)
			}

			v2AssetState[v] = state
		}

		shards, err := v1.QueryStateResponseFromState(resolver.bindings, v2AssetState)

		if err != nil {
			resolver.logger.Error("failed to cast to QueryFees: %v", err)
			jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "failed compatibility conversion", nil)
			return jsonrpc.NewResponse(id, nil, &jsonErr)
		}

		return jsonrpc.NewResponse(id, shards, nil)
	}
}

func (resolver *Resolver) QueryBlockState(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryBlockState, req *http.Request) jsonrpc.Response {
	return resolver.handleMessage(ctx, id, jsonrpc.MethodQueryBlockState, *params, req, false)
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

func (resolver *Resolver) handleMessage(ctx context.Context, id interface{}, method string, params interface{}, r *http.Request, isCompat bool) jsonrpc.Response {
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
	if method == jsonrpc.MethodSubmitTx && params.(jsonrpc.ParamsSubmitTx).Tx.Selector.IsCrossChain() {
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
