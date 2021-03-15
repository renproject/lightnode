module github.com/renproject/lightnode

go 1.15

require (
	github.com/alicebob/gopher-json v0.0.0-20200520072559-a9ecdc9d1d3a // indirect
	github.com/alicebob/miniredis v2.5.0+incompatible
	github.com/alicebob/miniredis/v2 v2.14.3
	github.com/btcsuite/btcd v0.21.0-beta
	github.com/davidlazar/go-crypto v0.0.0-20200604182044-b73af7476f6c // indirect
	github.com/dgryski/go-farm v0.0.0-20191112170834-c2139c5d712b // indirect
	github.com/ethereum/go-ethereum v1.9.20
	github.com/evalphobia/logrus_sentry v0.8.2
	github.com/filecoin-project/filecoin-ffi v0.30.4-0.20201006125140-a62d00da59d1 // indirect
	github.com/go-redis/redis/v7 v7.2.0
	github.com/gomodule/redigo v2.0.0+incompatible // indirect
	github.com/google/go-cmp v0.5.0
	github.com/jbenet/go-base58 v0.0.0-20150317085156-6237cf65f3a6
	github.com/lib/pq v1.7.0
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/onsi/ginkgo v1.14.0
	github.com/onsi/gomega v1.10.1
	github.com/renproject/aw v0.4.1-0.20210309032102-70551e9ba4f9
	github.com/renproject/darknode v0.5.3-0.20210316044833-07791871541b
	github.com/renproject/hyperdrive v0.4.5 // indirect
	github.com/renproject/id v0.4.2
	github.com/renproject/kv v1.1.2
	github.com/renproject/mercury v0.3.16
	github.com/renproject/multichain v0.2.24
	github.com/renproject/pack v0.2.9
	github.com/renproject/phi v0.1.0
	github.com/renproject/secp256k1 v0.0.0-20210111035356-d34e0ef6ea88 // indirect
	github.com/renproject/wal v0.0.0-20210115002619-70d369bcdde4 // indirect
	github.com/sirupsen/logrus v1.6.0
	github.com/yuin/gopher-lua v0.0.0-20200816102855-ee81675732da // indirect
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20200820211705-5c72a883971a
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b // indirect
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e // indirect
	honnef.co/go/tools v0.0.1-2020.1.3 // indirect
)

replace github.com/cosmos/ledger-cosmos-go => github.com/terra-project/ledger-terra-go v0.11.1-terra

replace github.com/CosmWasm/go-cosmwasm => github.com/terra-project/go-cosmwasm v0.10.1-terra

replace github.com/keybase/go-keychain => github.com/99designs/go-keychain v0.0.0-20191008050251-8e49817e8af4

replace github.com/filecoin-project/filecoin-ffi => ./extern/filecoin-ffi
