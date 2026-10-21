# Relay Billing Concurrency Retry Design

## Problem

Concurrent relay requests for the same wallet can finish together. The wallet
compare-and-swap update may then return `ErrWalletConcurrentMutation`. The
current billing finalizer propagates that transient conflict as
`update_data_error`, replacing a completed non-streaming response with HTTP 500
and leaving streaming requests with a post-response settlement error.

## Design

Retry the complete local billing transaction when, and only when, it returns
`model.ErrWalletConcurrentMutation`. Each attempt starts a new database
transaction and therefore reloads the current wallet, reservation, subscription,
and token state. Funding and token updates remain atomic; the upstream model
request is never replayed.

Use four bounded delays after the initial attempt: 10 ms, 25 ms, 50 ms, and
100 ms. Return the last concurrency error after the retry budget is exhausted.
Insufficient balance, token errors, database errors, and all other business
errors return immediately without retry.

The retry boundary is shared by transactional reserve, settle, and the
post-delivery reservation-preservation path. Refunds retain their existing
transaction retry path. This covers the observed settlement failures and the
smaller pre-consume conflict class without changing the public billing API.

## Accounting Invariants

- A failed attempt commits neither wallet ledger nor token quota changes.
- A successful retry commits wallet/subscription funding and token quota once.
- Funding commit callbacks and cache deltas run only after the successful
  transaction commits.
- The relay request and upstream response are not repeated.
- Existing operation IDs remain transaction-local; failed-attempt ledger rows
  roll back with their transaction.

## Verification

Add deterministic tests that inject one transient wallet concurrency error and
prove settlement succeeds on the next transaction with exactly one durable
wallet ledger adjustment and one token adjustment. Add an exhaustion test and a
non-concurrency test to prove bounded and selective retry behavior. Run focused
billing tests followed by the complete Go test suite available without Docker.
