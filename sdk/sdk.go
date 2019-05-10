package sdk

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	jrpc "github.com/renproject/lightnode/rpc/jsonrpc"
	"github.com/republicprotocol/darknode-go/health"
	"github.com/republicprotocol/darknode-go/processor"
	"github.com/republicprotocol/darknode-go/server/jsonrpc"
	"github.com/republicprotocol/renp2p-go/core/peer"
)

type Stats struct {
	Version  string       `json:"version"`
	Address  string       `json:"address"`
	CPUs     []health.CPU `json:"cpus"`
	RAM      int          `json:"ram"`
	Disk     int          `json:"disk"`
	Location string       `json:"location"`
}

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
	jsonReponse, err := client.sendRequest(url, jsonrpc.MethodSendMessage, request)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(jsonReponse.Result, &response)
	return response.MessageID, err
}

// ReceiveMessage is used to send a receiveMessageRequest to the lightnode.
func (client Client) ReceiveMessage(url, messageID string) ([]processor.Param, error) {
	request := jsonrpc.ReceiveMessageRequest{
		MessageID: messageID,
	}

	var response jsonrpc.ReceiveMessageResponse
	jsonResponse, err := client.sendRequest(url, jsonrpc.MethodReceiveMessage, request)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(jsonResponse.Result, &response); err != nil {
		return nil, err
	}
	results := struct {
		Values []processor.Param `json:"values"`
	}{}

	err = json.Unmarshal(response.Result, &results)

	return results.Values, err
}

func (client Client) QueryPeers(url string) ([]peer.MultiAddr, error) {
	request := jsonrpc.QueryPeersRequest{}

	var response jsonrpc.QueryPeersResponse
	jsonResponse, err := client.sendRequest(url, jsonrpc.MethodQueryPeers, request)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(jsonResponse.Result, &response); err != nil {
		return nil, err
	}

	multiAddrs := make([]peer.MultiAddr, len(response.Peers))
	for i := range response.Peers {
		multiAddrs[i], err = peer.NewMultiAddr(response.Peers[i], 0, [65]byte{})
		if err != nil {
			return nil, err
		}
	}

	return multiAddrs, nil
}

func (client Client) QueryNumPeers(url string) (int, error) {
	request := jsonrpc.QueryNumPeersRequest{}

	var response jsonrpc.QueryNumPeersResponse
	jsonResponse, err := client.sendRequest(url, jsonrpc.MethodQueryNumPeers, request)
	if err != nil {
		return 0, err
	}
	err = json.Unmarshal(jsonResponse.Result, &response)

	return response.NumPeers, err
}

func (client Client) QueryStats(url, darknodeID string) (Stats, error) {
	request := jsonrpc.QueryStatsRequest{
		DarknodeID: darknodeID,
	}

	var response jsonrpc.QueryStatsResponse
	jsonResponse, err := client.sendRequest(url, jsonrpc.MethodQueryStats, request)
	if err != nil {
		return Stats{}, err
	}
	err = json.Unmarshal(jsonResponse.Result, &response)
	return Stats{
		Version:  response.Version,
		Address:  response.Address,
		CPUs:     response.CPUs,
		RAM:      response.RAM,
		Disk:     response.Disk,
		Location: response.Location,
	}, err
}

func (client Client) sendRequest(url, method string, request jsonrpc.Request) (jsonrpc.JSONResponse, error) {
	data, err := json.Marshal(request)
	if err != nil {
		return jsonrpc.JSONResponse{}, err
	}
	req := jsonrpc.JSONRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  data,
		ID:      rand.Int31(),
	}
	resp, err := client.http.Call(url, req)
	if err != nil {
		return jsonrpc.JSONResponse{}, err
	}
	log.Println("response ", resp)
	if resp.Error != nil {
		return jsonrpc.JSONResponse{}, fmt.Errorf("[error] code=%v, message=%v", resp.Error.Code, resp.Error.Message)
	}

	return resp, nil
}
