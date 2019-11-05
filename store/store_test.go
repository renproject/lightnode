package store_test

import (
	"math/rand"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/store"

	"github.com/renproject/darknode/addr"
	"github.com/renproject/kv"
	"github.com/renproject/lightnode/testutils"
)

var _ = Describe("Store", func() {
	Context("when running", func() {
		It("should insert multi-addrs and return the correct size", func() {
			expectedSize := rand.Intn(100)
			multiaddrs := make(addr.MultiAddresses, expectedSize)
			for i := 0; i < expectedSize; i++ {
				multiaddrs[i] = testutils.RandomMultiAddress()
			}
			multiaddrStore := New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses"), multiaddrs[0])
			size, err := multiaddrStore.Size()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(size).To((Equal(0)))

			for i := 0; i < expectedSize; i++ {
				Expect(multiaddrStore.Insert(multiaddrs[i])).ShouldNot(HaveOccurred())
				size, err = multiaddrStore.Size()
				Expect(err).ShouldNot(HaveOccurred())
				Expect(size).To((Equal(i + 1)))
			}

			size, err = multiaddrStore.Size()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(size).To((Equal(expectedSize)))
		})

		It("should insert multi-addrs and delete multi-addrs", func() {
			expectedSize := rand.Intn(100) + 1
			multiaddrs := make(addr.MultiAddresses, expectedSize)
			for i := 0; i < expectedSize; i++ {
				multiaddrs[i] = testutils.RandomMultiAddress()
			}
			multiaddrStore := New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses"), multiaddrs[0])

			// We need to increment the size by 1 since zero can be returned by rand.Intn
			// and if we call rand.Intn(0) it will panic
			deleteIndex := rand.Intn(expectedSize)

			multiaddr := addr.MultiAddress{}
			for i := 0; i < expectedSize; i++ {
				if i == deleteIndex {
					multiaddr = multiaddrs[i]
					Expect(multiaddrStore.Insert(multiaddr)).ShouldNot(HaveOccurred())
					continue
				}
				Expect(multiaddrStore.Insert(multiaddrs[i])).ShouldNot(HaveOccurred())
			}
			Expect(multiaddrStore.Delete(multiaddr)).ShouldNot(HaveOccurred())

			size, err := multiaddrStore.Size()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(size).To((Equal(expectedSize - 1)))
		})

		It("should return all multi-addrs on AddrsAll", func() {
			expectedSize := rand.Intn(100) + 1
			multiaddrs := make(addr.MultiAddresses, expectedSize)
			for i := 0; i < expectedSize; i++ {
				multiaddrs[i] = testutils.RandomMultiAddress()
			}
			multiaddrStore := New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses"), multiaddrs[0])

			for i := 0; i < expectedSize; i++ {
				Expect(multiaddrStore.Insert(multiaddrs[i])).ShouldNot(HaveOccurred())
			}
			addrs, err := multiaddrStore.AddrsAll()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(addrs)).To((Equal(expectedSize)))
		})

		It("should return random multi-addrs on AddrsRandom", func() {
			expectedSize := rand.Intn(100) + 1
			multiaddrs := make(addr.MultiAddresses, expectedSize)
			for i := 0; i < expectedSize; i++ {
				multiaddrs[i] = testutils.RandomMultiAddress()
			}
			multiaddrStore := New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses"), multiaddrs[0])

			for i := 0; i < expectedSize; i++ {
				Expect(multiaddrStore.Insert(multiaddrs[i])).ShouldNot(HaveOccurred())
			}
			addrs, err := multiaddrStore.AddrsRandom(expectedSize + 1)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(addrs)).To((Equal(expectedSize)))

			randomSize := rand.Intn(expectedSize)
			addrs, err = multiaddrStore.AddrsRandom(randomSize)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(addrs)).To((Equal(randomSize)))
		})
	})
})
