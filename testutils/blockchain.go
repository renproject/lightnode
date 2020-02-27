package testutils

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/renproject/darknode"
	"github.com/renproject/darknode/consensus/txcheck/transform/blockchain"
	"github.com/renproject/darknode/ethrpc"
	"github.com/renproject/mercury/sdk/client/btcclient"
	"github.com/renproject/mercury/sdk/client/ethclient"
	"github.com/renproject/mercury/types"
	"github.com/sirupsen/logrus"
)

func InitConnPool(logger logrus.FieldLogger, network darknode.Network, protocolAddr common.Address) blockchain.ConnPool {
	btcClient := btcclient.NewClient(logger, darknode.BtcNetwork(types.Bitcoin, network))
	zecClient := btcclient.NewClient(logger, darknode.BtcNetwork(types.ZCash, network))
	bchClient := btcclient.NewClient(logger, darknode.BtcNetwork(types.BitcoinCash, network))

	// Initialize Ethereum client and contracts.
	ethClient, err := ethclient.New(logger, darknode.EthShifterNetwork(network))
	if err != nil {
		logger.Panicf("[connPool] failed to connect to Ethereum: %v", err)
	}
	protocol, err := ethrpc.NewProtocol(ethClient.EthClient(), protocolAddr)
	if err != nil {
		panic(fmt.Errorf("cannot initialise protocol contract: %v", err))
	}

	return blockchain.New(logger, btcClient, zecClient, bchClient, ethClient, protocol)
}
