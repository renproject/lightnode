## 0.1.15
- Update to use latest Darknode (increase underlying blockchain fee)

## 0.1.14
- Fix an issue when trying to insert a nil address list to storage. 
- Update mint fees in JSON-RPC response

## 0.1.13
- Update to Darknode v0.2.24 (add recover mechanism in the rpc server)
- Use persistent storage (database) for MultiAddress store. 
- Improve p2p boostrap logic.  

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
