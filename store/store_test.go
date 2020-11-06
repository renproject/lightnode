package store_test

import (
	"math/rand"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/store"

	"github.com/renproject/aw/wire"
	"github.com/renproject/aw/wire/wireutil"
	"github.com/renproject/id"
	"github.com/renproject/kv"
)

var _ = Describe("Store", func() {
	randomAddress := func(r *rand.Rand) wire.Address {
		return wireutil.NewAddressBuilder(id.NewPrivKey(), r).
			WithProtocol(wire.TCP).
			WithValue(wireutil.RandomOkAddrValue(r)).
			WithNonce(wireutil.RandomAddrNonce(r)).Build()
	}

	Context("when running", func() {
		It("should insert addresses and return the correct size", func() {
			r := rand.New(rand.NewSource(GinkgoRandomSeed()))
			expectedSize := rand.Intn(100)
			addrs := make([]wire.Address, expectedSize)
			for i := 0; i < expectedSize; i++ {
				addrs[i] = randomAddress(r)
			}
			addrStore := New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses"), nil)
			size, err := addrStore.Size()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(size).Should(BeZero())

			for i := 0; i < expectedSize; i++ {
				Expect(addrStore.Insert(addrs[i])).ShouldNot(HaveOccurred())
				size, err = addrStore.Size()
				Expect(err).ShouldNot(HaveOccurred())
				Expect(size).To(Equal(i + 1))
			}

			size, err = addrStore.Size()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(size).To(Equal(expectedSize))
		})

		It("should insert addresses and delete addresses", func() {
			r := rand.New(rand.NewSource(GinkgoRandomSeed()))
			expectedSize := rand.Intn(100) + 1
			addrs := make([]wire.Address, expectedSize)
			for i := 0; i < expectedSize; i++ {
				addrs[i] = randomAddress(r)
			}
			addrStore := New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses"), nil)

			// We need to increment the size by 1 since zero can be returned by rand.Intn
			// and if we call rand.Intn(0) it will panic
			deleteIndex := rand.Intn(expectedSize)

			addr := wire.Address{}
			for i := 0; i < expectedSize; i++ {
				if i == deleteIndex {
					addr = addrs[i]
					Expect(addrStore.Insert(addr)).ShouldNot(HaveOccurred())
					continue
				}
				Expect(addrStore.Insert(addrs[i])).ShouldNot(HaveOccurred())
			}
			Expect(addrStore.Delete(addr)).ShouldNot(HaveOccurred())

			size, err := addrStore.Size()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(size).To(Equal(expectedSize - 1))
		})

		It("should return all addresses on AddrsAll", func() {
			r := rand.New(rand.NewSource(GinkgoRandomSeed()))
			expectedSize := rand.Intn(100) + 1
			addrs := make([]wire.Address, expectedSize)
			for i := 0; i < expectedSize; i++ {
				addrs[i] = randomAddress(r)
			}
			addrStore := New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses"), nil)

			for i := 0; i < expectedSize; i++ {
				Expect(addrStore.Insert(addrs[i])).ShouldNot(HaveOccurred())
			}
			addrs, err := addrStore.AddrsAll()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(addrs)).To(Equal(expectedSize))
		})

		It("should return random addresses on RandomAddrs", func() {
			r := rand.New(rand.NewSource(GinkgoRandomSeed()))
			expectedSize := rand.Intn(100) + 1
			addrs := make([]wire.Address, expectedSize)
			for i := 0; i < expectedSize; i++ {
				addrs[i] = randomAddress(r)
			}
			addrStore := New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses"), nil)

			for i := 0; i < expectedSize; i++ {
				Expect(addrStore.Insert(addrs[i])).ShouldNot(HaveOccurred())
			}
			addrs, err := addrStore.RandomAddrs(expectedSize + 1)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(addrs)).To(Equal(expectedSize))

			randomSize := rand.Intn(expectedSize)
			addrs, err = addrStore.RandomAddrs(randomSize)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(addrs)).To(Equal(randomSize))
		})

		It("should fetch the correct multiaddr for a given signatory", func() {
			r := rand.New(rand.NewSource(GinkgoRandomSeed()))
			expectedSize := rand.Intn(100) + 1
			addrs := make([]wire.Address, expectedSize)
			for i := 0; i < expectedSize; i++ {
				addrs[i] = randomAddress(r)
			}
			addrStore := New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses"), nil)

			for i := 0; i < expectedSize; i++ {
				Expect(addrStore.Insert(addrs[i])).ShouldNot(HaveOccurred())
				signatory, err := addrs[i].Signatory()
				Expect(err).ShouldNot(HaveOccurred())
				fetchedAddr, err := addrStore.Get(signatory.String())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(fetchedAddr).To(Equal(addrs[i]))

			}
			Expect(len(addrs)).To(Equal(expectedSize))
		})
	})
})
