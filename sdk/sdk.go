package sdk

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	jrpc "github.com/renproject/lightnode/rpc/jsonrpc"
	"github.com/republicprotocol/darknode-go/processor"
	"github.com/republicprotocol/darknode-go/server/jsonrpc"
)

// Client is a lightnode Client which can be used to interact with the lightnode.
type Client struct {
	http jrpc.Client
}

// NewClient returns a new Client.
func NewClient(timeout time.Duration) Client {
	return Client{
		http: jrpc.NewClient(timeout),
	}
}

// SendMessage is used to send a sendMessageRequest to the lightnode.
func (client Client) SendMessage(url, to, signature, method string, args []processor.Param) (string, error) {
	data, err := json.Marshal(args)
	if err != nil {
		return "", err
	}
	request := jsonrpc.SendMessageRequest{
		Nonce:     rand.Uint64(),
		To:        to,
		Signature: signature,
		Payload: jsonrpc.Payload{
			Method: method,
			Args:   data,
		},
	}

	var response jsonrpc.SendMessageResponse
	err = client.sendRequest(url, jsonrpc.MethodSendMessage, request, response)
	return response.MessageID, err
}

// ReceiveMessage is used to send a receiveMessageRequest to the lightnode.
func (client Client) ReceiveMessage(url, messageID string) ([]processor.Param, error) {
	request := jsonrpc.ReceiveMessageRequest{
		MessageID: messageID,
	}

	var response jsonrpc.ReceiveMessageResponse
	if err := client.sendRequest(url, jsonrpc.MethodReceiveMessage, request, response); err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, response.Error
	}
	results := struct {
		Values []processor.Param `json:"values"`
	}{}
	err := json.Unmarshal(response.Result, results)
	return results.Values, err
}

func (client Client) sendRequest(url, method string, request jsonrpc.Request, response jsonrpc.Response) error {
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	req := jsonrpc.JSONRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  data,
		ID:      rand.Int31(),
	}
	resp, err := client.http.Call(url, req)
	if err != nil {
		return err
	}
	log.Println("response ", resp)
	if resp.Error != nil {
		return fmt.Errorf("[error] code=%v, message=%v", resp.Error.Code, resp.Error.Message)
	}

	return json.Unmarshal(resp.Result, &response)
}
