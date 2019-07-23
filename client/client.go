package client

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/republicprotocol/darknode-go/addr"
	"github.com/republicprotocol/darknode-go/jsonrpc"
)

type Client struct {
	timeout time.Duration
}

func New(timeout time.Duration) Client {
	return Client{timeout}
}

func (client *Client) SendToDarknode(addr addr.MultiAddress, req jsonrpc.Request) (jsonrpc.Response, error) {
	httpClient := new(http.Client)
	httpClient.Timeout = client.timeout

	// FIXME: This will give the wrong port, we need to instead use the jsonrpc
	// port.
	netAddr := addr.NetworkAddress()
	url := "http://" + netAddr.String()

	// Construct HTTP request.
	body, err := json.Marshal(req)
	if err != nil {
		return jsonrpc.Response{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), httpClient.Timeout)
	defer cancel()
	r, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return jsonrpc.Response{}, err
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
