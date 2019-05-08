package testutils

import (
	"crypto/ecdsa"
	"crypto/rand"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/renproject/lightnode/store"
	"github.com/republicprotocol/renp2p-go/core/peer"
	"github.com/republicprotocol/renp2p-go/foundation/addr"
)

func InitStore(multis ...peer.MultiAddr) (store.KVStore, error) {
	store := store.NewCache(0)
	for _, multi := range multis {
		if err := store.Write(multi.Addr().String(), multi); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func RandomAddress() (addr.Addr, error) {
	ecdsaPK, err := ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
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

	privateKey, err := ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
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
