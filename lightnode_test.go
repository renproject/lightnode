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
	"github.com/republicprotocol/darknode-go/crypter"
	"github.com/republicprotocol/darknode-go/server/jsonrpc"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/sirupsen/logrus"
)

var _ = Describe("light nodes local tests", func() {

	generateConfigs := func(n int) []darknode.Config {
		configs := make([]darknode.Config, n)
		multiAddrs := make([]peer.MultiAddr, n)

		for i := 0; i < int(n); i++ {
			ecdsaKey, err := ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
			Expect(err).NotTo(HaveOccurred())
			rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
			Expect(err).NotTo(HaveOccurred())
			keyStore := darknode.Keystore{
				Rsa:   crypter.RsaKey{PrivateKey: rsaKey},
				Ecdsa: crypter.EcdsaKey{PrivateKey: ecdsaKey},
			}

			configs[i] = darknode.Config{
				Keystore:    keyStore,
				Address:     keyStore.Address(),
				Host:        "0.0.0.0",
				Port:        fmt.Sprintf("%d", 6000+2*i),
				JSONRPCPort: fmt.Sprintf("%d", 6000+2*i+1),
				Home:        fmt.Sprintf("%s/.darknode_test/darknode_%d", os.Getenv("HOME"), i),
			}

			multiAddr, err := peer.NewMultiAddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%s/ren/%s", fmt.Sprintf("%d", 6000+2*i), keyStore.Address()), 0, [65]byte{})
			Expect(err).NotTo(HaveOccurred())
			multiAddrs[i] = multiAddr
		}

		for i := 0; i < int(n); i++ {
			configs[i].BootstrapMultiAddresses = multiAddrs
		}

		return configs
	}

	initNodes := func(n int, done chan struct{}, logger logrus.FieldLogger) []peer.MultiAddr {
		configs := generateConfigs(n)
		multis := make([]peer.MultiAddr, n)
		for i, config := range configs {
			multis[i] = config.BootstrapMultiAddresses[i]
		}

		go co.ParForAll(configs, func(i int) {
			config := configs[i]
			darknode := testutils.NewMockDarknode(config)
			darknode.Run(done)
		})

		log.Print("starting all the darknodes...")
		time.Sleep(2 * time.Second)

		return multis
	}

	testSendMessage := func() {
		client := jrpc.NewClient(time.Minute)
		data, err := json.Marshal(jsonrpc.SendMessageRequest{
			To:    "WarpGate",
			Nonce: 100,
			Payload: jsonrpc.Payload{
				Method: "MintZBTC",
				Args: json.RawMessage(`[
                {
                    "name": "uid",
                    "type": "public",
                    "value": "567faC43Fb59a490076B4873dCE351f75a7E5b38"
                }
            ]`),
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
		Expect(json.Unmarshal(response.Result, &resp)).NotTo(HaveOccurred())
		Expect(resp.Err()).Should(BeNil())
		Expect(resp.MessageID).ShouldNot(BeEmpty())
	}

	testReceiveMessage := func() {
		client := jrpc.NewClient(time.Minute)
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
		Expect(json.Unmarshal(response.Result, &resp)).NotTo(HaveOccurred())
		Expect(resp.Err()).Should(BeNil())
	}

	testQueryPeers := func() {
		client := jrpc.NewClient(time.Minute)
		request := jsonrpc.JSONRequest{
			JSONRPC: "2.0",
			Method:  jsonrpc.MethodQueryPeers,
			ID:      "100",
		}
		response, err := client.Call("http://0.0.0.0:5000", request)
		Expect(err).NotTo(HaveOccurred())
		Expect(response.Error).Should(BeNil())

		var resp jsonrpc.QueryPeersResponse
		Expect(json.Unmarshal(response.Result, &resp)).NotTo(HaveOccurred())
		Expect(len(resp.Peers)).Should(BeNumerically(">", 0))
	}

	testQueryNumPeers := func() {
		client := jrpc.NewClient(time.Minute)
		request := jsonrpc.JSONRequest{
			JSONRPC: "2.0",
			Method:  jsonrpc.MethodQueryNumPeers,
			ID:      "100",
		}
		response, err := client.Call("http://0.0.0.0:5000", request)
		Expect(err).NotTo(HaveOccurred())
		Expect(response.Error).Should(BeNil())

		var resp jsonrpc.QueryNumPeersResponse
		Expect(json.Unmarshal(response.Result, &resp)).NotTo(HaveOccurred())
		Expect(resp.Error).Should(BeNil())
		Expect(resp.NumPeers).Should(Equal(8))
	}

	Context("when querying the light nodes", func() {
		It("should get non-error response", func() {
			logger := logrus.New()
			done := make(chan struct{})
			defer close(done)

			bootstrapAddrs := initNodes(8, done, logger)
			lightNode := NewLightnode(logger, 128, 3, 60, "5000", bootstrapAddrs)
			go lightNode.Run(done)

			time.Sleep(5 * time.Second)
			testSendMessage()
			testReceiveMessage()
			testQueryPeers()
			testQueryNumPeers()
		})
	})
})
