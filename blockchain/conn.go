package blockchain

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/renproject/darknode"
	"github.com/renproject/darknode/abi"
	"github.com/renproject/darknode/ethrpc/bindings"
	"github.com/renproject/mercury/sdk/client/btcclient"
	"github.com/renproject/mercury/sdk/client/ethclient"
	"github.com/renproject/mercury/sdk/gateway/btcgateway"
	"github.com/renproject/mercury/types"
	"github.com/renproject/mercury/types/btctypes"
	"github.com/renproject/mercury/types/ethtypes"
	"github.com/sirupsen/logrus"
)

type ConnPool struct {
	logger     logrus.FieldLogger
	ethClient  ethclient.Client
	btcClient  btcclient.Client
	zecClient  btcclient.Client
	bchClient  btcclient.Client
	btcShifter *bindings.Shifter
	zecShifter *bindings.Shifter
	bchShifter *bindings.Shifter
}

func New(logger logrus.FieldLogger, network darknode.Network, btcShifterAddr, zecShifterAddr, bchShifterAddr common.Address) ConnPool {
	ethClient, err := ethclient.New(logger, ethNetwork(network))
	if err != nil {
		logger.Panicf("cannot connect to ethereum, err = %v", err)
	}

	btcClient := btcclient.NewClient(logger, btcNetwork(types.Bitcoin, network))
	zecClient := btcclient.NewClient(logger, btcNetwork(types.ZCash, network))
	bchClient := btcclient.NewClient(logger, btcNetwork(types.BitcoinCash, network))

	btcShifter, err := bindings.NewShifter(btcShifterAddr, ethClient.EthClient())
	if err != nil {
		logger.Panicf("cannot initialize btc shifter, err = %v", err)
	}
	zecShifter, err := bindings.NewShifter(zecShifterAddr, ethClient.EthClient())
	if err != nil {
		logger.Panicf("cannot initialize zec shifter, err = %v", err)
	}
	bchShifter, err := bindings.NewShifter(bchShifterAddr, ethClient.EthClient())
	if err != nil {
		logger.Panicf("cannot initialize bch shifter, err = %v", err)
	}

	return ConnPool{
		logger:     logger,
		ethClient:  ethClient,
		btcClient:  btcClient,
		zecClient:  zecClient,
		bchClient:  bchClient,
		btcShifter: btcShifter,
		zecShifter: zecShifter,
		bchShifter: bchShifter,
	}
}

func (cp ConnPool) ClientByAddress(addr abi.Address) btcclient.Client {
	switch addr {
	case abi.IntrinsicBTC0Btc2Eth.Address:
		return cp.btcClient
	case abi.IntrinsicZEC0Zec2Eth.Address:
		return cp.zecClient
	case abi.IntrinsicBCH0Bch2Eth.Address:
		return cp.bchClient
	default:
		return nil
	}
}

func (cp ConnPool) ShifterByAddress(addr abi.Address) *bindings.Shifter {
	switch addr {
	case abi.IntrinsicBTC0Eth2Btc.Address:
		return cp.btcShifter
	case abi.IntrinsicZEC0Eth2Zec.Address:
		return cp.zecShifter
	case abi.IntrinsicBCH0Eth2Bch.Address:
		return cp.bchShifter
	default:
		cp.logger.Panicf("[validator] invalid shiftOut address = %v", addr)
		return nil
	}
}

func (cp ConnPool) ShiftOut(addr abi.Address, ref uint64) ([]byte, uint64, error) {
	shifter := cp.ShifterByAddress(addr)
	shiftID := big.NewInt(int64(ref))

	// Filter for all epoch events in this range of blocks
	iter, err := shifter.FilterLogShiftOut(nil, []*big.Int{shiftID}, nil)
	if err != nil {
		return nil, 0, err
	}

	// Loop through the filtered logs and find the event with given ref.
	for iter.Next() {
		to := iter.Event.To
		amount := iter.Event.Amount
		return to, amount.Uint64(), nil
	}

	return nil, 0, fmt.Errorf("invalid ref, no event with ref =%v can be found", ref)
}

func (cp ConnPool) Utxo(ctx context.Context, addr abi.Address, hash abi.B32, vout abi.U32) (btctypes.UTXO, error) {
	client := cp.ClientByAddress(addr)
	txHash := types.TxHash(hex.EncodeToString(hash[:]))
	outpoint := btctypes.NewOutPoint(txHash, uint32(vout.Int.Uint64()))
	return client.UTXO(ctx, outpoint)
}

func (cp ConnPool) UtxoConfirmations(ctx context.Context, addr abi.Address, hash abi.B32) (uint64, error) {
	client := cp.ClientByAddress(addr)
	txHash := types.TxHash(hex.EncodeToString(hash[:]))
	return client.Confirmations(ctx, txHash)
}

func (cp ConnPool) EventConfirmations(ctx context.Context, addr abi.Address, ref uint64) (uint64, error) {
	shifter := cp.ShifterByAddress(addr)
	shiftID := big.NewInt(int64(ref))

	// Get latest block number
	latestBlock, err := cp.ethClient.BlockNumber(ctx)
	if err != nil {
		return 0, err
	}

	// Find the event log which has given ref.
	opts := &bind.FilterOpts{
		Context: ctx,
	}
	iter, err := shifter.FilterLogShiftOut(opts, []*big.Int{shiftID}, nil)
	if err != nil {
		return 0, err
	}

	// Loop through the filtered logs and find block number of that log.
	for iter.Next() {
		eventBlock := iter.Event.Raw.BlockNumber
		return latestBlock.Uint64() - eventBlock, nil
	}

	return 0, fmt.Errorf("invalid ref, no event with ref =%v can be found", ref)
}

func (cp ConnPool) VerifyScriptPubKey(addr abi.Address, ghash []byte, distPubKey ecdsa.PublicKey, utxo btctypes.UTXO) error {
	client := cp.ClientByAddress(addr)
	gateway := btcgateway.New(client, distPubKey, ghash)
	expectedSPK, err := btctypes.PayToAddrScript(gateway.Address(), client.Network())
	if err != nil {
		return err
	}
	if !bytes.Equal(expectedSPK, utxo.ScriptPubKey()) {
		return errors.New("invalid scriptPubkey")
	}
	return nil
}

func (cp ConnPool) IsShiftIn(tx abi.Tx) bool {
	switch tx.To {
	case abi.IntrinsicBTC0Btc2Eth.Address, abi.IntrinsicZEC0Zec2Eth.Address, abi.IntrinsicBCH0Bch2Eth.Address:
		return true
	case abi.IntrinsicBTC0Eth2Btc.Address, abi.IntrinsicZEC0Eth2Zec.Address, abi.IntrinsicBCH0Eth2Bch.Address:
		return false
	default:
		cp.logger.Panicf("[connPool] expected contract address = %v", tx.To)
		return false
	}
}

func btcNetwork(chain types.Chain, network darknode.Network) btctypes.Network {
	switch network {
	case darknode.Localnet:
		return btctypes.NewNetwork(chain, "localnet")
	case darknode.Devnet, darknode.Testnet:
		return btctypes.NewNetwork(chain, "testnet")
	case darknode.Chaosnet, darknode.Mainnet:
		return btctypes.NewNetwork(chain, "mainnet")
	default:
		panic(fmt.Sprintf("unknown network =%v", network))
	}
}

func ethNetwork(network darknode.Network) ethtypes.Network {
	switch network {
	case darknode.Localnet:
		return ethtypes.Ganache
	case darknode.Devnet, darknode.Testnet:
		return ethtypes.Kovan
	case darknode.Chaosnet, darknode.Mainnet:
		return ethtypes.Mainnet
	default:
		panic(fmt.Sprintf("unknown network =%v", network))
	}
}
