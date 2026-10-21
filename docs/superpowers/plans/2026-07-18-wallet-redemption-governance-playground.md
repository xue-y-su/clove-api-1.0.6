# Wallet Redemption Governance and Playground Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver the approved wallet invitation/redemption experience, complete administrator governance and financial settlement for user-created redemption codes, make Playground reasoning/image selection follow the final semantics, and expose custom API Keys under advanced settings in both themes.

**Architecture:** Preserve Router -> Controller -> Service -> Model ownership. Keep system redemption codes and wallet-funded user redemption codes separate, centralize user-redemption transitions in one transactional service state machine, and use the existing wallet ledger plus audit outbox for idempotent accounting and durable structured logs. Default and Classic consume the same API contracts while using their native component systems.

**Tech Stack:** Go 1.22+, Gin, GORM, SQLite/MySQL/PostgreSQL, React 19/TypeScript/Bun, React 18/JavaScript/Bun, Redis, Docker Compose.

---

## File Map

- Modify `model/user_redemption.go`: add governance, forfeiture, and soft-delete fields and statuses.
- Modify `model/user_redemption_test.go`: protect portable indexes and migration defaults.
- Modify `model/audit_outbox.go`, `service/audit_outbox.go`: separate true actor from log owner.
- Modify audit outbox tests: prove durable log ownership and cross-database behavior.
- Split redemption service responsibilities between `service/user_redemption.go` and a new `service/user_redemption_governance.go`.
- Modify redemption service/controller/router tests: cover the state table, accounting, permissions, DTO redaction, and APIs.
- Modify Default wallet, invitation, redemption, sidebar, route, Playground, image settings, API-key drawer, clipboard consumers, and locales.
- Modify Classic top-up, invitation, redemption, Playground, image panel, token modal, clipboard consumers, and locales.
- Update delivery/readiness documentation only after code and QA pass.

## Task 1: Redemption Model and Accounting Invariants

**Files:**
- Modify: `model/user_redemption.go`
- Modify: `model/user_redemption_test.go`
- Test: `service/user_redemption_test.go`

- [x] **Step 1: Write failing model migration tests**

```go
func TestUserRedemptionGovernanceFieldsMigratePortably(t *testing.T) {
    setupUserRedemptionModelTest(t)
    require.NoError(t, DB.AutoMigrate(&UserRedemptionCode{}))
    assert.True(t, DB.Migrator().HasColumn(&UserRedemptionCode{}, "ForfeitedQuota"))
    assert.True(t, DB.Migrator().HasColumn(&UserRedemptionCode{}, "DeletedAt"))
}
```

- [x] **Step 2: Run the model test and verify RED**

Run: `go test ./model -run TestUserRedemptionGovernanceFieldsMigratePortably -count=1`

Expected: FAIL because the governance fields do not exist.

- [x] **Step 3: Add portable statuses and fields**

Use integer timestamps and scalar defaults; do not use `gorm.DeletedAt` or database-specific types.

- [x] **Step 4: Write a failing remaining-quota test**

```go
func TestUserRedemptionRemainingQuotaAccountsForForfeiture(t *testing.T) {
    row := model.UserRedemptionCode{
        TotalQuota: 100, ClaimedQuota: 25, RefundedQuota: 15, ForfeitedQuota: 10,
    }
    assert.Equal(t, 50, UserRedemptionRemainingQuota(row))
}
```

- [x] **Step 5: Implement the shared remaining-quota helper**

Clamp malformed historical negative results to zero; all new transitions must preserve exact equality.

- [x] **Step 6: Run targeted tests and verify GREEN**

Run: `go test ./model ./service -run 'TestUserRedemption(GovernanceFields|RemainingQuota)' -count=1`

Expected: PASS.

## Task 2: Transactional Governance State Machine

**Files:**
- Create: `service/user_redemption_governance.go`
- Modify: `service/user_redemption.go`
- Modify: `service/user_redemption_test.go`

- [x] **Step 1: Write a failing transition-table test**

```go
func TestAdminUserRedemptionGovernanceTransitions(t *testing.T) {
    tests := []struct {
        name       string
        start      string
        action     UserRedemptionAdminAction
        settlement UserRedemptionSettlement
        want       string
    }{
        {"disable", model.UserRedemptionStatusActive, UserRedemptionActionDisable, "", model.UserRedemptionStatusDisabled},
        {"refund", model.UserRedemptionStatusDisabled, UserRedemptionActionTerminate, UserRedemptionSettlementRefund, model.UserRedemptionStatusCancelled},
        {"forfeit", model.UserRedemptionStatusActive, UserRedemptionActionTerminate, UserRedemptionSettlementForfeit, model.UserRedemptionStatusForfeited},
    }
    // Seed, execute, and assert status plus wallet balance for each row.
}
```

- [x] **Step 2: Run the transition test and verify RED**

Run: `go test ./service -run TestAdminUserRedemptionGovernanceTransitions -count=1`

Expected: FAIL because the action types and statuses are absent.

- [x] **Step 3: Implement one locked transition entry point**

```go
type UserRedemptionAdminCommand struct {
    Actor      UserIdentity
    CodeID     int64
    Action     UserRedemptionAdminAction
    Settlement UserRedemptionSettlement
    Reason     string
}
```

The service locks the row, re-checks state, computes remaining quota, settles, updates governance fields, enqueues audit, and returns the result in one transaction.

- [x] **Step 4: Write failing idempotency and conflict tests**

Repeat identical commands and assert one ledger entry plus one outbox event. Attempt the opposite settlement after termination and assert a stable conflict error.

- [x] **Step 5: Implement refund, forfeiture, expiry, and soft delete**

Keep existing reserve/refund operation IDs. Add a stable forfeiture operation ID. Deleting active/disabled rows settles and marks deleted in the same transaction.

- [x] **Step 6: Pass expiry and transition tests**

Run: `go test ./service -run 'Test(UserRedemptionGovernance|FinalizeExpiredUserRedemption)' -count=1`

Expected: PASS.

## Task 3: Durable User Logs and Administrator Audit

**Files:**
- Modify: `model/audit_outbox.go`
- Modify: `service/audit_outbox.go`
- Modify: `service/audit_outbox_test.go`
- Modify: `service/audit_outbox_mysql_test.go`
- Modify: redemption service files

- [x] **Step 1: Write a failing actor/log-owner test**

```go
func TestAuditOutboxPreservesActorWhileAssigningLogOwner(t *testing.T) {
    event := AuditEvent{
        EventID: "redemption:disable:1",
        Type: "user.redemption.disable",
        ActorID: 9,
        LogUserID: 4,
    }
    require.NoError(t, EnqueueWithTx(tx, event))
    // Deliver and assert log.UserId == 4 while admin_info.actor_id == 9.
}
```

- [x] **Step 2: Run the audit test and verify RED**

Run: `go test ./service -run TestAuditOutboxPreservesActorWhileAssigningLogOwner -count=1`

Expected: FAIL because `LogUserID` is unavailable.

- [x] **Step 3: Add backward-compatible log ownership**

Persist `LogUserID`; default it to `ActorID`. Include it in canonical snapshots, retries, and MySQL conflict checks.

- [x] **Step 4: Add stable redemption audit parameters**

Allow owner, claimant, settlement, remaining/refunded/forfeited quota, deletion, and reason fields. Keep full codes, API Keys, email, and claim IP out.

- [x] **Step 5: Emit every approved redemption action transactionally**

Use one stable event ID per business result. User-visible `op.params` stays sanitized; full actor/owner/claimant data stays admin-only.

- [ ] **Step 6: Run SQLite and MySQL audit tests**

Run: `go test ./service -run 'Test(AuditOutbox|UserRedemption.*Audit)' -count=1`

Expected: PASS.

## Task 4: Permission-Aware DTOs and APIs

**Files:**
- Modify: `service/user_redemption.go`
- Modify: `controller/user_redemption.go`
- Modify: `controller/user_redemption_test.go`
- Modify: `router/api-router.go`
- Modify: backend i18n keys/locales

- [ ] **Step 1: Write failing DTO redaction tests**

Creator detail returns a masked claimant name without claimant ID/email/IP. Admin detail returns claimant ID and full username without email/IP.

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./service ./controller -run 'Test.*UserRedemption.*(Detail|Redact|Admin)' -count=1`

Expected: FAIL because GORM models are serialized directly.

- [ ] **Step 3: Implement explicit list/detail DTOs**

Add server-derived `remaining_quota`, creator summary, deletion metadata, and role-specific claim views. Mask usernames by Unicode rune count.

- [ ] **Step 4: Write failing route tests for admin actions**

Cover disable, enable, refund termination, forfeit termination, and delete. Assert ordinary users are rejected and every Admin can manage every owner.

- [ ] **Step 5: Add controllers and routes**

Keep legacy admin cancel as refund termination and map invalid transitions to stable localized errors.

- [ ] **Step 6: Pass targeted API tests**

Run: `go test ./controller ./router -run 'Test.*UserRedemption' -count=1`

Expected: PASS.

## Task 5: Default Wallet, Invitation, and Redemption UX

**Files:**
- Modify: `web/default/src/features/wallet/index.tsx`
- Modify: `web/default/src/routes/_authenticated/wallet/index.tsx`
- Modify: invitation card/API and add an application dialog
- Modify: user-redemption API/types/components/index
- Modify: `web/default/src/routes/_authenticated/user-redemptions/index.tsx`
- Modify: `web/default/src/hooks/use-sidebar-data.ts`
- Modify: related Bun tests

- [ ] **Step 1: Write failing wallet composition tests**

Assert create/list controls exist, `WalletLedgerTable` is absent, the personal redemption link is absent, and invitation reason input exists only in a dialog.

- [ ] **Step 2: Run focused tests and verify RED**

Run: `bun test src/features/wallet src/features/invitations src/features/user-redemptions`

Expected: FAIL on the old page split, ledger table, and exposed invitation textarea.

- [ ] **Step 3: Build wallet redemption controls**

Use an admin-style side sheet with only mode, amount, red-packet claims, expiry, and caution text. Show the created code immediately with reliable copy feedback.

- [ ] **Step 4: Move self list/detail into wallet dialogs**

Keep soft-deleted history visible. Allow creator cancellation only for active rows. Fetch details lazily.

- [ ] **Step 5: Make `/user-redemptions` admin-only**

Move its sidebar item to Admin, remove create, add filters/actions, and redirect ordinary users to `/wallet?show_redemptions=true`.

- [ ] **Step 6: Compact invitation/reward layout**

Use apply button -> reason dialog, one main code copy action, withdraw/revoke dialogs, and content-sized sections.

- [ ] **Step 7: Run Default tests, typecheck, and touched-file lint**

```powershell
bun test src/features/wallet src/features/invitations src/features/user-redemptions
bun run typecheck
bunx oxlint src/features/wallet src/features/invitations src/features/user-redemptions src/routes/_authenticated/user-redemptions src/hooks/use-sidebar-data.ts
```

Expected: PASS.

## Task 6: Classic Wallet, Invitation, and Redemption UX

**Files:**
- Modify: `web/classic/src/components/topup/index.jsx`
- Modify: `web/classic/src/components/invitations/UserInvitationCard.jsx`
- Modify: `web/classic/src/pages/UserRedemption/index.jsx`
- Modify: Classic redemption components, routing/sidebar, and tests

- [ ] **Step 1: Write failing Classic composition tests**

Assert no wallet ledger, no ordinary-user creation page, compact invitation application modal, red-packet terminology, and no create button on the admin page.

- [ ] **Step 2: Run tests and verify RED**

Run: `bun test src/components/invitations src/components/user-redemptions src/pages/UserRedemption`

- [ ] **Step 3: Implement equivalent Semi UI behavior**

Match Default fields, permissions, transitions, redaction, confirmations, and copy feedback.

- [ ] **Step 4: Run tests and formatting checks**

```powershell
bun test src/components/invitations src/components/user-redemptions src/pages/UserRedemption
bunx prettier --check src/components/invitations src/components/user-redemptions src/pages/UserRedemption
```

Expected: PASS.

## Task 7: Playground Reasoning Best-Effort and Hidden Output

**Files:**
- Modify: `relay/helper/playground_reasoning.go` and tests
- Modify: OpenAI Playground reasoning tests
- Modify: Default payload, stream, message, request, storage, and control files/tests
- Modify: Classic Playground helper, request hook, settings panel, and tests

- [ ] **Step 1: Write failing backend tests for ignored unsupported controls**

Valid values on unsupported routes clear the field and return nil instead of an unsupported error.

- [ ] **Step 2: Run Go tests and verify RED**

Run: `go test ./relay/helper ./relay/channel/openai -run Test.*PlaygroundReasoning -count=1`

- [ ] **Step 3: Implement backend best-effort normalization**

Reject invalid values, map off -> none, preserve known adapter mappings, and record ignored controls without sending unsupported fields.

- [ ] **Step 4: Write failing frontend tests**

Cover always-visible controls, independent reasoning fields, complete tags, split tags, dangling tags, non-stream responses, debug state, and stored messages.

- [ ] **Step 5: Implement a request-scoped think-tag filter**

The filter owns pending delimiter text and inside-think state so split SSE chunks cannot leak tags. Off mode drops reasoning before message/debug/storage mutation.

- [ ] **Step 6: Enable controls and add cost tooltip**

Remove model-name gating in both themes. Low/medium/high still display reasoning; off ends with a stable no-visible-content result if needed.

- [ ] **Step 7: Run Go and frontend reasoning tests**

Expected: PASS.

## Task 8: Playground Image Model List

**Files:**
- Modify: `controller/user.go` and adjacent tests
- Modify: image-model policy service/tests as compatibility cleanup requires
- Modify: Default Playground API/image panel/settings
- Modify: Classic image panel/settings

- [ ] **Step 1: Write failing tests expecting all authorized group models**

Image mode must use the same model list as chat for an authorized group and reject unauthorized groups.

- [ ] **Step 2: Verify RED on endpoint filtering**

Run relevant Go and Bun tests; models lacking image endpoint metadata should currently be omitted.

- [ ] **Step 3: Reuse the ordinary group model endpoint**

Remove `endpoint_type=image-generation` in both frontends. Unsupported models fail normally through `/pg/images/generations`.

- [ ] **Step 4: Remove obsolete image-policy controls**

Keep stored option keys readable for compatibility, but stop applying them to the selector.

- [ ] **Step 5: Run model-list and image-panel tests**

Expected: PASS.

## Task 9: Custom API Key and Clipboard Consistency

**Files:**
- Modify: Default API-key form/drawer/tests
- Modify: Classic token modal
- Modify: touched invitation/redemption/image copy consumers
- Modify: shared copy helper tests

- [ ] **Step 1: Write a failing batch omission test**

```ts
test('omits custom_key for batch creation', () => {
  expect(
    transformFormDataToPayload({
      ...defaults,
      tokenCount: 2,
      custom_key: 'abcdefghijklmnop',
    })
  ).not.toHaveProperty('custom_key')
})
```

Add assertions that the field is under Advanced Settings and absent during edit.

- [ ] **Step 2: Run tests and verify RED**

Run: `bun test src/features/keys/lib/api-key-form.test.ts`

- [ ] **Step 3: Implement create-only advanced fields**

Clear stale values when switching to batch, preserve backend collision handling, and never expose the field during edit.

- [ ] **Step 4: Replace direct clipboard calls in touched features**

Use the existing Default utility and Classic `copy()` helper. Every action reports success/failure.

- [ ] **Step 5: Run key and clipboard tests**

Expected: PASS.

## Task 10: Internationalization and Full Verification

**Files:**
- Modify: Default locale JSON and static-key tests
- Modify: Classic locale JSON and i18n tests
- Modify: delivery/readiness docs after verification

- [ ] **Step 1: Add failing locale coverage tests**

Cover red packet, governance states/actions, settlement warnings, invitation dialogs, reasoning tooltip, no-visible-content, and custom-key descriptions.

- [ ] **Step 2: Fill all maintained locales**

Run Default and Classic i18n checks. Do not leave English fallback text in non-English locale files.

- [ ] **Step 3: Run targeted and full Go tests**

```powershell
go test ./model ./service ./controller ./router ./relay/helper ./relay/channel/openai -count=1
go test ./... -count=1
```

- [ ] **Step 4: Run Default full checks**

```powershell
bun test
bun run typecheck
bun run lint
bun run build
```

- [ ] **Step 5: Run Classic full checks**

```powershell
bun test
bun run eslint
bun run i18n:lint
bun run build
```

- [ ] **Step 6: Run MySQL and Docker QA**

Rebuild the QA image, migrate `new_api_qa`, verify Redis cache invalidation, execute concurrency/refund/forfeit cases, and confirm the QA account still authenticates.

- [ ] **Step 7: Run browser acceptance**

Verify user/Admin workflows, desktop/mobile layout, no 500s, no untranslated controls, no duplicate copy buttons, and no leaked reasoning while off.

- [ ] **Step 8: Review the final diff against the approved design**

Every acceptance criterion in `docs/superpowers/specs/2026-07-18-wallet-redemption-governance-playground-design.md` must have source, automated tests, and runtime evidence.

No commit step is included because the user has not requested a commit.
