package jsonrpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/republicprotocol/darknode-go/server/jsonrpc"
)

// Client is able to send JSON-RPC 2.0 request through http.
type Client struct {
	http *http.Client
}

// NewClient returns a new Client with given timeout.
func NewClient(timeout time.Duration) Client {
	httpClient := new(http.Client)
	httpClient.Timeout = timeout

	return Client{
		http: httpClient,
	}
}

// Call sends the given JSON-RPC request to the given url address.
func (client Client) Call(url string, request jsonrpc.JSONRequest) (jsonrpc.JSONResponse, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return jsonrpc.JSONResponse{}, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return jsonrpc.JSONResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	response, err := client.http.Do(req)
	if err != nil {
		return jsonrpc.JSONResponse{}, err
	}
	if response.StatusCode != http.StatusOK {
		return jsonrpc.JSONResponse{}, fmt.Errorf("unexpected status code %v", response.StatusCode)
	}

	var resp jsonrpc.JSONResponse
	err = json.NewDecoder(response.Body).Decode(&resp)
	return resp, err
}
