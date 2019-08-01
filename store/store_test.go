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
			multiaddrStore := New(kv.NewMemDB())
			size, err := multiaddrStore.Size()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(size).To((Equal(0)))

			expectedSize := rand.Intn(100)
			for i := 0; i < expectedSize; i++ {
				Expect(multiaddrStore.Insert(testutils.RandomMultiAddress())).ShouldNot(HaveOccurred())
				size, err = multiaddrStore.Size()
				Expect(err).ShouldNot(HaveOccurred())
				Expect(size).To((Equal(i + 1)))
			}

			size, err = multiaddrStore.Size()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(size).To((Equal(expectedSize)))
		})

		It("should insert multi-addrs and delete multi-addrs", func() {
			multiaddrStore := New(kv.NewMemDB())

			// We need to increment the size by 1 since zero can be returned by rand.Intn
			// and if we call rand.Intn(0) it will panic
			expectedSize := rand.Intn(100) + 1
			deleteIndex := rand.Intn(expectedSize)

			multiAddr := addr.MultiAddress{}
			for i := 0; i < expectedSize; i++ {
				if i == deleteIndex {
					multiAddr = testutils.RandomMultiAddress()
					Expect(multiaddrStore.Insert(multiAddr)).ShouldNot(HaveOccurred())
					continue
				}
				Expect(multiaddrStore.Insert(testutils.RandomMultiAddress())).ShouldNot(HaveOccurred())
			}
			Expect(multiaddrStore.Delete(multiAddr)).ShouldNot(HaveOccurred())

			size, err := multiaddrStore.Size()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(size).To((Equal(expectedSize - 1)))
		})

		It("should return all multi-addrs on AddrsAll", func() {
			multiaddrStore := New(kv.NewMemDB())
			expectedSize := rand.Intn(100)

			for i := 0; i < expectedSize; i++ {
				Expect(multiaddrStore.Insert(testutils.RandomMultiAddress())).ShouldNot(HaveOccurred())
			}
			addrs, err := multiaddrStore.AddrsAll()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(addrs)).To((Equal(expectedSize)))
		})

		It("should return random multi-addrs on AddrsRandom", func() {
			multiaddrStore := New(kv.NewMemDB())
			expectedSize := rand.Intn(100)

			for i := 0; i < expectedSize; i++ {
				Expect(multiaddrStore.Insert(testutils.RandomMultiAddress())).ShouldNot(HaveOccurred())
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
