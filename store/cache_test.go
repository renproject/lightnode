package store_test

import (
	"reflect"
	"testing/quick"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/republicprotocol/dcc/store"
)

const ExpiredTime = 30

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

		It("should be able to store a struct value", func() {
			type testStruct struct {
				A string
				B int
				C bool
			}

			cache := NewCache(ExpiredTime)
			readAndWrite := func(key string, value testStruct) bool {
				Expect(cache.Write(key, value)).NotTo(HaveOccurred())
				var newValue testStruct
				Expect(cache.Read(key, &newValue)).NotTo(HaveOccurred())
				return reflect.DeepEqual(value, newValue)
			}
			Expect(quick.Check(readAndWrite, nil)).NotTo(HaveOccurred())
		})

		It("should be able to store struct value", func() {
			type testStruct struct {
				A string
				B int
				C bool
			}

			cache := NewCache(ExpiredTime)
			readAndWrite := func(key string, value testStruct) bool {
				Expect(cache.Write(key, value)).To(Succeed())
				var newValue testStruct
				Expect(cache.Read(key, &newValue)).To(Succeed())
				return reflect.DeepEqual(value, newValue)
			}
			Expect(quick.Check(readAndWrite, nil)).To(Succeed())
		})
	})

	Context("when using concurrently", func() {
		It("should be safe to do concurrent read and write operations", func() {

		})
	})
})
