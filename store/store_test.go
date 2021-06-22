package store_test

import (
	"fmt"
	"math/rand"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/store"

	"github.com/renproject/aw/wire"
	"github.com/renproject/id"
	"github.com/renproject/kv"
)

func RandomOkAddrValue(r *rand.Rand) string {
	switch r.Int() % 10 {
	case 0:
		return fmt.Sprintf("127.0.0.1:%v", uint16(r.Int()))
	case 1:
		return fmt.Sprintf("0.0.0.0:%v", uint16(r.Int()))
	default:
		return fmt.Sprintf("%v.%v.%v.%v:%v", uint8(r.Int()), uint8(r.Int()), uint8(r.Int()), uint8(r.Int()), uint16(r.Int()))
	}
}

var _ = Describe("Store", func() {
	randomAddress := func(r *rand.Rand) wire.Address {
		a := wire.NewUnsignedAddress(wire.TCP, RandomOkAddrValue(r), r.Uint64())
		e := a.Sign(id.NewPrivKey())
		Expect(e).NotTo(HaveOccurred())
		return a
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

		It("should return random addresses on RandomBootstrapAddrs", func() {
			r := rand.New(rand.NewSource(GinkgoRandomSeed()))
			expectedSize := rand.Intn(100) + 1
			addrs := make([]wire.Address, expectedSize)
			for i := 0; i < expectedSize; i++ {
				addrs[i] = randomAddress(r)
			}
			addrStore := New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "randomaddresses"), addrs)

			addrs, err := addrStore.RandomBootstrapAddrs(expectedSize + 1)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(addrs)).To(Equal(expectedSize))

			randomSize := rand.Intn(expectedSize)
			addrs, err = addrStore.RandomBootstrapAddrs(randomSize)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(addrs)).To(Equal(randomSize))
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
