package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/renproject/lightnode/test/a/bindings"
	"github.com/renproject/mercury/sdk/client/ethclient"
	"github.com/renproject/mercury/types/ethtypes"
	"github.com/sirupsen/logrus"
)

type RelayTransactionRequest struct {
	EncodedFunction string
	ApprovalData    []byte
	Signature       []byte
	From            common.Address
	To              common.Address
	GasPrice        int64
	GasLimit        int64
	RecipientNonce  int64
	RelayMaxNonce   int64
	RelayFee        int64
	RelayHubAddress common.Address
	UserAgent       string // This field is optional
}

type RelayTransactionRequestMarshalled struct {
	EncodedFunction string
	ApprovalData    []byte
	Signature       []byte
	From            common.Address
	To              common.Address
	GasPrice        big.Int
	GasLimit        big.Int
	RecipientNonce  big.Int
	RelayMaxNonce   big.Int
	RelayFee        big.Int
	RelayHubAddress common.Address
	UserAgent       string // This field is optional
}

type Response struct {
	Nonce    string `json:"nonce"`
	GasPrice string `json:"gasPrice"`
	Gas      string `json:"gas"`
	To       string `json:"to"`
	Value    string `json:"value"`
	Input    string `json:"input"`
	R        string `json:"r"`
	S        string `json:"s"`
	V        string `json:"v"`
}

func main() {

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	callOpts := &bind.CallOpts{Context: ctx}

	// Init ethereum client
	client, err := ethclient.New(logrus.New(), ethtypes.Kovan)
	if err != nil {
		panic(err)
	}
	num, err := client.BlockNumber(ctx)
	if err != nil {
		panic(err)
	}
	log.Printf("current block number = %v", num.Int64())

	// Init the relayHub contract
	hubAddress := common.HexToAddress("0xD216153c06E857cD7f72665E0aF1d7D82172F494")
	relayHub, err := bindings.NewRelayHub(hubAddress, client.EthClient())
	if err != nil {
		panic(err)
	}

	// Init the `From` and `To` address
	fromKey, err := crypto.HexToECDSA("6EBCAEBD464419C0BEFF48D5043A0176E90C5D7FD936F3C485100A7C577AC4CE")
	if err != nil {
		panic(err)
	}
	from := crypto.PubkeyToAddress(fromKey.PublicKey)
	to := common.HexToAddress("0x82c7ba13fb340f3a4e492a8ec0208ed87cb816b0") //kovan
	// to := common.HexToAddress("0x28e1c4bfaba04a5b6967dcd233795bb15fd66269")  //rinkeby

	// Get from nonce
	nonce, err := relayHub.GetNonce(callOpts, from)
	if err != nil {
		panic(err)
	}
	log.Printf("nonce = %v", nonce)

	// Get network gas price and provided gas price
	gasPrice, err := client.EthClient().SuggestGasPrice(ctx)
	if err != nil {
		panic(err)
	}
	gasPrice = big.NewInt(0).Mul(gasPrice, big.NewInt(2))
	log.Printf("gas price = %v ", gasPrice.Int64())

	// Estimate gas limit
	// Todo : the data needs to be changed according to the contract and function to call
	dataString := fmt.Sprintf("e8927fbc000000000000000000000000%v", from.Hex()[2:])
	data, err := hex.DecodeString(dataString)
	if err != nil {
		panic(err)
	}
	msg := ethereum.CallMsg{
		From:     hubAddress,
		To:       &to,
		GasPrice: gasPrice,
		Data:     data,
	}
	gasLimit, err := client.EthClient().EstimateGas(ctx, msg)
	if err != nil {
		panic(err)
	}
	log.Printf("gas limit = %v", gasLimit)

	// Validate the recipient has enough balance
	balance, err := relayHub.BalanceOf(callOpts, to)
	if err != nil {
		panic(err)
	}
	maxCharge, err := relayHub.MaxPossibleCharge(callOpts, big.NewInt(int64(gasLimit)), gasPrice, big.NewInt(0))
	if err != nil {
		panic(err)
	}
	log.Printf("receipt balance = %v, max possible charg = %v", balance, maxCharge)
	if balance.Cmp(big.NewInt(0)) <= 0 || maxCharge.Cmp(balance) == 1 {
		log.Fatalf("validation failed, balance = %v, max possible charge = %v", balance.Int64(), maxCharge.Int64())
	}

	// Find the relay according to our selecting algorithm.
	relayHubAddr := common.HexToAddress("0xD216153c06E857cD7f72665E0aF1d7D82172F494")
	relayURL, relayAddr, err := selectRelay()
	if err != nil {
		panic(err)
	}
	log.Printf("relay addr = %v", relayAddr.Hex())

	// Calculate the hash of the transaction hash
	txFee := uint64(70)
	fn := "e8927fbc"
	hash := getTransactionHash(from, to, relayHubAddr, relayAddr, fn, gasPrice, nonce, txFee, gasLimit)

	// Get max max relay nonce
	maxNonce, err := client.EthClient().NonceAt(ctx, relayAddr, nil)
	if err != nil {
		panic(err)
	}
	log.Print("max nonce = ", maxNonce)

	log.Printf("signature = %x", signature(fromKey, hash))
	request := RelayTransactionRequest{
		EncodedFunction: "0x" + fn,
		ApprovalData:    nil,
		Signature:       signature(fromKey, hash),
		From:            from,
		To:              to,
		GasPrice:        gasPrice.Int64(),
		GasLimit:        int64(gasLimit),
		RecipientNonce:  nonce.Int64(),
		RelayMaxNonce:   int64(maxNonce) + 3,
		RelayFee:        70,
		RelayHubAddress: relayHubAddr,
		UserAgent:       "oz-gsn-provider-0.1.9",
	}
	marshalledData, err := json.Marshal(request)
	if err != nil {
		panic(err)
	}
	log.Printf("request = %v", string(marshalledData))
	buf := bytes.NewBuffer(marshalledData)

	url := relayURL + "/relay"
	log.Print("url = ", url)
	response, err := http.Post(relayURL+"/relay", "application/json", buf)
	if err != nil {
		panic(err)
	}
	log.Printf("response status code = %v", response.StatusCode)

	responseBody ,err := ioutil.ReadAll(response.Body)
	if err != nil {
		panic(err)
	}

	log.Printf("response body = %v", string(responseBody))
}

func selectRelay() (string, common.Address, error) {
	addr := common.HexToAddress("0xda070a7f40fe13923d8144ae32c4a0a459becdee") // kovan
	url := "https://kovan-01.gsn.openzeppelin.org"                            //kovan

	// addr := common.HexToAddress("0x2bd5ad3f1bd8c96463ea834f110251fd1d5fc560") // rinkeby
	// url := "https://rinkeby-02.gsn.openzeppelin.org" //rinkeby

	return url, addr, nil
}

func getTransactionHash(from, to, relayHubAddress, relayAddress common.Address, tx string, gasPrice, nonce *big.Int, txfee, gas_limit uint64) []byte {
	relayPrefix := "726c783a"

	hashStr := relayPrefix +
		strings.TrimPrefix(from.Hex(), "0x") +
		strings.TrimPrefix(to.Hex(), "0x") +
		tx +
		fmt.Sprintf("%064x", txfee) +
		fmt.Sprintf("%064x", gasPrice.Int64()) +
		fmt.Sprintf("%064x", gas_limit) +
		fmt.Sprintf("%064x", nonce.Int64()) +
		strings.TrimPrefix(relayHubAddress.Hex(), "0x") +
		strings.TrimPrefix(relayAddress.Hex(), "0x")
	log.Print("hashStr = ", hashStr)
	data, err := hex.DecodeString(hashStr)
	if err != nil {
		panic(err)
	}
	return crypto.Keccak256(data)
}

func signature(key *ecdsa.PrivateKey, hash []byte) []byte {
	str := "\x19Ethereum Signed Message:\n" + fmt.Sprintf("%v", len(string(hash))) + string(hash)
	hashed := crypto.Keccak256([]byte(str))
	sig, err := crypto.Sign(hashed, key)
	if err != nil {
		panic(err)
	}
	sig[len(sig)-1] += 27
	return sig
}

func test1(key *ecdsa.PrivateKey) {
	from := common.HexToAddress("0x02430a1344936187a2ffdcf61acc4cb56b87aea3")
	to := common.HexToAddress("0x28e1c4bfaba04a5b6967dcd233795bb15fd66269")
	relayHubAddr := common.HexToAddress("0xD216153c06E857cD7f72665E0aF1d7D82172F494")
	relayAddr := common.HexToAddress("0x2bd5ad3f1bd8c96463ea834f110251fd1d5fc560")

	hash := getTransactionHash(from, to, relayHubAddr, relayAddr, "e8927fbc", big.NewInt(1000000000), big.NewInt(2), 70, 30320)
	log.Printf("test hash = %x", hash)
	sig := signature(key, hash)
	log.Printf("test signature = %x", sig)
}

func test(key *ecdsa.PrivateKey) {

	addr := crypto.PubkeyToAddress(key.PublicKey)
	log.Print("addr = ", addr.Hex())

	hash := "0c95ddf6797add6dd1d44cb28293db6cbd64d648c1a59c0c8076d565e925cda8"
	data, err := hex.DecodeString(hash)
	if err != nil {
		panic(err)
	}
	log.Printf("testing signature function = %x", signature(key, data))

	ethSign, err := crypto.Sign(data, key)
	if err != nil {
		panic(err)
	}
	log.Printf("crypto.Sign = %x", ethSign)
	str := "\x19Ethereum Signed Message:\n" + fmt.Sprintf("%v", len(string(data))) + string(data)
	hashed := crypto.Keccak256([]byte(str))
	ethSign, err = crypto.Sign(hashed, key)
	if err != nil {
		panic(err)
	}
	log.Printf("crypto.Sign = %x", ethSign)

	// "6671e7e9876af3b73056ef52dbd3e08ca5067a4d74c54c8c8fe5b04c544f431a4340e550ee64134f89d91a7abfc43e085246f76be3b119aeea258f2a46631bba01"
	// "6671e7e9876af3b73056ef52dbd3e08ca5067a4d74c54c8c8fe5b04c544f431a4340e550ee64134f89d91a7abfc43e085246f76be3b119aeea258f2a46631bba1c"

}
