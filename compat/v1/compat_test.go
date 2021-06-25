package v1_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v7"
	"github.com/renproject/darknode/binding"
	"github.com/renproject/darknode/chainstate"
	"github.com/renproject/darknode/engine"
	"github.com/renproject/darknode/tx"
	v1 "github.com/renproject/lightnode/compat/v1"
	"github.com/renproject/lightnode/testutils"
	"github.com/renproject/multichain"
	"github.com/renproject/pack"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Compat V0", func() {
	init := func() *v1.Store {
		mr, err := miniredis.Run()
		Expect(err).ToNot(HaveOccurred())

		client := redis.NewClient(&redis.Options{
			Addr: mr.Addr(),
		})
		return v1.NewCompatStore(client)
	}

	It("should convert a QueryBlockState response into a QueryState response", func() {
		stateResponse, err := v1.QueryStateResponseFromState(testutils.MockBindings(logrus.New(), 0), testutils.MockEngineState())

		Expect(err).ShouldNot(HaveOccurred())
		Expect(stateResponse.State.Bitcoin.Gaslimit).Should(Equal("3"))
	})

	It("should omit empty revert reasons from a queryTxResponse", func() {
		output := engine.LockMintBurnReleaseOutput{
			Revert: "some reason",
		}
		txResponse := v1.TxOutputFromV2QueryTxOutput(output)

		b, err := json.Marshal(txResponse)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(fmt.Sprintf("%v", string(b))).ShouldNot(ContainSubstring("\"revert\":\"some reason\""))
	})

	Context("when removing a gpubkey", func() {
		It("should be able to fetch the new tx hash using the old one", func() {
			gpubkeyStore := init()
			r := rand.New(rand.NewSource(GinkgoRandomSeed()))
			ctx := chainstate.CodeContext{
				Bindings: binding.Callbacks{
					HandleAccountBurnInfo: func(ctx context.Context, chain multichain.Chain, asset multichain.Asset, nonce pack.Bytes32) (pack.U256, pack.String, pack.Bytes, error) {
						return pack.U256{}, "", nil, errors.New("error")
					},
				},
			}

			// Generate a random transaction with a gpubkey specified.
			selector := tx.Selector(fmt.Sprintf("%v/to%v", multichain.AMOCK1, multichain.AccountMocker2))
			input := engine.LockMintBurnReleaseInput{
				Txid:    pack.Bytes{},
				Txindex: 0,
				Amount:  pack.NewU256([32]byte{}),
				Payload: pack.Bytes{},
				Phash:   pack.Bytes32{},
				To:      "",
				Nonce:   pack.Bytes32{},
				Nhash:   pack.Bytes32{},
				Gpubkey: pack.Bytes{}.Generate(r, r.Intn(50)+1).Interface().(pack.Bytes),
				Ghash:   pack.Bytes32{},
			}
			inputEncoded, err := pack.Encode(input)
			Expect(err).ToNot(HaveOccurred())
			tx, err := tx.NewTx(selector, pack.Typed(inputEncoded.(pack.Struct)))
			Expect(err).ToNot(HaveOccurred())

			err = engine.XValidateBurnExtrinsicTx(ctx, nil, tx, input)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("expected gpubkey len=0, got len=%v", len(input.Gpubkey))))

			// Remove the gpubkey from the transaction and validate the result.
			newTx, err := gpubkeyStore.RemoveGpubkey(tx)
			Expect(err).ToNot(HaveOccurred())
			var newInput engine.LockMintBurnReleaseInput
			err = pack.Decode(&newInput, newTx.Input)
			Expect(err).ToNot(HaveOccurred())
			Expect(newInput.Gpubkey).To(HaveLen(0))
			err = engine.XValidateBurnExtrinsicTx(ctx, nil, newTx, newInput)
			Expect(err).To(HaveOccurred()) // An error will still occur since we are providing mock bindings, however it should not be due to the gpubkey.
			Expect(err.Error()).ToNot(ContainSubstring("gpubkey"))

			// Ensure we can query the transaction using the original hash.
			newTxHash, err := gpubkeyStore.UpdatedHash(tx.Hash)
			Expect(err).ToNot(HaveOccurred())
			Expect(newTxHash).To(Equal(newTx.Hash))
		})
	})
})
