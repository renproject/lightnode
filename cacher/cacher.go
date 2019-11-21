package cacher

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/renproject/darknode"
	"github.com/renproject/darknode/abi"
	"github.com/renproject/darknode/abi/ethabi"
	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/kv"
	"github.com/renproject/lightnode/db"
	"github.com/renproject/lightnode/server"
	"github.com/renproject/mercury/sdk/client/btcclient"
	"github.com/renproject/mercury/types"
	"github.com/renproject/mercury/types/btctypes"
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/sha3"
)

type CacheLevel uint8

const (
	CacheLevelNil = CacheLevel(0)
	CacheLevelMin = CacheLevel(1)
	CacheLevelMax = CacheLevel(2)
)

// ID is a key for a cached response.
type ID [32]byte

func (id ID) String() string {
	return string(id[:32])
}

// Cacher is a task responsible for caching responses for corresponding
// requests. Upon receiving a request (in the current architecture this request
// comes from the `Validator`) it will check its cache to see if it has a
// cached response. If it does, it will write this immediately as a repsonse,
// otherwise it will forward the request on to the `Dispatcher`. Once the
// `Dispatcher` has a response ready, the `Cacher` will store this response in
// its cache with a key derived from the request, and then pass the repsonse
// along to be given to the client. Currently, idempotent requests are stored
// in a LRU cache, and non-idempotent requests are stored in a TTL cache.
type Cacher struct {
	logger     logrus.FieldLogger
	dispatcher phi.Sender
	network    darknode.Network
	db         db.DB

	minTTLCache kv.Table
	maxTTLCache kv.Table
}

// New constructs a new `Cacher` as a `phi.Task` which can be `Run()`.
func New(ctx context.Context, network darknode.Network, db db.DB, dispatcher phi.Sender, logger logrus.FieldLogger, cap int, minTTL, maxTTL time.Duration, opts phi.Options) phi.Task {
	minTTLCache := kv.NewTTLCache(ctx, kv.NewMemDB(kv.JSONCodec), "responses", minTTL)
	maxTTLCache := kv.NewTTLCache(ctx, kv.NewMemDB(kv.JSONCodec), "responses", maxTTL)
	return phi.New(&Cacher{logger, dispatcher, network, db, minTTLCache, maxTTLCache}, opts)
}

// Handle implements the `phi.Handler` interface.
func (cacher *Cacher) Handle(_ phi.Task, message phi.Message) {
	msg, ok := message.(server.RequestWithResponder)
	if !ok {
		cacher.logger.Panicf("[cacher] unexpected message type %T", message)
	}

	params, err := msg.Request.Params.MarshalJSON()
	if err != nil {
		cacher.logger.Errorf("[cacher] cannot marshal request to json: %v", err)
	}

	data := append(params, []byte(msg.Request.Method)...)
	reqID := hash(data)

	cacheLevel := cacheLevel(msg.Request.Method)
	response, cached := cacher.get(reqID, msg.DarknodeID, cacheLevel)
	if cacheLevel != CacheLevelNil && cached {
		msg.Responder <- response
	} else {
		if err := cacher.storeGHash(msg.Request); err != nil {
			cacher.logger.Errorf("[cacher] cannot store ghash in db: %v", err)
		}
		responder := make(chan jsonrpc.Response, 1)
		cacher.dispatcher.Send(server.RequestWithResponder{
			Request:    msg.Request,
			Responder:  responder,
			DarknodeID: msg.DarknodeID,
		})

		// TODO: What do we do when a second request comes in that is already
		// being fetched at the moment? Currently it will also send it to the
		// dispatcher, which is probably not ideal.
		go func() {
			response := <-responder
			// If we're caching for the maximum amount of time, we should ensure
			// there is no error.
			if cacheLevel == CacheLevelMin || (cacheLevel == CacheLevelMax && response.Error == nil) {
				// TODO: Consider thread safety of insertion.
				cacher.insert(reqID, msg.DarknodeID, response, cacheLevel)
			}
			msg.Responder <- response
		}()
	}
}

func (cacher *Cacher) insert(reqID ID, darknodeID string, response jsonrpc.Response, cacheLevel CacheLevel) {
	id := reqID.String() + darknodeID

	var err error
	switch cacheLevel {
	case CacheLevelMin:
		err = cacher.minTTLCache.Insert(id, response)
	case CacheLevelMax:
		err = cacher.maxTTLCache.Insert(id, response)
	}
	if err != nil {
		cacher.logger.Panicf("[cacher] could not insert response into TTL cache: %v", err)
	}
}

func (cacher *Cacher) get(reqID ID, darknodeID string, cacheLevel CacheLevel) (jsonrpc.Response, bool) {
	id := reqID.String() + darknodeID

	var response jsonrpc.Response
	var err error
	switch cacheLevel {
	case CacheLevelMin:
		err = cacher.minTTLCache.Get(id, &response)
	case CacheLevelMax:
		err = cacher.maxTTLCache.Get(id, &response)
	}
	if err != nil {
		return jsonrpc.Response{}, false
	}
	return response, true
}

// storeGHash is used to calculate and store the gateway hash and UTXO.
func (cacher *Cacher) storeGHash(request jsonrpc.Request) error {
	if request.Method != jsonrpc.MethodSubmitTx {
		return nil
	}
	params := jsonrpc.ParamsSubmitTx{}
	if err := json.Unmarshal(request.Params, &params); err != nil {
		return err
	}

	// Check if the request is to a shift-in contract.
	network, shiftIn := isShiftIn(cacher.network, params.Tx.To)
	if !shiftIn {
		return nil
	}

	// Validate the transaction arguments.
	if err := cacher.validate(network, params.Tx.Args); err != nil {
		return err
	}
	return nil
}

func (cacher *Cacher) validate(network btctypes.Network, args abi.Args) error {
	client := btcclient.NewClient(cacher.logger.WithField("blockchain", "btc"), network)
	utxo := args.Get("utxo").Value.(abi.ExtBtcCompatUTXO)

	// Calculate the gateway hash from the input arguments.
	copy(utxo.GHash[:], crypto.Keccak256(ethabi.Encode(abi.Args{
		args.Get("phash"),
		args.Get("amount"),
		args.Get("token"),
		args.Get("to"),
		args.Get("n"),
	})))

	// Derive the outpoint from the input arguments.
	op := btctypes.NewOutPoint(
		types.TxHash(hex.EncodeToString(utxo.TxHash[:])),
		uint32(utxo.VOut),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Get additional details for a UTXO and ensure they are valid.
	mUTXO, err := client.UTXO(ctx, op)
	if err != nil {
		return err
	}

	// Populate remaining UTXO details.
	utxo.ScriptPubKey = abi.B(mUTXO.ScriptPubKey())
	utxo.Amount = abi.U64(mUTXO.Amount())

	// Insert the UTXO into the database.
	return cacher.db.InsertGateway(utxo)
}

// isShiftIn checks whether the given contract address is for a shift-in.
func isShiftIn(darknodeNet darknode.Network, addr abi.Addr) (btctypes.Network, bool) {
	switch addr {
	case abi.IntrinsicBTC0Btc2Eth.Addr:
		return getBlockchainNetwork(darknodeNet, types.Bitcoin), true
	case abi.IntrinsicBCH0Bch2Eth.Addr:
		return getBlockchainNetwork(darknodeNet, types.BitcoinCash), true
	case abi.IntrinsicZEC0Zec2Eth.Addr:
		return getBlockchainNetwork(darknodeNet, types.ZCash), true
	default:
		return nil, false
	}
}

// getBlockchainNetwork returns the blockchain network RenVM uses for the given
// Darknode network.
func getBlockchainNetwork(darknodeNet darknode.Network, chain types.Chain) btctypes.Network {
	switch darknodeNet {
	case darknode.Chaosnet:
		return btctypes.NewNetwork(chain, "mainnet")
	case darknode.Testnet, darknode.Devnet:
		return btctypes.NewNetwork(chain, "testnet")
	case darknode.Localnet:
		return btctypes.NewNetwork(chain, "localnet")
	default:
		panic(fmt.Sprintf("unsupported network: %v", darknodeNet))
	}
}

func cacheLevel(method string) CacheLevel {
	switch method {
	case jsonrpc.MethodSubmitTx:
		return CacheLevelMax
	case jsonrpc.MethodQueryBlock,
		jsonrpc.MethodQueryBlocks,
		jsonrpc.MethodQueryNumPeers,
		jsonrpc.MethodQueryPeers,
		jsonrpc.MethodQueryEpoch,
		jsonrpc.MethodQueryStat:
		return CacheLevelMin
	case jsonrpc.MethodQueryTx:
		return CacheLevelNil
	default:
		panic(fmt.Sprintf("[cacher] unsupported method %s encountered which should have been rejected by the previous checks", method))
	}
}

func hash(data []byte) ID {
	return sha3.Sum256(data)
}
