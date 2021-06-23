package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/renproject/darknode/binding"
	"github.com/renproject/darknode/engine"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/darknode/tx"
	"github.com/renproject/id"
	v0 "github.com/renproject/lightnode/compat/v0"
	v1 "github.com/renproject/lightnode/compat/v1"
	"github.com/renproject/pack"
	"github.com/sirupsen/logrus"
)

// The lightnode Validator checks requests and also casts in case of compat changes
type LightnodeValidator struct {
	bindings     binding.Bindings
	pubkey       *id.PubKey
	versionStore v0.CompatStore
	gpubkeyStore v1.GpubkeyCompatStore
	limiter      *LightnodeRateLimiter
	logger       logrus.FieldLogger
}

func NewValidator(bindings binding.Bindings, pubkey *id.PubKey, versionStore v0.CompatStore, gpubkeyStore v1.GpubkeyCompatStore, limiter *LightnodeRateLimiter, logger logrus.FieldLogger) *LightnodeValidator {
	return &LightnodeValidator{
		bindings:     bindings,
		pubkey:       pubkey,
		versionStore: versionStore,
		gpubkeyStore: gpubkeyStore,
		limiter:      limiter,
		logger:       logger,
	}
}

// The validator usually checks if the params are in the correct shape for a given method
// We override the checker for certain methods here to cast invalid v0 params into v1 versions
func (validator *LightnodeValidator) ValidateRequest(ctx context.Context, r *http.Request, req jsonrpc.Request) (interface{}, jsonrpc.Response) {
	// We rate limit in the validator, as it is the earliest entry point we can hook into
	// for range
	ipString := r.Header.Get("x-forwarded-for")
	if ipString == "" {
		ipString = r.RemoteAddr
	} else if ipStrings := strings.Split(ipString, ","); len(ipStrings) > 0 {
		ipString = ipStrings[len(ipStrings)-1]
		// if there is a trailling comma, or the x-forwarded-for header is malformed,
		// skip parsing
		if ipString == "" {
			return nil, jsonrpc.NewResponse(req.ID, nil, &jsonrpc.Error{
				Code:    jsonrpc.ErrorCodeInvalidRequest,
				Message: fmt.Sprintf("could not determine ip for %v", strings.Join(ipStrings, ",")),
			})
		}
	}
	ip := net.ParseIP(ipString)
	// If we fail to parse a "plain" ip, we check if it is in host:port format
	// This can't be done in an easy split manner due to ipv6.
	// We also skip requiring an ip if we haven't picked up a string yet to
	// allow for testing, as we should always have a value from r.RemoteAddr
	// in an actual server
	if ip == nil && ipString != "" {
		ip2, _, err := net.SplitHostPort(ipString)
		ip = net.ParseIP(ip2)
		if err != nil {
			return nil, jsonrpc.NewResponse(req.ID, nil, &jsonrpc.Error{
				Code:    jsonrpc.ErrorCodeInvalidRequest,
				Message: fmt.Sprintf("could not determine ip for %v", ipString),
			})
		}
	}

	if !(validator.limiter.Allow(req.Method, net.IP(ip))) {
		validator.logger.Warn("Rate limit exceeded for ip:", ipString)
		return nil, jsonrpc.NewResponse(req.ID, nil, &jsonrpc.Error{
			Code:    jsonrpc.ErrorCodeInvalidRequest,
			Message: fmt.Sprintf("rate limit exceeded for %v", ipString),
		})
	}

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
		// It might still be a v1 query, if there are no clashing base64url characters
		// But, it doesn't matter as we check in the resolver, where we take the query as v1 by default
		// any only override it if we find a mapping.

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
				// If the transaction is a burn, and contains a gpubkey,
				// construct an updated transaction input excluding the
				// field.
				if v1params.Tx.Selector.IsLock() {
					break
				}

				var input engine.LockMintBurnReleaseInput
				if err := pack.Decode(&input, v1params.Tx.Input); err != nil {
					return nil, jsonrpc.NewResponse(req.ID, nil, &jsonrpc.Error{
						Code:    jsonrpc.ErrorCodeInvalidParams,
						Message: fmt.Sprintf("invalid params: %v", err),
					})
				}
				if len(input.Gpubkey) == 0 {
					break
				}
				v1params.Tx, err = validator.gpubkeyStore.RemoveGpubkey(v1params.Tx)
				if err != nil {
					validator.logger.Warn("[validator] building tx: %v", err)
					return nil, jsonrpc.NewResponse(req.ID, nil, &jsonrpc.Error{
						Code:    jsonrpc.ErrorCodeInvalidParams,
						Message: fmt.Sprintf("invalid params: %v", err),
					})
				}

				raw, err := json.Marshal(v1params)
				if err != nil {
					return nil, jsonrpc.NewResponse(req.ID, nil, &jsonrpc.Error{
						Code:    jsonrpc.ErrorCodeInvalidParams,
						Message: fmt.Sprintf("invalid params: %v", err),
					})
				}
				req.Params = raw
			}
		}

		var params v0.ParamsSubmitTx
		if err := json.Unmarshal(req.Params, &params); err == nil {
			castParams, err := v0.V1TxParamsFromTx(ctx, params, validator.bindings, validator.pubkey, validator.versionStore)
			if err != nil {
				validator.logger.Errorf("[validator] upgrading tx params: %v", err)
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
