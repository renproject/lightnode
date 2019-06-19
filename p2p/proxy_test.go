package p2p_test

import (
	"fmt"
	"reflect"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/p2p"

	"github.com/renproject/kv"
	"github.com/renproject/lightnode/testutils"
	"github.com/republicprotocol/darknode-go/rpc/jsonrpc"
	storeAdapter "github.com/republicprotocol/renp2p-go/adapter/store"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/republicprotocol/renp2p-go/foundation/addr"
)

var _ = Describe("Proxy store", func() {
	initStores := func() (kv.Iterable, kv.Iterable, Proxy) {
		multiStore := kv.NewJSON(kv.NewMemDB())
		statsStore := kv.NewJSON(kv.NewMemDB())
		store := NewProxy(storeAdapter.NewMultiAddrStore(multiStore), statsStore)
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
			Expect(store.InsertMultiAddr(multiAddr)).To(Succeed())

			// Ensure multi-address was added to the multiStore.
			var value peer.MultiAddr
			size, err := multiStore.Size()
			Expect(err).NotTo(HaveOccurred())
			Expect(size).To(Equal(1))
			Expect(multiStore.Get(addr.String(), &value)).To(Succeed())
			Expect(value.Value()).To(Equal(multiAddr.Value()))
		})

		It("should be able to delete multi-addresses", func() {
			multiStore, _, proxy := initStores()

			// Insert multi-address using proxy.
			addr, multiAddr := randAddr()
			Expect(proxy.InsertMultiAddr(multiAddr)).To(Succeed())

			// Delete multi-address using proxy.
			Expect(proxy.DeleteMultiAddr(addr)).To(Succeed())

			// Ensure multi-address was removed from the multiStore.
			var value peer.MultiAddr
			Expect(multiStore.Get(addr.String(), &value)).To(Equal(kv.ErrNotFound))
		})

		It("should be able to retrieve multi-addresses", func() {
			_, _, proxy := initStores()

			// Insert multi-address using proxy.
			addr, multiAddr := randAddr()
			Expect(proxy.InsertMultiAddr(multiAddr)).To(Succeed())

			// Retrieve multi-address using proxy.
			value, err := proxy.MultiAddr(addr)
			Expect(err).ToNot(HaveOccurred())
			Expect(value.Value()).To(Equal(multiAddr.Value()))
		})

		It("should be able to retrieve a list of multi-addresses", func() {
			_, _, proxy := initStores()

			// Insert multi-addresses using proxy.
			_, fstMulti := randAddr()
			Expect(proxy.InsertMultiAddr(fstMulti)).To(Succeed())
			_, sndMulti := randAddr()
			Expect(proxy.InsertMultiAddr(sndMulti)).To(Succeed())

			// Retrieve multi-addresses using proxy.
			values, err := proxy.MultiAddrs()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(values)).To(Equal(2))

			for _, value := range values {
				if value.Value() != fstMulti.Value() && value.Value() != sndMulti.Value() {
					Fail("unknown value")
				}
			}
		})

		It("should be able to retrieve the number of multi-addresses", func() {
			_, _, proxy := initStores()

			// Insert multi-addresses using proxy.
			_, multiAddr := randAddr()
			Expect(proxy.InsertMultiAddr(multiAddr)).To(Succeed())
			_, multiAddr = randAddr()
			Expect(proxy.InsertMultiAddr(multiAddr)).To(Succeed())

			// Retrieve number of multi-addresses using proxy.
			multis, err := proxy.MultiAddrs()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(multis)).To(Equal(2))
		})

		It("should error when retrieving a multi-address that does not exist", func() {
			_, _, store := initStores()

			// Try to retrieve multi-address for an address that does not exist.
			addr, _ := randAddr()
			_, err := store.MultiAddr(addr)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when interacting with stats store through the proxy", func() {
		It("should be able to insert stats", func() {
			_, statsStore, store := initStores()

			// Insert stats using proxy.
			addr, _ := randAddr()
			stats := jsonrpc.QueryStatsResponse{
				RAM:      1,
				Disk:     1,
				Location: "Canberra",
				Version:  "1",
			}
			Expect(store.InsertStats(addr, stats)).To(Succeed())

			// Ensure stats were added to the statsStore.
			var value jsonrpc.QueryStatsResponse
			size, err := statsStore.Size()
			Expect(err).NotTo(HaveOccurred())
			Expect(size).To(Equal(1))
			Expect(statsStore.Get(addr.String(), &value)).To(Succeed())
			Expect(value.Version).To(Equal("1"))
			Expect(value.RAM).To(Equal(1))
			Expect(value.Disk).To(Equal(1))
			Expect(value.Location).To(Equal("Canberra"))
		})

		It("should be able to delete stats", func() {
			_, statsStore, proxy := initStores()

			// Insert stats using proxy.
			addr, _ := randAddr()
			stats := jsonrpc.QueryStatsResponse{}
			Expect(proxy.InsertStats(addr, stats)).To(Succeed())

			// Delete stats using proxy.
			Expect(proxy.DeleteStats(addr)).To(Succeed())

			// Ensure stats were removed from the statsStore.
			var value jsonrpc.QueryStatsResponse
			Expect(statsStore.Get(addr.String(), &value)).To(Equal(kv.ErrNotFound))
		})

		It("should be able to retrieve stats", func() {
			_, _, store := initStores()

			// Insert multi-address using proxy.
			addr, _ := randAddr()
			stats := jsonrpc.QueryStatsResponse{
				RAM:      1,
				Disk:     1,
				Location: "Canberra",
				Version:  "1",
			}
			Expect(store.InsertStats(addr, stats)).To(Succeed())

			// Retrieve stats using proxy.
			value := store.Stats(addr)
			Expect(reflect.DeepEqual(value, stats)).To(BeTrue())
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
