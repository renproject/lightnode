package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/renproject/darknode/jsonrpc"
)

// RetryOptions can be passed to Client when trying to send a request, so that
// Client will retry sending the request if it fails.
type RetryOptions struct {
	Base   time.Duration
	Max    time.Duration
	Factor float64
}

// DefaultRetryOptions is the recommended retry setting for Lightnode.
var DefaultRetryOptions = RetryOptions{
	Base:   time.Second,
	Max:    5 * time.Second,
	Factor: 0.2,
}

// DefaultClientTimeout is the recommended time for the Client.
var DefaultClientTimeout = 5 * time.Second

// Client is an http.Client with fixed timeout.
type Client struct {
	*http.Client
}

// NewClient returns a new client with given timeout.
func NewClient(timeout time.Duration) Client {
	return Client{
		Client: &http.Client{
			Timeout: timeout,
		},
	}
}

// SendRequest sends the jsonrpc.Request to the provided url. It only retries
// sending the request if the RetryOptions is not nil. Otherwise it returns the
// error immediately if failing to get the response.
func (c Client) SendRequest(ctx context.Context, url string, request jsonrpc.Request, options *RetryOptions) (jsonrpc.Response, error) {
	// Construct HTTP request.
	body, err := json.Marshal(request)
	if err != nil {
		return jsonrpc.Response{}, fmt.Errorf("[client] could not marshal request: %v", err)
	}
	r, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return jsonrpc.Response{}, fmt.Errorf("[client] could not create http request: %v", err)
	}
	r = r.WithContext(ctx)
	r.Header.Set("Content-Type", "application/json")

	// Check if the retry option has been passed.
	if options == nil {
		return c.send(r)
	}
	return c.retry(ctx, r, options)
}

// send the request without retrying.
func (c Client) send(r *http.Request) (jsonrpc.Response, error) {
	response, err := c.Do(r)
	if err != nil {
		return jsonrpc.Response{}, err
	}
	var resp jsonrpc.Response
	err = json.NewDecoder(response.Body).Decode(&resp)
	return resp, err
}

// send the request with the passed RetryOptions.
func (c Client) retry(ctx context.Context, r *http.Request, options *RetryOptions) (jsonrpc.Response, error) {
	interval := options.Base
	for {
		response, err := c.send(r)
		if err == nil {
			return response, err
		}
		select {
		case <-ctx.Done():
			return jsonrpc.Response{}, ctx.Err()
		case <-time.After(interval):
			interval = time.Duration(float64(interval) * (1 + options.Factor))
			if interval > options.Max {
				interval = options.Max
			}
		}
	}
}
