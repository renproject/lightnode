package testutils

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/renproject/darknode"
	"github.com/renproject/darknode/abi"
	"github.com/renproject/darknode/consensus/txcheck/transform/blockchain"
	"github.com/renproject/darknode/ethrpc"
	"github.com/renproject/mercury/sdk/client/btcclient"
	"github.com/renproject/mercury/sdk/client/ethclient"
	"github.com/renproject/mercury/types"
	"github.com/renproject/mercury/types/btctypes"
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

type connPool struct {
	logger                    logrus.FieldLogger
	maxAttemptsUntilConfirmed int
	numAttempts               map[uint64]int
	numAttemptsMu             *sync.Mutex
}

func MockConnPool(logger logrus.FieldLogger, maxAttemptsUntilConfirmed int) *connPool {
	return &connPool{
		logger:                    logger,
		maxAttemptsUntilConfirmed: maxAttemptsUntilConfirmed,
		numAttempts:               make(map[uint64]int),
		numAttemptsMu:             new(sync.Mutex),
	}
}

func (cp *connPool) Utxo(ctx context.Context, addr abi.Address, hash abi.B32, vout abi.U32) (btctypes.UTXO, error) {
	ref := binary.BigEndian.Uint64(hash[:])
	return btctypes.NewUTXO(btctypes.NewOutPoint("", 0), btctypes.Amount(rand.Uint64()), []byte{}, cp.confirmations(ref), []byte{}), nil
}

func (cp *connPool) EventConfirmations(ctx context.Context, addr abi.Address, ref uint64) (uint64, error) {
	return cp.confirmations(ref), nil
}

func (cp *connPool) confirmations(ref uint64) uint64 {
	cp.numAttemptsMu.Lock()
	defer cp.numAttemptsMu.Unlock()

	cp.numAttempts[ref]++

	confirmations := uint64(0)
	// There is a 50% chance the UTXO will be marked as confirmed, until it
	// exceeds the threshold.
	if rand.Int()%2 == 0 || cp.numAttempts[ref] >= cp.maxAttemptsUntilConfirmed {
		confirmations = math.MaxUint64

		// Increase the number of attempts so in future queries this tx remains
		// confirmed.
		cp.numAttempts[ref] = cp.maxAttemptsUntilConfirmed
	} else {
		cp.logger.Infof("tx with ref=%v is not confirmed (attempt %d/%d)", ref, cp.numAttempts[ref], cp.maxAttemptsUntilConfirmed)
	}

	return confirmations
}
