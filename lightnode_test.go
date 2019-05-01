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
	"github.com/republicprotocol/darknode-go/registry"
	"github.com/republicprotocol/darknode-go/server/jsonrpc"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/sirupsen/logrus"
)

var _ = Describe("light nodes local tests", func() {

	generateConfigs := func(n, k uint64) []darknode.Config {
		signers := testutils.BuildSigners(n, k)
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
				Signer:      signers[i],
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

	initNodes := func(n, k uint64, done chan struct{}, logger logrus.FieldLogger) []peer.MultiAddr {
		configs := generateConfigs(n, k)
		multis := make([]peer.MultiAddr, n)
		for i, config := range configs {
			multis[i] = config.BootstrapMultiAddresses[i]
		}

		go co.ParForAll(configs, func(i int) {
			config := configs[i]
			dnr := registry.NewStatic(config.BootstrapMultiAddresses)
			darknode := darknode.New("1.0", 256, dnr, config, logger.WithField("darknode", i+1))
			darknode.Run(done)
		})

		log.Print("starting all the darknodes...")
		time.Sleep(2 * time.Second)

		return multis
	}

	Context("when sending sendMessageRequest to darknodes through light nodes", func() {
		It("should get response back", func() {
			logger := logrus.New()
			done := make(chan struct{})
			defer close(done)

			bootstrapAddrs := initNodes(8, 6, done, logger)
			lightNode := NewLightnode(logger, 128, 3, 60, "5000", bootstrapAddrs)
			go lightNode.Run(done)

			time.Sleep(5 * time.Second)

			client := jrpc.NewClient(time.Minute)
			request := jsonrpc.JSONRequest{
				JSONRPC: "2.0",
				Version: "1.0",
				Method:  jsonrpc.MethodQueryPeers,
				ID:      "100",
			}
			response, err := client.Call("http://0.0.0.0:5000", request)
			Expect(err).NotTo(HaveOccurred())
			Expect(response.Error).Should(BeNil())

			var resp jsonrpc.QueryPeersResponse
			Expect(json.Unmarshal(response.Result, &resp)).NotTo(HaveOccurred())
			Expect(len(resp.Peers)).Should(BeNumerically(">", 0))
		})
	})
})
