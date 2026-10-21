# Relay Billing Concurrency Retry Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent transient same-wallet billing conflicts from becoming relay HTTP 500 responses without replaying the upstream request.

**Architecture:** Wrap only local transactional billing mutations in a bounded retry helper. A retry always opens a fresh GORM transaction and only handles `model.ErrWalletConcurrentMutation`; all other errors keep their current behavior.

**Tech Stack:** Go 1.22+, GORM v2, testify, SQLite test database with GORM callbacks for deterministic fault injection.

---

### Task 1: Reproduce transient settlement conflict

**Files:**
- Modify: `service/billing_reservation_test.go`

- [ ] Add a test that injects `model.ErrWalletConcurrentMutation` into the first wallet update during `BillingSession.Settle`.
- [ ] Run `go test ./service -run TestBillingSessionRetriesTransientWalletSettlementConflict -count=1` and verify it fails because settlement returns the injected conflict.

### Task 2: Add bounded transaction retry

**Files:**
- Modify: `service/billing_session.go`

- [ ] Add a four-delay local billing retry helper using 10/25/50/100 ms.
- [ ] Apply it to transactional settle and reserve paths while keeping commit callbacks outside failed attempts; preserve the existing refund retry path.
- [ ] Re-run the focused test and verify one wallet ledger entry, the expected final wallet balance, and the expected final token quota.

### Task 3: Prove selectivity and exhaustion

**Files:**
- Modify: `service/billing_reservation_test.go`

- [ ] Add a test proving a non-concurrency persistence error is attempted once.
- [ ] Add a test proving persistent wallet concurrency errors stop after five total attempts and leave wallet, token, and reservation state unchanged.
- [ ] Run `go test ./service -run 'TestBillingSession(Retries|Stops|DoesNotRetry)' -count=1` and verify all cases pass.

### Task 4: Regression verification

**Files:**
- Test only

- [ ] Run `gofmt` on changed Go files.
- [ ] Run focused billing and model tests.
- [ ] Run the available complete Go test suite without Docker and record any environment-only gaps.
- [ ] Review `git diff` to ensure unrelated working-tree changes remain untouched.
