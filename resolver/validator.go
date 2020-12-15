package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/darknode/txengine/txenginebindings"
	"github.com/renproject/id"
	v0 "github.com/renproject/lightnode/compat/v0"
)

// The lightnode Validator checks requests and also casts in case of compat changes
type LightnodeValidator struct {
	bindings *txenginebindings.Bindings
	pubkey   *id.PubKey
	store    v0.CompatStore
}

func NewVerifier(bindings *txenginebindings.Bindings, pubkey *id.PubKey, store v0.CompatStore) *LightnodeValidator {
	return &LightnodeValidator{
		bindings: bindings,
		pubkey:   pubkey,
		store:    store,
	}
}

// The validator usually checks if the params are in the correct shape for a given method
// We override the checker for certain methods here to cast invalid v0 params into v1 versions
func (validator *LightnodeValidator) ValidateRequest(ctx context.Context, r *http.Request, req jsonrpc.Request) (interface{}, jsonrpc.Response) {
	switch req.Method {
	case jsonrpc.MethodQueryTx:
		// Check if the params deserializes to v1 queryTx
		// to check if we need to do compat or not
		var v1params jsonrpc.ParamsQueryTx
		// This will throw an error if it's a v1 query because the base64 encoding is different
		if err := json.Unmarshal(req.Params, &v1params); err == nil {
			// if it's v1, continue as normal
			break
		}

		// Check if the params deserialize to v0 queryTx
		// so that we can perform compat
		var params v0.ParamsQueryTx
		if err := json.Unmarshal(req.Params, &params); err == nil {
			castParams := v0.V1QueryTxFromQueryTx(params)

			raw, err := json.Marshal(castParams)
			if err != nil {
				return nil, jsonrpc.NewResponse(req.ID, nil, &jsonrpc.Error{
					Code:    jsonrpc.ErrorCodeInvalidParams,
					Message: fmt.Sprintf("invalid params: %v", err),
				})
			}
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
			castParams, err := v0.V1TxParamsFromTx(ctx, params, validator.bindings, validator.pubkey, validator.store)
			if err != nil {
				return nil, jsonrpc.NewResponse(req.ID, nil, &jsonrpc.Error{
					Code:    jsonrpc.ErrorCodeInvalidParams,
					Message: fmt.Sprintf("invalid params: %v", err),
				})
			}
			raw, err := json.Marshal(castParams)
			req.Params = raw
		}
	}

	// By this point, all params should be valid v1 params
	val := jsonrpc.NewValidator()
	return val.ValidateRequest(ctx, r, req)
}
