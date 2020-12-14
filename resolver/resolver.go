package resolver

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/go-redis/redis/v7"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/darknode/txengine"
	"github.com/renproject/darknode/txpool"
	v0 "github.com/renproject/lightnode/compat/v0"
	"github.com/renproject/lightnode/db"
	lhttp "github.com/renproject/lightnode/http"
	"github.com/renproject/lightnode/store"
	"github.com/renproject/multichain"
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
	store             redis.Cmdable
	bindings          txengine.Bindings
}

func New(logger logrus.FieldLogger, cacher phi.Task, multiStore store.MultiAddrStore, verifier txpool.Verifier, db db.DB, serverOptions jsonrpc.Options, store redis.Cmdable, bindings txengine.Bindings) *Resolver {
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
		store:             store,
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
	return resolver.handleMessage(ctx, id, jsonrpc.MethodSubmitTx, *params, req, true)
}

func (resolver *Resolver) QueryTx(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryTx, req *http.Request) jsonrpc.Response {
	v0tx := false
	// check if tx is v0 or v1 due to its presence in the mapping store
	// We have to encode as non-url safe because that's the format v0 uses
	queryHash := base64.StdEncoding.EncodeToString(params.TxHash[:])
	resolver.logger.Printf("checking compat hash %s", queryHash)
	hash, err := resolver.store.Get(queryHash).Result()
	hashBytes, err := base64.RawURLEncoding.DecodeString(hash)
	resolver.logger.Printf("found hash %s err %v", hash, err)
	txhash := [32]byte{}
	copy(txhash[:], hashBytes)
	v0txhash := [32]byte{}
	copy(v0txhash[:], params.TxHash[:])
	if err == nil && hashBytes != nil && txhash != [32]byte{} {
		resolver.logger.Printf("v0 tx %v", txhash)
		params.TxHash = txhash
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
				v0tx, err := v0.V0TxFromV1(transaction, v0txhash, false, resolver.bindings)
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

	resolver.logger.Printf("queryparams %s", params)
	reqWithResponder := lhttp.NewRequestWithResponder(ctx, id, jsonrpc.MethodQueryTx, params, req.URL.Query())
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
		if v0tx {
			raw, err := json.Marshal(res.Result)
			if err != nil {
				resolver.logger.Errorf("[resolver] error marshaling queryState result: %v", err)
			}
			resolver.logger.Printf("resp %s", raw)
			if raw == nil {
				return res
			}

			var resp jsonrpc.ResponseQueryTx
			if err := json.Unmarshal(raw, &resp); err != nil {
				resolver.logger.Warnf("[resolver] cannot unmarshal queryState result from %v", err)
			}

			if resp.Tx.Hash != params.TxHash {
				resolver.logger.Warnf("[resolver] darknode query does not match lightnode hash %s", txhash)
				return res
			}
			v0tx, err := v0.V0TxFromV1(resp.Tx, v0txhash, true, resolver.bindings)
			return jsonrpc.NewResponse(id, v0.ResponseQueryTx{
				Tx: v0tx, TxStatus: resp.TxStatus.String()}, nil)
		} else {
			return res
		}
	}
	// return resolver.handleMessage(ctx, id, jsonrpc.MethodQueryTx, *params, req, false)
}

func (resolver *Resolver) QueryPeers(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryPeers, req *http.Request) jsonrpc.Response {
	return resolver.handleMessage(ctx, id, jsonrpc.MethodQueryPeers, *params, req, false)
}

func (resolver *Resolver) QueryNumPeers(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryNumPeers, req *http.Request) jsonrpc.Response {
	return resolver.handleMessage(ctx, id, jsonrpc.MethodQueryNumPeers, *params, req, false)
}

func (resolver *Resolver) QueryShards(ctx context.Context, id interface{}, params *jsonrpc.ParamsQueryShards, req *http.Request) jsonrpc.Response {
	// This is required for compatibility with renjs v1

	reqWithResponder := lhttp.NewRequestWithResponder(ctx, id, jsonrpc.MethodQueryState, params, nil)
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
			resolver.logger.Errorf("[resolver] error marshaling queryState result: %v", err)
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(raw, &resp); err != nil {
			resolver.logger.Warnf("[resolver] cannot unmarshal queryState result from %v", err)
		}

		state := jsonrpc.ResponseQueryState{
			State: map[multichain.Chain]pack.Struct{
				multichain.Bitcoin: pack.NewStruct(
					"pubKey",
					pack.String(resp["state"].(map[string]interface{})["Bitcoin"].(map[string]interface{})["pubKey"].(string))),
			},
		}
		shards, err := v0.ShardsResponseFromState(state)

		if err != nil {
			resolver.logger.Error("failed to cast to QueryShards")
			jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "failed compatability conversion", nil)
			return jsonrpc.NewResponse(id, nil, &jsonErr)
		}

		return jsonrpc.NewResponse(id, shards, nil)
	}
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
	return resolver.handleMessage(ctx, id, jsonrpc.MethodQueryTxs, *params, req, false)
	// var offset int
	// if params.Offset == nil {
	// 	// If the offset is nil, set it to 0.
	// 	offset = 0
	// } else {
	// 	offset = int(*params.Offset)
	// }

	// var limit int
	// if params.Limit == nil {
	// 	// If the limit is nil, set it to 8.
	// 	limit = 8
	// } else {
	// 	limit = int(*params.Limit)
	// }

	// // Fetch the matching transactions from the database.
	// txs, err := resolver.db.Txs(offset, limit)
	// if err != nil {
	// 	jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, fmt.Sprintf("failed to fetch txs: %v", err), nil)
	// 	return jsonrpc.NewResponse(id, nil, &jsonErr)
	// }

	// return jsonrpc.NewResponse(id, jsonrpc.ResponseQueryTxs{Txs: txs}, nil)
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
	if method == jsonrpc.MethodSubmitTx {
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
