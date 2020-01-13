package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/renproject/darknode/addr"
	"github.com/renproject/darknode/jsonrpc"
)

// SendToDarknode sends a given request to the darknode at the given url. The
// timeout is the timeout for the request.
//
// NOTE: If err is not nil, it is expected that the caller will construct an
// appropriate error response message.
func SendToDarknode(url string, req jsonrpc.Request, timeout time.Duration) (jsonrpc.Response, error) {
	httpClient := new(http.Client)
	httpClient.Timeout = timeout

	// Construct HTTP request.
	body, err := json.Marshal(req)
	if err != nil {
		return jsonrpc.Response{}, fmt.Errorf("[client] could not marshal request: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), httpClient.Timeout)
	defer cancel()
	r, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return jsonrpc.Response{}, fmt.Errorf("[client] could not create http request: %v", err)
	}
	r = r.WithContext(ctx)
	r.Header.Set("Content-Type", "application/json")

	endTime := time.Now().Add(timeout)
	for {
		// Retry until timeout.
		if time.Now().After(endTime) {
			return jsonrpc.Response{}, fmt.Errorf("timeout: %v", err)
		}

		resp, err := func() (jsonrpc.Response, error) {
			// Send request.
			response, err := httpClient.Do(r)
			if err != nil {
				return jsonrpc.Response{}, err
			}
			defer response.Body.Close()

			// Read response.
			var resp jsonrpc.Response
			if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
				return jsonrpc.Response{}, err
			}
			return resp, nil
		}()
		if err != nil {
			log.Printf("err = %v", err)
			continue
		}
		return resp, nil
	}
}

// URLFromMulti converts a `addr.MultiAddress` to a url string appropriate for
// sending a JSON-RPC request to a darknode. The port is defined in the multi
// address is incremented by one because of darknode specific logic about what
// the JSON-RPC port is.
func URLFromMulti(addr addr.MultiAddress) string {
	return fmt.Sprintf("http://%s:%v", addr.IP4(), addr.Port()+1)
}
