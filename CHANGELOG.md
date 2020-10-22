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
