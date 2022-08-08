package resolver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/renproject/pack"
)

// A list of addresses which have been blacklisted.
var Blacklist = map[string]bool{
	// Solana wallet
	"Htp9MGP8Tig923ZFY7Qf2zzbMUmYneFRAhSp7vSg4wxV": true,
	"CEzN7mqP9xoxn2HdyW6fjEJ73t7qaX9Rp2zyS6hb3iEu": true,
	"5WwBYgQG6BdErM2nNNyUmQXfcUnB68b6kesxBywh1J3n": true,
	"GeEccGJ9BEzVbVor1njkBCCiqXJbXVeDHaXDCrBDbmuy": true,
}

type Address struct {
	Address string `json:"address"`
}

type Response struct {
	Address      string `json:"address"`
	IsSanctioned bool   `json:"isSanctioned"`
}

type Screener struct {
	key string
}

func NewScreener(key string) Screener {
	return Screener{
		key: key,
	}
}

func (screener Screener) IsSanctioned(addr pack.String) (bool, error) {
	// Check if the address is in our Blacklist
	if sanctioned := Blacklist[strings.TrimSpace(string(addr))]; sanctioned {
		return true, nil
	}

	// Generate the request body
	client := new(http.Client)
	addresses := []Address{
		{
			Address: string(addr),
		},
	}
	data, err := json.Marshal(addresses)
	if err != nil {
		return false, fmt.Errorf("[screener] unable to marshal address [%v], err = %v", addresses, err)
	}
	input := bytes.NewBuffer(data)

	// Construct the request
	request, err := http.NewRequest("POST", "https://api.trmlabs.com/public/v1/sanctions/screening", input)
	if err != nil {
		return false, err
	}
	request.Header.Set("Content-Type", "application/json")
	if screener.key != "" {
		request.SetBasicAuth(screener.key, screener.key)
	}

	response, err := client.Do(request)
	if err != nil {
		return false, fmt.Errorf("[screener] error sending request, err = %v", err)
	}
	if response.StatusCode != http.StatusCreated {
		return false, fmt.Errorf("[screener] invalid status code, expect 201, got %v", response.StatusCode)
	}

	// Parse the response
	var resp []Response
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return false, fmt.Errorf("[screener] unexpected response, %v", err)
	}
	defer response.Body.Close()
	if len(resp) != 1 {
		return false, fmt.Errorf("[screener] invalid number of reponse, expected 1, got %v", len(resp))
	}
	if resp[0].Address != string(addr) {
		return false, fmt.Errorf("[screener] invalid response of address, expect %v, got %v", string(addr), resp[0].Address)
	}
	return resp[0].IsSanctioned, nil
}
