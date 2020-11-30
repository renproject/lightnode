package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-redis/redis/v7"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/darknode/txengine/txenginebindings"
	"github.com/renproject/id"
	v0 "github.com/renproject/lightnode/compat/v0"
	"github.com/renproject/lightnode/db"
)

// The lightnode Validator checks requests and also casts in case of compat changes
type LightnodeValidator struct {
	bindings *txenginebindings.Bindings
	pubkey   *id.PubKey
	store    redis.Cmdable
	db       db.DB
}

func NewVerifier(bindings *txenginebindings.Bindings, pubkey *id.PubKey, store redis.Cmdable, database db.DB) *LightnodeValidator {
	return &LightnodeValidator{
		bindings: bindings,
		pubkey:   pubkey,
		store:    store,
		db:       database,
	}
}

func (validator *LightnodeValidator) ValidateRequest(ctx context.Context, r *http.Request, req jsonrpc.Request) (interface{}, jsonrpc.Response) {
	switch req.Method {
	case jsonrpc.MethodQueryTx:
		// Check if the params deserializes to v1 queryTx
		// to check if we need to do compat or not
		var v1params jsonrpc.ParamsQueryTx
		if err := json.Unmarshal(req.Params, &v1params); err == nil {
			// if it's v1, continue as normal
			break
		}

		// Check if the params deserialize to v0 queryTx
		// so that we can perform compat
		var params v0.ParamsQueryTx
		if err := json.Unmarshal(req.Params, &params); err == nil {
			castParams, err := v0.V1QueryTxFromQueryTx(ctx, params, validator.store)
			if err != nil {
				return nil, jsonrpc.NewResponse(req.ID, nil, &jsonrpc.Error{
					Code:    jsonrpc.ErrorCodeInvalidParams,
					Message: fmt.Sprintf("invalid params: %v", err),
				})
			}
			raw, err := json.Marshal(castParams)
			req.Params = raw
		}

	case jsonrpc.MethodSubmitTx:
		// Both v0 and v1 txs successfully deseralize from json
		// so lets try deserializing into v1 and use other checks to
		// determine if it is indeed a v0 tx
		var v1params jsonrpc.ParamsSubmitTx
		if err := json.Unmarshal(req.Params, &v1params); err == nil {
			if v1params.Tx.Version == tx.Version1 {
				// Tx is actually v1
				break
			}
		}
		var params v0.ParamsSubmitTx
		if err := json.Unmarshal(req.Params, &params); err == nil {
			// Check if we have seen this tx before, and skip casting if so
			v1Hash, err := validator.store.Get(params.Tx.Hash.String()).Bytes()
			v1Hash32 := [32]byte{}
			copy(v1Hash32[:], v1Hash)
			castParams := jsonrpc.ParamsSubmitTx{}
			if err == nil {
				restoredtx, err := validator.db.Tx(v1Hash32)
				// If there was an error restoring, we will do the usual casting
				// because the transaction may be valid and persistence just failed for some reason.
				// If we have a result we can skip the casting by setting the transaction
				if err == nil {
					castParams.Tx = restoredtx
				}
			}

			// If we didn't restore a previous transaction, continue with the cast
			if castParams.Tx.Hash != v1Hash32 {
				castParams, err = v0.V1TxParamsFromTx(ctx, params, validator.bindings, validator.pubkey, validator.store)
				if err != nil {
					return nil, jsonrpc.NewResponse(req.ID, nil, &jsonrpc.Error{
						Code:    jsonrpc.ErrorCodeInvalidParams,
						Message: fmt.Sprintf("invalid params: %v", err),
					})
				}
			}
			raw, err := json.Marshal(castParams)
			req.Params = raw
		}
	}

	// We use the Darknode's validator for most methods
	val := jsonrpc.NewValidator()
	return val.ValidateRequest(ctx, r, req)
}
