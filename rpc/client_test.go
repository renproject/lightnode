package rpc_test

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode/rpc"

	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/renproject/lightnode/store"
	"github.com/republicprotocol/darknode-go/processor"
	"github.com/republicprotocol/darknode-go/server/jsonrpc"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/republicprotocol/renp2p-go/foundation/addr"
	"github.com/sirupsen/logrus"
)

var _ = Describe("RPC client", func() {

	initServer := func() *http.Server {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var request jsonrpc.JSONRequest
			Expect(json.NewDecoder(r.Body).Decode(&request)).To(Succeed())

			response := jsonrpc.JSONResponse{
				JSONRPC: "2.0",
				ID:      request.ID,
			}

			switch request.Method {
			case jsonrpc.MethodSendMessage:
				response.Result = json.RawMessage([]byte(`{"messageID":"messageID","ok":true}`))
			case jsonrpc.MethodReceiveMessage:
				response.Result = json.RawMessage([]byte(`{"values":[{"type":"private","value":"0"}]}`))
			default:
				panic("unknown message type")
			}

			time.Sleep(100 * time.Millisecond)
			Expect(json.NewEncoder(w).Encode(response)).To(Succeed())
		})
		server := &http.Server{Addr: "0.0.0.0:18515", Handler: handler}

		go func() {
			defer GinkgoRecover()

			Expect(func() {
				server.ListenAndServe()
			}).NotTo(Panic())
		}()

		return server
	}

	initStore := func(multis ...peer.MultiAddr) store.KVStore {
		store := store.NewCache(0)
		for _, multi := range multis {
			Expect(store.Write(multi.Addr().String(), multi)).NotTo(HaveOccurred())
		}
		return store
	}

	serverMulti := func(server *http.Server) peer.MultiAddr {
		url := strings.TrimPrefix(server.Addr, "http://")
		address, err := net.ResolveTCPAddr("tcp", url)
		Expect(err).NotTo(HaveOccurred())

		privateKey, err := ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
		Expect(err).NotTo(HaveOccurred())

		addr := addr.FromPublicKey(&privateKey.PublicKey)
		multi, err := peer.NewMultiAddr(fmt.Sprintf("/ip4/%v/tcp/%v/ren/%v", address.IP, address.Port, addr), 1, [65]byte{})
		Expect(err).NotTo(HaveOccurred())

		return multi
	}

	// // Construct a test server that always returns errors.
	// initErrorServer := func() *http.Server {
	// 	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	// 		w.WriteHeader(http.StatusInternalServerError)
	// 	})
	// 	server := &http.Server{Addr: "0.0.0.0:18515", Handler: handler}
	//
	// 	return server
	// }

	// Construct a test server that always return a bad-formatted response.
	badResponseServer := func() *http.Server {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := json.NewEncoder(w).Encode(jsonrpc.JSONResponse{
				JSONRPC: "2.0",
				Version: "0.1",
				Result:  []byte(""),
			})
			Expect(err).NotTo(HaveOccurred())
		})
		server := &http.Server{Addr: "0.0.0.0:18515", Handler: handler}

		go func() {
			defer GinkgoRecover()

			Expect(func() {
				server.ListenAndServe()
			}).NotTo(Panic())
		}()

		return server
	}

	Context("when we receive an InvokeRPC message", func() {
		It("should get a response from the server for a SendMessageRequest", func() {
			// Intialise Darknode.
			done := make(chan struct{})
			defer close(done)
			server := initServer()
			defer server.Close()
			multi := serverMulti(server)
			store := initStore(multi)

			// Initialise the client task.
			logger := logrus.New()
			client := NewClient(logger, 32, 8, time.Second, store)
			go client.Run(done)
			responder := make(chan jsonrpc.Response, 1)

			// Send a request to the task.
			for i := 0; i < 32; i++ {
				client.IO().InputWriter() <- InvokeRPC{
					Request: jsonrpc.SendMessageRequest{
						Responder: responder,
					},
					Addresses: []addr.Addr{multi.Addr()},
				}

				// Expect to receive a response from the responder channel.
				select {
				case response := <-responder:
					resp, ok := response.(jsonrpc.SendMessageResponse)
					Expect(ok).To(BeTrue())
					Expect(resp.Ok).To(BeTrue())
				case <-time.After(time.Second):
					Fail("timeout")
				}
			}
		})

		It("should get a response from the server for a SendMessageRequest", func() {
			// Intialise Darknode.
			done := make(chan struct{})
			defer close(done)
			server := initServer()
			defer server.Close()
			multi := serverMulti(server)
			store := initStore(multi)

			// Initialise the client task.
			logger := logrus.New()
			client := NewClient(logger, 32, 8, time.Second, store)
			go client.Run(done)
			responder := make(chan jsonrpc.Response, 1)

			// Send a request to the task.
			for i := 0; i < 32; i++ {
				client.IO().InputWriter() <- InvokeRPC{
					Request: jsonrpc.ReceiveMessageRequest{
						Responder: responder,
					},
					Addresses: []addr.Addr{multi.Addr()},
				}

				// Expect to receive a response from the responder channel.
				select {
				case response := <-responder:
					resp, ok := response.(jsonrpc.ReceiveMessageResponse)
					Expect(ok).To(BeTrue())

					var params []processor.Param
					Expect(json.Unmarshal(resp.Result, &params)).To(Succeed())
					Expect(len(params)).To(Equal(1))
				case <-time.After(time.Second):
					Fail("timeout")
				}
			}
		})
	})

	Context("when the darknode is offline", func() {
		It("should return proper error ", func() {
			// init the server
			done := make(chan struct{})
			defer close(done)
			multi, err := peer.NewMultiAddr("/ip4/0.0.0.0/tcp/18515/ren/8MKXcuQAjR2eEq8bsSHDPkYEmqmjtj", 1, [65]byte{})
			Expect(err).NotTo(HaveOccurred())
			store := initStore(multi)

			// init the client task
			logger := logrus.New()
			client := NewClient(logger, 32, 8, time.Second, store)
			go client.Run(done)
			responder := make(chan jsonrpc.Response, 1)

			// send a message to the task which contains a SendMessageRequest
			client.IO().InputWriter() <- InvokeRPC{
				Request: jsonrpc.SendMessageRequest{
					Responder: responder,
				},
				Addresses: []addr.Addr{multi.Addr()},
			}

			// expect to receive a response from the responder channel
			select {
			case response := <-responder:
				resp, ok := response.(jsonrpc.SendMessageResponse)
				Expect(ok).To(BeTrue())
				Expect(resp.Ok).To(BeFalse())
				Expect(resp.Error).NotTo(BeNil())
			}
		})
	})

	Context("when the darknode gives a bad response", func() {
		It("should return proper error when it's a sendMessage request", func() {
			// init the server
			done := make(chan struct{})
			defer close(done)
			server := badResponseServer()
			defer server.Close()
			multi := serverMulti(server)
			store := initStore(multi)

			// Initialise the client task.
			logger := logrus.New()
			client := NewClient(logger, 32, 8, time.Second, store)
			go client.Run(done)
			responder := make(chan jsonrpc.Response, 1)

			// send a message to the task which contains a SendMessageRequest
			client.IO().InputWriter() <- InvokeRPC{
				Request: jsonrpc.SendMessageRequest{
					Responder: responder,
				},
				Addresses: []addr.Addr{multi.Addr()},
			}

			// expect to receive a response from the responder channel
			select {
			case response := <-responder:
				resp, ok := response.(jsonrpc.SendMessageResponse)
				Expect(ok).To(BeTrue())
				Expect(resp.Ok).To(BeFalse())
				Expect(resp.Error).NotTo(BeNil())
			case <-time.After(time.Second):
				Fail("timeout")
			}
		})

		It("should return proper error when it's a receiveMessage request", func() {
			// init the server
			done := make(chan struct{})
			defer close(done)
			server := badResponseServer()
			defer server.Close()
			multi := serverMulti(server)
			store := initStore(multi)

			// init the client task
			logger := logrus.New()
			client := NewClient(logger, 32, 8, time.Second, store)
			go client.Run(done)
			responder := make(chan jsonrpc.Response, 1)

			// send a message to the task which contains a SendMessageRequest
			client.IO().InputWriter() <- InvokeRPC{
				Request: jsonrpc.ReceiveMessageRequest{
					Responder: responder,
				},
				Addresses: []addr.Addr{multi.Addr()},
			}

			// expect to receive a response from the responder channel
			select {
			case response := <-responder:
				resp, ok := response.(jsonrpc.ReceiveMessageResponse)
				Expect(ok).To(BeTrue())
				Expect(resp.Error).NotTo(BeNil())
			case <-time.After(time.Second):
				Fail("timeout")
			}
		})
	})
})
