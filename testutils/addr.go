package testutils

import (
	"crypto/ecdsa"
	cryptorand "crypto/rand"
	"fmt"
	"math/rand"

	"github.com/btcsuite/btcd/btcec"
	"github.com/renproject/darknode/addr"
)

// RandomMultiAddress returns a randomly generated `addr.MultiAddrss` for
// testing.
func RandomMultiAddress() addr.MultiAddress {
	ip4 := fmt.Sprintf("%v.%v.%v.%v", rand.Intn(256), rand.Intn(256), rand.Intn(256), rand.Intn(256))
	tcp := fmt.Sprintf("%v", rand.Intn(8000))

	privateKey, err := ecdsa.GenerateKey(btcec.S256(), cryptorand.Reader)
	if err != nil {
		panic(err)
	}

	address := addr.FromPublicKey(privateKey.PublicKey)
	value := fmt.Sprintf("/ip4/%v/tcp/%v/ren/%s", ip4, tcp, address.ToBase58())
	multiAddr, err := addr.NewMultiAddressFromString(value)
	if err != nil {
		panic(err)
	}
	return multiAddr
}

// NewMultiFromIPAndPort creates a new multi address with the given ip addres
// and port, where the REN address is constant and valid.
func NewMultiFromIPAndPort(ip string, port int) addr.MultiAddress {
	privateKey, err := ecdsa.GenerateKey(btcec.S256(), cryptorand.Reader)
	if err != nil {
		panic(err)
	}

	address := addr.FromPublicKey(privateKey.PublicKey)
	value := fmt.Sprintf("/ren/%s/ip4/%v/tcp/%v", address.ToBase58(), ip, port)
	multi, err := addr.NewMultiAddressFromString(value)
	if err != nil {
		panic("could not create multi address")
	}
	return multi
}
