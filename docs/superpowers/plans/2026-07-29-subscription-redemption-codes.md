# Subscription Redemption Codes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add wallet-funded, multi-unit subscription redemption codes while preserving existing balance redemption behavior and accounting invariants.

**Architecture:** Keep `UserRedemptionCode` as the lifecycle and money ledger aggregate, store immutable subscription terms in a one-to-one entitlement row, and put subscription-specific transactions in a focused service module. Claiming locks plan, claimant, and code in a fixed order; plan disablement synchronously seals affected codes and an idempotent worker refunds them in batches.

**Tech Stack:** Go 1.22+, Gin, GORM v2, SQLite/MySQL/PostgreSQL, React 19/TypeScript/Base UI, React 18/Semi UI, Bun tests.

---

## File Structure

- Modify `model/user_redemption.go`: reward types, fixed-unit kind, refund-pending status, entitlement and claim fields.
- Modify `model/subscription.go`: subscription reset snapshot fields and snapshot-aware reset calculations.
- Modify `model/main.go`: cross-database migration registration.
- Modify `setting/user_redemption_setting/config.go`: subscription-code feature switch.
- Create `service/user_redemption_subscription.go`: create, claim, remaining-unit settlement, and plan sealing transactions.
- Modify `service/user_redemption.go`: reward-type dispatch, shared views/list filters, automatic expiry branch.
- Modify `service/user_redemption_governance.go`: subscription-aware refund/forfeit and batch exclusions.
- Create `service/user_redemption_refund_task.go`: batched idempotent `refund_pending` processing.
- Modify `service/subscription_reset_task.go`: run refund batches from the existing master-node maintenance loop.
- Modify `controller/user_redemption.go`: typed create/claim contracts and error codes.
- Modify `controller/subscription.go`: central plan status transition service call.
- Modify `controller/security_setting.go` and its feature-setting tests to expose the switch through the established security settings contract.
- Modify `web/default/src/features/user-redemptions/*`: types, API, creation controls, tables, details, filters.
- Modify `web/classic/src/components/user-redemptions/*`: Classic parity.
- Tests remain beside the corresponding backend and frontend modules.

### Task 1: Persist Reward Types and Subscription Entitlements

**Files:**
- Modify: `model/user_redemption.go`
- Modify: `model/main.go`
- Modify: `setting/user_redemption_setting/config.go`
- Test: `model/user_redemption_test.go`
- Test: `setting/user_redemption_setting/config_test.go`

- [ ] **Step 1: Write failing model and setting tests**

```go
func TestSubscriptionEntitlementUsesOneRowPerCode(t *testing.T) {
    db := setupUserRedemptionModelTestDB(t)
    code := UserRedemptionCode{Code: "SUB-ONE", RewardType: UserRedemptionRewardSubscription}
    require.NoError(t, db.Create(&code).Error)
    first := UserRedemptionSubscriptionEntitlement{CodeID: code.ID, PlanID: 9, UnitPaidQuota: 500}
    require.NoError(t, db.Create(&first).Error)
    duplicate := first
    duplicate.ID = 0
    require.Error(t, db.Create(&duplicate).Error)
}

func TestUserRedemptionSettingsExposeSubscriptionSwitch(t *testing.T) {
    withUserRedemptionOptions(t, map[string]string{SubscriptionEnabledKey: "false"})
    assert.False(t, GetSettings().SubscriptionEnabled)
}
```

- [ ] **Step 2: Run tests and verify failure**

Run: `go test ./model ./setting/user_redemption_setting -run 'SubscriptionEntitlement|SubscriptionSwitch' -count=1`

Expected: FAIL because reward constants, entitlement model, and setting key do not exist.

- [ ] **Step 3: Add models, constants, and safe defaults**

```go
const (
    UserRedemptionRewardBalance      = "balance"
    UserRedemptionRewardSubscription = "subscription"
    UserRedemptionKindFixed          = "fixed"
    UserRedemptionStatusRefundPending = "refund_pending"
)

type UserRedemptionSubscriptionEntitlement struct {
    ID                          int64  `gorm:"primaryKey;autoIncrement"`
    CodeID                      int64  `gorm:"not null;uniqueIndex"`
    PlanID                      int    `gorm:"not null;index"`
    PlanTitle                   string `gorm:"type:varchar(128);not null"`
    PlanSubtitle                string `gorm:"type:varchar(255);not null;default:''"`
    Currency                    string `gorm:"type:varchar(8);not null"`
    PriceAmount                 float64 `gorm:"type:decimal(10,6);not null"`
    UnitPaidQuota               int64  `gorm:"type:bigint;not null"`
    DurationUnit                string `gorm:"type:varchar(16);not null"`
    DurationValue               int    `gorm:"not null"`
    CustomSeconds               int64  `gorm:"type:bigint;not null"`
    TotalAmount                 int64  `gorm:"type:bigint;not null"`
    QuotaResetPeriod            string `gorm:"type:varchar(16);not null"`
    QuotaResetCustomSeconds     int64  `gorm:"type:bigint;not null"`
    AllowWalletOverflow         bool   `gorm:"not null"`
    UpgradeGroup                string `gorm:"type:varchar(64);not null"`
    DowngradeGroup              string `gorm:"type:varchar(64);not null"`
    MaxPurchasePerUser          int    `gorm:"not null"`
    CreatedAt                   int64  `gorm:"bigint"`
    UpdatedAt                   int64  `gorm:"bigint"`
}
```

Add `RewardType` to codes and claims, `UserSubscriptionID` to claims, register the entitlement in both migration paths, and migrate blank reward types to `balance`. Add `SubscriptionEnabledKey` with a code-owned default of `true` and round-trip validation.

- [ ] **Step 4: Run focused tests**

Run: `go test ./model ./setting/user_redemption_setting -count=1`

Expected: PASS.

- [ ] **Step 5: Commit schema and setting changes**

```bash
git add model/user_redemption.go model/main.go model/user_redemption_test.go setting/user_redemption_setting/config.go setting/user_redemption_setting/config_test.go
git commit -m "feat(redemption): add subscription entitlement schema"
```

### Task 2: Make Subscription Resets Snapshot-Safe

**Files:**
- Modify: `model/subscription.go`
- Test: `model/subscription_reset_test.go`

- [ ] **Step 1: Write a failing reset snapshot test**

```go
func TestRedemptionSubscriptionResetUsesInstanceSnapshot(t *testing.T) {
    setupSubscriptionTestDB(t)
    plan := createPlan(t, SubscriptionResetDaily)
    sub := createSubscription(t, plan)
    sub.Source = "redemption"
    sub.QuotaResetPeriod = SubscriptionResetWeekly
    sub.QuotaResetCustomSeconds = 0
    require.NoError(t, DB.Save(&sub).Error)
    plan.QuotaResetPeriod = SubscriptionResetHourly
    require.NoError(t, DB.Save(&plan).Error)

    require.NoError(t, maybeResetUserSubscriptionWithPlanTx(DB, &sub, &plan, sub.NextResetTime))
    assert.Equal(t, SubscriptionResetWeekly, sub.QuotaResetPeriod)
    assert.Greater(t, sub.NextResetTime, sub.LastResetTime+int64(6*24*time.Hour/time.Second))
}
```

- [ ] **Step 2: Verify the old implementation fails**

Run: `go test ./model -run 'ResetUsesInstanceSnapshot' -count=1`

Expected: FAIL because reset calculations still use the mutated plan.

- [ ] **Step 3: Add reset snapshot fields and resolve reset policy per instance**

```go
func subscriptionResetPlan(sub *UserSubscription, current *SubscriptionPlan) SubscriptionPlan {
    snapshot := *current
    if strings.TrimSpace(sub.QuotaResetPeriod) != "" {
        snapshot.QuotaResetPeriod = sub.QuotaResetPeriod
        snapshot.QuotaResetCustomSeconds = sub.QuotaResetCustomSeconds
    }
    return snapshot
}
```

Populate snapshot fields in `CreateUserSubscriptionFromPlanTx`, use the resolved plan in automatic and administrator resets, and retain current-plan fallback for legacy blank rows.

- [ ] **Step 4: Run subscription model tests**

Run: `go test ./model -run 'Subscription|Reset' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit snapshot behavior**

```bash
git add model/subscription.go model/subscription_reset_test.go
git commit -m "fix(subscription): preserve reset policy snapshots"
```

### Task 3: Create Wallet-Funded Subscription Codes

**Files:**
- Create: `service/user_redemption_subscription.go`
- Modify: `service/user_redemption.go`
- Test: `service/user_redemption_test.go`

- [ ] **Step 1: Add failing creation/accounting tests**

```go
func TestCreateSubscriptionRedemptionReservesExactUnits(t *testing.T) {
    setupUserRedemptionTestDB(t)
    creator := createUserWithQuota(t, 1, 10_000)
    plan := createBalancePlan(t, 1.25, 1000)
    code, err := CreateSubscriptionRedemptionCode(context.Background(), CreateSubscriptionRedemptionInput{
        CreatorUserID: creator.Id, PlanID: plan.Id, Units: 3, ExpiresAt: common.GetTimestamp() + 3600,
    })
    require.NoError(t, err)
    assert.Equal(t, 3, code.MaxClaims)
    assert.Equal(t, int64(3)*mustPlanWalletQuota(t, plan), code.TotalQuota)
    assert.Equal(t, int64(10_000)-code.TotalQuota, getUserQuota(t, creator.Id))
}
```

Also cover disabled plan, disabled balance pay, feature switch, zero price, insufficient funds, claim cap, and checked multiplication overflow.

- [ ] **Step 2: Run creation tests and verify failure**

Run: `go test ./service -run 'CreateSubscriptionRedemption' -count=1`

Expected: FAIL because the subscription creation service is absent.

- [ ] **Step 3: Implement the dedicated creation transaction**

```go
type CreateSubscriptionRedemptionInput struct {
    CreatorUserID int
    PlanID        int
    Units         int
    ExpiresAt     int64
}

func CreateSubscriptionRedemptionCode(ctx context.Context, input CreateSubscriptionRedemptionInput) (*model.UserRedemptionCode, error) {
    // Validate settings and expiry before opening the transaction.
    // In the transaction: lock plan, lock creator, calculate exact unit quota,
    // create code + entitlement, adjust wallet with a stable operation ID,
    // enqueue one business audit event, and return the row.
}
```

Use `common.QuotaFromDecimalStrict`/existing subscription balance conversion and an overflow-checked integer multiplication. Do not call the personal purchase function that creates an immediate subscription.

- [ ] **Step 4: Run service tests**

Run: `go test ./service -run 'SubscriptionRedemption|UserRedemption' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit creation flow**

```bash
git add service/user_redemption_subscription.go service/user_redemption.go service/user_redemption_test.go
git commit -m "feat(redemption): purchase subscription code units"
```

### Task 4: Claim Subscription Units with Concurrency-Safe Limits

**Files:**
- Modify: `service/user_redemption_subscription.go`
- Modify: `service/user_redemption.go`
- Modify: `service/wallet_redemption.go`
- Test: `service/user_redemption_concurrency_test.go`
- Test: `service/user_redemption_test.go`

- [ ] **Step 1: Write failing claim and concurrency tests**

```go
func TestConcurrentSubscriptionClaimsCannotExceedPlanLimit(t *testing.T) {
    codeA, codeB, claimer := setupTwoSubscriptionCodesWithLimit(t, 1)
    results := claimConcurrently(t, claimer.Id, codeA.Code, codeB.Code)
    assert.Equal(t, 1, countSuccessfulClaims(results))
    assert.Equal(t, int64(1), countUserPlanSubscriptions(t, claimer.Id, codeA.PlanID))
}
```

Cover expected-type mismatch, same-code repeat, independent parallel subscription creation, exhausted code, and a free plan with `TotalQuota == 0`.

- [ ] **Step 2: Run tests and verify the race contract fails**

Run: `go test ./service -run 'SubscriptionClaim|ConcurrentSubscriptionClaims' -count=1`

Expected: FAIL because claim dispatch and claimant locking do not exist.

- [ ] **Step 3: Implement fixed-order locking and snapshot activation**

```go
type ClaimUserRedemptionInput struct {
    ClaimerUserID     int
    Code              string
    ExpectedRewardType string
}

// Lock order for subscription claims:
// 1. plan row, 2. claimant user row, 3. redemption code row.
// Then re-read entitlement, count plan subscriptions, create the snapshot
// subscription, create the unique claim, and update code counters atomically.
```

Return a typed mismatch error containing the actual reward type. Invalidate the claimant group/cache after the transaction if the snapshot upgrades their group.

- [ ] **Step 4: Run race-focused tests**

Run: `go test -race ./service -run 'SubscriptionClaim|ConcurrentSubscriptionClaims' -count=1`

Expected: PASS with one successful limited claim and no duplicate subscriptions.

- [ ] **Step 5: Commit claim flow**

```bash
git add service/user_redemption_subscription.go service/user_redemption.go service/wallet_redemption.go service/user_redemption_concurrency_test.go service/user_redemption_test.go
git commit -m "feat(redemption): claim subscription code units"
```

### Task 5: Seal and Refund Subscription Codes

**Files:**
- Modify: `service/user_redemption_subscription.go`
- Modify: `service/user_redemption_governance.go`
- Create: `service/user_redemption_refund_task.go`
- Modify: `service/subscription_reset_task.go`
- Modify: `controller/subscription.go`
- Test: `service/user_redemption_test.go`
- Test: `controller/user_redemption_test.go`

- [ ] **Step 1: Write failing lifecycle tests**

```go
func TestDisablePlanSealsCodesBeforeRefundWorker(t *testing.T) {
    code := createPartiallyClaimedSubscriptionCode(t, 5, 2)
    require.NoError(t, SetSubscriptionPlanEnabled(context.Background(), code.PlanID, false, 1))
    assert.Equal(t, model.UserRedemptionStatusRefundPending, getCode(t, code.ID).Status)
    _, err := ClaimUserRedemptionCode(context.Background(), ClaimUserRedemptionInput{ClaimerUserID: 99, Code: code.Code})
    require.ErrorIs(t, err, ErrUserRedemptionInactive)
    count, err := FinalizePendingSubscriptionRedemptionRefunds(context.Background(), 100)
    require.NoError(t, err)
    assert.Equal(t, 1, count)
    assert.Equal(t, unitQuota(code)*3, refundedQuota(t, code.ID))
}
```

Also cover creator cancellation, expiry, retry idempotency, plan re-enable not reviving old codes, and administrator forfeit.

- [ ] **Step 2: Run lifecycle tests and verify failure**

Run: `go test ./service ./controller -run 'SubscriptionRedemption.*Refund|DisablePlanSeals' -count=1`

Expected: FAIL because sealing and worker functions do not exist.

- [ ] **Step 3: Implement synchronous sealing and batched settlement**

```go
func SealSubscriptionRedemptionsForPlanTx(tx *gorm.DB, planID int, actorID int, now int64) error {
    subquery := tx.Model(&model.UserRedemptionSubscriptionEntitlement{}).
        Select("code_id").Where("plan_id = ?", planID)
    return tx.Model(&model.UserRedemptionCode{}).
        Where("id IN (?) AND reward_type = ? AND status IN ?", subquery,
            model.UserRedemptionRewardSubscription,
            []string{model.UserRedemptionStatusActive, model.UserRedemptionStatusDisabled}).
        Updates(map[string]any{"status": model.UserRedemptionStatusRefundPending, "terminal_at": now, "terminal_actor_id": actorID, "terminal_reason": "plan_disabled"}).Error
}
```

Centralize both full plan update and status patch through one service. The worker locks one code at a time and uses `user-redemption:{id}:refund` as the stable wallet operation ID. Invoke a bounded batch from the existing master maintenance loop.

- [ ] **Step 4: Run lifecycle and governance tests**

Run: `go test ./service ./controller -run 'UserRedemption|SubscriptionPlan' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit lifecycle changes**

```bash
git add service/user_redemption_subscription.go service/user_redemption_governance.go service/user_redemption_refund_task.go service/subscription_reset_task.go controller/subscription.go service/user_redemption_test.go controller/user_redemption_test.go
git commit -m "feat(redemption): refund unused subscription units"
```

### Task 6: Extend API Contracts and Default Frontend

**Files:**
- Modify: `controller/user_redemption.go`
- Modify: `i18n/keys.go`
- Modify: `i18n/locales/en.yaml`
- Modify: `i18n/locales/zh-CN.yaml`
- Modify: `i18n/locales/zh-TW.yaml`
- Modify: `controller/security_setting.go`
- Modify: `controller/feature_setting_test.go`
- Modify: `web/default/src/features/user-redemptions/types.ts`
- Modify: `web/default/src/features/user-redemptions/api.ts`
- Modify: `web/default/src/features/user-redemptions/create-state.ts`
- Modify: `web/default/src/features/user-redemptions/components/create-dialog.tsx`
- Modify: `web/default/src/features/user-redemptions/components/user-redemptions-table.tsx`
- Modify: `web/default/src/features/user-redemptions/components/user-redemption-detail-dialog.tsx`
- Modify: `web/default/src/features/system-settings/types.ts`
- Modify: `web/default/src/features/system-settings/security/user-redemption-controls.tsx`
- Modify: `web/default/src/features/system-settings/security/feature-controls-section.tsx`
- Test: `controller/user_redemption_test.go`
- Test: `web/default/src/features/user-redemptions/api.test.ts`
- Test: `web/default/src/features/user-redemptions/user-redemptions.state.test.ts`

- [ ] **Step 1: Write API and UI state tests**

```ts
test('builds a subscription redemption purchase request', () => {
  expect(buildCreateRedemptionRequest({ rewardType: 'subscription', planId: 7, units: 3, expiresAt: 1000 })).toEqual({
    reward_type: 'subscription', plan_id: 7, units: 3, expires_at: 1000,
  })
})
```

- [ ] **Step 2: Verify tests fail**

Run: `go test ./controller -run UserRedemption -count=1`

Run: `cd web/default && bun test src/features/user-redemptions`

Expected: FAIL because union request types and subscription UI state are absent.

- [ ] **Step 3: Implement contracts and work-focused controls**

Use a discriminated request union, a segmented reward-type control, existing subscription plan query data, a numeric stepper for units, and a read-only computed total. Add `reward_type` filters and render plan title plus `claim_count / max_claims`; keep batch pending settlement limited to `balance` records.

- [ ] **Step 4: Run backend and default frontend checks**

Run: `go test ./controller -run UserRedemption -count=1`

Run: `cd web/default && bun test src/features/user-redemptions && bun run typecheck && bun run lint`

Expected: PASS with no TypeScript or lint errors.

- [ ] **Step 5: Commit API and default UI**

```bash
git add controller/user_redemption.go controller/security_setting.go controller/feature_setting_test.go i18n web/default/src/features/user-redemptions web/default/src/features/system-settings/types.ts web/default/src/features/system-settings/security/user-redemption-controls.tsx web/default/src/features/system-settings/security/feature-controls-section.tsx
git commit -m "feat(web): manage subscription redemption codes"
```

### Task 7: Add Classic Parity and Full Verification

**Files:**
- Modify: `web/classic/src/components/user-redemptions/api.js`
- Modify: `web/classic/src/components/user-redemptions/CreateUserRedemptionModal.jsx`
- Modify: `web/classic/src/components/user-redemptions/UserRedemptionsTable.jsx`
- Modify: `web/classic/src/components/user-redemptions/UserRedemptionDetailModal.jsx`
- Modify: `web/classic/src/components/user-redemptions/user-redemption-state.js`
- Test: `web/classic/src/components/user-redemptions/user-redemptions-state.test.js`
- Test: `web/classic/src/checks/classic-feature-parity.test.js`

- [ ] **Step 1: Add failing Classic parity assertions**

```js
test('supports fixed subscription redemption units', () => {
  expect(buildCreatePayload({ rewardType: 'subscription', planId: 7, units: 2, expiresAt: 1000 })).toEqual({
    reward_type: 'subscription', plan_id: 7, units: 2, expires_at: 1000,
  });
});
```

- [ ] **Step 2: Verify Classic tests fail**

Run: `cd web/classic && bun test src/components/user-redemptions src/checks/classic-feature-parity.test.js`

Expected: FAIL because Classic only supports balance codes.

- [ ] **Step 3: Implement Classic controls and views**

Mirror the confirmed behavior with Semi UI segmented controls, Select, InputNumber, and existing table/modal patterns. Do not duplicate backend-derived pricing calculations beyond displaying the server-provided unit and total values.

- [ ] **Step 4: Run the complete feature verification**

Run: `go test ./model ./service ./controller ./setting/user_redemption_setting -count=1`

Run: `cd web/default && bun test src/features/user-redemptions && bun run build:check`

Run: `cd web/classic && bun test src/components/user-redemptions src/checks/classic-feature-parity.test.js && bun run build`

Expected: all commands PASS.

- [ ] **Step 5: Commit Classic parity**

```bash
git add web/classic/src/components/user-redemptions web/classic/src/checks/classic-feature-parity.test.js
git commit -m "feat(classic): support subscription redemption codes"
```

### Task 8: Local Browser Acceptance

**Files:**
- Modify only files required by defects found during acceptance.

- [ ] **Step 1: Start or rebuild the local stack without production deployment**

Run the repository's existing local Docker/dev workflow and preserve the attached local database. Confirm `http://127.0.0.1:18104` responds.

- [ ] **Step 2: Exercise creator flow**

Create a three-unit subscription code, verify exact wallet debit, inspect the code in user and admin lists, and claim from two distinct local users.

- [ ] **Step 3: Exercise governance flow**

Cancel the partially claimed code and verify only one remaining unit is refunded; disable another plan and verify associated codes seal before the refund worker runs.

- [ ] **Step 4: Capture desktop/mobile evidence**

Use Playwright at `1440x900` and `390x844` for creation, list, detail, and claim views. Verify no overflow, nested cards, or control overlap.

- [ ] **Step 5: Close acceptance findings through their owning task**

If acceptance finds a defect, return to the task that owns that file, add a failing regression test there, fix it, rerun that task's exact verification command, and use that task's listed `git add` paths. If acceptance finds no defect, do not create an empty acceptance commit.

Do not push, deploy, publish a Release, or change the production version.
