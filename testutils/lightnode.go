package testutils

import (
	"github.com/renproject/darknode/jsonrpc"
)

var QueryRequests = []string{
	jsonrpc.MethodQueryBlock,
	jsonrpc.MethodQueryBlocks,
	jsonrpc.MethodQueryFees,
	jsonrpc.MethodQueryNumPeers,
	jsonrpc.MethodQueryPeers,
	jsonrpc.MethodQueryShards,
	jsonrpc.MethodQueryStat,
	jsonrpc.MethodQueryTx,
	jsonrpc.MethodQueryTxs,
}
