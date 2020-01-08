## Overview

Transaction confirmation is important to the safety of the system. It makes sure transactions are unlikely be re-ordered 
on blockchain. It also reduce the risk of double spending. 

## Motivation

Currently darknodes accept transaction with 0-confirmation and wait for it to reach required number of confirmations 
before processing. This takes a lot of resources of darknode and can be a vulnerability for attack. We want to minimise 
the amount of work darknode is doing to avoid application-level DOS attack(See [issue](github.com/renproject/darknode/issues/118)). 
Lightnode should do the validation and pre-processing to reduce the number of bad transactions sent to darknode. 

## Design

Persistent storage is needed for the transaction. So lightnode won't lose transaction details and can recover from an 
unexpected crash. We decide to use SQL databases, as it's easy to setup and fast for querying. The interface of storage
needs to be persistent so that users can use different kinds of SQL databases. (i.e. sql lite, postgres sql...)

We'll have a confirmer which takes transaction from validator and checks zero-confirmation for the `SubmitTx` request. 
It should reject the transaction if it's not confirmed. After required number of confirmations are reached, confirmer 
sends the transaction to the next stage. 


## Implementation 

- Implement the transaction confirmer.
  - Have recover logic in the confirmer when unexpected crash happens
  - Periodically check confirmations stored and sends confirmed tx to validator

- Define a good storage interface and implement it with both sql lite and postgres sql. 
- Remove the code of storing gHash as it can be retrieved from the tx details. 
