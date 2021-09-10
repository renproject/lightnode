package testutils

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/renproject/darknode/binding"
	"github.com/renproject/multichain"
	"github.com/renproject/multichain/api/address"
	"github.com/renproject/multichain/api/utxo"
	"github.com/renproject/pack"
	"github.com/sirupsen/logrus"
)

type mockAccountTx struct {
}

func (tx mockAccountTx) Hash() pack.Bytes {
	return pack.Bytes{}
}

func (tx mockAccountTx) From() multichain.Address {
	return ""
}

func (tx mockAccountTx) To() multichain.Address {
	return ""
}

func (tx mockAccountTx) Value() pack.U256 {
	return pack.NewU256([32]byte{})
}

func (tx mockAccountTx) Nonce() pack.U256 {
	return pack.NewU256([32]byte{})
}

func (tx mockAccountTx) Payload() multichain.ContractCallData {
	return []byte{}
}

func (tx mockAccountTx) Sighashes() ([]pack.Bytes32, error) {
	return []pack.Bytes32{}, nil
}

func (tx mockAccountTx) Sign(signatures []pack.Bytes65, pubKey pack.Bytes) error {
	return nil
}

func (tx mockAccountTx) Serialize() (pack.Bytes, error) {
	return pack.Bytes{}, nil
}

type mockBindings struct {
	logger                    logrus.FieldLogger
	maxAttemptsUntilConfirmed int
	numAttempts               map[string]int
	numAttemptsMu             *sync.Mutex
}

func MockBindings(logger logrus.FieldLogger, maxAttemptsUntilConfirmed int) *binding.Callbacks {
	mock := mockBindings{
		logger:                    logger,
		maxAttemptsUntilConfirmed: maxAttemptsUntilConfirmed,
		numAttempts:               make(map[string]int),
		numAttemptsMu:             new(sync.Mutex),
	}
	return &binding.Callbacks{
		HandleAccountBurnInfo:       mock.AccountBurnInfo,
		HandleAccountLockInfo:       mock.AccountLockInfo,
		HandleUTXOLockInfo:          mock.UTXOLockInfo,
		HandleTokenAddressFromAsset: mock.TokenAddressFromAsset,
	}
}

func (b mockBindings) AccountBurnInfo(ctx context.Context, chain multichain.Chain, asset multichain.Asset, txid pack.Bytes, nonce pack.Bytes32) (amount pack.U256, recipient pack.String, payload pack.Bytes, err error) {
	return pack.U256{}, "", nil, b.isConfirmed(nonce.String())
}

func (b mockBindings) AccountLockInfo(ctx context.Context, sourceChain, destinationChain multichain.Chain, asset multichain.Asset, txid pack.Bytes, nonce pack.Bytes32) (pack.U256, pack.String, error) {
	return pack.U256{}, "", nil
}

func (b mockBindings) UTXOLockInfo(ctx context.Context, chain multichain.Chain, asset multichain.Asset, outpoint multichain.UTXOutpoint) (multichain.UTXOutput, error) {
	return utxo.Output{}, b.isConfirmed(outpoint.Hash.String())
}

func (b mockBindings) TokenAddressFromAsset(chain multichain.Chain, asset multichain.Asset) (address.RawAddress, error) {
	addr := [20]byte{}
	return address.RawAddress(addr[:]), nil
}

func (b mockBindings) isConfirmed(hash string) error {
	b.numAttemptsMu.Lock()
	defer b.numAttemptsMu.Unlock()

	r := rand.New(rand.NewSource(time.Now().Unix()))

	b.numAttempts[hash]++

	// There is a 50% chance the UTXO will be marked as confirmed until it
	// exceeds the threshold.
	if r.Int()%2 == 0 || b.numAttempts[hash] >= b.maxAttemptsUntilConfirmed {
		// Increase the number of attempts so in future queries for this
		// transaction remain confirmed.
		b.numAttempts[hash] = b.maxAttemptsUntilConfirmed
		return nil
	}

	return fmt.Errorf("tx is not confirmed (attempt %d/%d)", b.numAttempts[hash], b.maxAttemptsUntilConfirmed)
}
