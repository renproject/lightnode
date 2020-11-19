package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/renproject/darknode/jsonrpc"
	v0 "github.com/renproject/lightnode/compat/v0"
)

// The lightnode Validator checks requests and also casts in case of compat changes
type LightnodeValidator struct{}

func (validator *LightnodeValidator) ValidateRequest(ctx context.Context, r *http.Request, req jsonrpc.Request) (interface{}, jsonrpc.Response) {
	switch req.Method {
	case jsonrpc.MethodSubmitTx:
		// Check if the params deserialize to v0 tx
		var params v0.ParamsSubmitTx
		if err := json.Unmarshal(req.Params, &params); err == nil {
			castParams, err := v0.V1TxParamsFromTx(params)
			if err != nil {
				return nil, jsonrpc.NewResponse(req.ID, nil, &jsonrpc.Error{
					Code:    jsonrpc.ErrorCodeInvalidParams,
					Message: fmt.Sprintf("invalid params: %v", err),
				})
			}
			raw, err := json.Marshal(castParams)
			req.Params = raw
		}
		// If there isn't a v0 ParamsSubmitTx; handle as default
		fallthrough
	default:
		// We use the Darknode's validator for most methods
		val := jsonrpc.DarknodeValidator{}
		i, erres := val.ValidateRequest(ctx, r, req)
		if erres.Error != nil {
			fmt.Printf("i %v erresdata %v message %v", i, erres.Error.Message, erres.Error.Data)
		}
		return i, erres
	}

}

//return params, jsonrpc.Response{}
