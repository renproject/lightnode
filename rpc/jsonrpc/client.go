// Package jsonrpc handles sending `JSONRequest` objects to a given URL. In our case, the URL provided should be the
// address of a Darknode as the response is expected to be of type `JSONResponse`.
package jsonrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/republicprotocol/darknode-go/rpc/jsonrpc"
	"github.com/sirupsen/logrus"
)

// Client is able to send JSON-RPC 2.0 request through http.
type Client struct {
	http   *http.Client
	logger logrus.FieldLogger
}

// NewClient returns a new Client with given timeout.
func NewClient(logger logrus.FieldLogger, timeout time.Duration) Client {
	httpClient := new(http.Client)
	httpClient.Timeout = timeout

	return Client{
		http:   httpClient,
		logger: logger,
	}
}

// Call sends the given JSON-RPC request to the given URL.
func (client Client) Call(url string, request jsonrpc.JSONRequest) (jsonrpc.JSONResponse, error) {
	// Construct HTTP request.
	body, err := json.Marshal(request)
	if err != nil {
		return jsonrpc.JSONResponse{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), client.http.Timeout)
	defer cancel()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return jsonrpc.JSONResponse{}, err
	}
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	// Read response.
	var response *http.Response
	if err := client.backoff(ctx, func() error {
		response, err = client.http.Do(req)
		if err != nil {
			return err
		}
		if response.StatusCode != http.StatusOK {
			errmsg, err := ioutil.ReadAll(response.Body)
			if err != nil {
				return err
			}

			log.Print(string(errmsg))
			return fmt.Errorf("unexpected status code %v", response.StatusCode)
		}
		return nil
	}); err != nil {
		return jsonrpc.JSONResponse{}, err
	}

	var resp jsonrpc.JSONResponse
	err = json.NewDecoder(response.Body).Decode(&resp)
	return resp, err
}

func (client Client) backoff(ctx context.Context, f func() error) error {
	duration := 5 * time.Second
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			err := f()
			if err == nil {
				return nil
			}
			client.logger.Infof("%v, will try again in %f sec\n", err, duration.Seconds())
			time.Sleep(duration)
		}
	}
}
