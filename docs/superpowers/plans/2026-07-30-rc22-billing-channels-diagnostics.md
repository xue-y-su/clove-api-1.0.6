# Rc22 Billing Channels And Diagnostics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add evidence-based tool surcharges, atomic mixed subscription/wallet reservation, Alpha Search, Sub2API with explicit catalog publication, and bounded upstream-error diagnostics without duplicate billing or logs.

**Architecture:** A request-scoped immutable `ToolBillingMeter` is the only tool evidence and pricing source. Every relay owns a zero-capable `BillingSession`; `ReserveToTarget` atomically reserves the chosen funding plan and `FinalizeRelayBilling` is the only successful accounting/logging terminus. Alpha Search and Sub2API plug into existing relay selection, while a pure error parser emits a redacted diagnostic object to one sink. Default and Classic share backend schemas but keep theme-native controls.

**Tech Stack:** Go 1.25, Gin, GORM, shopspring/decimal, React/TypeScript, Classic React/JavaScript, Bun, Playwright, SQLite/MySQL/PostgreSQL, Docker.

---

## Execution Order And File Map

1. Tool configuration/snapshot: `setting/operation_setting/tools.go`, new focused helpers/tests.
2. Meter/evidence: new `relay/common/tool_billing_meter.go`, protocol adaptors and tests.
3. Funding/finalization: `service/billing_session.go`, `service/funding_source.go`, `service/text_quota.go`, `controller/relay.go`.
4. Alpha Search: `dto/alpha_search_request.go`, `relay/alpha_search_handler.go`, router/adaptors/tests.
5. Sub2API/publication: constants, adaptor, channel model-fetch flow, new publication model/table, two frontends.
6. Diagnostics: `service/error.go`, new `service/upstream_diagnostic.go`, failed-usage integration and settings.
7. Wallet/log UI aggregation and all settings UI.

Do not cherry-pick `2d23cdf2`: use it as a behavior reference because its quota path, single frontend, and error-body behavior conflict with Clove.

### Task 1: Build An Atomic Immutable Tool Pricing Snapshot

**Files:**
- Modify: `setting/operation_setting/tools.go`
- Modify: `setting/operation_setting/tools_price_test.go`
- Create: `setting/operation_setting/tool_pricing_snapshot.go`
- Create: `setting/operation_setting/tool_pricing_snapshot_test.go`
- Modify: `model/option.go`

- [ ] **Step 1: Write failing validation, prefix, image-mode, and snapshot tests**

```go
func TestToolPricingSnapshotUsesPublicModelAndLongestPrefix(t *testing.T) {
    snap := mustToolPricingSnapshot(t, ToolPriceSetting{Enabled: true, Prices: map[string]float64{
        "web_search_preview": 10,
        "web_search_preview:gpt-*": 20,
        "web_search_preview:gpt-4.1*": 25,
    }})
    assert.Equal(t, decimal.NewFromInt(25), snap.Price("web_search_preview", "gpt-4.1-mini"))
    assert.Equal(t, decimal.NewFromInt(10), snap.Price("web_search_preview", "mapped-upstream-name"))
}

func TestToolPriceUpdateRejectsEntireInvalidDocument(t *testing.T) {
    before := CurrentToolPricingSnapshot()
    err := ApplyToolPriceSetting(`{"prices":{"web_search":10,"bad":-1}}`)
    require.Error(t, err)
    assert.Same(t, before, CurrentToolPricingSnapshot())
}
```

Cover zero prices, NaN/Inf rejection, legacy invalid entries skipped one-by-one with one warning, defaults, group-only/full multiplier modes, exact matrix/flat image modes, model-prefix overrides, reset defaults, and concurrent readers observing one complete snapshot.

- [ ] **Step 2: Run focused tests and verify RED**

Run: `go test ./setting/operation_setting -run 'ToolPric|ImagePric' -count=1 -v`

- [ ] **Step 3: Implement strict save and tolerant startup parsing**

Extend the registered setting:

```go
type ToolPriceSetting struct {
    Enabled                 bool                         `json:"enabled"`
    Prices                  map[string]float64           `json:"prices"`
    MultiplierMode          string                       `json:"multiplier_mode"`
    ImagePricingMode        string                       `json:"image_pricing_mode"`
    ImagePriceMatrix        map[string]map[string]float64 `json:"image_price_matrix"`
    ImageModelModeOverrides map[string]string             `json:"image_model_mode_overrides"`
}
```

Build a fully validated `atomic.Pointer[ToolPricingSnapshot]` before replacing the current pointer. Store decimal values and sorted prefix slices in the snapshot. Save accepts every finite non-negative value; high prices return a warning descriptor for the controller UI confirmation rather than a validation error.

- [ ] **Step 4: Run tests to GREEN and commit**

Run: `go test ./setting/operation_setting ./model -run 'ToolPric|Option' -count=1`

```bash
git add setting/operation_setting model/option.go
git commit -m "feat(billing): snapshot tool pricing"
```

### Task 2: Make One Meter Own Evidence, Dedupe, Rounding, And Attempts

**Files:**
- Create: `relay/common/tool_billing_meter.go`
- Create: `relay/common/tool_billing_meter_test.go`
- Modify: `relay/common/relay_info.go`
- Modify: `dto/openai_response.go`
- Modify: `dto/gemini.go`
- Modify: `relay/channel/openai/relay_responses.go`
- Modify: `relay/channel/openai/relay_responses_billing_test.go`
- Modify: `relay/channel/claude/relay-claude.go`
- Modify: `relay/channel/claude/tool_billing_test.go`
- Modify: `relay/channel/gemini/relay-gemini.go`
- Delete after callers migrate: `service/tool_billing.go`

- [ ] **Step 1: Write failing evidence and attempt tests**

```go
func TestToolBillingMeterCommitsOnlySuccessfulAttempt(t *testing.T) {
    meter := newMeterFixture(t)
    first := meter.NewAttempt()
    require.NoError(t, first.Confirm(ToolEvent{Name: "web_search_preview", StableID: "call-1"}))
    meter.DiscardAttempt(first)
    second := meter.NewAttempt()
    require.NoError(t, second.Confirm(ToolEvent{Name: "file_search", StableID: "call-2"}))
    meter.CommitAttempt(second)
    got := meter.Total()
    assert.Equal(t, 1, got.Items[0].Count)
    assert.Equal(t, "file_search", got.Items[0].Name)
}
```

Cover Responses output-only evidence, streaming `item_id/call_id`, repeated terminal events, protocol output index, normalized anonymous payload digest, declared-but-unused tools, Claude usage, Gemini grounding, completed custom functions only when explicitly priced, actual image output count, free group, zero price, minimum one, overflow, full/group-only multipliers, and price immutability after admin update.

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./relay/common ./relay/channel/openai ./relay/channel/claude ./relay/channel/gemini -run 'Tool|Billing|Responses' -count=1 -v`

- [ ] **Step 3: Implement meter and stable largest-remainder allocation**

```go
type ToolBillingMeter struct {
    snapshot ToolBillingRequestSnapshot
    mu sync.Mutex
    committed map[string]*confirmedToolUsage
}

type ToolBillingAttempt struct {
    parent *ToolBillingMeter
    events map[string]ToolEvent
    closed bool
}

func (m *ToolBillingMeter) NewAttempt() *ToolBillingAttempt
func (m *ToolBillingMeter) CommitAttempt(*ToolBillingAttempt) error
func (m *ToolBillingMeter) DiscardAttempt(*ToolBillingAttempt)
func (m *ToolBillingMeter) TargetToolQuota() (ToolBillingResult, error)
```

Use exact decimal sums and stable `(name, unitPrice, evidenceKey)` ordering. Floor each exact line, distribute remaining integer quota by descending fractional remainder then stable order, and assign minimum one to the first positive-price item. Sum of line subtotals must equal `ToolQuota`.

- [ ] **Step 4: Wire protocol adaptors and remove old charge paths**

Adaptors call only `relayInfo.ToolBillingMeter` attempt methods. Remove `calculateTextToolCallSurcharge`, old context surcharge additions, and every call to `ComputeToolCallQuota`. Preserve old fields solely when decoding historical log `other` JSON.

- [ ] **Step 5: Run focused/race tests to GREEN and commit**

Run the Step 2 command plus `go test -race ./relay/common -run ToolBilling -count=1`.

```bash
git add relay/common relay/channel/openai relay/channel/claude relay/channel/gemini dto service/tool_billing.go
git commit -m "feat(relay): meter confirmed tool use"
```

### Task 3: Add Zero Sessions, Atomic Funding Targets, And One Finalizer

**Files:**
- Modify: `service/billing_session.go`
- Modify: `service/billing_session_subscription_error_test.go`
- Modify: `service/funding_source.go`
- Create: `service/billing_reservation_test.go`
- Create: `service/relay_billing_finalizer.go`
- Create: `service/relay_billing_finalizer_test.go`
- Modify: `service/text_quota.go`
- Modify: `service/text_quota_test.go`
- Modify: `controller/relay.go`
- Modify: `relay/responses_handler.go`
- Modify: `relay/channel/openai/relay_responses.go`

- [ ] **Step 1: Write failing zero/trusted/mixed-funding/finalizer tests**

Cover zero-fee free-model success, zero-balance paid tool rejection before its completion event, trusted wallet first reserve doing a real ledger adjustment, subscription partial balance plus wallet overflow, `subscription_only`, `wallet_only`, overflow disabled, reserve idempotency, finalize idempotency, non-stream failure delivering zero upstream bytes, and one usage log/stat update.

```go
func TestTrustedSessionReserveToTargetLocksRealFunds(t *testing.T) {
    session := newTrustedWalletSession(t, 100)
    require.NoError(t, session.ReserveToTarget(25))
    assert.Equal(t, 25, session.GetPreConsumedQuota())
    assertWalletLedgerDelta(t, session.RequestID(), -25)
}
```

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./service ./controller ./relay -run 'BillingSession|ReserveToTarget|FinalizeRelayBilling|NonStream' -count=1 -v`

- [ ] **Step 3: Implement zero-capable sessions and atomic funding plan**

Remove the free-model branch that skips `BillingSession`. Represent request funding as cumulative subscription/wallet amounts, not a single string:

```go
type FundingAllocation struct { SubscriptionQuota, WalletQuota int }

func (s *BillingSession) ReserveToTarget(target int) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    if target <= s.reservedTotal { return nil }
    allocation, err := s.fundingPlan.ReserveAtomic(target, s.allocation)
    if err != nil { return err }
    s.trusted = false
    s.allocation = allocation
    s.reservedTotal = target
    return nil
}
```

The storage transaction locks user, subscription, and wallet in the repository's established order. Apply preference/mode/overflow rules before mutating either side. Write both ledger families with the same request ID; rollback both on any failure.

- [ ] **Step 4: Implement one error-returning finalizer**

```go
func FinalizeRelayBilling(ctx *gin.Context, info *relaycommon.RelayInfo, usage *dto.Usage, delivery DeliveryState) error
```

It calculates model quota even when tokens are zero but an approved fixed/model/tool charge exists, combines the committed meter, reserves/settles, then updates token/user/channel stats and writes exactly one consume log. Remove direct success-stat/log writes from `PostTextConsumeQuota`; leave a compatibility wrapper only until every caller migrates.

Non-stream handlers keep the bounded decoded payload in memory, call the finalizer, then write status/body. Stream handlers call `ReserveToTarget` before forwarding each newly billable completed event. Disconnect finalizes forwarded local usage and committed tool events once.

- [ ] **Step 5: Run tests/race suite to GREEN and commit**

Run: `go test -race ./service ./controller ./relay -run 'Billing|Quota|Responses' -count=1`

```bash
git add service/billing_session.go service/funding_source.go service/relay_billing_finalizer.go service/*billing*test.go service/text_quota.go service/text_quota_test.go controller/relay.go relay/responses_handler.go relay/channel/openai/relay_responses.go
git commit -m "fix(billing): finalize each relay once"
```

### Task 4: Add Alpha Search Through Existing Relay Selection

**Files:**
- Create: `dto/alpha_search_request.go`
- Create: `relay/alpha_search_handler.go`
- Create: `relay/alpha_search_handler_test.go`
- Modify: endpoint/api/relay constants and `router/relay-router.go`
- Modify: Codex and Advanced Custom adaptors/tests
- Create: `setting/operation_setting/alpha_search.go`

- [ ] **Step 1: Write failing route, retry, body, and billing tests**

Assert disabled-before-routing, unknown JSON fields retained, only structured `model` rewrite, unsupported channel retry, first 500/second 200, all failures refund, 2xx-before-client-disconnect charge, token model tool-only, per-request model-plus-tool, and admin channel test without user billing.

- [ ] **Step 2: Run RED**

Run: `go test ./relay ./router ./relay/channel/codex ./relay/channel/advancedcustom -run 'AlphaSearch' -count=1 -v`

- [ ] **Step 3: Implement the route and request-level fixed reservation**

Register `POST /v1/alpha/search`. Preserve request JSON as `json.RawMessage`, decode a small typed envelope for validation, and rewrite only `model` through `map[string]json.RawMessage`. Reserve the request-level fixed fee once. Non-2xx discards an attempt without refunding; outer all-failed handling refunds once. First 2xx confirms `web_search_preview`, finalizes, then sends status/body.

- [ ] **Step 4: Add administrator channel testing and GREEN tests**

Reuse bounded channel-test response handling, mark it admin diagnostic, and bypass wallet/usage accounting. Run focused tests and commit:

```bash
git add dto/alpha_search_request.go relay/alpha_search_handler.go relay/alpha_search_handler_test.go relay/channel/codex relay/channel/advancedcustom router/relay-router.go setting/operation_setting/alpha_search.go common constant types
git commit -m "feat(relay): add alpha search"
```

### Task 5: Add Sub2API And Explicit Model Publication

**Files:**
- Add channel type `59` to `constant/channel.go` and related API/endpoint enums.
- Create: `relay/channel/sub2api/adaptor.go`, `constants.go`, and tests.
- Create: `model/channel_model_publication.go` and tests.
- Modify: `model/main.go`, channel deletion, channel fetch/update controllers/tests.
- Modify Default and Classic channel constants/forms/fetch-model dialogs.

- [ ] **Step 1: Write failing adaptor/publication/diff tests**

Cover OpenAI/Claude/Gemini delegation, auth headers, Alpha Search, unsupported audio/rerank, `/v1/models` 8MiB cap/timeout, parse failure unchanged, new/existing/missing diff, missing retained by default, explicit removal, preview creating no approval, confirmed additions writing approval, deletion cleanup, and same-name channel isolation.

- [ ] **Step 2: Run RED**

Run: `go test ./relay/channel/sub2api ./controller ./model -run 'Sub2API|ChannelModelPublication|FetchModels' -count=1 -v`

- [ ] **Step 3: Implement protocol delegation and publication model**

```go
type ChannelModelPublication struct {
    ChannelID   int    `gorm:"primaryKey;autoIncrement:false"`
    PublicModel string `gorm:"primaryKey;type:varchar(255);autoIncrement:false"`
    Approved    bool   `gorm:"not null;default:false"`
    CreatedAt   int64  `gorm:"bigint;not null"`
    UpdatedAt   int64  `gorm:"bigint;not null"`
}
```

Expose `IsChannelModelPublicationApproved(channelID, publicModel)` through a lock-safe cache invalidated on confirmation/deletion. Saving a channel's raw model list does not approve. The confirm endpoint atomically updates selected channel models and publication rows; omitted upstream models remain unless selected for removal.

- [ ] **Step 4: Implement Default/Classic diff confirmation UI**

Show Added/Retained/Missing sections, default missing checkboxes off, require confirmation, and keep unpriced/ungrouped warning. Use type-native channel icon from existing icon libraries and stable mobile dialog dimensions.

- [ ] **Step 5: Run backend/frontend tests and commit**

Run focused Go tests, Default channel tests/typecheck, Classic channel tests/build.

```bash
git add constant common dto relay/channel/sub2api model/channel_model_publication.go model/channel_model_publication_test.go model/main.go controller/channel.go controller/channel_upstream_update.go controller/channel_upstream_update_test.go web/default/src/features/channels web/classic/src/components/table/channels
git commit -m "feat(channel): add sub2api discovery"
```

### Task 6: Create A Pure Error Parser And One Diagnostic Sink

**Files:**
- Modify: `service/error.go`, `service/error_test.go`
- Create: `service/upstream_diagnostic.go`, `service/upstream_diagnostic_test.go`
- Modify: `controller/relay.go`
- Modify: failed usage-log construction
- Add operation settings model/controller tests.

- [ ] **Step 1: Write failing bounded-read/redaction/dedupe tests**

Cover invalid JSON, HTML, empty-message JSON, useful-message fallback behavior, all-errors mode, 64KiB cap, 256/8192 character bounds, UTF-8-safe truncation, Debug parity, structured secrets, Bearer/JWT/API key/prompt text redaction, digest, 60-second coalescing, zero window, bounded cache, sync into the existing failed row, and sink failure preserving the original error/refund.

- [ ] **Step 2: Run RED**

Run: `go test ./service ./controller -run 'RelayError|UpstreamDiagnostic' -count=1 -v`

- [ ] **Step 3: Split parser and sink**

```go
type UpstreamDiagnostic struct {
    ChannelID int
    Status int
    RequestID string
    Preview string
    Digest string
    RawLength int64
    Truncated bool
}

func ParseUpstreamError(resp *http.Response, cfg DiagnosticSnapshot) (*types.NewAPIError, *UpstreamDiagnostic)
func (s *DiagnosticSink) Record(ctx context.Context, d UpstreamDiagnostic, failedLog *model.Log)
```

Read once through `io.LimitReader(64<<10 + 1)`. Remove the full-body `showBodyWhenFail` branch. Never put preview bytes into error metadata. The sink writes one server preview and attaches to the already-created failed log `other` when enabled; duplicate summaries are server-log-only.

- [ ] **Step 4: Run GREEN/race tests and commit**

```bash
git add service/error.go service/error_test.go service/upstream_diagnostic.go service/upstream_diagnostic_test.go controller/relay.go setting/operation_setting model/option.go
git commit -m "fix(relay): bound upstream diagnostics"
```

### Task 7: Expose Settings, Compact Cost Details, And Ledger Aggregation

**Files:**
- Modify Default/Classic tool price settings and locale files.
- Create Default/Classic Alpha/diagnostic setting sections.
- Modify usage-log detail types/components/helpers/tests.
- Modify `service/wallet_ledger.go` and tests plus the two wallet ledger pages.

- [ ] **Step 1: Write failing API/UI and aggregation tests**

Require tool switch, multiplier mode, image matrix/flat mode, prefix overrides, Alpha switch, diagnostic mode/cap/sync/dedupe, atomic invalid-save rejection, high-price confirmation, list total-only and compact detail breakdown, and SQL/service grouping before pagination by `(reference_type, reference_id)`.

- [ ] **Step 2: Run RED in Go, Default, and Classic**

Run focused `WalletLedger|ToolPrice|DiagnosticSetting` Go tests and the matching frontend test files.

- [ ] **Step 3: Implement settings and log/ledger projection**

Keep one setting save audit per document. Render `billing_breakdown` and `tool_surcharges` only in details. Group ledger rows in the query/service layer before offset/limit and expose `entry_count` plus optional expanded immutable entries; do not mutate the ledger table.

- [ ] **Step 4: Run GREEN, typecheck/build, and commit**

```bash
git add service/wallet_ledger.go service/*wallet_ledger*test.go web/default/src/features/system-settings web/default/src/features/usage-logs web/default/src/features/wallet web/classic/src/pages/Setting web/classic/src/components/table web/classic/src/components/topup
git commit -m "feat(admin): configure relay billing diagnostics"
```

### Task 8: Full Billing And Channel Acceptance

- [ ] Run `go test ./... -count=1`, `go vet ./...`, and focused race tests for meter, billing, diagnostics, publication, and ledger aggregation.
- [ ] Run Default full tests/typecheck/lint/build and Classic full tests/eslint/build.
- [ ] Run SQLite/MySQL/PostgreSQL migration tests against disposable databases.
- [ ] Add Playwright desktop/mobile cases for settings, Sub2API fetch/confirm, Alpha channel test, compact billing detail, hidden unapproved candidates, and Snow-to-Default admin navigation.
- [ ] Capture retained MySQL counts and balances, rebuild `cloveapi-current-source-mysql-qa` on port `18104` without replacing `cloveapi-current-source-mysql-qa-data`, run browser suites, and compare the baseline.
- [ ] Inspect runtime logs for panic, SQL errors, duplicate consume rows, negative balances, unredacted previews, and repeated auth noise.
- [ ] Run `git diff --check` and commit only tested residual fixes. Do not deploy, push, publish a release, or edit `web/snow/dist`.
