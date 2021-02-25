package resolver

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

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
	compatStore       v0.CompatStore
	bindings          txengine.Bindings
}

func New(logger logrus.FieldLogger, cacher phi.Task, multiStore store.MultiAddrStore, verifier txpool.Verifier, db db.DB,
	serverOptions jsonrpc.Options, compatStore v0.CompatStore, bindings txengine.Bindings) *Resolver {
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
	// When a v0 burn tx gets submitted via RPC, we have to wait for the watcher to detect it before submitting
	// because it does not have sufficient data to create a valid v1 tx hash
	// (it just contains a ref to the burn event height + the v0 selector,
	// and the contract doesn't have a way to query by event height, and can't really filter either)
	//
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
	if params.Tx.Version == tx.Version1 {
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
		if v0tx {
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

			v0tx, err := v0.TxFromV1Tx(resp.Tx, true, resolver.bindings)
			if err != nil {
				resolver.logger.Errorf("[resolver] error casting tx from v1 to v0: %v", err)
				jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "failed to cast v1 to v0 tx", nil)
				return jsonrpc.NewResponse(id, nil, &jsonErr)
			}

			return jsonrpc.NewResponse(id, v0.ResponseQueryTx{Tx: v0tx, TxStatus: resp.TxStatus.String()}, nil)
		} else {
			return res
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
			jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "failed marshal darknode queryState", nil)
			return jsonrpc.NewResponse(id, nil, &jsonErr)
		}

		var resp map[string]map[multichain.Chain]map[string]interface{}
		if err := json.Unmarshal(raw, &resp); err != nil {
			resolver.logger.Errorf("[resolver] cannot unmarshal queryState result: %v", err)
			jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "failed unmarshal darknode queryState", nil)
			return jsonrpc.NewResponse(id, nil, &jsonErr)
		}

		state := jsonrpc.ResponseQueryState{}
		state.State = make(map[multichain.Chain]pack.Struct)

		for i := range resp["state"] {
			gasCap, err := strconv.Atoi(resp["state"][i]["gasCap"].(string))
			if err != nil {
				resolver.logger.Error("[resolver] missing gasCap for %v", i)
				jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "failed compatibility conversion", nil)
				return jsonrpc.NewResponse(id, nil, &jsonErr)
			}

			gasLimit, err := strconv.Atoi(resp["state"][i]["gasLimit"].(string))
			if err != nil {
				resolver.logger.Error("[resolver] missing gasLimit for %v", i)
				jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "failed compatibility conversion", nil)
				return jsonrpc.NewResponse(id, nil, &jsonErr)
			}

			state.State[multichain.Chain(i)] = pack.NewStruct(
				"gasCap", pack.NewU64(uint64(gasCap)),
				"gasLimit", pack.NewU64(uint64(gasLimit)),
			)
		}

		shards, err := v0.QueryFeesResponseFromState(state)

		if err != nil {
			resolver.logger.Error("failed to cast to QueryFees")
			jsonErr := jsonrpc.NewError(jsonrpc.ErrorCodeInternal, "failed compatibility conversion", nil)
			return jsonrpc.NewResponse(id, nil, &jsonErr)
		}

		return jsonrpc.NewResponse(id, shards, nil)
	}
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
