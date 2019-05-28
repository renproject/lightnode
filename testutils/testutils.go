package testutils

import (
	"crypto/ecdsa"
	cryptorand "crypto/rand"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/renproject/kv"
	storeAdapter "github.com/republicprotocol/renp2p-go/adapter/store"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/republicprotocol/renp2p-go/foundation/addr"
)

func InitStore(multis ...peer.MultiAddr) (peer.MultiAddrStore, error) {
	store := kv.NewJSON(kv.NewMemDB())
	for _, multi := range multis {
		if err := store.Insert(multi.Addr().String(), multi); err != nil {
			return nil, err
		}
	}

	return storeAdapter.NewMultiAddrStore(store), nil
}

func RandomMultiAddress() (peer.MultiAddr, error) {
	addr, err := RandomAddress()
	if err != nil {
		return peer.MultiAddr{}, err
	}
	ip4 := fmt.Sprintf("%v.%v.%v.%v", rand.Intn(256), rand.Intn(256), rand.Intn(256), rand.Intn(256))
	tcp := fmt.Sprintf("%v", rand.Intn(8000))
	value := fmt.Sprintf("/ren/%s/ip4/%v/tcp/%v", addr.String(), ip4, tcp)
	return peer.NewMultiAddr(value, uint64(time.Now().Unix()), [65]byte{})
}

func RandomAddress() (addr.Addr, error) {
	ecdsaPK, err := ecdsa.GenerateKey(secp256k1.S256(), cryptorand.Reader)
	if err != nil {
		return addr.Addr{}, nil
	}
	return addr.FromPublicKey(&ecdsaPK.PublicKey), nil
}

func ServerMultiAddress(server *http.Server) (peer.MultiAddr, error) {
	url := strings.TrimPrefix(server.Addr, "http://")
	address, err := net.ResolveTCPAddr("tcp", url)
	if err != nil {
		return peer.MultiAddr{}, err
	}

	privateKey, err := ecdsa.GenerateKey(secp256k1.S256(), cryptorand.Reader)
	if err != nil {
		return peer.MultiAddr{}, err
	}

	addr := addr.FromPublicKey(&privateKey.PublicKey)
	multi, err := peer.NewMultiAddr(fmt.Sprintf("/ip4/%v/tcp/%v/ren/%v", address.IP, address.Port-1, addr), 1, [65]byte{})
	if err != nil {
		return peer.MultiAddr{}, err
	}

	return multi, nil
}
