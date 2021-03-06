package dispatcher_test

import (
	"context"
	"encoding/json"
	"math/rand"
	"testing/quick"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/dispatcher"
	. "github.com/renproject/lightnode/testutils"

	"github.com/renproject/darknode/jsonrpc"
	"github.com/sirupsen/logrus"
)

var _ = Describe("iterator", func() {
	Context("first successful response iterator", func() {
		It("should return the first success response", func() {
			iter := NewFirstResponseIterator()

			test := func() bool {
				responses := make(chan jsonrpc.Response, 13)
				ctx, cancel := context.WithCancel(context.Background())

				// Simulate piping responses from Darknodes to the channel.
				rs := make([]jsonrpc.Response, 13)
				index := rand.Intn(13) // Index of the Darknode which returns a successful response.
				for i := 0; i < 13; i++ {
					data, err := json.Marshal(i)
					Expect(err).NotTo(HaveOccurred())
					response := RandomResponse(i == index, data)
					rs[i] = response
					responses <- response
				}

				// Collect the response selected by the iterator.
				res := iter.Collect(0.0, cancel, responses)
				Expect(res).Should(Equal(rs[index]))

				// Ensure the context is canceled by the iterator.
				_, ok := <-ctx.Done()
				Expect(ok).Should(BeFalse())
				return len(responses) == (12 - index)
			}

			Expect(quick.Check(test, nil)).NotTo(HaveOccurred())
		})

		It("should return an error if failed to get any successful response from darknode", func() {
			iter := NewFirstResponseIterator()

			test := func() bool {
				responses := make(chan jsonrpc.Response, 13)
				ctx, cancel := context.WithCancel(context.Background())

				// Simulate piping responses from Darknodes to the channel.
				for i := 0; i < 13; i++ {
					response := RandomResponse(false, nil)
					responses <- response
				}
				close(responses)

				// Collect the response selected by the iterator.
				response := iter.Collect(0.0, cancel, responses)
				Expect(response.Error).ShouldNot(BeNil())

				// Ensure the context is canceled by the iterator.
				_, ok := <-ctx.Done()
				Expect(ok).Should(BeFalse())
				return len(responses) == 0
			}

			Expect(quick.Check(test, nil)).NotTo(HaveOccurred())
		})

		It("should return an error if failed to connect to all darknodes", func() {
			iter := NewFirstResponseIterator()

			test := func() bool {
				responses := make(chan jsonrpc.Response, 13)
				_, cancel := context.WithCancel(context.Background())
				close(responses)

				// Collect the response selected by the iterator.
				response := iter.Collect(0.0, cancel, responses)
				return response.Error != nil
			}

			Expect(quick.Check(test, nil)).NotTo(HaveOccurred())
		})
	})

	Context("majority response iterator", func() {
		It("should return the response returned by majority of darknodes", func() {
			iter := NewMajorityResponseIterator(logrus.New())

			test := func() bool {
				responses := make(chan jsonrpc.Response, 13)
				ctx, cancel := context.WithCancel(context.Background())

				// Simulate piping responses from Darknodes to the channel.
				for i := 0; i < 13; i++ {
					if i > 4 {
						data, err := json.Marshal(0)
						Expect(err).NotTo(HaveOccurred())
						responses <- RandomResponse(true, data)
					} else {
						data, err := json.Marshal(i)
						Expect(err).NotTo(HaveOccurred())
						responses <- RandomResponse(true, data)
					}
				}
				close(responses)

				// Collect the response selected by the iterator.
				res := iter.Collect(0.0, cancel, responses)
				Expect(res.Error).Should(BeNil())

				// Ensure the context is canceled by the iterator.
				_, ok := <-ctx.Done()
				Expect(ok).Should(BeFalse())
				return len(responses) == 0
			}

			Expect(quick.Check(test, nil)).NotTo(HaveOccurred())
		})

		It("should return an error when more than 1/3 of the responses are errors", func() {
			iter := NewMajorityResponseIterator(logrus.New())

			test := func() bool {
				responses := make(chan jsonrpc.Response, 13)
				ctx, cancel := context.WithCancel(context.Background())

				// Simulate piping responses from Darknodes to the channel.
				for i := 0; i < 13; i++ {
					response := RandomResponse(false, nil)
					responses <- response
				}

				// Collect the response selected by the iterator.
				res := iter.Collect(0.0, cancel, responses)
				Expect(res.Error).ShouldNot(BeNil())

				// Ensure the context is canceled by the iterator.
				_, ok := <-ctx.Done()
				Expect(ok).Should(BeFalse())
				return true
			}

			Expect(quick.Check(test, nil)).NotTo(HaveOccurred())
		})

		It("should return an error if failed to get any successful response from darknode", func() {
			iter := NewMajorityResponseIterator(logrus.New())

			test := func() bool {
				responses := make(chan jsonrpc.Response, 13)
				_, cancel := context.WithCancel(context.Background())

				close(responses)

				// Collect the response selected by the iterator.
				response := iter.Collect(0.0, cancel, responses)
				return response.Error != nil
			}

			Expect(quick.Check(test, nil)).NotTo(HaveOccurred())
		})
	})
})
