package resolver

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/renproject/multichain"
	"github.com/renproject/pack"
)

type AddressScreeningRequest struct {
	Address           string `json:"address"`
	Chain             string `json:"chain"`
	AccountExternalID string `json:"accountExternalId"`
}

type AddressScreeningResponse struct {
	AddressRiskIndicators []AddressRiskIndicator `json:"addressRiskIndicators"`
	Address               string                 `json:"address"`
	Entities              []Entity               `json:"entities"`
}

type AddressRiskIndicator struct {
	Category                    string `json:"category"`
	CategoryID                  string `json:"categoryId"`
	CategoryRiskScoreLevel      int    `json:"categoryRiskScoreLevel"`
	CategoryRiskScoreLevelLabel string `json:"categoryRiskScoreLevelLabel"`
	RiskType                    string `json:"risk_type"`
}

type Entity struct {
	Category            string `json:"category"`
	CategoryID          string `json:"categoryId"`
	Entity              string `json:"entity"`
	RiskScoreLevel      int    `json:"riskScoreLevel"`
	RiskScoreLevelLabel string `json:"riskScoreLevelLabel"`
}

type Screener struct {
	db  *sql.DB
	key string
}

func NewScreener(db *sql.DB, key string) Screener {
	screener := Screener{
		db:  db,
		key: key,
	}
	if err := screener.init(); err != nil {
		panic(fmt.Errorf("failed to initialized db, err = %v", err))
	}
	return screener
}

func (screener Screener) init() error {
	if screener.db == nil {
		return nil
	}
	script := `CREATE TABLE IF NOT EXISTS blacklist (
		address            VARCHAR NOT NULL PRIMARY KEY
	);`
	_, err := screener.db.Exec(script)
	return err
}

func (screener Screener) IsBlacklisted(addr pack.String, chain multichain.Chain) (bool, error) {
	// First check if the address has been blacklisted in the db
	blacklisted, err := screener.isBlacklistedFromDB(addr)
	if err != nil {
		return false, err
	}
	if blacklisted {
		return true, nil
	}

	// Check against external API
	return screener.isBlacklistedFromAPI(addr, chain)
}

func (screener Screener) isBlacklistedFromDB(addr pack.String) (bool, error) {
	if screener.db == nil {
		return false, nil
	}
	rows, err := screener.db.Query("SELECT * FROM blacklist where address=$1", FormatAddress(string(addr)))
	if err != nil {
		return false, err
	}

	defer rows.Close()
	blacklisted := rows.Next()

	return blacklisted, rows.Err()
}

func (screener Screener) addToDB(addr string) error {
	if screener.db == nil {
		return nil
	}
	script := "INSERT INTO blacklist values ($1);"
	_, err := screener.db.Exec(script, FormatAddress(addr))
	return err
}

func (screener Screener) isBlacklistedFromAPI(addr pack.String, chain multichain.Chain) (bool, error) {
	// Disable the external API call when key is not set
	if screener.key == "" {
		fmt.Printf("screener disabled : key not set")
		return false, nil
	}

	// Generate the request body
	client := new(http.Client)
	chainIdentifier := trmIdentifier(chain)
	if chainIdentifier == "" {
		return false, nil
	}
	req := []AddressScreeningRequest{
		{
			Address:           string(addr),
			Chain:             chainIdentifier,
			AccountExternalID: fmt.Sprintf("%v_%v", chainIdentifier, addr),
		},
	}
	data, err := json.Marshal(req)
	if err != nil {
		return false, fmt.Errorf("[screener] unable to marshal request, err = %v", err)
	}
	input := bytes.NewBuffer(data)

	// Construct the request
	request, err := http.NewRequest("POST", "https://api.trmlabs.com/public/v2/screening/addresses", input)
	if err != nil {
		return false, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.SetBasicAuth(screener.key, screener.key)

	// Send the request and parse the response
	response, err := client.Do(request)
	if err != nil {
		return false, fmt.Errorf("[screener] error sending request, err = %v", err)
	}
	if response.StatusCode != http.StatusCreated {
		errMsg, err := ioutil.ReadAll(response.Body)
		if err != nil {
			panic(err)
		}
		return false, fmt.Errorf("[screener] invalid status code, expect 201, got %v, message = %v", response.StatusCode, string(errMsg))
	}

	// Parse the response
	var resp []AddressScreeningResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return false, fmt.Errorf("[screener] unexpected response, %v", err)
	}
	defer response.Body.Close()

	if len(resp) != 1 {
		return false, fmt.Errorf("[screener] invalid number of reponse, expected 1, got %v", len(resp))
	}

	for _, indicator := range resp[0].AddressRiskIndicators {
		if indicator.CategoryRiskScoreLevel >= 10 {
			return true, screener.addToDB(string(addr))
		}
	}

	for _, entity := range resp[0].Entities {
		if entity.RiskScoreLevel >= 10 {
			return true, screener.addToDB(string(addr))
		}
	}
	return false, nil
}

func trmIdentifier(chain multichain.Chain) string {
	switch chain {
	case multichain.Avalanche:
		return "avalanche_c_chain"
	case multichain.BinanceSmartChain:
		return "binance_smart_chain"
	case multichain.BitcoinCash:
		return "bitcoin_cash"
	case multichain.Bitcoin:
		return "bitcoin"
	case multichain.Dogecoin:
		return "dogecoin"
	case multichain.Ethereum, multichain.Fantom, multichain.Arbitrum, multichain.Kava, multichain.Moonbeam, multichain.Optimism:
		return "ethereum"
	case multichain.Filecoin:
		return "filecoin"
	case multichain.Polygon:
		return "polygon"
	case multichain.Solana:
		return "solana"
	case multichain.Zcash:
		return "zcash"
	default:
		return ""
	}
}

func FormatAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	addr = strings.TrimPrefix(addr, "0x")
	addr = strings.ToLower(addr)
	addr = strings.TrimSpace(addr)
	return addr
}
