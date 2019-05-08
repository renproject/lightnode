package store_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/store"

	"github.com/renproject/lightnode/testutils"
	"github.com/republicprotocol/darknode-go/server/jsonrpc"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/republicprotocol/renp2p-go/foundation/addr"
)

var _ = Describe("Proxy store", func() {
	initStores := func() (KVStore, KVStore, Proxy) {
		multiStore := NewCache(0)
		statsStore := NewCache(0)
		store := NewProxy(multiStore, statsStore)
		return multiStore, statsStore, store
	}

	randAddr := func() (addr.Addr, peer.MultiAddr) {
		addr, err := testutils.RandomAddress()
		Expect(err).ToNot(HaveOccurred())
		multiAddr, err := peer.NewMultiAddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/5000/ren/%s", addr.String()), 0, [65]byte{})
		Expect(err).ToNot(HaveOccurred())
		return addr, multiAddr
	}

	Context("when interacting with multi-address store through the proxy", func() {
		It("should be able to insert multi-addresses", func() {
			multiStore, _, store := initStores()

			// Insert multi-address using proxy.
			addr, multiAddr := randAddr()
			Expect(store.InsertMultiAddress(addr, multiAddr)).To(Succeed())

			// Ensure multi-address was added to the multiStore.
			var value peer.MultiAddr
			Expect(multiStore.Entries()).To(Equal(1))
			Expect(multiStore.Read(addr.String(), &value)).To(Succeed())
			Expect(value.Value()).To(Equal(multiAddr.Value()))
		})

		It("should be able to delete multi-addresses", func() {
			multiStore, _, store := initStores()

			// Insert multi-address using proxy.
			addr, multiAddr := randAddr()
			Expect(store.InsertMultiAddress(addr, multiAddr)).To(Succeed())

			// Delete multi-address using proxy.
			Expect(store.DeleteMultiAddress(addr)).To(Succeed())

			// Ensure multi-address was removed from the multiStore.
			var value peer.MultiAddr
			Expect(multiStore.Read(addr.String(), &value)).To(Equal(ErrKeyNotFound))
		})

		It("should be able to retrieve multi-addresses", func() {
			_, _, store := initStores()

			// Insert multi-address using proxy.
			addr, multiAddr := randAddr()
			Expect(store.InsertMultiAddress(addr, multiAddr)).To(Succeed())

			// Retrieve multi-address using proxy.
			value, err := store.MultiAddress(addr)
			Expect(err).ToNot(HaveOccurred())
			Expect(value.Value()).To(Equal(multiAddr.Value()))
		})

		It("should be able to retrieve a list of multi-addresses", func() {
			_, _, store := initStores()

			// Insert multi-addresses using proxy.
			fstAddr, fstMulti := randAddr()
			Expect(store.InsertMultiAddress(fstAddr, fstMulti)).To(Succeed())
			sndAddr, sndMulti := randAddr()
			Expect(store.InsertMultiAddress(sndAddr, sndMulti)).To(Succeed())

			// Retrieve multi-addresses using proxy.
			value := store.MultiAddresses()
			Expect(len(value)).To(Equal(2))
			Expect(value[0]).To(Equal(fstMulti.Value()))
			Expect(value[1]).To(Equal(sndMulti.Value()))
		})

		It("should be able to retrieve the number of multi-addresses", func() {
			_, _, store := initStores()

			// Insert multi-addresses using proxy.
			addr, multiAddr := randAddr()
			Expect(store.InsertMultiAddress(addr, multiAddr)).To(Succeed())
			addr, multiAddr = randAddr()
			Expect(store.InsertMultiAddress(addr, multiAddr)).To(Succeed())

			// Retrieve number of multi-addresses using proxy.
			value := store.MultiAddressEntries()
			Expect(value).To(Equal(2))
		})

		It("should error when retrieving a mult-address that does not exist", func() {
			_, _, store := initStores()

			// Try to retrieve multi-address for an address that does not exist.
			addr, _ := randAddr()
			_, err := store.MultiAddress(addr)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when interacting with stats store through the proxy", func() {
		It("should be able to insert stats", func() {
			_, statsStore, store := initStores()

			// Insert stats using proxy.
			addr, _ := randAddr()
			stats := jsonrpc.QueryStatsResponse{
				Location: "Earth",
			}
			Expect(store.InsertStats(addr, stats)).To(Succeed())

			// Ensure stats were added to the statsStore.
			var value jsonrpc.QueryStatsResponse
			Expect(statsStore.Entries()).To(Equal(1))
			Expect(statsStore.Read(addr.String(), &value)).To(Succeed())
			Expect(value.Location).To(Equal(stats.Location))
		})

		It("should be able to delete stats", func() {
			_, statsStore, store := initStores()

			// Insert stats using proxy.
			addr, _ := randAddr()
			stats := jsonrpc.QueryStatsResponse{}
			Expect(store.InsertStats(addr, stats)).To(Succeed())

			// Delete stats using proxy.
			Expect(store.DeleteStats(addr)).To(Succeed())

			// Ensure stats were removed from the statsStore.
			var value jsonrpc.QueryStatsResponse
			Expect(statsStore.Read(addr.String(), &value)).To(Equal(ErrKeyNotFound))
		})

		It("should be able to retrieve stats", func() {
			_, _, store := initStores()

			// Insert multi-address using proxy.
			addr, _ := randAddr()
			stats := jsonrpc.QueryStatsResponse{
				Location: "Earth",
			}
			Expect(store.InsertStats(addr, stats)).To(Succeed())

			// Retrieve stats using proxy.
			value := store.Stats(addr)
			Expect(value.Error).To(BeNil())
			Expect(value.Location).To(Equal(stats.Location))
		})

		It("should error when retrieving stats that do not exist", func() {
			_, _, store := initStores()

			// Try to retrieve stats for an address that does not exist.
			addr, _ := randAddr()
			value := store.Stats(addr)
			Expect(value.Error).To(Equal(ErrInvalidDarknodeAddress))
		})
	})
})
