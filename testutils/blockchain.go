package testutils

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/renproject/darknode/txengine"
	"github.com/renproject/id"
	"github.com/renproject/multichain"
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

func MockBindings(logger logrus.FieldLogger, maxAttemptsUntilConfirmed int) txengine.Bindings {
	return &mockBindings{
		logger:                    logger,
		maxAttemptsUntilConfirmed: maxAttemptsUntilConfirmed,
		numAttempts:               make(map[string]int),
		numAttemptsMu:             new(sync.Mutex),
	}
}

func (b mockBindings) EncodeAddress(chain multichain.Chain, rawAddr multichain.RawAddress) (multichain.Address, error) {
	panic("unimplemented")
}

func (b mockBindings) DecodeAddress(chain multichain.Chain, addr multichain.Address) (multichain.RawAddress, error) {
	panic("unimplemented")
}

func (b mockBindings) AddressFromPubKey(chain multichain.Chain, pubKey *id.PubKey) (multichain.Address, error) {
	panic("unimplemented")
}

func (b mockBindings) AccountBurnInfo(ctx context.Context, chain multichain.Chain, asset multichain.Asset, nonce pack.Bytes32) (amount pack.U256, recipient pack.String, payload pack.Bytes, err error) {
	return pack.U256{}, "", nil, b.isConfirmed(nonce.String())
}

func (b mockBindings) AccountLockInfo(ctx context.Context, chain multichain.Chain, asset multichain.Asset, txid pack.Bytes) (multichain.AccountTx, error) {
	return mockAccountTx{}, nil
}

func (b mockBindings) AccountBuildTx(ctx context.Context, chain multichain.Chain, asset multichain.Asset, from, to multichain.Address, value, nonce, gasLimit, gasPrice pack.U256, payload pack.Bytes) (multichain.AccountTx, error) {
	panic("unimplemented")
}

func (b mockBindings) AccountSubmitTx(ctx context.Context, chain multichain.Chain, tx multichain.AccountTx) error {
	panic("unimplemented")
}

func (b mockBindings) UTXOLockInfo(ctx context.Context, chain multichain.Chain, asset multichain.Asset, outpoint multichain.UTXOutpoint) (multichain.UTXOutput, error) {
	return utxo.Output{}, b.isConfirmed(outpoint.Hash.String())
}

func (b mockBindings) UTXOBuildTx(chain multichain.Chain, asset multichain.Asset, inputs []multichain.UTXOInput, recipients []multichain.UTXORecipient) (multichain.UTXOTx, error) {
	panic("unimplemented")
}

func (b mockBindings) UTXOSubmitTx(ctx context.Context, chain multichain.Chain, tx multichain.UTXOTx) error {
	panic("unimplemented")
}

func (b mockBindings) GasPrice(chain multichain.Chain) (gasPrice pack.U256, err error) {
	panic("unimplemented")
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
