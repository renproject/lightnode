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

type MockAccountTx struct {
	TxHash  pack.Bytes
	TxFrom  multichain.Address
	TxTo    multichain.Address
	TxValue pack.U256
	TxNonce pack.U256
}

func (tx MockAccountTx) Hash() pack.Bytes {
	return tx.TxHash
}

func (tx MockAccountTx) From() multichain.Address {
	return tx.TxFrom
}

func (tx MockAccountTx) To() multichain.Address {
	return tx.TxTo
}

func (tx MockAccountTx) Value() pack.U256 {
	return tx.TxValue
}

func (tx MockAccountTx) Nonce() pack.U256 {
	return tx.TxNonce
}

func (tx MockAccountTx) Payload() multichain.ContractCallData {
	return []byte{}
}

func (tx MockAccountTx) Sighashes() ([]pack.Bytes32, error) {
	return []pack.Bytes32{}, nil
}

func (tx MockAccountTx) Sign(signatures []pack.Bytes65, pubKey pack.Bytes) error {
	return nil
}

func (tx MockAccountTx) Serialize() (pack.Bytes, error) {
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

func (b mockBindings) AccountLockInfo(ctx context.Context, sourceChain, destinationChain multichain.Chain, asset multichain.Asset, txid, payload pack.Bytes, nonce pack.Bytes32) (pack.U256, pack.String, error) {
	return pack.U256{}, "", nil
}

func (b mockBindings) UTXOLockInfo(ctx context.Context, chain multichain.Chain, outpoint multichain.UTXOutpoint) (multichain.UTXOutput, error) {
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
