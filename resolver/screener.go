package resolver

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
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
	AddressSubmitted      string                 `json:"addressSubmitted"`
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

func (screener Screener) IsBlacklisted(addrs []pack.String, chain multichain.Chain) (bool, error) {
	if len(addrs) == 0 {
		return false, nil
	}

	// First check if the address has been blacklisted in the db
	blacklisted, err := screener.isBlacklistedFromDB(addrs)
	if err != nil {
		return false, err
	}
	if blacklisted {
		return true, nil
	}

	// Check against external API
	return screener.isBlacklistedFromAPI(addrs, chain)
}

func (screener Screener) isBlacklistedFromDB(addrs []pack.String) (bool, error) {
	if screener.db == nil {
		return false, nil
	}

	// Query the db
	addresses := make([]string, len(addrs))
	for i := range addrs {
		addresses[i] = "'" + FormatAddress(string(addrs[i])) + "'"
	}
	array := "(" + strings.Join(addresses, ",") + ")"
	query := fmt.Sprintf("SELECT * FROM blacklist where address in %v", array)
	rows, err := screener.db.Query(query)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	return rows.Next(), rows.Err()
}

func (screener Screener) addToDB(addr string) error {
	if screener.db == nil {
		return nil
	}
	script := "INSERT INTO blacklist values ($1);"
	_, err := screener.db.Exec(script, FormatAddress(addr))
	return err
}

func (screener Screener) isBlacklistedFromAPI(addrs []pack.String, chain multichain.Chain) (bool, error) {
	// Disable the external API call when key is not set
	if screener.key == "" {
		fmt.Printf("screener disabled : key not set")
		return false, nil
	}

	// Generate the request body
	client := new(http.Client)
	requestData := make([]AddressScreeningRequest, 0, len(addrs))
	for _, addr := range addrs {
		chainIdentifier := trmIdentifier(chain)
		if chainIdentifier == "" {
			continue
		}
		requestData = append(requestData, AddressScreeningRequest{
			Address:           string(addr),
			Chain:             chainIdentifier,
			AccountExternalID: fmt.Sprintf("%v_%v", chainIdentifier, addr),
		})
	}
	if len(requestData) == 0 {
		return false, nil
	}

	data, err := json.Marshal(requestData)
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
		return false, fmt.Errorf("[screener] invalid status code, expect 201, got %v", response.StatusCode)
	}

	// Parse the response
	var resps []AddressScreeningResponse
	if err := json.NewDecoder(response.Body).Decode(&resps); err != nil {
		return false, fmt.Errorf("[screener] unexpected response, %v", err)
	}
	defer response.Body.Close()

	if len(resps) != len(addrs) {
		return false, fmt.Errorf("[screener] invalid number of reponse, expected %v , got %v", len(addrs), len(resps))
	}

	blacklisted := false
Responses:
	for _, resp := range resps {
		for _, indicator := range resp.AddressRiskIndicators {
			if indicator.CategoryRiskScoreLevel >= 10 {
				blacklisted = true
				if err := screener.addToDB(FormatAddress(resp.AddressSubmitted)); err != nil {
					return blacklisted, err
				}
				continue Responses
			}
		}

		for _, entity := range resp.Entities {
			if entity.RiskScoreLevel >= 10 {
				if err := screener.addToDB(FormatAddress(resp.AddressSubmitted)); err != nil {
					return blacklisted, err
				}
				continue Responses
			}
		}
	}

	return blacklisted, nil
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
