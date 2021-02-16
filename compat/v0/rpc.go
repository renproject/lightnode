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