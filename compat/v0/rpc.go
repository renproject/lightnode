package v0

const (
	// MethodQueryShards returns information about the currently online/offline
	// Shards.
	// Deprecated in v1 by queryState
	MethodQueryShards = "ren_queryShards"

	// MethodQueryFees returns information about the current RenVM fees and
	// undelrying blockchain fees. This information cannot be verified.
	// Deprecated in v1 by query
	MethodQueryFees = "ren_queryFees"
)

type Gateway struct {
	Asset  string   `json:"asset"`
	Hosts  []string `json:"hosts"`
	Locked string   `json:"locked"`
	Origin string   `json:"origin"`
	PubKey string   `json:"pubKey"`
}

type CompatShard struct {
	DarknodesRootHash string    `json:"darknodesRootHash"`
	Gateways          []Gateway `json:"gateways"`
	GatewaysRootHash  string    `json:"gatewaysRootHash"`
	Primary           bool      `json:"primary"`
	PubKey            string    `json:"pubKey"`
}

// ResponseQueryShards defines the response of the MethodQueryShards.
type ResponseQueryShards struct {
	Shards []CompatShard `json:"shards"`
}

type Fees struct {
	Lock     U64             `json:"lock"`
	Release  U64             `json:"release"`
	Ethereum MintAndBurnFees `json:"ethereum"`
}

type MintAndBurnFees struct {
	Mint U64 `json:"mint"`
	Burn U64 `json:"burn"`
}

type ResponseQueryFees struct {
	Btc Fees `json:"btc"`
	Zec Fees `json:"zec"`
	Bch Fees `json:"bch"`
}

type ParamsSubmitTx struct {
	Tx Tx `json:"tx"`
}

type ParamsQueryTx struct {
	TxHash B32 `json:"txHash"`
}

type ResponseQueryTx struct {
	Tx       Tx     `json:"tx"`
	TxStatus string `json:"txStatus"`
}

type ResponseSubmitTx struct {
	Tx Tx `json:"tx"`
}
