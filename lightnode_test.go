package lightnode_test

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/renproject/lightnode"

	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	jrpc "github.com/renproject/lightnode/rpc/jsonrpc"
	"github.com/renproject/lightnode/testutils"
	"github.com/republicprotocol/co-go"
	"github.com/republicprotocol/darknode-go"
	"github.com/republicprotocol/darknode-go/abi"
	"github.com/republicprotocol/darknode-go/keystore"
	"github.com/republicprotocol/darknode-go/rpc/jsonrpc"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/sirupsen/logrus"
)

const Version = "1.0"

var _ = Describe("light nodes local tests", func() {

	// generate configs for n darknodes which connects to each other
	generateConfigs := func(n int) ([]darknode.Config, []peer.MultiAddr) {
		configs := make([]darknode.Config, n)
		multiAddrs := make([]peer.MultiAddr, n)

		for i := 0; i < int(n); i++ {
			// Create random ECDSA and RSA keys
			ecdsaKey, err := ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
			Expect(err).NotTo(HaveOccurred())
			rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
			Expect(err).NotTo(HaveOccurred())
			keyStore := keystore.Keystore{
				Rsa:   keystore.Rsa{PrivateKey: rsaKey},
				Ecdsa: keystore.Ecdsa{PrivateKey: ecdsaKey},
			}

			configs[i] = darknode.Config{
				Keystore:    keyStore,
				Address:     keyStore.Address(),
				Host:        "0.0.0.0",
				Port:        fmt.Sprintf("%d", 5500+10*i),
				JSONRPCPort: fmt.Sprintf("%d", 5500+10*i+1),
				Home:        fmt.Sprintf("%s/.darknode_test/darknode_%d", os.Getenv("HOME"), i),
			}

			multiAddr, err := peer.NewMultiAddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%s/ren/%s", fmt.Sprintf("%d", 5500+10*i), keyStore.Address()), 0, [65]byte{})
			Expect(err).NotTo(HaveOccurred())
			multiAddrs[i] = multiAddr
		}

		for i := 0; i < int(n); i++ {
			configs[i].BootstrapMultiAddresses = multiAddrs
		}

		return configs, multiAddrs
	}

	initNodes := func(n int, done chan struct{}, logger logrus.FieldLogger) []peer.MultiAddr {
		configs, multis := generateConfigs(n)

		go co.ParForAll(configs, func(i int) {
			config := configs[i]
			darknode := testutils.NewMockDarknode(config)
			darknode.Run(done)
		})

		log.Print("starting all the darknodes...")
		time.Sleep(1 * time.Second)

		return multis
	}

	testSendMessage := func(client jrpc.Client) {
		data, err := json.Marshal(jsonrpc.SendMessageRequest{
			TxJSON: abi.TxJSON{
				To: "Shifter",
				ArgsJSON: abi.ArgsJSON{
					abi.ArgJSON{
						Name:  "uid",
						Type:  "public",
						Value: []byte("{}"),
					},
					abi.ArgJSON{
						Name:  "commitment",
						Type:  "public",
						Value: []byte("{}"),
					},
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())
		request := jsonrpc.JSONRequest{
			JSONRPC: "2.0",
			Method:  jsonrpc.MethodSendMessage,
			Params:  json.RawMessage(data),
			ID:      "100",
		}
		response, err := client.Call("http://0.0.0.0:5000", request)
		Expect(err).NotTo(HaveOccurred())
		Expect(response.Error).Should(BeNil())

		var resp jsonrpc.SendMessageResponse
		Expect(json.Unmarshal(response.Result, &resp)).To(Succeed())
		Expect(resp.Err()).Should(BeNil())
		Expect(resp.MessageID).ShouldNot(BeEmpty())
	}

	testReceiveMessage := func(client jrpc.Client) {
		data, err := json.Marshal(jsonrpc.ReceiveMessageRequest{
			MessageID: "messageID",
		})
		Expect(err).NotTo(HaveOccurred())
		request := jsonrpc.JSONRequest{
			JSONRPC: "2.0",
			Method:  jsonrpc.MethodReceiveMessage,
			Params:  json.RawMessage(data),
			ID:      "100",
		}
		response, err := client.Call("http://0.0.0.0:5000", request)
		Expect(err).NotTo(HaveOccurred())
		Expect(response.Error).Should(BeNil())

		var resp jsonrpc.ReceiveMessageResponse
		Expect(json.Unmarshal(response.Result, &resp)).To(Succeed())
		Expect(resp.Err()).Should(BeNil())
	}

	testQueryPeers := func(client jrpc.Client) {
		request := jsonrpc.JSONRequest{
			JSONRPC: "2.0",
			Method:  jsonrpc.MethodQueryPeers,
			ID:      "100",
		}
		response, err := client.Call("http://0.0.0.0:5000", request)
		Expect(err).NotTo(HaveOccurred())
		Expect(response.Error).Should(BeNil())

		var resp jsonrpc.QueryPeersResponse
		Expect(json.Unmarshal(response.Result, &resp)).To(Succeed())
		Expect(len(resp.Peers)).Should(BeNumerically(">", 0))
	}

	testQueryNumPeers := func(client jrpc.Client) {
		request := jsonrpc.JSONRequest{
			JSONRPC: "2.0",
			Method:  jsonrpc.MethodQueryNumPeers,
			ID:      "100",
		}
		response, err := client.Call("http://0.0.0.0:5000", request)
		Expect(err).NotTo(HaveOccurred())
		Expect(response.Error).Should(BeNil())

		var resp jsonrpc.QueryNumPeersResponse
		Expect(json.Unmarshal(response.Result, &resp)).To(Succeed())
		Expect(resp.Error).Should(BeNil())
		Expect(resp.NumPeers).Should(Equal(8))
	}

	testQueryStats := func(client jrpc.Client) {
		request := jsonrpc.JSONRequest{
			JSONRPC: "2.0",
			Method:  jsonrpc.MethodQueryStats,
			ID:      "100",
		}
		response, err := client.Call("http://0.0.0.0:5000", request)
		Expect(err).NotTo(HaveOccurred())
		Expect(response.Error).Should(BeNil())

		var resp jsonrpc.QueryStatsResponse
		Expect(json.Unmarshal(response.Result, &resp)).To(Succeed())
		Expect(resp.Error).Should(BeNil())
		Expect(resp.Version).Should(Equal(Version))
		Expect(resp.RAM).Should(BeNumerically(">", 0))
		Expect(resp.Disk).Should(BeNumerically(">", 0))
		Expect(len(resp.CPUs)).Should(BeNumerically(">", 0))
	}

	Context("when querying the light nodes", func() {
		It("should get non-error response", func() {
			logger := logrus.New()
			done := make(chan struct{})
			defer close(done)

			bootstrapAddrs := initNodes(8, done, logger)
			lightnode := New(logger, 128, 3, 60*time.Second, 5*time.Minute, Version, "5000", bootstrapAddrs, 5*time.Minute, 5, 10)
			go lightnode.Run(done)

			client := jrpc.NewClient(logger, time.Minute)

			time.Sleep(1 * time.Second)
			testSendMessage(client)
			testReceiveMessage(client)
			testQueryPeers(client)
			testQueryNumPeers(client)
			testQueryStats(client)
		})
	})
})
