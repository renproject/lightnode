package store_test

import (
	"math/rand"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/store"

	"github.com/renproject/darknode/addr"
	"github.com/renproject/darknode/testutil"
	"github.com/renproject/kv"
)

var _ = Describe("Store", func() {
	Context("when running", func() {
		It("should insert multi-addrs and return the correct size", func() {
			expectedSize := rand.Intn(100)
			multiaddrs := make(addr.MultiAddresses, expectedSize)
			for i := 0; i < expectedSize; i++ {
				multiaddrs[i] = testutil.RandomMultiAddress()
			}
			multiaddrStore := New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses"), nil)
			size, err := multiaddrStore.Size()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(size).Should(BeZero())

			for i := 0; i < expectedSize; i++ {
				Expect(multiaddrStore.Insert(multiaddrs[i])).ShouldNot(HaveOccurred())
				size, err = multiaddrStore.Size()
				Expect(err).ShouldNot(HaveOccurred())
				Expect(size).To(Equal(i + 1))
			}

			size, err = multiaddrStore.Size()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(size).To(Equal(expectedSize))
		})

		It("should insert multi-addrs and delete multi-addrs", func() {
			expectedSize := rand.Intn(100) + 1
			multiaddrs := make(addr.MultiAddresses, expectedSize)
			for i := 0; i < expectedSize; i++ {
				multiaddrs[i] = testutil.RandomMultiAddress()
			}
			multiaddrStore := New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses"), nil)

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
			Expect(size).To(Equal(expectedSize - 1))
		})

		It("should return all multi-addrs on AddrsAll", func() {
			expectedSize := rand.Intn(100) + 1
			multiaddrs := make(addr.MultiAddresses, expectedSize)
			for i := 0; i < expectedSize; i++ {
				multiaddrs[i] = testutil.RandomMultiAddress()
			}
			multiaddrStore := New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses"), nil)

			for i := 0; i < expectedSize; i++ {
				Expect(multiaddrStore.Insert(multiaddrs[i])).ShouldNot(HaveOccurred())
			}
			addrs, err := multiaddrStore.AddrsAll()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(addrs)).To(Equal(expectedSize))
		})

		It("should return random multi-addrs on RandomAddrs", func() {
			expectedSize := rand.Intn(100) + 1
			multiaddrs := make(addr.MultiAddresses, expectedSize)
			for i := 0; i < expectedSize; i++ {
				multiaddrs[i] = testutil.RandomMultiAddress()
			}
			multiaddrStore := New(kv.NewTable(kv.NewMemDB(kv.JSONCodec), "addresses"), nil)

			for i := 0; i < expectedSize; i++ {
				Expect(multiaddrStore.Insert(multiaddrs[i])).ShouldNot(HaveOccurred())
			}
			addrs, err := multiaddrStore.RandomAddrs(expectedSize + 1)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(addrs)).To(Equal(expectedSize))

			randomSize := rand.Intn(expectedSize)
			addrs, err = multiaddrStore.RandomAddrs(randomSize)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(addrs)).To(Equal(randomSize))
		})
	})
})
