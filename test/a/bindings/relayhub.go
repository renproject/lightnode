// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package bindings

import (
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// RelayHubABI is the input ABI used to generate the binding from.
const RelayHubABI = "[{\"constant\":false,\"inputs\":[{\"name\":\"amount\",\"type\":\"uint256\"},{\"name\":\"dest\",\"type\":\"address\"}],\"name\":\"withdraw\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"transactionFee\",\"type\":\"uint256\"},{\"name\":\"url\",\"type\":\"string\"}],\"name\":\"registerRelay\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"relay\",\"type\":\"address\"},{\"name\":\"from\",\"type\":\"address\"},{\"name\":\"to\",\"type\":\"address\"},{\"name\":\"encodedFunction\",\"type\":\"bytes\"},{\"name\":\"transactionFee\",\"type\":\"uint256\"},{\"name\":\"gasPrice\",\"type\":\"uint256\"},{\"name\":\"gasLimit\",\"type\":\"uint256\"},{\"name\":\"nonce\",\"type\":\"uint256\"},{\"name\":\"signature\",\"type\":\"bytes\"},{\"name\":\"approvalData\",\"type\":\"bytes\"}],\"name\":\"canRelay\",\"outputs\":[{\"name\":\"status\",\"type\":\"uint256\"},{\"name\":\"recipientContext\",\"type\":\"bytes\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"recipient\",\"type\":\"address\"},{\"name\":\"encodedFunctionWithFrom\",\"type\":\"bytes\"},{\"name\":\"transactionFee\",\"type\":\"uint256\"},{\"name\":\"gasPrice\",\"type\":\"uint256\"},{\"name\":\"gasLimit\",\"type\":\"uint256\"},{\"name\":\"preChecksGas\",\"type\":\"uint256\"},{\"name\":\"recipientContext\",\"type\":\"bytes\"}],\"name\":\"recipientCallsAtomic\",\"outputs\":[{\"name\":\"\",\"type\":\"uint8\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"from\",\"type\":\"address\"}],\"name\":\"getNonce\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"unsignedTx\",\"type\":\"bytes\"},{\"name\":\"signature\",\"type\":\"bytes\"}],\"name\":\"penalizeIllegalTransaction\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"from\",\"type\":\"address\"},{\"name\":\"recipient\",\"type\":\"address\"},{\"name\":\"encodedFunction\",\"type\":\"bytes\"},{\"name\":\"transactionFee\",\"type\":\"uint256\"},{\"name\":\"gasPrice\",\"type\":\"uint256\"},{\"name\":\"gasLimit\",\"type\":\"uint256\"},{\"name\":\"nonce\",\"type\":\"uint256\"},{\"name\":\"signature\",\"type\":\"bytes\"},{\"name\":\"approvalData\",\"type\":\"bytes\"}],\"name\":\"relayCall\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"version\",\"outputs\":[{\"name\":\"\",\"type\":\"string\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"relayedCallStipend\",\"type\":\"uint256\"}],\"name\":\"requiredGas\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"target\",\"type\":\"address\"}],\"name\":\"balanceOf\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"relay\",\"type\":\"address\"}],\"name\":\"canUnstake\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"relay\",\"type\":\"address\"}],\"name\":\"getRelay\",\"outputs\":[{\"name\":\"totalStake\",\"type\":\"uint256\"},{\"name\":\"unstakeDelay\",\"type\":\"uint256\"},{\"name\":\"unstakeTime\",\"type\":\"uint256\"},{\"name\":\"owner\",\"type\":\"address\"},{\"name\":\"state\",\"type\":\"uint8\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"relayedCallStipend\",\"type\":\"uint256\"},{\"name\":\"gasPrice\",\"type\":\"uint256\"},{\"name\":\"transactionFee\",\"type\":\"uint256\"}],\"name\":\"maxPossibleCharge\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"unsignedTx1\",\"type\":\"bytes\"},{\"name\":\"signature1\",\"type\":\"bytes\"},{\"name\":\"unsignedTx2\",\"type\":\"bytes\"},{\"name\":\"signature2\",\"type\":\"bytes\"}],\"name\":\"penalizeRepeatedNonce\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"target\",\"type\":\"address\"}],\"name\":\"depositFor\",\"outputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"relay\",\"type\":\"address\"},{\"name\":\"unstakeDelay\",\"type\":\"uint256\"}],\"name\":\"stake\",\"outputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"relay\",\"type\":\"address\"}],\"name\":\"removeRelayByOwner\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"relay\",\"type\":\"address\"}],\"name\":\"unstake\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"relay\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"stake\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"unstakeDelay\",\"type\":\"uint256\"}],\"name\":\"Staked\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"relay\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"owner\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"transactionFee\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"stake\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"unstakeDelay\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"url\",\"type\":\"string\"}],\"name\":\"RelayAdded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"relay\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"unstakeTime\",\"type\":\"uint256\"}],\"name\":\"RelayRemoved\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"relay\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"stake\",\"type\":\"uint256\"}],\"name\":\"Unstaked\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"recipient\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"from\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"Deposited\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"account\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"dest\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"Withdrawn\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"relay\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"from\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"to\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"selector\",\"type\":\"bytes4\"},{\"indexed\":false,\"name\":\"reason\",\"type\":\"uint256\"}],\"name\":\"CanRelayFailed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"relay\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"from\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"to\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"selector\",\"type\":\"bytes4\"},{\"indexed\":false,\"name\":\"status\",\"type\":\"uint8\"},{\"indexed\":false,\"name\":\"charge\",\"type\":\"uint256\"}],\"name\":\"TransactionRelayed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"relay\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"Penalized\",\"type\":\"event\"}]"

// RelayHub is an auto generated Go binding around an Ethereum contract.
type RelayHub struct {
	RelayHubCaller     // Read-only binding to the contract
	RelayHubTransactor // Write-only binding to the contract
	RelayHubFilterer   // Log filterer for contract events
}

// RelayHubCaller is an auto generated read-only Go binding around an Ethereum contract.
type RelayHubCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RelayHubTransactor is an auto generated write-only Go binding around an Ethereum contract.
type RelayHubTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RelayHubFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type RelayHubFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RelayHubSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type RelayHubSession struct {
	Contract     *RelayHub         // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// RelayHubCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type RelayHubCallerSession struct {
	Contract *RelayHubCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts   // Call options to use throughout this session
}

// RelayHubTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type RelayHubTransactorSession struct {
	Contract     *RelayHubTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts   // Transaction auth options to use throughout this session
}

// RelayHubRaw is an auto generated low-level Go binding around an Ethereum contract.
type RelayHubRaw struct {
	Contract *RelayHub // Generic contract binding to access the raw methods on
}

// RelayHubCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type RelayHubCallerRaw struct {
	Contract *RelayHubCaller // Generic read-only contract binding to access the raw methods on
}

// RelayHubTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type RelayHubTransactorRaw struct {
	Contract *RelayHubTransactor // Generic write-only contract binding to access the raw methods on
}

// NewRelayHub creates a new instance of RelayHub, bound to a specific deployed contract.
func NewRelayHub(address common.Address, backend bind.ContractBackend) (*RelayHub, error) {
	contract, err := bindRelayHub(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &RelayHub{RelayHubCaller: RelayHubCaller{contract: contract}, RelayHubTransactor: RelayHubTransactor{contract: contract}, RelayHubFilterer: RelayHubFilterer{contract: contract}}, nil
}

// NewRelayHubCaller creates a new read-only instance of RelayHub, bound to a specific deployed contract.
func NewRelayHubCaller(address common.Address, caller bind.ContractCaller) (*RelayHubCaller, error) {
	contract, err := bindRelayHub(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &RelayHubCaller{contract: contract}, nil
}

// NewRelayHubTransactor creates a new write-only instance of RelayHub, bound to a specific deployed contract.
func NewRelayHubTransactor(address common.Address, transactor bind.ContractTransactor) (*RelayHubTransactor, error) {
	contract, err := bindRelayHub(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &RelayHubTransactor{contract: contract}, nil
}

// NewRelayHubFilterer creates a new log filterer instance of RelayHub, bound to a specific deployed contract.
func NewRelayHubFilterer(address common.Address, filterer bind.ContractFilterer) (*RelayHubFilterer, error) {
	contract, err := bindRelayHub(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &RelayHubFilterer{contract: contract}, nil
}

// bindRelayHub binds a generic wrapper to an already deployed contract.
func bindRelayHub(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(RelayHubABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_RelayHub *RelayHubRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _RelayHub.Contract.RelayHubCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_RelayHub *RelayHubRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _RelayHub.Contract.RelayHubTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_RelayHub *RelayHubRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _RelayHub.Contract.RelayHubTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_RelayHub *RelayHubCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _RelayHub.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_RelayHub *RelayHubTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _RelayHub.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_RelayHub *RelayHubTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _RelayHub.Contract.contract.Transact(opts, method, params...)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(target address) constant returns(uint256)
func (_RelayHub *RelayHubCaller) BalanceOf(opts *bind.CallOpts, target common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _RelayHub.contract.Call(opts, out, "balanceOf", target)
	return *ret0, err
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(target address) constant returns(uint256)
func (_RelayHub *RelayHubSession) BalanceOf(target common.Address) (*big.Int, error) {
	return _RelayHub.Contract.BalanceOf(&_RelayHub.CallOpts, target)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(target address) constant returns(uint256)
func (_RelayHub *RelayHubCallerSession) BalanceOf(target common.Address) (*big.Int, error) {
	return _RelayHub.Contract.BalanceOf(&_RelayHub.CallOpts, target)
}

// CanRelay is a free data retrieval call binding the contract method 0x2b601747.
//
// Solidity: function canRelay(relay address, from address, to address, encodedFunction bytes, transactionFee uint256, gasPrice uint256, gasLimit uint256, nonce uint256, signature bytes, approvalData bytes) constant returns(status uint256, recipientContext bytes)
func (_RelayHub *RelayHubCaller) CanRelay(opts *bind.CallOpts, relay common.Address, from common.Address, to common.Address, encodedFunction []byte, transactionFee *big.Int, gasPrice *big.Int, gasLimit *big.Int, nonce *big.Int, signature []byte, approvalData []byte) (struct {
	Status           *big.Int
	RecipientContext []byte
}, error) {
	ret := new(struct {
		Status           *big.Int
		RecipientContext []byte
	})
	out := ret
	err := _RelayHub.contract.Call(opts, out, "canRelay", relay, from, to, encodedFunction, transactionFee, gasPrice, gasLimit, nonce, signature, approvalData)
	return *ret, err
}

// CanRelay is a free data retrieval call binding the contract method 0x2b601747.
//
// Solidity: function canRelay(relay address, from address, to address, encodedFunction bytes, transactionFee uint256, gasPrice uint256, gasLimit uint256, nonce uint256, signature bytes, approvalData bytes) constant returns(status uint256, recipientContext bytes)
func (_RelayHub *RelayHubSession) CanRelay(relay common.Address, from common.Address, to common.Address, encodedFunction []byte, transactionFee *big.Int, gasPrice *big.Int, gasLimit *big.Int, nonce *big.Int, signature []byte, approvalData []byte) (struct {
	Status           *big.Int
	RecipientContext []byte
}, error) {
	return _RelayHub.Contract.CanRelay(&_RelayHub.CallOpts, relay, from, to, encodedFunction, transactionFee, gasPrice, gasLimit, nonce, signature, approvalData)
}

// CanRelay is a free data retrieval call binding the contract method 0x2b601747.
//
// Solidity: function canRelay(relay address, from address, to address, encodedFunction bytes, transactionFee uint256, gasPrice uint256, gasLimit uint256, nonce uint256, signature bytes, approvalData bytes) constant returns(status uint256, recipientContext bytes)
func (_RelayHub *RelayHubCallerSession) CanRelay(relay common.Address, from common.Address, to common.Address, encodedFunction []byte, transactionFee *big.Int, gasPrice *big.Int, gasLimit *big.Int, nonce *big.Int, signature []byte, approvalData []byte) (struct {
	Status           *big.Int
	RecipientContext []byte
}, error) {
	return _RelayHub.Contract.CanRelay(&_RelayHub.CallOpts, relay, from, to, encodedFunction, transactionFee, gasPrice, gasLimit, nonce, signature, approvalData)
}

// CanUnstake is a free data retrieval call binding the contract method 0x85f4498b.
//
// Solidity: function canUnstake(relay address) constant returns(bool)
func (_RelayHub *RelayHubCaller) CanUnstake(opts *bind.CallOpts, relay common.Address) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _RelayHub.contract.Call(opts, out, "canUnstake", relay)
	return *ret0, err
}

// CanUnstake is a free data retrieval call binding the contract method 0x85f4498b.
//
// Solidity: function canUnstake(relay address) constant returns(bool)
func (_RelayHub *RelayHubSession) CanUnstake(relay common.Address) (bool, error) {
	return _RelayHub.Contract.CanUnstake(&_RelayHub.CallOpts, relay)
}

// CanUnstake is a free data retrieval call binding the contract method 0x85f4498b.
//
// Solidity: function canUnstake(relay address) constant returns(bool)
func (_RelayHub *RelayHubCallerSession) CanUnstake(relay common.Address) (bool, error) {
	return _RelayHub.Contract.CanUnstake(&_RelayHub.CallOpts, relay)
}

// GetNonce is a free data retrieval call binding the contract method 0x2d0335ab.
//
// Solidity: function getNonce(from address) constant returns(uint256)
func (_RelayHub *RelayHubCaller) GetNonce(opts *bind.CallOpts, from common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _RelayHub.contract.Call(opts, out, "getNonce", from)
	return *ret0, err
}

// GetNonce is a free data retrieval call binding the contract method 0x2d0335ab.
//
// Solidity: function getNonce(from address) constant returns(uint256)
func (_RelayHub *RelayHubSession) GetNonce(from common.Address) (*big.Int, error) {
	return _RelayHub.Contract.GetNonce(&_RelayHub.CallOpts, from)
}

// GetNonce is a free data retrieval call binding the contract method 0x2d0335ab.
//
// Solidity: function getNonce(from address) constant returns(uint256)
func (_RelayHub *RelayHubCallerSession) GetNonce(from common.Address) (*big.Int, error) {
	return _RelayHub.Contract.GetNonce(&_RelayHub.CallOpts, from)
}

// GetRelay is a free data retrieval call binding the contract method 0x8d851460.
//
// Solidity: function getRelay(relay address) constant returns(totalStake uint256, unstakeDelay uint256, unstakeTime uint256, owner address, state uint8)
func (_RelayHub *RelayHubCaller) GetRelay(opts *bind.CallOpts, relay common.Address) (struct {
	TotalStake   *big.Int
	UnstakeDelay *big.Int
	UnstakeTime  *big.Int
	Owner        common.Address
	State        uint8
}, error) {
	ret := new(struct {
		TotalStake   *big.Int
		UnstakeDelay *big.Int
		UnstakeTime  *big.Int
		Owner        common.Address
		State        uint8
	})
	out := ret
	err := _RelayHub.contract.Call(opts, out, "getRelay", relay)
	return *ret, err
}

// GetRelay is a free data retrieval call binding the contract method 0x8d851460.
//
// Solidity: function getRelay(relay address) constant returns(totalStake uint256, unstakeDelay uint256, unstakeTime uint256, owner address, state uint8)
func (_RelayHub *RelayHubSession) GetRelay(relay common.Address) (struct {
	TotalStake   *big.Int
	UnstakeDelay *big.Int
	UnstakeTime  *big.Int
	Owner        common.Address
	State        uint8
}, error) {
	return _RelayHub.Contract.GetRelay(&_RelayHub.CallOpts, relay)
}

// GetRelay is a free data retrieval call binding the contract method 0x8d851460.
//
// Solidity: function getRelay(relay address) constant returns(totalStake uint256, unstakeDelay uint256, unstakeTime uint256, owner address, state uint8)
func (_RelayHub *RelayHubCallerSession) GetRelay(relay common.Address) (struct {
	TotalStake   *big.Int
	UnstakeDelay *big.Int
	UnstakeTime  *big.Int
	Owner        common.Address
	State        uint8
}, error) {
	return _RelayHub.Contract.GetRelay(&_RelayHub.CallOpts, relay)
}

// MaxPossibleCharge is a free data retrieval call binding the contract method 0xa863f8f9.
//
// Solidity: function maxPossibleCharge(relayedCallStipend uint256, gasPrice uint256, transactionFee uint256) constant returns(uint256)
func (_RelayHub *RelayHubCaller) MaxPossibleCharge(opts *bind.CallOpts, relayedCallStipend *big.Int, gasPrice *big.Int, transactionFee *big.Int) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _RelayHub.contract.Call(opts, out, "maxPossibleCharge", relayedCallStipend, gasPrice, transactionFee)
	return *ret0, err
}

// MaxPossibleCharge is a free data retrieval call binding the contract method 0xa863f8f9.
//
// Solidity: function maxPossibleCharge(relayedCallStipend uint256, gasPrice uint256, transactionFee uint256) constant returns(uint256)
func (_RelayHub *RelayHubSession) MaxPossibleCharge(relayedCallStipend *big.Int, gasPrice *big.Int, transactionFee *big.Int) (*big.Int, error) {
	return _RelayHub.Contract.MaxPossibleCharge(&_RelayHub.CallOpts, relayedCallStipend, gasPrice, transactionFee)
}

// MaxPossibleCharge is a free data retrieval call binding the contract method 0xa863f8f9.
//
// Solidity: function maxPossibleCharge(relayedCallStipend uint256, gasPrice uint256, transactionFee uint256) constant returns(uint256)
func (_RelayHub *RelayHubCallerSession) MaxPossibleCharge(relayedCallStipend *big.Int, gasPrice *big.Int, transactionFee *big.Int) (*big.Int, error) {
	return _RelayHub.Contract.MaxPossibleCharge(&_RelayHub.CallOpts, relayedCallStipend, gasPrice, transactionFee)
}

// RequiredGas is a free data retrieval call binding the contract method 0x6a7d84a4.
//
// Solidity: function requiredGas(relayedCallStipend uint256) constant returns(uint256)
func (_RelayHub *RelayHubCaller) RequiredGas(opts *bind.CallOpts, relayedCallStipend *big.Int) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _RelayHub.contract.Call(opts, out, "requiredGas", relayedCallStipend)
	return *ret0, err
}

// RequiredGas is a free data retrieval call binding the contract method 0x6a7d84a4.
//
// Solidity: function requiredGas(relayedCallStipend uint256) constant returns(uint256)
func (_RelayHub *RelayHubSession) RequiredGas(relayedCallStipend *big.Int) (*big.Int, error) {
	return _RelayHub.Contract.RequiredGas(&_RelayHub.CallOpts, relayedCallStipend)
}

// RequiredGas is a free data retrieval call binding the contract method 0x6a7d84a4.
//
// Solidity: function requiredGas(relayedCallStipend uint256) constant returns(uint256)
func (_RelayHub *RelayHubCallerSession) RequiredGas(relayedCallStipend *big.Int) (*big.Int, error) {
	return _RelayHub.Contract.RequiredGas(&_RelayHub.CallOpts, relayedCallStipend)
}

// Version is a free data retrieval call binding the contract method 0x54fd4d50.
//
// Solidity: function version() constant returns(string)
func (_RelayHub *RelayHubCaller) Version(opts *bind.CallOpts) (string, error) {
	var (
		ret0 = new(string)
	)
	out := ret0
	err := _RelayHub.contract.Call(opts, out, "version")
	return *ret0, err
}

// Version is a free data retrieval call binding the contract method 0x54fd4d50.
//
// Solidity: function version() constant returns(string)
func (_RelayHub *RelayHubSession) Version() (string, error) {
	return _RelayHub.Contract.Version(&_RelayHub.CallOpts)
}

// Version is a free data retrieval call binding the contract method 0x54fd4d50.
//
// Solidity: function version() constant returns(string)
func (_RelayHub *RelayHubCallerSession) Version() (string, error) {
	return _RelayHub.Contract.Version(&_RelayHub.CallOpts)
}

// DepositFor is a paid mutator transaction binding the contract method 0xaa67c919.
//
// Solidity: function depositFor(target address) returns()
func (_RelayHub *RelayHubTransactor) DepositFor(opts *bind.TransactOpts, target common.Address) (*types.Transaction, error) {
	return _RelayHub.contract.Transact(opts, "depositFor", target)
}

// DepositFor is a paid mutator transaction binding the contract method 0xaa67c919.
//
// Solidity: function depositFor(target address) returns()
func (_RelayHub *RelayHubSession) DepositFor(target common.Address) (*types.Transaction, error) {
	return _RelayHub.Contract.DepositFor(&_RelayHub.TransactOpts, target)
}

// DepositFor is a paid mutator transaction binding the contract method 0xaa67c919.
//
// Solidity: function depositFor(target address) returns()
func (_RelayHub *RelayHubTransactorSession) DepositFor(target common.Address) (*types.Transaction, error) {
	return _RelayHub.Contract.DepositFor(&_RelayHub.TransactOpts, target)
}

// PenalizeIllegalTransaction is a paid mutator transaction binding the contract method 0x39002432.
//
// Solidity: function penalizeIllegalTransaction(unsignedTx bytes, signature bytes) returns()
func (_RelayHub *RelayHubTransactor) PenalizeIllegalTransaction(opts *bind.TransactOpts, unsignedTx []byte, signature []byte) (*types.Transaction, error) {
	return _RelayHub.contract.Transact(opts, "penalizeIllegalTransaction", unsignedTx, signature)
}

// PenalizeIllegalTransaction is a paid mutator transaction binding the contract method 0x39002432.
//
// Solidity: function penalizeIllegalTransaction(unsignedTx bytes, signature bytes) returns()
func (_RelayHub *RelayHubSession) PenalizeIllegalTransaction(unsignedTx []byte, signature []byte) (*types.Transaction, error) {
	return _RelayHub.Contract.PenalizeIllegalTransaction(&_RelayHub.TransactOpts, unsignedTx, signature)
}

// PenalizeIllegalTransaction is a paid mutator transaction binding the contract method 0x39002432.
//
// Solidity: function penalizeIllegalTransaction(unsignedTx bytes, signature bytes) returns()
func (_RelayHub *RelayHubTransactorSession) PenalizeIllegalTransaction(unsignedTx []byte, signature []byte) (*types.Transaction, error) {
	return _RelayHub.Contract.PenalizeIllegalTransaction(&_RelayHub.TransactOpts, unsignedTx, signature)
}

// PenalizeRepeatedNonce is a paid mutator transaction binding the contract method 0xa8cd9572.
//
// Solidity: function penalizeRepeatedNonce(unsignedTx1 bytes, signature1 bytes, unsignedTx2 bytes, signature2 bytes) returns()
func (_RelayHub *RelayHubTransactor) PenalizeRepeatedNonce(opts *bind.TransactOpts, unsignedTx1 []byte, signature1 []byte, unsignedTx2 []byte, signature2 []byte) (*types.Transaction, error) {
	return _RelayHub.contract.Transact(opts, "penalizeRepeatedNonce", unsignedTx1, signature1, unsignedTx2, signature2)
}

// PenalizeRepeatedNonce is a paid mutator transaction binding the contract method 0xa8cd9572.
//
// Solidity: function penalizeRepeatedNonce(unsignedTx1 bytes, signature1 bytes, unsignedTx2 bytes, signature2 bytes) returns()
func (_RelayHub *RelayHubSession) PenalizeRepeatedNonce(unsignedTx1 []byte, signature1 []byte, unsignedTx2 []byte, signature2 []byte) (*types.Transaction, error) {
	return _RelayHub.Contract.PenalizeRepeatedNonce(&_RelayHub.TransactOpts, unsignedTx1, signature1, unsignedTx2, signature2)
}

// PenalizeRepeatedNonce is a paid mutator transaction binding the contract method 0xa8cd9572.
//
// Solidity: function penalizeRepeatedNonce(unsignedTx1 bytes, signature1 bytes, unsignedTx2 bytes, signature2 bytes) returns()
func (_RelayHub *RelayHubTransactorSession) PenalizeRepeatedNonce(unsignedTx1 []byte, signature1 []byte, unsignedTx2 []byte, signature2 []byte) (*types.Transaction, error) {
	return _RelayHub.Contract.PenalizeRepeatedNonce(&_RelayHub.TransactOpts, unsignedTx1, signature1, unsignedTx2, signature2)
}

// RecipientCallsAtomic is a paid mutator transaction binding the contract method 0x2ca70eba.
//
// Solidity: function recipientCallsAtomic(recipient address, encodedFunctionWithFrom bytes, transactionFee uint256, gasPrice uint256, gasLimit uint256, preChecksGas uint256, recipientContext bytes) returns(uint8)
func (_RelayHub *RelayHubTransactor) RecipientCallsAtomic(opts *bind.TransactOpts, recipient common.Address, encodedFunctionWithFrom []byte, transactionFee *big.Int, gasPrice *big.Int, gasLimit *big.Int, preChecksGas *big.Int, recipientContext []byte) (*types.Transaction, error) {
	return _RelayHub.contract.Transact(opts, "recipientCallsAtomic", recipient, encodedFunctionWithFrom, transactionFee, gasPrice, gasLimit, preChecksGas, recipientContext)
}

// RecipientCallsAtomic is a paid mutator transaction binding the contract method 0x2ca70eba.
//
// Solidity: function recipientCallsAtomic(recipient address, encodedFunctionWithFrom bytes, transactionFee uint256, gasPrice uint256, gasLimit uint256, preChecksGas uint256, recipientContext bytes) returns(uint8)
func (_RelayHub *RelayHubSession) RecipientCallsAtomic(recipient common.Address, encodedFunctionWithFrom []byte, transactionFee *big.Int, gasPrice *big.Int, gasLimit *big.Int, preChecksGas *big.Int, recipientContext []byte) (*types.Transaction, error) {
	return _RelayHub.Contract.RecipientCallsAtomic(&_RelayHub.TransactOpts, recipient, encodedFunctionWithFrom, transactionFee, gasPrice, gasLimit, preChecksGas, recipientContext)
}

// RecipientCallsAtomic is a paid mutator transaction binding the contract method 0x2ca70eba.
//
// Solidity: function recipientCallsAtomic(recipient address, encodedFunctionWithFrom bytes, transactionFee uint256, gasPrice uint256, gasLimit uint256, preChecksGas uint256, recipientContext bytes) returns(uint8)
func (_RelayHub *RelayHubTransactorSession) RecipientCallsAtomic(recipient common.Address, encodedFunctionWithFrom []byte, transactionFee *big.Int, gasPrice *big.Int, gasLimit *big.Int, preChecksGas *big.Int, recipientContext []byte) (*types.Transaction, error) {
	return _RelayHub.Contract.RecipientCallsAtomic(&_RelayHub.TransactOpts, recipient, encodedFunctionWithFrom, transactionFee, gasPrice, gasLimit, preChecksGas, recipientContext)
}

// RegisterRelay is a paid mutator transaction binding the contract method 0x1166073a.
//
// Solidity: function registerRelay(transactionFee uint256, url string) returns()
func (_RelayHub *RelayHubTransactor) RegisterRelay(opts *bind.TransactOpts, transactionFee *big.Int, url string) (*types.Transaction, error) {
	return _RelayHub.contract.Transact(opts, "registerRelay", transactionFee, url)
}

// RegisterRelay is a paid mutator transaction binding the contract method 0x1166073a.
//
// Solidity: function registerRelay(transactionFee uint256, url string) returns()
func (_RelayHub *RelayHubSession) RegisterRelay(transactionFee *big.Int, url string) (*types.Transaction, error) {
	return _RelayHub.Contract.RegisterRelay(&_RelayHub.TransactOpts, transactionFee, url)
}

// RegisterRelay is a paid mutator transaction binding the contract method 0x1166073a.
//
// Solidity: function registerRelay(transactionFee uint256, url string) returns()
func (_RelayHub *RelayHubTransactorSession) RegisterRelay(transactionFee *big.Int, url string) (*types.Transaction, error) {
	return _RelayHub.Contract.RegisterRelay(&_RelayHub.TransactOpts, transactionFee, url)
}

// RelayCall is a paid mutator transaction binding the contract method 0x405cec67.
//
// Solidity: function relayCall(from address, recipient address, encodedFunction bytes, transactionFee uint256, gasPrice uint256, gasLimit uint256, nonce uint256, signature bytes, approvalData bytes) returns()
func (_RelayHub *RelayHubTransactor) RelayCall(opts *bind.TransactOpts, from common.Address, recipient common.Address, encodedFunction []byte, transactionFee *big.Int, gasPrice *big.Int, gasLimit *big.Int, nonce *big.Int, signature []byte, approvalData []byte) (*types.Transaction, error) {
	return _RelayHub.contract.Transact(opts, "relayCall", from, recipient, encodedFunction, transactionFee, gasPrice, gasLimit, nonce, signature, approvalData)
}

// RelayCall is a paid mutator transaction binding the contract method 0x405cec67.
//
// Solidity: function relayCall(from address, recipient address, encodedFunction bytes, transactionFee uint256, gasPrice uint256, gasLimit uint256, nonce uint256, signature bytes, approvalData bytes) returns()
func (_RelayHub *RelayHubSession) RelayCall(from common.Address, recipient common.Address, encodedFunction []byte, transactionFee *big.Int, gasPrice *big.Int, gasLimit *big.Int, nonce *big.Int, signature []byte, approvalData []byte) (*types.Transaction, error) {
	return _RelayHub.Contract.RelayCall(&_RelayHub.TransactOpts, from, recipient, encodedFunction, transactionFee, gasPrice, gasLimit, nonce, signature, approvalData)
}

// RelayCall is a paid mutator transaction binding the contract method 0x405cec67.
//
// Solidity: function relayCall(from address, recipient address, encodedFunction bytes, transactionFee uint256, gasPrice uint256, gasLimit uint256, nonce uint256, signature bytes, approvalData bytes) returns()
func (_RelayHub *RelayHubTransactorSession) RelayCall(from common.Address, recipient common.Address, encodedFunction []byte, transactionFee *big.Int, gasPrice *big.Int, gasLimit *big.Int, nonce *big.Int, signature []byte, approvalData []byte) (*types.Transaction, error) {
	return _RelayHub.Contract.RelayCall(&_RelayHub.TransactOpts, from, recipient, encodedFunction, transactionFee, gasPrice, gasLimit, nonce, signature, approvalData)
}

// RemoveRelayByOwner is a paid mutator transaction binding the contract method 0xc3e712f2.
//
// Solidity: function removeRelayByOwner(relay address) returns()
func (_RelayHub *RelayHubTransactor) RemoveRelayByOwner(opts *bind.TransactOpts, relay common.Address) (*types.Transaction, error) {
	return _RelayHub.contract.Transact(opts, "removeRelayByOwner", relay)
}

// RemoveRelayByOwner is a paid mutator transaction binding the contract method 0xc3e712f2.
//
// Solidity: function removeRelayByOwner(relay address) returns()
func (_RelayHub *RelayHubSession) RemoveRelayByOwner(relay common.Address) (*types.Transaction, error) {
	return _RelayHub.Contract.RemoveRelayByOwner(&_RelayHub.TransactOpts, relay)
}

// RemoveRelayByOwner is a paid mutator transaction binding the contract method 0xc3e712f2.
//
// Solidity: function removeRelayByOwner(relay address) returns()
func (_RelayHub *RelayHubTransactorSession) RemoveRelayByOwner(relay common.Address) (*types.Transaction, error) {
	return _RelayHub.Contract.RemoveRelayByOwner(&_RelayHub.TransactOpts, relay)
}

// Stake is a paid mutator transaction binding the contract method 0xadc9772e.
//
// Solidity: function stake(relay address, unstakeDelay uint256) returns()
func (_RelayHub *RelayHubTransactor) Stake(opts *bind.TransactOpts, relay common.Address, unstakeDelay *big.Int) (*types.Transaction, error) {
	return _RelayHub.contract.Transact(opts, "stake", relay, unstakeDelay)
}

// Stake is a paid mutator transaction binding the contract method 0xadc9772e.
//
// Solidity: function stake(relay address, unstakeDelay uint256) returns()
func (_RelayHub *RelayHubSession) Stake(relay common.Address, unstakeDelay *big.Int) (*types.Transaction, error) {
	return _RelayHub.Contract.Stake(&_RelayHub.TransactOpts, relay, unstakeDelay)
}

// Stake is a paid mutator transaction binding the contract method 0xadc9772e.
//
// Solidity: function stake(relay address, unstakeDelay uint256) returns()
func (_RelayHub *RelayHubTransactorSession) Stake(relay common.Address, unstakeDelay *big.Int) (*types.Transaction, error) {
	return _RelayHub.Contract.Stake(&_RelayHub.TransactOpts, relay, unstakeDelay)
}

// Unstake is a paid mutator transaction binding the contract method 0xf2888dbb.
//
// Solidity: function unstake(relay address) returns()
func (_RelayHub *RelayHubTransactor) Unstake(opts *bind.TransactOpts, relay common.Address) (*types.Transaction, error) {
	return _RelayHub.contract.Transact(opts, "unstake", relay)
}

// Unstake is a paid mutator transaction binding the contract method 0xf2888dbb.
//
// Solidity: function unstake(relay address) returns()
func (_RelayHub *RelayHubSession) Unstake(relay common.Address) (*types.Transaction, error) {
	return _RelayHub.Contract.Unstake(&_RelayHub.TransactOpts, relay)
}

// Unstake is a paid mutator transaction binding the contract method 0xf2888dbb.
//
// Solidity: function unstake(relay address) returns()
func (_RelayHub *RelayHubTransactorSession) Unstake(relay common.Address) (*types.Transaction, error) {
	return _RelayHub.Contract.Unstake(&_RelayHub.TransactOpts, relay)
}

// Withdraw is a paid mutator transaction binding the contract method 0x00f714ce.
//
// Solidity: function withdraw(amount uint256, dest address) returns()
func (_RelayHub *RelayHubTransactor) Withdraw(opts *bind.TransactOpts, amount *big.Int, dest common.Address) (*types.Transaction, error) {
	return _RelayHub.contract.Transact(opts, "withdraw", amount, dest)
}

// Withdraw is a paid mutator transaction binding the contract method 0x00f714ce.
//
// Solidity: function withdraw(amount uint256, dest address) returns()
func (_RelayHub *RelayHubSession) Withdraw(amount *big.Int, dest common.Address) (*types.Transaction, error) {
	return _RelayHub.Contract.Withdraw(&_RelayHub.TransactOpts, amount, dest)
}

// Withdraw is a paid mutator transaction binding the contract method 0x00f714ce.
//
// Solidity: function withdraw(amount uint256, dest address) returns()
func (_RelayHub *RelayHubTransactorSession) Withdraw(amount *big.Int, dest common.Address) (*types.Transaction, error) {
	return _RelayHub.Contract.Withdraw(&_RelayHub.TransactOpts, amount, dest)
}

// RelayHubCanRelayFailedIterator is returned from FilterCanRelayFailed and is used to iterate over the raw logs and unpacked data for CanRelayFailed events raised by the RelayHub contract.
type RelayHubCanRelayFailedIterator struct {
	Event *RelayHubCanRelayFailed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RelayHubCanRelayFailedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayHubCanRelayFailed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RelayHubCanRelayFailed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RelayHubCanRelayFailedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayHubCanRelayFailedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayHubCanRelayFailed represents a CanRelayFailed event raised by the RelayHub contract.
type RelayHubCanRelayFailed struct {
	Relay    common.Address
	From     common.Address
	To       common.Address
	Selector [4]byte
	Reason   *big.Int
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterCanRelayFailed is a free log retrieval operation binding the contract event 0xafb5afd6d1c2e8ffbfb480e674a169f493ece0b22658d4f4484e7334f0241e22.
//
// Solidity: event CanRelayFailed(relay indexed address, from indexed address, to indexed address, selector bytes4, reason uint256)
func (_RelayHub *RelayHubFilterer) FilterCanRelayFailed(opts *bind.FilterOpts, relay []common.Address, from []common.Address, to []common.Address) (*RelayHubCanRelayFailedIterator, error) {

	var relayRule []interface{}
	for _, relayItem := range relay {
		relayRule = append(relayRule, relayItem)
	}
	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _RelayHub.contract.FilterLogs(opts, "CanRelayFailed", relayRule, fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return &RelayHubCanRelayFailedIterator{contract: _RelayHub.contract, event: "CanRelayFailed", logs: logs, sub: sub}, nil
}

// WatchCanRelayFailed is a free log subscription operation binding the contract event 0xafb5afd6d1c2e8ffbfb480e674a169f493ece0b22658d4f4484e7334f0241e22.
//
// Solidity: event CanRelayFailed(relay indexed address, from indexed address, to indexed address, selector bytes4, reason uint256)
func (_RelayHub *RelayHubFilterer) WatchCanRelayFailed(opts *bind.WatchOpts, sink chan<- *RelayHubCanRelayFailed, relay []common.Address, from []common.Address, to []common.Address) (event.Subscription, error) {

	var relayRule []interface{}
	for _, relayItem := range relay {
		relayRule = append(relayRule, relayItem)
	}
	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _RelayHub.contract.WatchLogs(opts, "CanRelayFailed", relayRule, fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayHubCanRelayFailed)
				if err := _RelayHub.contract.UnpackLog(event, "CanRelayFailed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// RelayHubDepositedIterator is returned from FilterDeposited and is used to iterate over the raw logs and unpacked data for Deposited events raised by the RelayHub contract.
type RelayHubDepositedIterator struct {
	Event *RelayHubDeposited // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RelayHubDepositedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayHubDeposited)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RelayHubDeposited)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RelayHubDepositedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayHubDepositedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayHubDeposited represents a Deposited event raised by the RelayHub contract.
type RelayHubDeposited struct {
	Recipient common.Address
	From      common.Address
	Amount    *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterDeposited is a free log retrieval operation binding the contract event 0x8752a472e571a816aea92eec8dae9baf628e840f4929fbcc2d155e6233ff68a7.
//
// Solidity: event Deposited(recipient indexed address, from indexed address, amount uint256)
func (_RelayHub *RelayHubFilterer) FilterDeposited(opts *bind.FilterOpts, recipient []common.Address, from []common.Address) (*RelayHubDepositedIterator, error) {

	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}
	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}

	logs, sub, err := _RelayHub.contract.FilterLogs(opts, "Deposited", recipientRule, fromRule)
	if err != nil {
		return nil, err
	}
	return &RelayHubDepositedIterator{contract: _RelayHub.contract, event: "Deposited", logs: logs, sub: sub}, nil
}

// WatchDeposited is a free log subscription operation binding the contract event 0x8752a472e571a816aea92eec8dae9baf628e840f4929fbcc2d155e6233ff68a7.
//
// Solidity: event Deposited(recipient indexed address, from indexed address, amount uint256)
func (_RelayHub *RelayHubFilterer) WatchDeposited(opts *bind.WatchOpts, sink chan<- *RelayHubDeposited, recipient []common.Address, from []common.Address) (event.Subscription, error) {

	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}
	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}

	logs, sub, err := _RelayHub.contract.WatchLogs(opts, "Deposited", recipientRule, fromRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayHubDeposited)
				if err := _RelayHub.contract.UnpackLog(event, "Deposited", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// RelayHubPenalizedIterator is returned from FilterPenalized and is used to iterate over the raw logs and unpacked data for Penalized events raised by the RelayHub contract.
type RelayHubPenalizedIterator struct {
	Event *RelayHubPenalized // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RelayHubPenalizedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayHubPenalized)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RelayHubPenalized)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RelayHubPenalizedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayHubPenalizedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayHubPenalized represents a Penalized event raised by the RelayHub contract.
type RelayHubPenalized struct {
	Relay  common.Address
	Sender common.Address
	Amount *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterPenalized is a free log retrieval operation binding the contract event 0xb0595266ccec357806b2691f348b128209f1060a0bda4f5c95f7090730351ff8.
//
// Solidity: event Penalized(relay indexed address, sender address, amount uint256)
func (_RelayHub *RelayHubFilterer) FilterPenalized(opts *bind.FilterOpts, relay []common.Address) (*RelayHubPenalizedIterator, error) {

	var relayRule []interface{}
	for _, relayItem := range relay {
		relayRule = append(relayRule, relayItem)
	}

	logs, sub, err := _RelayHub.contract.FilterLogs(opts, "Penalized", relayRule)
	if err != nil {
		return nil, err
	}
	return &RelayHubPenalizedIterator{contract: _RelayHub.contract, event: "Penalized", logs: logs, sub: sub}, nil
}

// WatchPenalized is a free log subscription operation binding the contract event 0xb0595266ccec357806b2691f348b128209f1060a0bda4f5c95f7090730351ff8.
//
// Solidity: event Penalized(relay indexed address, sender address, amount uint256)
func (_RelayHub *RelayHubFilterer) WatchPenalized(opts *bind.WatchOpts, sink chan<- *RelayHubPenalized, relay []common.Address) (event.Subscription, error) {

	var relayRule []interface{}
	for _, relayItem := range relay {
		relayRule = append(relayRule, relayItem)
	}

	logs, sub, err := _RelayHub.contract.WatchLogs(opts, "Penalized", relayRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayHubPenalized)
				if err := _RelayHub.contract.UnpackLog(event, "Penalized", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// RelayHubRelayAddedIterator is returned from FilterRelayAdded and is used to iterate over the raw logs and unpacked data for RelayAdded events raised by the RelayHub contract.
type RelayHubRelayAddedIterator struct {
	Event *RelayHubRelayAdded // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RelayHubRelayAddedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayHubRelayAdded)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RelayHubRelayAdded)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RelayHubRelayAddedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayHubRelayAddedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayHubRelayAdded represents a RelayAdded event raised by the RelayHub contract.
type RelayHubRelayAdded struct {
	Relay          common.Address
	Owner          common.Address
	TransactionFee *big.Int
	Stake          *big.Int
	UnstakeDelay   *big.Int
	Url            string
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterRelayAdded is a free log retrieval operation binding the contract event 0x85b3ae3aae9d3fcb31142fbd8c3b4722d57825b8edd6e1366e69204afa5a0dfa.
//
// Solidity: event RelayAdded(relay indexed address, owner indexed address, transactionFee uint256, stake uint256, unstakeDelay uint256, url string)
func (_RelayHub *RelayHubFilterer) FilterRelayAdded(opts *bind.FilterOpts, relay []common.Address, owner []common.Address) (*RelayHubRelayAddedIterator, error) {

	var relayRule []interface{}
	for _, relayItem := range relay {
		relayRule = append(relayRule, relayItem)
	}
	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}

	logs, sub, err := _RelayHub.contract.FilterLogs(opts, "RelayAdded", relayRule, ownerRule)
	if err != nil {
		return nil, err
	}
	return &RelayHubRelayAddedIterator{contract: _RelayHub.contract, event: "RelayAdded", logs: logs, sub: sub}, nil
}

// WatchRelayAdded is a free log subscription operation binding the contract event 0x85b3ae3aae9d3fcb31142fbd8c3b4722d57825b8edd6e1366e69204afa5a0dfa.
//
// Solidity: event RelayAdded(relay indexed address, owner indexed address, transactionFee uint256, stake uint256, unstakeDelay uint256, url string)
func (_RelayHub *RelayHubFilterer) WatchRelayAdded(opts *bind.WatchOpts, sink chan<- *RelayHubRelayAdded, relay []common.Address, owner []common.Address) (event.Subscription, error) {

	var relayRule []interface{}
	for _, relayItem := range relay {
		relayRule = append(relayRule, relayItem)
	}
	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}

	logs, sub, err := _RelayHub.contract.WatchLogs(opts, "RelayAdded", relayRule, ownerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayHubRelayAdded)
				if err := _RelayHub.contract.UnpackLog(event, "RelayAdded", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// RelayHubRelayRemovedIterator is returned from FilterRelayRemoved and is used to iterate over the raw logs and unpacked data for RelayRemoved events raised by the RelayHub contract.
type RelayHubRelayRemovedIterator struct {
	Event *RelayHubRelayRemoved // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RelayHubRelayRemovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayHubRelayRemoved)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RelayHubRelayRemoved)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RelayHubRelayRemovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayHubRelayRemovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayHubRelayRemoved represents a RelayRemoved event raised by the RelayHub contract.
type RelayHubRelayRemoved struct {
	Relay       common.Address
	UnstakeTime *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterRelayRemoved is a free log retrieval operation binding the contract event 0x5490afc1d818789c8b3d5d63bce3d2a3327d0bba4efb5a7751f783dc977d7d11.
//
// Solidity: event RelayRemoved(relay indexed address, unstakeTime uint256)
func (_RelayHub *RelayHubFilterer) FilterRelayRemoved(opts *bind.FilterOpts, relay []common.Address) (*RelayHubRelayRemovedIterator, error) {

	var relayRule []interface{}
	for _, relayItem := range relay {
		relayRule = append(relayRule, relayItem)
	}

	logs, sub, err := _RelayHub.contract.FilterLogs(opts, "RelayRemoved", relayRule)
	if err != nil {
		return nil, err
	}
	return &RelayHubRelayRemovedIterator{contract: _RelayHub.contract, event: "RelayRemoved", logs: logs, sub: sub}, nil
}

// WatchRelayRemoved is a free log subscription operation binding the contract event 0x5490afc1d818789c8b3d5d63bce3d2a3327d0bba4efb5a7751f783dc977d7d11.
//
// Solidity: event RelayRemoved(relay indexed address, unstakeTime uint256)
func (_RelayHub *RelayHubFilterer) WatchRelayRemoved(opts *bind.WatchOpts, sink chan<- *RelayHubRelayRemoved, relay []common.Address) (event.Subscription, error) {

	var relayRule []interface{}
	for _, relayItem := range relay {
		relayRule = append(relayRule, relayItem)
	}

	logs, sub, err := _RelayHub.contract.WatchLogs(opts, "RelayRemoved", relayRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayHubRelayRemoved)
				if err := _RelayHub.contract.UnpackLog(event, "RelayRemoved", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// RelayHubStakedIterator is returned from FilterStaked and is used to iterate over the raw logs and unpacked data for Staked events raised by the RelayHub contract.
type RelayHubStakedIterator struct {
	Event *RelayHubStaked // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RelayHubStakedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayHubStaked)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RelayHubStaked)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RelayHubStakedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayHubStakedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayHubStaked represents a Staked event raised by the RelayHub contract.
type RelayHubStaked struct {
	Relay        common.Address
	Stake        *big.Int
	UnstakeDelay *big.Int
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterStaked is a free log retrieval operation binding the contract event 0x1449c6dd7851abc30abf37f57715f492010519147cc2652fbc38202c18a6ee90.
//
// Solidity: event Staked(relay indexed address, stake uint256, unstakeDelay uint256)
func (_RelayHub *RelayHubFilterer) FilterStaked(opts *bind.FilterOpts, relay []common.Address) (*RelayHubStakedIterator, error) {

	var relayRule []interface{}
	for _, relayItem := range relay {
		relayRule = append(relayRule, relayItem)
	}

	logs, sub, err := _RelayHub.contract.FilterLogs(opts, "Staked", relayRule)
	if err != nil {
		return nil, err
	}
	return &RelayHubStakedIterator{contract: _RelayHub.contract, event: "Staked", logs: logs, sub: sub}, nil
}

// WatchStaked is a free log subscription operation binding the contract event 0x1449c6dd7851abc30abf37f57715f492010519147cc2652fbc38202c18a6ee90.
//
// Solidity: event Staked(relay indexed address, stake uint256, unstakeDelay uint256)
func (_RelayHub *RelayHubFilterer) WatchStaked(opts *bind.WatchOpts, sink chan<- *RelayHubStaked, relay []common.Address) (event.Subscription, error) {

	var relayRule []interface{}
	for _, relayItem := range relay {
		relayRule = append(relayRule, relayItem)
	}

	logs, sub, err := _RelayHub.contract.WatchLogs(opts, "Staked", relayRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayHubStaked)
				if err := _RelayHub.contract.UnpackLog(event, "Staked", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// RelayHubTransactionRelayedIterator is returned from FilterTransactionRelayed and is used to iterate over the raw logs and unpacked data for TransactionRelayed events raised by the RelayHub contract.
type RelayHubTransactionRelayedIterator struct {
	Event *RelayHubTransactionRelayed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RelayHubTransactionRelayedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayHubTransactionRelayed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RelayHubTransactionRelayed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RelayHubTransactionRelayedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayHubTransactionRelayedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayHubTransactionRelayed represents a TransactionRelayed event raised by the RelayHub contract.
type RelayHubTransactionRelayed struct {
	Relay    common.Address
	From     common.Address
	To       common.Address
	Selector [4]byte
	Status   uint8
	Charge   *big.Int
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterTransactionRelayed is a free log retrieval operation binding the contract event 0xab74390d395916d9e0006298d47938a5def5d367054dcca78fa6ec84381f3f22.
//
// Solidity: event TransactionRelayed(relay indexed address, from indexed address, to indexed address, selector bytes4, status uint8, charge uint256)
func (_RelayHub *RelayHubFilterer) FilterTransactionRelayed(opts *bind.FilterOpts, relay []common.Address, from []common.Address, to []common.Address) (*RelayHubTransactionRelayedIterator, error) {

	var relayRule []interface{}
	for _, relayItem := range relay {
		relayRule = append(relayRule, relayItem)
	}
	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _RelayHub.contract.FilterLogs(opts, "TransactionRelayed", relayRule, fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return &RelayHubTransactionRelayedIterator{contract: _RelayHub.contract, event: "TransactionRelayed", logs: logs, sub: sub}, nil
}

// WatchTransactionRelayed is a free log subscription operation binding the contract event 0xab74390d395916d9e0006298d47938a5def5d367054dcca78fa6ec84381f3f22.
//
// Solidity: event TransactionRelayed(relay indexed address, from indexed address, to indexed address, selector bytes4, status uint8, charge uint256)
func (_RelayHub *RelayHubFilterer) WatchTransactionRelayed(opts *bind.WatchOpts, sink chan<- *RelayHubTransactionRelayed, relay []common.Address, from []common.Address, to []common.Address) (event.Subscription, error) {

	var relayRule []interface{}
	for _, relayItem := range relay {
		relayRule = append(relayRule, relayItem)
	}
	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _RelayHub.contract.WatchLogs(opts, "TransactionRelayed", relayRule, fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayHubTransactionRelayed)
				if err := _RelayHub.contract.UnpackLog(event, "TransactionRelayed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// RelayHubUnstakedIterator is returned from FilterUnstaked and is used to iterate over the raw logs and unpacked data for Unstaked events raised by the RelayHub contract.
type RelayHubUnstakedIterator struct {
	Event *RelayHubUnstaked // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RelayHubUnstakedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayHubUnstaked)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RelayHubUnstaked)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RelayHubUnstakedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayHubUnstakedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayHubUnstaked represents a Unstaked event raised by the RelayHub contract.
type RelayHubUnstaked struct {
	Relay common.Address
	Stake *big.Int
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterUnstaked is a free log retrieval operation binding the contract event 0x0f5bb82176feb1b5e747e28471aa92156a04d9f3ab9f45f28e2d704232b93f75.
//
// Solidity: event Unstaked(relay indexed address, stake uint256)
func (_RelayHub *RelayHubFilterer) FilterUnstaked(opts *bind.FilterOpts, relay []common.Address) (*RelayHubUnstakedIterator, error) {

	var relayRule []interface{}
	for _, relayItem := range relay {
		relayRule = append(relayRule, relayItem)
	}

	logs, sub, err := _RelayHub.contract.FilterLogs(opts, "Unstaked", relayRule)
	if err != nil {
		return nil, err
	}
	return &RelayHubUnstakedIterator{contract: _RelayHub.contract, event: "Unstaked", logs: logs, sub: sub}, nil
}

// WatchUnstaked is a free log subscription operation binding the contract event 0x0f5bb82176feb1b5e747e28471aa92156a04d9f3ab9f45f28e2d704232b93f75.
//
// Solidity: event Unstaked(relay indexed address, stake uint256)
func (_RelayHub *RelayHubFilterer) WatchUnstaked(opts *bind.WatchOpts, sink chan<- *RelayHubUnstaked, relay []common.Address) (event.Subscription, error) {

	var relayRule []interface{}
	for _, relayItem := range relay {
		relayRule = append(relayRule, relayItem)
	}

	logs, sub, err := _RelayHub.contract.WatchLogs(opts, "Unstaked", relayRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayHubUnstaked)
				if err := _RelayHub.contract.UnpackLog(event, "Unstaked", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// RelayHubWithdrawnIterator is returned from FilterWithdrawn and is used to iterate over the raw logs and unpacked data for Withdrawn events raised by the RelayHub contract.
type RelayHubWithdrawnIterator struct {
	Event *RelayHubWithdrawn // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RelayHubWithdrawnIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RelayHubWithdrawn)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RelayHubWithdrawn)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RelayHubWithdrawnIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RelayHubWithdrawnIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RelayHubWithdrawn represents a Withdrawn event raised by the RelayHub contract.
type RelayHubWithdrawn struct {
	Account common.Address
	Dest    common.Address
	Amount  *big.Int
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterWithdrawn is a free log retrieval operation binding the contract event 0xd1c19fbcd4551a5edfb66d43d2e337c04837afda3482b42bdf569a8fccdae5fb.
//
// Solidity: event Withdrawn(account indexed address, dest indexed address, amount uint256)
func (_RelayHub *RelayHubFilterer) FilterWithdrawn(opts *bind.FilterOpts, account []common.Address, dest []common.Address) (*RelayHubWithdrawnIterator, error) {

	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}
	var destRule []interface{}
	for _, destItem := range dest {
		destRule = append(destRule, destItem)
	}

	logs, sub, err := _RelayHub.contract.FilterLogs(opts, "Withdrawn", accountRule, destRule)
	if err != nil {
		return nil, err
	}
	return &RelayHubWithdrawnIterator{contract: _RelayHub.contract, event: "Withdrawn", logs: logs, sub: sub}, nil
}

// WatchWithdrawn is a free log subscription operation binding the contract event 0xd1c19fbcd4551a5edfb66d43d2e337c04837afda3482b42bdf569a8fccdae5fb.
//
// Solidity: event Withdrawn(account indexed address, dest indexed address, amount uint256)
func (_RelayHub *RelayHubFilterer) WatchWithdrawn(opts *bind.WatchOpts, sink chan<- *RelayHubWithdrawn, account []common.Address, dest []common.Address) (event.Subscription, error) {

	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}
	var destRule []interface{}
	for _, destItem := range dest {
		destRule = append(destRule, destItem)
	}

	logs, sub, err := _RelayHub.contract.WatchLogs(opts, "Withdrawn", accountRule, destRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RelayHubWithdrawn)
				if err := _RelayHub.contract.UnpackLog(event, "Withdrawn", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}
