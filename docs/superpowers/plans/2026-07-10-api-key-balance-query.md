# API Key Balance Query Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a public New API-styled page that reports an API Key's real remaining spending capacity while making subscription group eligibility identical to live billing.

**Architecture:** A shared service-level eligibility policy resolves the Key's effective groups and filters subscription snapshots. The model layer keeps transaction and row-lock ownership while accepting the eligibility predicate from the service layer; a separate balance snapshot service calculates cumulative capacity, single-request capacity, and redacted response data. A thin controller exposes the snapshot through a rate-limited no-store POST endpoint, and the default React frontend renders it with existing New API components and i18n.

**Tech Stack:** Go 1.25, Gin, GORM, SQLite/MySQL/PostgreSQL, React 19, TypeScript, TanStack Router, Tailwind, existing New API UI components and i18next.

---

## File Map

- Create `service/subscription_eligibility.go`: resolve fixed/default/auto Key groups and decide whether a subscription upgrade group applies.
- Create `service/subscription_eligibility_test.go`: cover historical empty groups and special child-group rules.
- Modify `model/subscription.go`: let active-subscription checks, wallet fallback, and pre-consume use an eligibility predicate while retaining transaction ownership.
- Modify `service/funding_source.go`: pass the request group eligibility predicate into subscription pre-consume.
- Modify `service/billing_session.go`: build group-scoped subscription funding and group-scoped fallback checks.
- Create `service/token_balance.go`: build the redacted balance snapshot and calculate four funding preference paths.
- Create `service/token_balance_test.go`: table-test quota arithmetic, source omission, unlimited values, and status restrictions.
- Create `controller/token_balance.go`: parse the POST body and return the common response envelope without logging the Key.
- Create `controller/token_balance_test.go`: test invalid input, redaction, status handling, and no-store behavior.
- Modify `router/api-router.go`: register the anonymous, limited, no-store endpoint.
- Create `middleware/token_balance_rate_limit.go` and test: add a dedicated IP bucket stricter than the global API limit.
- Create `web/default/src/features/token-balance/{api.ts,types.ts,index.tsx}`: API client and public result screen.
- Create `web/default/src/features/token-balance/components/{balance-progress.tsx,balance-restrictions.tsx}`: focused result components.
- Create `web/default/src/features/token-balance/lib/balance.ts` and test: deterministic progress segment and label calculations.
- Create `web/default/src/routes/token-balance.tsx`: public TanStack file route.
- Modify default frontend locale JSON files through the existing i18n sync workflow.
- Regenerate `web/default/src/routeTree.gen.ts` through the normal frontend build.

### Task 1: Shared Subscription Eligibility

- [ ] Add failing tests proving that an empty historical `upgrade_group` is unrestricted, a matching root group is allowed, an authorized child from `GetUserUsableGroups` is allowed, a `-:child` removal is denied, and an `auto` Key counts one subscription only once.
- [ ] Run `go test ./service -run 'TestSubscription(Applies|EffectiveGroups)' -count=1`; expect failures because the policy functions do not exist.
- [ ] Implement `ResolveTokenEffectiveGroups(userGroup, tokenGroup string) []string` and `SubscriptionAppliesToGroups(subscriptionGroup string, effectiveGroups []string) bool` in `service/subscription_eligibility.go`. Normalize and deduplicate groups; use `GetUserAutoGroup` for `auto`; treat empty subscription groups as unrestricted.
- [ ] Run the focused service tests and `go test ./service -count=1`; expect PASS.
- [ ] Commit with `feat(billing): share subscription group eligibility`.

### Task 2: Align Live Billing With Group Eligibility

- [ ] Add model/service regression tests for these cases: a Pro request cannot consume an unrelated Standard subscription; a Pro request can consume a Pro subscription; `subscription_first` ignores unrelated strict subscriptions when deciding wallet fallback; and multiple matching subscriptions retain `end_time asc, id asc` selection.
- [ ] Run `go test ./model ./service -run 'Test.*Subscription.*Group|Test.*Wallet.*Fallback' -count=1`; expect at least the unrelated-group cases to fail.
- [ ] Add a model-level `SubscriptionEligibility` callback type and group-scoped variants for active checks, wallet-overflow checks, and `PreConsumeUserSubscription`. Apply the callback after locking each candidate and before resetting or consuming it. Preserve compatibility wrappers only where existing callers/tests require them.
- [ ] Extend `SubscriptionFunding` with `eligible model.SubscriptionEligibility`; pass it to pre-consume.
- [ ] In `NewBillingSession`, construct eligibility from `relayInfo.UsingGroup`, use it for subscription funding, active checks, and wallet overflow checks. Keep the four existing preference semantics and existing transaction order.
- [ ] Run the focused tests, then `go test ./model ./service -count=1`; expect PASS.
- [ ] Commit with `fix(billing): scope subscriptions to request groups`.

### Task 3: Balance Snapshot Service And Public API

- [ ] Add failing table tests for finite Key arithmetic (`150 = 60 available + 50 used + 40 restricted`), unlimited Keys without magic numbers, all four preferences, strict subscription fallback, single-request maximum, disabled/expired/exhausted state, and omission of non-participating source amounts.
- [ ] Run `go test ./service -run TestBuildTokenBalance -count=1`; expect failure because the snapshot types and builder do not exist.
- [ ] Implement nullable/unlimited quota value types and `BuildTokenBalanceSnapshot(key string)` in `service/token_balance.go`. Read the token and user cache, resolve effective groups, retrieve reset-aware active subscriptions, filter with the shared policy, calculate cumulative and per-request capacity with saturating integer arithmetic, mask the Key, and omit sources excluded by the preference.
- [ ] Add a dedicated `TokenBalanceRateLimit` using the existing Redis/in-memory factory with an IP bucket of 10 requests per minute.
- [ ] Add controller tests using a Gin test router. Assert `POST` body validation, a generic invalid-Key response, no full Key in serialized output, `Cache-Control: no-store`, structured rate-limit behavior, and successful disabled/expired status rendering.
- [ ] Implement `controller.GetTokenBalance` with a bounded JSON body `{ "key": "sk-..." }`, common response envelope, translated public messages, and no Key logging.
- [ ] Register `POST /api/token-balance` with `TokenBalanceRateLimit`, `AnonymousRequestBodyLimit`, and `DisableCache` before authenticated token routes.
- [ ] Run `go test ./middleware ./controller ./service ./model -count=1`; expect PASS.
- [ ] Commit with `feat(balance): add secure token balance api`.

### Task 4: Default Frontend Page In New API Style

- [ ] Add failing pure frontend tests for progress calculations: finite three-segment totals, zero quota, unlimited Key behavior, and hidden wallet/subscription fields for exclusive modes.
- [ ] From `web/default`, run `bun test src/features/token-balance/lib/balance.test.ts`; expect failure before the module exists.
- [ ] Implement typed API access in `features/token-balance/api.ts` using the shared API client. Keep the entered Key in component state only and clear it after a successful response; never use query parameters or Web Storage.
- [ ] Build the page with `PublicLayout`, existing `Card`, `Input`, `Button`, `Badge`, tooltip and theme tokens. Use a compact responsive layout: query form, masked Key/status, primary current-available value, blue/gray/red progress bar, two supporting quota values, and a restrictions grid.
- [ ] For `subscription_only`, render no account-balance field and no subscription list or “available subscriptions” heading. For `wallet_only`, render no subscription data. Render only source types and restrictions returned by the API.
- [ ] Add the public `/token-balance` route and an entry in the existing public header navigation using the same link treatment as other New API modules.
- [ ] Add English, Simplified Chinese, and Traditional Chinese source strings, then run the repository's i18n synchronization command so all locale files remain structurally valid.
- [ ] From `web/default`, run `bun test src/features/token-balance`, `pnpm typecheck`, `pnpm lint`, and `pnpm build`; expect PASS and a regenerated route tree.
- [ ] Commit with `feat(frontend): add token balance query page`.

### Task 5: Cross-Database And Security Verification

- [ ] Add database-backed tests that seed users, Keys, plans, and subscriptions without database-specific SQL. Cover finite/unlimited quotas, group filtering, reset periods, and fallback behavior on SQLite.
- [ ] Run the same focused package tests with the repository's MySQL and PostgreSQL test environment variables when local services are available. Confirm no migration is generated and no query depends on SQLite syntax.
- [ ] Run `go test ./... -count=1` and `go vet ./...`; then from `web/default` run `bun test src`, `pnpm typecheck`, `pnpm lint`, `pnpm format:check`, `pnpm copyright:check`, and `pnpm build`.
- [ ] Search tracked changes and generated output for a seeded full API Key and for accidental response fields such as `user_id`, `username`, `email`, `subscription_id`, and `order`.
- [ ] Fix any failures in the owning task's files and repeat the failed command until green.
- [ ] Commit with `test(balance): cover database and privacy behavior` if this task adds test-only changes.

### Task 6: Browser And Production-Like Acceptance

- [ ] Start the Go backend against the existing local test database and serve the freshly built default frontend on an unused local port.
- [ ] Seed or reuse Keys covering finite Key plus constrained subscription, unlimited Key, `subscription_only`, `wallet_only`, `subscription_first`, `wallet_first`, expired, disabled, and model-limited states.
- [ ] Use Playwright at desktop and mobile viewports to submit each Key, verify displayed arithmetic and hidden-source rules, capture screenshots, and check browser console/network logs for errors and Key persistence.
- [ ] Confirm `/token-balance` survives refresh without retaining the Key, the Key never enters the URL, response headers include `no-store`, and repeated requests trigger the dedicated structured rate limit.
- [ ] Compare the page against existing New API public pages for header height, container width, typography, border radius, dark mode, loading state, and mobile wrapping. Correct any mismatch and rebuild.
- [ ] Run `git diff --check`, inspect `git status --short`, and record final commands and evidence under `.verification-output/` without committing local databases, logs, screenshots, or secrets.
