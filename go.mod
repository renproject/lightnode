module github.com/renproject/lightnode

go 1.16

require (
	github.com/alicebob/miniredis/v2 v2.14.3
	github.com/btcsuite/btcd v0.22.0-beta
	github.com/btcsuite/btcutil v1.0.3-0.20201208143702-a53e38424cce
	github.com/cosmos/cosmos-sdk v0.44.0
	github.com/dfuse-io/solana-go v0.2.1-0.20210622202728-1d0a90faa723
	github.com/ethereum/go-ethereum v1.10.7
	github.com/evalphobia/logrus_sentry v0.8.2
	github.com/filecoin-project/go-address v0.0.5
	github.com/go-redis/redis/v7 v7.2.0
	github.com/google/go-cmp v0.5.6
	github.com/jbenet/go-base58 v0.0.0-20150317085156-6237cf65f3a6
	github.com/lib/pq v1.7.0
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/near/borsh-go v0.3.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.10.1
	github.com/renproject/aw v0.5.3
	github.com/renproject/darknode v0.5.3-0.20211207031811-c3433e4f56b6
	github.com/renproject/id v0.4.2
	github.com/renproject/kv v1.1.2
	github.com/renproject/multichain v0.5.2
	github.com/renproject/pack v0.2.11
	github.com/renproject/phi v0.1.0
	github.com/renproject/surge v1.2.6
	github.com/sirupsen/logrus v1.7.0
	github.com/xlab/c-for-go v0.0.0-20201223145653-3ba5db515dcb // indirect
	go.uber.org/zap v1.17.0
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2
	golang.org/x/time v0.0.0-20201208040808-7e3f01d25324
)

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1

replace github.com/filecoin-project/filecoin-ffi => ./extern/filecoin-ffi

replace github.com/renproject/solana-ffi => ./extern/solana-ffi
