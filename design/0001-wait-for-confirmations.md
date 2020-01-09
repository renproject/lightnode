## Overview

Waiting for confirmations is important to the safety of the system. It makes sure transactions are unlikely be re-ordered on blockchain and also reduces the risk of double spending.

## Motivation

Currently, Darknodes accept transactions with 0-confirmations and wait for them to reach the required number of confirmations before processing. This takes a lot of resources of Darknode and can be a vulnerability for attack. We want to minimise the amount of work Darknodes are doing (https://github.com/renproject/darknode/issues/118) to avoid an application-level DOS attack.  Lightnodes should do the validation and pre-processing to reduce the number of bad transactions sent to Darknodes.

## Design

Persistent storage is needed for the transaction so Lightnode won't lose transaction details and can recover from an unexpected crash. They will use SQL databases, as they are easy to set up and fast for querying. The interface of storage needs to be consistent so that users can use different kinds of SQL databases (i.e. SQLite, PostgreSQL, etc.).

After transactions have been validated, Lightnode should check the `SubmitTx` transaction has received zero-confirmations. It should reject the transaction if it's not confirmed. After the required number of confirmations are reached, the transaction should then be broadcast to the Darknodes.

## Implementation 

- Implement the transaction confirmer.
  - Have recover logic in the confirmer for unexpected crashes.
  - Periodically check confirmations stored and send confirmed transactions to validator.
- Define a good storage interface and implement it with both SQLite and PostgreSQL.
- Remove the code for storing ghashes as they can be retrieved from the transaction details.