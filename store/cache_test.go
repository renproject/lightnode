package store_test

import (
	"reflect"
	"testing/quick"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/store"
)

const ExpiredTime = 30

type testStruct struct {
	A string
	B int
	C bool
}

var _ = Describe("Cache implementation of KVStore", func() {
	Context("when reading and writing", func() {
		It("should be able to store int value", func() {
			cache := NewCache(ExpiredTime)
			readAndWrite := func(key string, value int) bool {
				Expect(cache.Write(key, value)).To(Succeed())
				var newValue int
				Expect(cache.Read(key, &newValue)).To(Succeed())
				return reflect.DeepEqual(value, newValue)
			}
			Expect(quick.Check(readAndWrite, nil)).To(Succeed())
		})

		It("should be able to store string value", func() {
			cache := NewCache(ExpiredTime)
			readAndWrite := func(key string, value string) bool {
				Expect(cache.Write(key, value)).To(Succeed())
				var newValue string
				Expect(cache.Read(key, &newValue)).To(Succeed())
				return reflect.DeepEqual(value, newValue)
			}
			Expect(quick.Check(readAndWrite, nil)).To(Succeed())
		})

		It("should be able to store boolean value", func() {
			cache := NewCache(ExpiredTime)
			readAndWrite := func(key string, value bool) bool {
				Expect(cache.Write(key, value)).To(Succeed())
				var newValue bool
				Expect(cache.Read(key, &newValue)).To(Succeed())
				return reflect.DeepEqual(value, newValue)
			}
			Expect(quick.Check(readAndWrite, nil)).To(Succeed())
		})

		It("should iterate through all the key-values in the store", func() {
			cache := NewCache(ExpiredTime)

			readAndWrite := func(key string, value testStruct) bool {
				Expect(cache.Entries()).Should(Equal(0))
				Expect(cache.Write(key, value)).NotTo(HaveOccurred())
				var newValue testStruct
				Expect(cache.Read(key, &newValue)).NotTo(HaveOccurred())
				Expect(reflect.DeepEqual(value, newValue)).Should(BeTrue())
				Expect(cache.Entries()).Should(Equal(1))

				var deletedValue testStruct
				Expect(cache.Delete(key)).NotTo(HaveOccurred())
				Expect(cache.Read(key, &deletedValue)).Should(Equal(ErrKeyNotFound))
				return true
			}
			Expect(quick.Check(readAndWrite, nil)).NotTo(HaveOccurred())
		})
	})

	Context("when iterating the data in the store", func() {
		It("should iterate through all the key-values in the store", func() {
			cache := NewCache(ExpiredTime)
			type testStruct struct {
				A string
				B int
				C bool
			}

			readAndWrite := func(key string, value testStruct) bool {
				Expect(cache.Entries()).Should(Equal(0))
				Expect(cache.Write(key, value)).NotTo(HaveOccurred())
				var newValue testStruct
				Expect(cache.Read(key, &newValue)).NotTo(HaveOccurred())
				Expect(reflect.DeepEqual(value, newValue)).Should(BeTrue())
				Expect(cache.Entries()).Should(Equal(1))

				var deletedValue testStruct
				Expect(cache.Delete(key)).NotTo(HaveOccurred())
				Expect(cache.Read(key, &deletedValue)).Should(Equal(ErrKeyNotFound))
				return true
			}
			Expect(quick.Check(readAndWrite, nil)).NotTo(HaveOccurred())
		})
	})
})
