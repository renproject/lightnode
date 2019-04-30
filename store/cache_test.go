package store_test

import (
	"math/rand"
	"reflect"
	"testing/quick"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/store"
)

const ExpiredTime = 30

type testStruct struct {
	A string
	B int
	C bool
	D []byte
	E map[string]float64
}

func randomTestStruct(ran *rand.Rand) testStruct {
	t := reflect.TypeOf(testStruct{})
	vaule, _ := quick.Value(t, ran)
	return vaule.Interface().(testStruct)
}

var _ = Describe("Cache implementation of KVStore", func() {
	Context("when reading and writing with data-expiration", func() {
		It("should be able to store a struct with pre-defined value type", func() {
			readAndWrite := func(key string, value testStruct) bool {
				cache := NewCache(ExpiredTime)
				Expect(cache.Entries()).Should(Equal(0))

				var newValue testStruct
				Expect(cache.Read(key, &newValue)).Should(Equal(ErrKeyNotFound))
				Expect(cache.Write(key, value)).NotTo(HaveOccurred())

				Expect(cache.Read(key, &newValue)).NotTo(HaveOccurred())
				Expect(reflect.DeepEqual(value, newValue)).Should(BeTrue())
				Expect(cache.Entries()).Should(Equal(1))

				Expect(cache.Delete(key)).NotTo(HaveOccurred())
				Expect(cache.Read(key, &newValue)).Should(Equal(ErrKeyNotFound))
				Expect(cache.Entries()).Should(Equal(0))
				return true
			}
			Expect(quick.Check(readAndWrite, nil)).NotTo(HaveOccurred())
		})

		It("should be able to return the number of entries in the store ", func() {
			ran := rand.New(rand.NewSource(time.Now().Unix()))

			addingData := func() bool {
				cache := NewCache(ExpiredTime)
				num := rand.Intn(128)
				for i := 0; i < num; i++ {
					value := randomTestStruct(ran)
					value.A = string(i)
					Expect(cache.Write(value.A, value)).NotTo(HaveOccurred())
				}

				return cache.Entries() == num
			}
			Expect(quick.Check(addingData, nil)).NotTo(HaveOccurred())
		})
	})

	Context("when reading and writing without data-expiration", func() {
		It("should be able to store a struct with pre-defined value type", func() {
			readAndWrite := func(key string, value testStruct) bool {
				cache := NewCache(0)
				Expect(cache.Entries()).Should(Equal(0))

				var newValue testStruct
				Expect(cache.Read(key, &newValue)).Should(Equal(ErrKeyNotFound))
				Expect(cache.Write(key, value)).NotTo(HaveOccurred())

				Expect(cache.Read(key, &newValue)).NotTo(HaveOccurred())
				Expect(reflect.DeepEqual(value, newValue)).Should(BeTrue())
				Expect(cache.Entries()).Should(Equal(1))

				Expect(cache.Delete(key)).NotTo(HaveOccurred())
				Expect(cache.Read(key, &newValue)).Should(Equal(ErrKeyNotFound))
				Expect(cache.Entries()).Should(Equal(0))
				return true
			}
			Expect(quick.Check(readAndWrite, nil)).NotTo(HaveOccurred())
		})

		It("should be able to return the number of entries in the store ", func() {
			ran := rand.New(rand.NewSource(time.Now().Unix()))

			addingData := func() bool {
				cache := NewCache(0)
				num := rand.Intn(128)
				for i := 0; i < num; i++ {
					value := randomTestStruct(ran)
					value.A = string(i)
					Expect(cache.Write(value.A, value)).NotTo(HaveOccurred())
				}

				return cache.Entries() == num
			}
			Expect(quick.Check(addingData, nil)).NotTo(HaveOccurred())
		})
	})


	Context("when iterating the data in the store", func() {
		It("should iterate through all the key-values in the store", func() {
			ran := rand.New(rand.NewSource(time.Now().Unix()))

			iterating := func() bool {
				cache := NewCache(ExpiredTime)

				Expect(cache.Entries()).Should(Equal(0))
				num := rand.Intn(128)
				allData := map[string]testStruct{}
				for i := 0; i < num; i++ {
					value := randomTestStruct(ran)
					allData[value.A] = value
					Expect(cache.Write(value.A, value)).NotTo(HaveOccurred())
				}
				Expect(cache.Entries()).Should(Equal(len(allData)))

				iter := cache.Iterator()
				for iter.Next() {
					var wrongType []byte
					_, err := iter.KV(&wrongType)
					Expect(err).To(HaveOccurred())

					var value testStruct
					key, err := iter.KV(&value)
					Expect(err).NotTo(HaveOccurred())
					_, ok := allData[key]
					Expect(ok).Should(BeTrue())
					Expect(cache.Delete(key)).NotTo(HaveOccurred())
					delete(allData, key)
				}

				Expect(cache.Entries()).Should(Equal(0))
				return len(allData) == 0
			}
			Expect(quick.Check(iterating, nil)).NotTo(HaveOccurred())
		})

		It("should return error when there is no next key-value pair", func() {
			iterating := func(key string, value testStruct) bool {
				cache := NewCache(ExpiredTime)
				Expect(cache.Write(key, value)).NotTo(HaveOccurred())
				iter := cache.Iterator()
				for iter.Next() {
				}

				var val testStruct
				key, err := iter.KV(&val)
				return err == ErrNoMoreItems
			}
			Expect(quick.Check(iterating, nil)).NotTo(HaveOccurred())
		})
	})

	Context("when querying data which is expired", func() {
		It("should return ErrDataExpired", func() {
			ran := rand.New(rand.NewSource(time.Now().Unix()))
			value := randomTestStruct(ran)
			cache := NewCache(1)
			Expect(cache.Write(value.A, value)).NotTo(HaveOccurred())

			time.Sleep(2 * time.Second)
			var newValue testStruct
			Expect(cache.Read(value.A, &newValue)).Should(Equal(ErrDataExpired))
		})
	})

	Context("when giving wrong data type of the value", func() {
		It("should return an error", func() {
			wrongType := func(key string, value testStruct) bool {
				cache := NewCache(ExpiredTime)
				Expect(cache.Write(value.A, value)).NotTo(HaveOccurred())

				var wrongType []byte
				return cache.Read(value.A, &wrongType) != nil
			}
			Expect(quick.Check(wrongType, nil)).NotTo(HaveOccurred())
		})
	})

	Context("when trying to store some data which is no marshalable", func() {
		It("should fail and return an error", func() {
			key, value := "key", make(chan struct{})
			cache := NewCache(ExpiredTime)
			Expect(cache.Write(key, value)).To(HaveOccurred())
		})
	})
})
