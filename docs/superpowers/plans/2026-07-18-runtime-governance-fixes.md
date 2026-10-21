# Runtime Governance Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Playground reasoning controls, IP audit projection, invitation revocation, user redemption amounts, subscription group eligibility, and API Key balance snapshots match their real runtime behavior.

**Architecture:** Keep Router -> Controller -> Service -> Model ownership. Add one service-level subscription eligibility policy shared by live billing and balance snapshots, one authenticated unified wallet redemption endpoint, and explicit capability/configuration boundaries for Playground reasoning and the IP projection worker. Preserve existing transaction, lease, ledger, and audit primitives.

**Tech Stack:** Go, Gin, GORM, SQLite/MySQL/PostgreSQL, React 19/TypeScript/Bun, React 18/JavaScript/Bun, Docker Compose, Playwright.

---

## File Map

- Create `service/subscription_eligibility.go`: resolve request groups and filter active subscriptions.
- Create `service/subscription_eligibility_test.go`: group-policy regression tests.
- Modify `model/subscription.go`: allow transaction-owned pre-consume to receive eligible subscription IDs.
- Modify `service/funding_source.go`, `service/billing_session.go`: pass the effective request group through live billing.
- Modify `service/token_balance.go`: reuse the same eligibility policy.
- Modify `service/token_balance_test.go`: prove snapshot and billing agreement.
- Modify `relay/helper/playground_reasoning.go` and tests: accept explicit off only for capable routes.
- Modify Playground payload/capability files and tests: send explicit off and disable unsupported controls.
- Modify `service/ip_security_projection_worker.go`, `model/ip_security_backfill.go`, and tests: explicit worker enablement and stale-lease recovery.
- Modify invitation service/controller/router and both frontends: owner revoke and compact actions.
- Modify user redemption service/controller/router and both frontends: standard-unit conversion and wallet-only claim entry.
- Modify wallet API/hooks: call a unified authenticated redemption endpoint.
- Modify locale files and i18n tests for every touched view.

### Task 1: Playground reasoning capability

- [ ] Add a payload test expecting `reasoning_effort: 'off'` for a model that advertises explicit control.
- [ ] Run `bun test src/features/playground/lib/streaming/payload-builder.test.ts`; expect the new assertion to fail because off is omitted.
- [ ] Add backend tests for accepted off on a capable route and rejected off on an incapable route.
- [ ] Run `go test ./relay/helper -run TestPlaygroundReasoning -count=1`; expect off validation to fail.
- [ ] Implement a shared Playground capability result with supported effort values; map explicit off only through proven provider adapters.
- [ ] Disable the Default and Classic controls when the selected model cannot control reasoning and localize the explanation.
- [ ] Re-run the targeted Bun and Go tests and expect PASS.

### Task 2: IP projection recovery

- [ ] Add tests for `IP_SECURITY_PROJECTION_WORKER_ENABLED=true` on a slave and for the default slave-disabled behavior.
- [ ] Add a model test with `status=running`, expired `locked_until`, and remaining rows; expect backfill to claim the stale lease and reach ready.
- [ ] Run `go test ./service ./model -run 'TestIPSecurityProjection' -count=1`; expect the explicit slave enablement test to fail.
- [ ] Parse the explicit environment flag while preserving master/slave defaults, and retain the database lease as the concurrency guard.
- [ ] Return structured projection status through the existing IP audit error path and render retryable Default/Classic states.
- [ ] Re-run targeted tests and expect PASS.

### Task 3: Invitation owner revoke and compact UI

- [ ] Add service/controller tests proving an owner can revoke an active code, cannot revoke another user's code, and an Admin can revoke any active code.
- [ ] Run `go test ./service ./controller -run 'Test.*Invite.*Revoke' -count=1`; expect the owner route/controller test to fail.
- [ ] Add `POST /api/invitation/codes/:id/revoke` using the existing `RevokeInviteCode` ownership checks and stable audit behavior.
- [ ] Remove the duplicate registration-link copy button, keep QR/link actions in the QR dialog, and reduce reward-section height without nested cards.
- [ ] Add user and admin revoke mutations to Default and Classic, with confirmation and query refresh.
- [ ] Run invitation Go/Bun tests and expect PASS.

### Task 4: Standard user-redemption quota

- [ ] Add controller/service tests proving `amount: 1` becomes `common.QuotaPerUnit`, and a random packet's minimum claim is one standard unit.
- [ ] Run `go test ./controller ./service -run Test.*UserRedemption.*Amount -count=1`; expect raw quota `1` and fail.
- [ ] Convert display `amount` with a checked quota helper after multiplying by `QuotaPerUnit`; keep explicit raw `total_quota` compatibility.
- [ ] Update Default and Classic create payloads and amount formatting to use the public quota configuration.
- [ ] Re-run user-redemption tests and expect PASS.

### Task 5: Unified wallet redemption

- [ ] Add controller tests for one authenticated endpoint redeeming both legacy admin codes and user-created codes without string-based fallback.
- [ ] Run the new controller test and expect route/handler absence.
- [ ] Add a service discriminator that looks up exact code ownership in each table and dispatches to the existing transactional claim/redeem logic.
- [ ] Register the endpoint under the authenticated wallet route and return one stable response shape containing added quota and code type.
- [ ] Remove claim actions/dialogs from user-redemption pages; make both wallet themes use the unified endpoint.
- [ ] Run controller/service/frontend tests and expect PASS.

### Task 6: Shared subscription group eligibility

- [ ] Add table tests for empty historical group, exact group, authorized child, removed child, unrelated group, and duplicate auto candidates.
- [ ] Run `go test ./service -run TestSubscriptionEligibility -count=1`; expect the new module to be missing.
- [ ] Implement `SubscriptionAppliesToGroup` and effective fixed/default/auto group resolution using `GetUserUsableGroups`.
- [ ] Add model entry points that pre-consume only from service-selected active subscription IDs while keeping row locks and reset logic in `model`.
- [ ] Scope `HasActiveUserSubscription`, overflow checks, `SubscriptionFunding`, and `NewBillingSession` to the request's effective group.
- [ ] Run service/model billing tests and expect PASS.

### Task 7: Token balance parity

- [ ] Extend snapshot tests with two subscriptions in different groups and all four funding preferences.
- [ ] Add an agreement test asserting the subscription IDs visible to the snapshot are the same IDs eligible for live pre-consume.
- [ ] Run `go test ./service ./controller -run TestTokenBalance -count=1`; expect unrelated-group quota to be incorrectly counted.
- [ ] Replace unfiltered `GetAllActiveUserSubscriptions` aggregation with the shared eligibility result.
- [ ] Keep fixed/default/auto response metadata and redaction unchanged while recalculating cumulative and single-request capacity.
- [ ] Re-run token balance and billing tests and expect PASS.

### Task 8: Full verification and runtime QA

- [ ] Run targeted Go tests for all six areas, then `go test ./...`.
- [ ] Run Default related tests, `bun run typecheck`, lint for touched files, and `bun run build`.
- [ ] Run Classic related tests, i18n/lint checks, and `bun run build`.
- [ ] Run MySQL integration tests for invitations, redemption ledger/refunds, subscription eligibility, token balance, and IP projection.
- [ ] Rebuild the QA Docker image with `IP_SECURITY_PROJECTION_WORKER_ENABLED=true` and verify the projection reaches `ready`.
- [ ] Use the QA account to verify user/Admin flows and desktop/mobile layout with Playwright; capture network/console evidence and screenshots.
- [ ] Compare every requirement in `docs/superpowers/specs/2026-07-18-runtime-governance-fixes-design.md` against source, tests, database state, and browser evidence before completing the goal.

No commit step is included because the user has not requested a commit for this change set.
