package resolver

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/renproject/pack"
)

type Address struct {
	Address string `json:"address"`
}

type Response struct {
	Address      string `json:"address"`
	IsSanctioned bool   `json:"isSanctioned"`
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

func (screener Screener) IsSanctioned(addr pack.String) (bool, error) {
	// First check if the address has been blacklisted in the db
	sanctioned, err := screener.isSanctionedFromDB(addr)
	if err != nil {
		return false, err
	}
	if sanctioned {
		return true, nil
	}

	// Query external API
	return screener.isSanctionedFromAPI(addr)
}

func (screener Screener) isSanctionedFromDB(addr pack.String) (bool, error) {
	if screener.db == nil {
		return false, nil
	}
	rows, err := screener.db.Query("SELECT * FROM blacklist where address=$1", addr.String())
	if err != nil {
		return false, err
	}

	defer rows.Close()
	sanctioned := rows.Next()

	return sanctioned, rows.Err()
}

func (screener Screener) isSanctionedFromAPI(addr pack.String) (bool, error) {
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
