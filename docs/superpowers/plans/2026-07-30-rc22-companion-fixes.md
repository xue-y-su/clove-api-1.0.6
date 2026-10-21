# Rc22 Companion Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Manually adapt the approved independent rc22 correctness and UX fixes to Clove without Auth v2, frontend flattening, the reverted task scanner, or regressions to wallet/subscription/security behavior.

**Architecture:** Treat each upstream commit as test evidence, not a merge unit. Reproduce the failing behavior on Clove first, apply the smallest compatible change in the owning module, and commit each independent fix after its focused regression passes. Shared proxy/task/user changes preserve Clove's existing TLS, SSRF, immutable ledger, audit, Default, and Classic contracts.

**Tech Stack:** Go 1.25, Gin/GORM, React/TypeScript, Classic React/JavaScript, Bun, SQLite/MySQL/PostgreSQL, Playwright.

---

### Task 1: Dedupe Responses-To-Chat Streaming Tool Calls

**Files:**
- Modify: `service/relayconvert/internal/oai_responses/to_oai_chat_stream_resp.go`
- Modify: `service/relayconvert/internal/oai_responses/to_oai_chat_resp_test.go`

- [ ] Add a failing stream fixture where delta and terminal events repeat the same `item_id/call_id`; assert one chat tool call with complete arguments and no duplicated final event.
- [ ] Run `go test ./service/relayconvert/internal/oai_responses -run 'ToolCall|Stream' -count=1` and observe the duplicate.
- [ ] Add conversion-state reuse keyed by stable item/call ID; anonymous events use output index, not content substring matching.
- [ ] Run the focused test and the Task 2 meter dedupe tests from the billing plan; both conversion and billing must independently remain idempotent.
- [ ] Commit `fix(responses): dedupe streamed tool calls`.

### Task 2: Preserve Existing Pricing On Same-Name Model Create Or Rename

**Files:**
- Modify: `web/default/src/features/models/components/drawers/model-mutate-drawer.tsx`
- Modify: its focused drawer/state tests
- Modify Classic model form only if its current behavior reproduces the same defect.

- [ ] Write failing tests for creating a metadata row whose name already has ModelRatio/ModelPrice and renaming onto such a name; the payload must omit destructive default pricing writes.
- [ ] Run focused Default tests and observe current overwrite behavior.
- [ ] Separate metadata defaults from explicit administrator pricing edits. Existing-name creation/rename loads the target's current price settings and sends changes only when the corresponding field is dirty.
- [ ] Test create, rename, genuinely new model, and explicit price replacement; run Default typecheck/build and Classic parity tests.
- [ ] Commit `fix(models): preserve same-name pricing`.

### Task 3: Keep Task Terminal State And Refunds Idempotent

**Files:**
- Modify: `model/task.go`, `model/task_cas_test.go`
- Modify: `service/task_billing.go`, `service/task_billing_test.go`
- Modify: `service/task_polling.go`, `service/task_polling_test.go`
- Modify: `controller/system_task_handlers.go`

- [ ] Write failing concurrent terminal callbacks proving only one CAS transition and one Clove funding refund; after refund, task quota is zero and persisted.
- [ ] Add tests for success-versus-failure races, retry after process restart, wallet and subscription funding, and already-terminal no-op.
- [ ] Run `go test ./model ./service ./controller -run 'Task.*CAS|Task.*Refund|Polling' -count=1` and verify RED.
- [ ] Adapt the rc22 terminal CAS but call Clove's `BillingSession`/funding refund path. Persist terminal status and zero quota in the same transaction/conditional update.
- [ ] Do not add or restore an automatic reconciliation scanner; retain existing scheduled polling only.
- [ ] Run focused race tests and commit `fix(task): make terminal refunds idempotent`.

### Task 4: Purge Existing Authentication Data On Hard User Delete

**Files:**
- Modify: `model/user.go`, `model/user_authentication_test.go`
- Modify: `model/token.go`, `model/twofa.go`
- Modify: `controller/user.go` and relevant OAuth/Telegram controller tests only where cleanup ownership currently lives.

- [ ] Seed a user with Token, OAuth bindings, 2FA secret, backup codes, Passkey, plus Clove wallet/audit records; hard-delete and assert every credential row is gone while required immutable financial/audit rows remain valid/anonymized according to existing policy.
- [ ] Run `go test ./model ./controller -run 'HardDelete|Authentication.*Delete' -count=1` and verify missing cleanup.
- [ ] Add cleanup for only tables present in the Clove schema. Do not import Access JWT, refresh cookie, device session, auth-version cache, or Auth v2 migrations.
- [ ] Keep deletion transactional and idempotent; invalidate token/user caches after commit.
- [ ] Run focused/full model tests and commit `fix(user): purge credentials on hard delete`.

### Task 5: Apply Realtime GA And Auto-Group Corrections

**Files:**
- Modify: `relay/channel/openai/adaptor.go`, `relay/channel/openai/constant.go`, tests.
- Modify: `service/group.go`, `controller/model.go`, `controller/user.go`, `controller/model_list_test.go`.

- [ ] Write table tests: GA realtime models omit beta header, preview models retain it, newly approved GA names are registered.
- [ ] Write API tests: requested `auto` expands to owner/resolved groups and never leaks or stretches the literal sentinel into a model list.
- [ ] Run focused OpenAI/model-list tests and verify RED.
- [ ] Apply the small upstream header/model tables and centralize auto group expansion in `service/group.go`; keep model-catalog visibility out of Relay authorization.
- [ ] Run tests and commit `fix(relay): align realtime and auto groups`.

### Task 6: Add Codex And Advanced Custom Model Discovery

**Files:**
- Create/adapt: `service/codex_channel_models.go`, `service/codex_models.go`, tests.
- Modify: `relay/channel/codex/constants.go`, adaptor tests.
- Modify: `relay/channel/advancedcustom/adaptor.go`, tests.
- Modify: `controller/channel.go`, `controller/channel_upstream_update.go`, tests.
- Modify Default/Classic channel forms and fetch dialogs.

- [ ] Write failing header/URL/format/limit tests for Codex discovery and each Advanced Custom model-fetch template.
- [ ] Prove non-2xx, timeout, invalid JSON, and over-8MiB responses leave `Channel.Models` and publication approvals unchanged.
- [ ] Implement discovery through existing SSRF-protected/timeout-aware clients and structured `dto.ChannelSettings`; never concatenate unvalidated response fragments.
- [ ] Feed results into the same preview/diff/confirmation flow as Sub2API. Discovery alone does not price, group, or publish a marketplace model.
- [ ] Run Go and both frontend channel tests; commit `feat(channel): discover codex custom models`.

### Task 7: Add Server-Side User Sorting And Independent UX Fixes

**Files:**
- Modify: `model/user.go`, `model/user_pagination_test.go`, `controller/user.go`.
- Modify Default users API/types/table, search hook, top-up input, and quota cell/tests.
- Modify equivalent Classic users/top-up components/tests.

- [ ] Write backend allowlist tests for sort field/direction, deterministic ID tiebreaker, page boundaries, invalid sort rejection/fallback, and no raw SQL injection.
- [ ] Write frontend tests for debounced server query, no client sort on a paged slice, clearing the top-up input to an empty string, and very large quota text fitting without numeric overflow.
- [ ] Run focused tests and observe RED.
- [ ] Implement a fixed map from API sort keys to quoted columns and append stable `id`; pass sort state through both frontends.
- [ ] Keep numeric values as strings/bigint-safe formatted quota where JavaScript precision could be lost. Use container constraints/ellipsis plus tooltip for long display, not viewport font scaling.
- [ ] Run Default/Classic typecheck/build and commit `fix(users): stabilize paged sorting and amounts`.

### Task 8: Hand-Merge Proxy URL And Client Cache Lifecycle Fixes

**Files:**
- Create/adapt: `common/proxy_url.go`, tests.
- Modify: `service/http_client.go`, protected fetch tests.
- Modify: `model/channel.go`, controller channel tests.
- Modify only relay call sites that construct proxy clients.

- [ ] Write failing tests for canonical proxy URL/cache key equivalence, credential-sensitive differences, invalid schemes, connection invalidation after channel proxy edits, redirect/SSRF checks, TLS setting separation, and concurrent cache access.
- [ ] Run `go test ./common ./service ./model ./controller -run 'Proxy|HTTPClient|SSRF' -count=1` and verify RED.
- [ ] Port URL canonicalization and cache lifecycle concepts while retaining Clove's existing SSRF protected client, redirect policy, TLS minimum, timeouts, response caps, and invitation-logo protections.
- [ ] Never log proxy credentials or include them in user-facing errors. Cache keys may hash secrets but must not expose them.
- [ ] Run focused race tests and commit `fix(proxy): normalize client cache lifecycle`.

### Task 9: Companion-Fix Regression Gate

- [ ] Run `go test ./... -count=1` and `go vet ./...`.
- [ ] Run Default full test/typecheck/lint/build and Classic full test/eslint/build.
- [ ] Run Playwright smoke for users sorting/search, cleared top-up input, large balances, channel discovery, auto group, realtime headers through a fixture, and task status views.
- [ ] Exercise SQLite/MySQL/PostgreSQL task CAS and user deletion migrations only on disposable databases.
- [ ] Rebuild the local QA container through `scripts/rebuild-local-mysql-qa.ps1`, preserving `cloveapi-current-source-mysql-qa-data`; compare counts and balances and inspect logs.
- [ ] Confirm Auth v2 tables/code, automatic task reconciliation scanner, flattened `web/src`, vendored Snow edits, production deployment, remote push, and Release are absent.
- [ ] Run `git diff --check` and commit only a reproduced regression plus its test.
