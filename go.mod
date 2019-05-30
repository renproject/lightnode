module github.com/renproject/lightnode

go 1.12

require (
	cloud.google.com/go v0.39.0 // indirect
	github.com/ethereum/go-ethereum v1.8.27
	github.com/evalphobia/logrus_sentry v0.8.2
	github.com/getsentry/raven-go v0.2.0
	github.com/golang/mock v1.3.1 // indirect
	github.com/google/btree v1.0.0 // indirect
	github.com/google/go-cmp v0.3.0 // indirect
	github.com/google/pprof v0.0.0-20190515194954-54271f7e092f // indirect
	github.com/kkdai/bstream v1.0.0 // indirect
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/renproject/kv v0.0.0-20190523055239-db127735289a
	github.com/renproject/libeth-go v0.0.0-20190529111148-e46e5f24e65b // indirect
	github.com/republicprotocol/co-go v0.0.0-20180723052914-4e299fdb0e80
	github.com/republicprotocol/darknode-go v0.0.0-20190529041638-60bea37b2ea3
	github.com/republicprotocol/renp2p-go v0.0.0-20190529035817-2a02dbd91340
	github.com/republicprotocol/tau v0.0.0-20190116001021-54c2ea27fbc3
	github.com/rs/cors v1.6.0
	github.com/sirupsen/logrus v1.4.2
	golang.org/x/exp v0.0.0-20190510132918-efd6b22b2522 // indirect
	golang.org/x/image v0.0.0-20190523035834-f03afa92d3ff // indirect
	golang.org/x/lint v0.0.0-20190409202823-959b441ac422 // indirect
	golang.org/x/mobile v0.0.0-20190509164839-32b2708ab171 // indirect
	golang.org/x/oauth2 v0.0.0-20190523182746-aaccbc9213b0 // indirect
	golang.org/x/sys v0.0.0-20190529164535-6a60838ec259 // indirect
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4
	golang.org/x/tools v0.0.0-20190529203303-fb6c8ffd2207 // indirect
	google.golang.org/appengine v1.6.0 // indirect
	honnef.co/go/tools v0.0.0-20190523083050-ea95bdfd59fc // indirect
)

replace github.com/republicprotocol/darknode-go => ../../republicprotocol/darknode-go
