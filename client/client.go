package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/renproject/darknode/addr"
	"github.com/renproject/darknode/jsonrpc"
)

type Client struct {
	timeout time.Duration
}

func New(timeout time.Duration) Client {
	return Client{timeout}
}

// NOTE: If err is not nil, it is expected that the caller will construct an
// appropriate error response message.
func (client *Client) SendToDarknode(addr addr.MultiAddress, req jsonrpc.Request) (jsonrpc.Response, error) {
	httpClient := new(http.Client)
	httpClient.Timeout = client.timeout

	url := fmt.Sprintf("http://%s:%v", addr.IP4(), addr.Port()+1)

	// Construct HTTP request.
	body, err := json.Marshal(req)
	if err != nil {
		panic(fmt.Sprintf("[client] could not marshal request: %v", err))
	}
	ctx, cancel := context.WithTimeout(context.Background(), httpClient.Timeout)
	defer cancel()
	r, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		panic(fmt.Sprintf("[client] could not create http request: %v", err))
	}
	r = r.WithContext(ctx)
	r.Header.Set("Content-Type", "application/json")

	// Read response.
	response, err := httpClient.Do(r)
	if err != nil {
		return jsonrpc.Response{}, err
	}

	var resp jsonrpc.Response
	err = json.NewDecoder(response.Body).Decode(&resp)
	return resp, err
}
