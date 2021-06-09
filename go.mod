module github.com/renproject/lightnode

go 1.15

require (
	github.com/alicebob/miniredis/v2 v2.14.3
	github.com/btcsuite/btcd v0.21.0-beta
	github.com/dfuse-io/solana-go v0.2.0
	github.com/dgryski/go-farm v0.0.0-20191112170834-c2139c5d712b // indirect
	github.com/ethereum/go-ethereum v1.10.1
	github.com/evalphobia/logrus_sentry v0.8.2
	github.com/go-redis/redis/v7 v7.2.0
	github.com/google/go-cmp v0.5.2
	github.com/jbenet/go-base58 v0.0.0-20150317085156-6237cf65f3a6
	github.com/lib/pq v1.7.0
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/near/borsh-go v0.3.0
	github.com/onsi/ginkgo v1.14.0
	github.com/onsi/gomega v1.10.1
	github.com/renproject/aw v0.4.1-0.20210604011747-50d6a643dc76
	github.com/renproject/darknode v0.5.3-0.20210607041649-eb88efa30d37
	github.com/renproject/id v0.4.2
	github.com/renproject/kv v1.1.2
	github.com/renproject/multichain v0.3.12
	github.com/renproject/pack v0.2.10
	github.com/renproject/phi v0.1.0
	github.com/renproject/surge v1.2.6
	github.com/sirupsen/logrus v1.7.0
	golang.org/x/crypto v0.0.0-20200820211705-5c72a883971a
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e // indirect
)

replace github.com/cosmos/ledger-cosmos-go => github.com/terra-project/ledger-terra-go v0.11.1-terra

replace github.com/CosmWasm/go-cosmwasm => github.com/terra-project/go-cosmwasm v0.10.1-terra

replace github.com/keybase/go-keychain => github.com/99designs/go-keychain v0.0.0-20191008050251-8e49817e8af4

replace github.com/filecoin-project/filecoin-ffi => ./extern/filecoin-ffi

replace github.com/renproject/solana-ffi => ./extern/solana-ffi
