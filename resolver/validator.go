package resolver

import (
	"context"
	"encoding/base64"
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
			// lookup by utxo because we don't have a renvm hash in the submission
			utxo := params.Tx.In.Get("utxo").Value.(v0.ExtBtcCompatUTXO)
			txid := utxo.TxHash.String()

			// Check if we have seen this tx before, and skip casting if so
			v1HashS, err := validator.store.Get(txid).Result()
			v1Hash, err := base64.RawURLEncoding.DecodeString(v1HashS)

			v1Hash32 := [32]byte{}
			copy(v1Hash32[:], v1Hash)
			castParams := jsonrpc.ParamsSubmitTx{}
			if err == nil {
				restoredtx, err := validator.db.Tx(v1Hash32)
				fmt.Printf("\n\n\nrestoredtx %v\n\n\n", restoredtx)
				// If there was an error restoring, we will do the usual casting
				// because the transaction may be valid and persistence just failed for some reason.
				// If we have a result we can skip the casting by setting the transaction
				if err == nil {
					castParams.Tx = restoredtx
				}
			} else {
				fmt.Printf("\n\n\nerr %v\n\n\n", err)
				fmt.Printf("\n\n\nv1HashS %v\n\n\n", v1HashS)
			}

			// If we didn't restore a previous transaction, continue with the cast
			if castParams.Tx.Hash != v1Hash32 || v1Hash32 == [32]byte{} {
				fmt.Printf("\n\n\ncasting tx %v\n\n\n", castParams.Tx.Hash)
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
