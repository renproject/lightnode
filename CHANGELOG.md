## 0.4.5
- Update compatibility layer for v0 burns
- Use connection pool for Redis interaction
- Fetch public key and max confirmations from Darknode when booting
- Fix incorrect `amount` field for v0 mints

## 0.4.4

- Partial transaction persistence for gateway recovery
- Compatibility updates for empty gpubkey

## 0.4.3

- Fix network params for watcher

## 0.4.2

- Update Multichain to v0.3.12
- Add support for Polygon + Avalanche
- Add support for QueryTxByTxid

## 0.4.1

- Update Multichain to v0.3.10
- Add support for watching Solana burns
- Pull cross-chain fees from RenVM state
- Add support for Fantom

## 0.3.2

- Improve watcher reliability by extracting Ethereum log filter function to make it more testable and configurable

## 0.3.1

- Update to Multichain v0.2.24

## 0.3.0

- Add compatibility with v0 txs
- Update multichain dependency

## 0.2.7

- Update Darknode to fix preparation for account based chains (payload removal)

## 0.2.6

- Fix Dockerfile issues with filecoin-ffi submodule
- Make watcher more robust to issues with underlying blockchain nodes
- Add debug logs for watcher

## 0.2.5

- Add support for Devnet
- Update Darknode dependency
- Use `ren_queryConfig` RPC to load selector whitelist and confirmations
- Add unit tests for resolver
- Use Darknode signatory for query parameter

## 0.2.4

- Fix overriding confirmations
- Update Darknode/Multichain dependencies
- Fix function parsing the network

## 0.2.3

- Update to Multichain v0.2.14
- Add support for whitelisting selectors

## 0.2.2

- Update to Multichain v0.2.12
- Build Dockerfile using Multichain base image
- Fix parsing of confirmations from environment variables
- Improve error messages

## 0.2.1

- Darknode transaction input compatibility updates
- Integrate Terra

## 0.2.0

- Darknode v0.3.0 compatibility

## 0.1.12

- Update transaction if Darknodes return a "done" status
- Increase default unconfirmed transaction expiry to 14 days
- Print the error message for invalid burn transactions

## 0.1.11

- Update to Darknode v0.2.22 to set the minimum mint/burn amount

## 0.1.10

- Improve P2P logic to reduce connection times for new Darknodes
- Update to Darknode v0.2.17

## 0.1.9

- Update to Darknode v0.2.14

## 0.1.8

- Add support for the QueryFees method

## 0.1.7

- Update confirmations to be the same as Darknodes

## 0.1.6

- Add support for Mainnet network
- Update to go v1.13

## 0.1.5

- Update Lightnode to use the Darknode JSON-RPC server
- Add support for the QueryShards method
- Ensure bootstrap Darknodes are not removed from storage
