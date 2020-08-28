package testutils

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/renproject/multichain"
	"github.com/renproject/multichain/api/utxo"
	"github.com/renproject/pack"
	"github.com/sirupsen/logrus"
)

type mockBindings struct {
	logger                    logrus.FieldLogger
	maxAttemptsUntilConfirmed int
	numAttempts               map[string]int
	numAttemptsMu             *sync.Mutex
}

func MockBindings(logger logrus.FieldLogger, maxAttemptsUntilConfirmed int) *mockBindings {
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

func (b mockBindings) Phash(chain multichain.Chain, payload pack.Bytes) pack.Bytes32 {
	panic("unimplemented")
}

func (b mockBindings) Nhash(chain multichain.Chain, nonce pack.Bytes32, outpoint multichain.UTXOutpoint) pack.Bytes32 {
	panic("unimplemented")
}

func (b mockBindings) Ghash(chain multichain.Chain, pHash pack.Bytes32, token, to pack.Bytes, nonce pack.Bytes32) pack.Bytes32 {
	panic("unimplemented")
}

func (b mockBindings) Sighash(chain multichain.Chain, phash pack.Bytes32, amount pack.U256, token, to pack.Bytes, nhash pack.Bytes32) (pack.Bytes32, error) {
	panic("unimplemented")
}

func (b mockBindings) AccountBurnInfo(ctx context.Context, chain multichain.Chain, asset multichain.Asset, nonce pack.Bytes32) (amount pack.U256, recipient pack.String, payload pack.Bytes, err error) {
	return pack.U256{}, "", nil, b.isConfirmed(nonce.String())
}

func (b mockBindings) AccountLockInfo(ctx context.Context, chain multichain.Chain, asset multichain.Asset, nonce pack.Bytes32) (amount pack.U256, recipient pack.String, payload pack.Bytes, err error) {
	panic("unimplemented")
}

func (b mockBindings) AccountBuildTx(chain multichain.Chain, asset multichain.Asset, from, to multichain.Address, value, nonce pack.U256, payload pack.Bytes) (multichain.AccountTx, error) {
	panic("unimplemented")
}

func (b mockBindings) AccountSubmitTx(ctx context.Context, chain multichain.Chain, tx multichain.AccountTx) error {
	panic("unimplemented")
}

func (b mockBindings) UTXOLockInfo(ctx context.Context, chain multichain.Chain, asset multichain.Asset, outpoint multichain.UTXOutpoint) (multichain.UTXOutput, error) {
	return utxo.Output{}, b.isConfirmed(outpoint.Hash.String())
}

func (b mockBindings) UTXOBuildTx(ctx context.Context, chain multichain.Chain, asset multichain.Asset, inputs []multichain.UTXOInput, recipients []multichain.UTXORecipient) (multichain.UTXOTx, error) {
	panic("unimplemented")
}

func (b mockBindings) UTXOSubmitTx(ctx context.Context, chain multichain.Chain, tx multichain.UTXOTx) error {
	panic("unimplemented")
}

func (b mockBindings) UTXOGatewayPubKeyScript(chain multichain.Chain, asset multichain.Asset, gpubkey pack.Bytes, ghash pack.Bytes32) (pack.Bytes, error) {
	panic("unimplemented")
}

func (b mockBindings) UTXOGatewayScript(chain multichain.Chain, asset multichain.Asset, gpubkey pack.Bytes, ghash pack.Bytes32) (pack.Bytes, error) {
	panic("unimplemented")
}

func (b mockBindings) GasFee(chain multichain.Chain, asset multichain.Asset, gasCost pack.U256) (gasFee pack.U256, err error) {
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
