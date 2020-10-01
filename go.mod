module github.com/renproject/lightnode

go 1.13

require (
	github.com/allegro/bigcache v1.2.1 // indirect
	github.com/aristanetworks/goarista v0.0.0-20200310212843-2da4c1f5881b // indirect
	github.com/btcsuite/btcd v0.21.0-beta
	github.com/certifi/gocertifi v0.0.0-20190905060710-a5e0173ced67 // indirect
	github.com/cespare/cp v1.1.1 // indirect
	github.com/deckarep/golang-set v1.7.1 // indirect
	github.com/dgryski/go-farm v0.0.0-20191112170834-c2139c5d712b // indirect
	github.com/etcd-io/bbolt v1.3.3 // indirect
	github.com/ethereum/go-ethereum v1.9.20
	github.com/evalphobia/logrus_sentry v0.8.2
	github.com/filecoin-project/go-amt-ipld v0.0.0-20191205011053-79efc22d6cdc // indirect
	github.com/fjl/memsize v0.0.0-20190710130421-bcb5799ab5e5 // indirect
	github.com/gballet/go-libpcsclite v0.0.0-20191108122812-4678299bea08 // indirect
	github.com/getsentry/raven-go v0.2.0 // indirect
	github.com/go-redis/redis/v7 v7.0.0-beta.5
	github.com/google/go-cmp v0.4.0
	github.com/karalabe/usb v0.0.0-20191104083709-911d15fe12a9 // indirect
	github.com/lib/pq v1.7.0
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/olekukonko/tablewriter v0.0.4 // indirect
	github.com/onsi/ginkgo v1.14.0
	github.com/onsi/gomega v1.10.1
	github.com/pierrec/xxHash v0.1.5 // indirect
	github.com/prometheus/tsdb v0.10.0 // indirect
	github.com/renproject/aw v0.4.0-9
	github.com/renproject/darknode v0.5.3-0.20201001044422-e7b680305ffe
	github.com/renproject/id v0.4.2
	github.com/renproject/kv v1.1.2
	github.com/renproject/multichain v0.2.8-0.20200929114230-302423f836e7
	github.com/renproject/pack v0.2.5
	github.com/renproject/phi v0.1.0
	github.com/rjeczalik/notify v0.9.2 // indirect
	github.com/sirupsen/logrus v1.6.0
	github.com/status-im/keycard-go v0.0.0-20200107115650-f38e9a19958e // indirect
	github.com/stumble/gorocksdb v0.0.3 // indirect
	github.com/tyler-smith/go-bip39 v1.0.2 // indirect
	golang.org/x/crypto v0.0.0-20200820211705-5c72a883971a
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e // indirect
)

replace github.com/cosmos/ledger-cosmos-go => github.com/terra-project/ledger-terra-go v0.11.1-terra

replace github.com/CosmWasm/go-cosmwasm => github.com/terra-project/go-cosmwasm v0.10.1-terra

replace github.com/keybase/go-keychain => github.com/99designs/go-keychain v0.0.0-20191008050251-8e49817e8af4
