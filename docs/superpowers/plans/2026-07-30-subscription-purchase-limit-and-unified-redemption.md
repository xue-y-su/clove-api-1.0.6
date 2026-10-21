# Subscription Purchase Limit And Unified Redemption Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enforce one lifetime per-user subscription purchase limit across every acquisition path, provide one auto-detecting redemption endpoint and wallet input, and render every subscription-redemption audit event with a localized operation name.

**Architecture:** A durable `(user_id, plan_id)` counter becomes the sole transactional purchase-limit authority inside `CreateUserSubscriptionFromPlanTx`; all payment, admin, and redemption paths already converge there. A unified service routes a code by its server-owned table/type and returns a discriminated result while legacy endpoints remain intact. Default and Classic remove their reward-type selector and consume the same endpoint; their audit template maps remain parity-tested against the backend event inventory.

**Tech Stack:** Go 1.25, Gin, GORM, SQLite/MySQL/PostgreSQL, React/TypeScript, Classic React/JavaScript, Bun, Playwright, Docker with the retained MySQL volume.

---

## File Structure

- Create `model/subscription_purchase_counter.go`: counter model, lock/read/increment helpers, strict positive limit resolver, and idempotent evidence backfill.
- Create `model/subscription_purchase_counter_test.go`: migration, backfill, monotonicity, rollback, deletion, and concurrency tests.
- Modify `model/main.go`: migrate and backfill the counter without destructive table operations.
- Modify `model/subscription.go`: replace current-row counting with the counter in `CreateUserSubscriptionFromPlanTx` and expose the authoritative count to callers.
- Modify `service/user_redemption_subscription.go`: use the stricter current/snapshot limit and remove its duplicate row-count check.
- Modify `service/user_redemption_test.go` and `service/user_redemption_subscription_hardening_test.go`: sequential and concurrent bypass regressions.
- Modify `service/wallet_redemption.go` and its tests: extend the existing ambiguity-safe administrator/user balance routing to subscription codes and a discriminated result; keep its legacy wrapper.
- Create `controller/redemption_claim.go` and `controller/redemption_claim_test.go`: stable unified response/error contract.
- Modify `router/api-router.go` and `router/invitation_routes_test.go`: authenticated critical-write route.
- Modify Default wallet API, hook, page, form card, tests, and all seven locale JSON files.
- Modify Classic wallet page/card, tests, audit helper/tests, and all eight locale JSON files.
- Modify `web/default/src/features/usage-logs/lib/format.ts` and `audit-locales.test.ts` for full event coverage.
- Modify `web/default/e2e/production-business-flows.spec.ts` for browser and retained-database acceptance.

### Task 1: Add A Monotonic Purchase Counter And Safe Backfill

**Files:**
- Create: `model/subscription_purchase_counter.go`
- Create: `model/subscription_purchase_counter_test.go`
- Modify: `model/main.go`

- [ ] **Step 1: Write failing counter and effective-limit tests**

```go
func TestStricterPositiveSubscriptionLimit(t *testing.T) {
    cases := []struct{ snapshot, current, want int }{
        {0, 0, 0}, {0, 2, 2}, {3, 0, 3}, {3, 2, 2}, {1, 4, 1},
    }
    for _, tc := range cases {
        assert.Equal(t, tc.want, StricterPositiveSubscriptionLimit(tc.snapshot, tc.current))
    }
}

func TestPurchaseCounterDoesNotDecreaseWhenSubscriptionIsDeleted(t *testing.T) {
    db := setupSubscriptionCounterDB(t)
    require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
        return incrementSubscriptionPurchaseCounterTx(tx, 41, 7, 0)
    }))
    require.NoError(t, db.Where("user_id = ? AND plan_id = ?", 41, 7).
        Delete(&UserSubscription{}).Error)
    got, err := CountSubscriptionPurchasesByPlan(41, 7)
    require.NoError(t, err)
    assert.EqualValues(t, 1, got)
}
```

Add tests proving a transaction rollback leaves `used_count` unchanged, unlimited acquisitions are recorded, negative/corrupt counters fail closed, and two concurrent increments under limit one yield one success.

- [ ] **Step 2: Run the focused tests and verify RED**

Run: `go test ./model -run 'SubscriptionPurchaseCounter|StricterPositive' -count=1 -v`

Expected: FAIL because the model and helpers do not exist.

- [ ] **Step 3: Implement the model and transactional increment**

```go
type SubscriptionPurchaseCounter struct {
    UserID    int   `gorm:"primaryKey;autoIncrement:false"`
    PlanID    int   `gorm:"primaryKey;autoIncrement:false"`
    UsedCount int64 `gorm:"not null;default:0"`
    CreatedAt int64 `gorm:"bigint;not null"`
    UpdatedAt int64 `gorm:"bigint;not null"`
}

func StricterPositiveSubscriptionLimit(snapshot, current int) int {
    if snapshot <= 0 { return max(current, 0) }
    if current <= 0 { return snapshot }
    return min(snapshot, current)
}

func reserveSubscriptionPurchaseSlotTx(tx *gorm.DB, userID, planID, limit int) error {
    var counter SubscriptionPurchaseCounter
    err := lockForUpdate(tx).Where("user_id = ? AND plan_id = ?", userID, planID).
        First(&counter).Error
    if errors.Is(err, gorm.ErrRecordNotFound) {
        counter = SubscriptionPurchaseCounter{UserID: userID, PlanID: planID}
        if err = tx.Create(&counter).Error; err != nil { return err }
    } else if err != nil { return err }
    if counter.UsedCount < 0 { return ErrSubscriptionPurchaseCounterCorrupt }
    if limit > 0 && counter.UsedCount >= int64(limit) { return ErrSubscriptionPurchaseLimit }
    result := tx.Model(&SubscriptionPurchaseCounter{}).
        Where("user_id = ? AND plan_id = ? AND used_count = ?", userID, planID, counter.UsedCount).
        Update("used_count", counter.UsedCount+1)
    if result.Error != nil { return result.Error }
    if result.RowsAffected != 1 { return ErrSubscriptionPurchaseConflict }
    return nil
}
```

Use repository error conventions rather than raw text. The caller already locks the user before this helper, so every acquisition path has the same cross-dialect lock order.

- [ ] **Step 4: Add idempotent startup migration and evidence backfill**

Add the model to the existing AutoMigrate list. Backfill in one migration transaction using dialect-safe grouped queries. For each `(user_id, plan_id)`, compute:

```text
current_subscriptions = count(user_subscriptions)
recorded_sources = successful subscription_orders + subscription user_redemption_claims
baseline = max(existing_counter, current_subscriptions, recorded_sources)
```

Upsert only when `baseline > used_count`; never lower or delete a counter. A second run must update zero rows. Tests seed overlapping current/order/claim evidence and assert it is not double-counted.

- [ ] **Step 5: Run focused tests to GREEN and commit**

Run: `go test ./model -run 'SubscriptionPurchaseCounter|StricterPositive' -count=1 -v`

Run: `go test -race ./model -run 'SubscriptionPurchaseCounter' -count=1`

```bash
git add model/subscription_purchase_counter.go model/subscription_purchase_counter_test.go model/main.go
git commit -m "fix(subscription): persist purchase limits"
```

### Task 2: Make The Counter The Only Subscription Creation Gate

**Files:**
- Modify: `model/subscription.go`
- Create: `model/subscription_purchase_limit_test.go`
- Modify: `service/user_redemption_subscription.go`
- Modify: `service/user_redemption_test.go`
- Modify: `service/user_redemption_subscription_hardening_test.go`

- [ ] **Step 1: Write failing sequential and mixed-source tests**

```go
func TestLimitedPlanCountsBalanceAndRedemptionTogether(t *testing.T) {
    plan := seedLimitedPlan(t, 1)
    seedWalletUser(t, 7001, enoughQuota)
    require.NoError(t, model.PurchaseSubscriptionWithBalance(7001, plan.Id))
    code := createSubscriptionRedemptionForTest(t, 7002, plan, 1)
    _, err := claimSubscriptionRedemptionForTest(7001, code.Code)
    require.ErrorIs(t, err, ErrUserRedemptionPlanPurchaseLimit)
    assertRedemptionUnclaimed(t, code.ID)
}
```

Add cases for delete/expiry/refund not releasing a slot, limit raised from one to two allowing exactly one more, old unlimited snapshot plus current limit one, snapshot one plus current unlimited, concurrent balance purchase versus redemption, and admin bind versus redemption.

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./model ./service -run 'PurchaseLimit|LimitedPlan|Concurrent.*Redemption' -count=1 -v`

Expected: at least the deletion/current-limit cases FAIL against current-row and snapshot-only checks.

- [ ] **Step 3: Centralize enforcement in subscription creation**

In `CreateUserSubscriptionFromPlanTx`, keep the user row lock, remove the `UserSubscription.Count` block, and call:

```go
if err := reserveSubscriptionPurchaseSlotTx(tx, userId, plan.Id, plan.MaxPurchasePerUser); err != nil {
    return nil, normalizeSubscriptionLimitError(err)
}
```

Call it immediately before creating `UserSubscription`; any later error rolls it back. Replace preflight `CountUserSubscriptionsByPlan` implementations with `CountSubscriptionPurchasesByPlan` so UI hints agree, while keeping the transactional helper authoritative.

- [ ] **Step 4: Apply the stricter redemption limit and remove duplicate counting**

After locking the current plan and entitlement:

```go
snapshotPlan := subscriptionPlanFromEntitlement(entitlement)
snapshotPlan.MaxPurchasePerUser = model.StricterPositiveSubscriptionLimit(
    entitlement.MaxPurchasePerUser,
    plan.MaxPurchasePerUser,
)
subscription, err := model.CreateUserSubscriptionFromPlanTx(
    tx, input.ClaimerUserID, snapshotPlan, "redemption",
)
```

Delete the earlier direct `UserSubscription` count. Map the model limit sentinel to `ErrUserRedemptionPlanPurchaseLimit` without string matching.

- [ ] **Step 5: Run mixed-source and full subscription tests to GREEN**

Run: `go test ./model ./service -run 'Subscription|UserRedemption' -count=1`

Run: `go test -race ./model ./service -run 'PurchaseLimit|Concurrent.*Redemption' -count=1`

```bash
git add model/subscription.go model/subscription_purchase_limit_test.go service/user_redemption_subscription.go service/user_redemption_test.go service/user_redemption_subscription_hardening_test.go
git commit -m "fix(redemption): enforce subscription limits"
```

### Task 3: Add One Server-Owned Redemption Route

**Files:**
- Modify: `service/wallet_redemption.go`
- Modify: `service/wallet_redemption_test.go`
- Create: `controller/redemption_claim.go`
- Create: `controller/redemption_claim_test.go`
- Modify: `router/api-router.go`
- Modify: `router/invitation_routes_test.go`

- [ ] **Step 1: Write failing routing and response tests**

Cover administrator balance codes, user-created balance codes, subscription codes, unknown codes, disabled/exhausted codes, a limit error, and a concurrent duplicate submit. Assert a failed first route never retries through another mutation endpoint.

```go
func TestRedeemCodeRoutesByStoredType(t *testing.T) {
    result, err := RedeemCode(context.Background(), claimantID, subscriptionCode)
    require.NoError(t, err)
    assert.Equal(t, model.UserRedemptionRewardSubscription, result.RewardType)
    assert.Equal(t, "user", result.Source)
    assert.Positive(t, result.SubscriptionID)
    assert.Zero(t, result.Quota)
}
```

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./service ./controller ./router -run 'AnyRedemption|UnifiedRedemption' -count=1 -v`

- [ ] **Step 3: Implement service routing without client type input**

```go
type RedemptionClaimResult struct {
    RewardType    string `json:"reward_type"`
    Source        string `json:"source"`
    Quota         int    `json:"quota"`
    SubscriptionID int64 `json:"subscription_id"`
}

func RedeemCode(ctx context.Context, userID int, code string) (RedemptionClaimResult, error) {
    code = strings.TrimSpace(code)
    if code == "" { return RedemptionClaimResult{}, ErrUserRedemptionInvalid }
    var route model.UserRedemptionCode
    userCodeErr := model.DB.WithContext(ctx).Select("id", "reward_type").Where("code = ?", code).First(&route).Error
    var legacy model.Redemption
    legacyErr := model.DB.WithContext(ctx).Where(&model.Redemption{Key: code}).First(&legacy).Error
    if userCodeErr == nil && legacyErr == nil { return RedemptionClaimResult{}, ErrWalletRedemptionAmbiguous }
    if userCodeErr == nil {
        claim, claimErr := ClaimUserRedemptionCodeWithInput(ctx, ClaimUserRedemptionInput{ClaimerUserID: userID, Code: code})
        if claimErr != nil { return RedemptionClaimResult{}, claimErr }
        return claimResultFromUserRedemption(claim), nil
    }
    if !errors.Is(userCodeErr, gorm.ErrRecordNotFound) { return RedemptionClaimResult{}, userCodeErr }
    if !errors.Is(legacyErr, gorm.ErrRecordNotFound) { return RedemptionClaimResult{}, legacyErr }
    quota, redeemErr := model.Redeem(code, userID)
    if redeemErr != nil { return RedemptionClaimResult{}, redeemErr }
    return RedemptionClaimResult{RewardType: "balance", Source: "administrator", Quota: quota}, nil
}
```

Put the implementation in `redeemCodeWithExpectation(ctx, userID, code, expectedRewardType)`. `RedeemCode` passes an empty expectation for auto-detection; legacy `RedeemWalletCode` passes `balance`, so a subscription route is rejected before any claim mutation. Retain the current two-table lookup and `ErrWalletRedemptionAmbiguous`, because historical data may predate cross-table creation checks. Never fall through after a user-code mutation error.

- [ ] **Step 4: Register the authenticated critical-write controller**

Add `POST /api/redemption/claim` under `UserAuth` with `CriticalUserRateLimit("redemption.claim")`. Require `operation_setting.IsPaymentComplianceConfirmed()` exactly like `controller.TopUp`, bind only `{code}`, and return `common.ApiSuccess`. Reuse existing normalized redemption error mappings, record one `user.redeem_code` security audit with the true result type/source, and add a stable purchase-limit code.

- [ ] **Step 5: Run tests to GREEN and commit**

Run: `go test ./service ./controller ./router -run 'Redemption' -count=1`

```bash
git add service/wallet_redemption.go service/wallet_redemption_test.go controller/redemption_claim.go controller/redemption_claim_test.go router/api-router.go router/invitation_routes_test.go
git commit -m "feat(redemption): unify code claims"
```

### Task 4: Cover Every Subscription Redemption Audit Event

**Files:**
- Modify: `web/default/src/features/usage-logs/lib/format.ts`
- Modify: `web/default/src/features/usage-logs/lib/audit-locales.test.ts`
- Modify: `web/classic/src/helpers/log.js`
- Modify: `web/classic/src/helpers/log.test.js`
- Modify: all Default and Classic locale JSON files

- [ ] **Step 1: Extend required-action tests before templates**

Add the eight event keys from the design to both required-action arrays. Add a source inventory assertion that scans `service/user_redemption*.go` for audit event literals and requires both template maps to cover every result.

- [ ] **Step 2: Run tests and verify RED**

Run: `cd web/default; bun test src/features/usage-logs/lib/audit-locales.test.ts`

Run: `cd web/classic; bun test src/helpers/log.test.js`

Expected: FAIL on `user.redemption.subscription.claim` and the other missing keys.

- [ ] **Step 3: Add templates and every locale**

```ts
'user.redemption.subscription.create': 'Created subscription redemption code {{id}}',
'user.redemption.subscription.claim': 'Claimed subscription redemption code {{id}}',
'user.redemption.subscription.plan_disabled': 'Closed subscription redemption code {{id}} because its plan was disabled',
'user.redemption.subscription.refund_pending': 'Queued subscription redemption code {{id}} for refund',
'user.redemption.subscription.refund': 'Refunded subscription redemption code {{id}}',
'user.redemption.subscription.forfeit': 'Forfeited subscription redemption code {{id}}',
'user.redemption.cancel_pending': 'Queued user redemption code {{id}} for cancellation',
'user.redemption.expire_pending': 'Queued expired user redemption code {{id}} for settlement',
```

Preserve interpolation variables in all languages. Do not create additional backend log rows.

- [ ] **Step 4: Run locale and rendering tests to GREEN**

Run the two focused commands plus Default/Classic production i18n tests.

```bash
git add web/default/src/features/usage-logs web/default/src/i18n/locales web/classic/src/helpers web/classic/src/i18n/locales
git commit -m "fix(logs): label subscription redemption events"
```

### Task 5: Replace Default Wallet Type Selection With One Input

**Files:**
- Modify: `web/default/src/features/wallet/api.ts`
- Modify: `web/default/src/features/wallet/api.test.ts`
- Modify: `web/default/src/features/wallet/hooks/use-redemption.ts`
- Modify: `web/default/src/features/wallet/index.tsx`
- Modify: `web/default/src/features/wallet/components/recharge-form-card.tsx`
- Modify: `web/default/src/features/wallet/wallet-composition.test.ts`
- Modify: Default locale JSON files

- [ ] **Step 1: Write failing API and composition tests**

Assert one request to `/api/redemption/claim` with `{code}`, no `redemptionRewardType` state/props, no Balance/Subscription segmented buttons, and success branches driven by `response.reward_type`.

- [ ] **Step 2: Run tests and verify RED**

Run: `cd web/default; bun test src/features/wallet`

- [ ] **Step 3: Implement the one-input flow**

Add a typed `claimRedemptionCode(code)` API. Change `redeemCode(code)` to call it once, show the existing balance amount message for `balance`, the subscription message for `subscription`, then refresh both `getSelf()` and subscription queries. Remove type state and props from the page/card; use one “Enter redemption code” placeholder and stable 36px input/button tracks.

- [ ] **Step 4: Run tests, typecheck, and commit**

Run: `cd web/default; bun test src/features/wallet && bun run typecheck`

```bash
git add web/default/src/features/wallet web/default/src/i18n/locales
git commit -m "feat(wallet): auto-detect redemption codes"
```

### Task 6: Apply The Same One-Input Contract In Classic

**Files:**
- Modify: `web/classic/src/components/topup/index.jsx`
- Modify: `web/classic/src/components/topup/RechargeCard.jsx`
- Modify: `web/classic/src/components/topup/wallet-composition.test.js`
- Modify: Classic locale JSON files

- [ ] **Step 1: Write the failing source/interaction assertions**

Require one `/api/redemption/claim` request, response-driven messages, and absence of `redemptionRewardType`, `expected_reward_type`, and the two-button group.

- [ ] **Step 2: Run the Classic test and verify RED**

Run: `cd web/classic; bun test src/components/topup/wallet-composition.test.js`

- [ ] **Step 3: Implement parity and refresh both account domains**

Submit `{code: redemptionCode}` once. On success call both `getUserQuota()` and `getSubscriptionSelf()` regardless of result type, clear the input, and display the appropriate server-discriminated success. Keep one full-width input/button row that wraps safely at 320px.

- [ ] **Step 4: Run tests and build, then commit**

Run: `cd web/classic; bun test src/components/topup && bun run eslint && bun run build`

```bash
git add web/classic/src/components/topup web/classic/src/i18n/locales
git commit -m "feat(classic): unify redemption input"
```

### Task 7: Cross-Dialect, Browser, And Preserved-Database Acceptance

**Files:**
- Modify: `web/default/e2e/production-business-flows.spec.ts`
- Modify only owning source/tests if a reproduced defect fails a gate.

- [ ] **Step 1: Add reversible browser fixtures**

Create one administrator balance code, one user balance code, two subscription codes for a limit-one plan, and one claimant. In both Default and Classic, use the same input to redeem the first three types, assert the second subscription code returns the purchase-limit message, and confirm the failed code remains unclaimed. Restore frontend theme and remove fixture rows/counters in `finally`.

- [ ] **Step 2: Run disposable SQLite/MySQL/PostgreSQL migration and concurrency tests**

Run the repository migration matrix with the new table/backfill tests. Never point a migration reset at `cloveapi-current-source-mysql-qa-data`.

- [ ] **Step 3: Run full backend and frontend gates**

```powershell
go test ./... -count=1
go vet ./...
Set-Location web/default
bun test ./src ./rsbuild.config.test.ts
bun run typecheck
bun run lint
bun run build
Set-Location ../classic
bun test
bun run eslint
bun run build
```

- [ ] **Step 4: Rebuild local QA without replacing its volume**

Capture counts and total balances from `cloveapi-remote-copy-mysql`, then run:

```powershell
pwsh -File scripts/rebuild-local-mysql-qa.ps1 -ContainerName cloveapi-current-source-mysql-qa -ImageTag clove-api:local-20260730-unified-redemption -HostPort 18104
pwsh -File scripts/verify-runtime-source.ps1 -ContainerName cloveapi-current-source-mysql-qa -ExpectedCommit (git rev-parse HEAD)
```

Run Playwright desktop and 320px mobile cases. Recheck database counts/balances and logs; only the new counter backfill and explicitly reversible fixtures may differ.

- [ ] **Step 5: Verify final scope**

Run `git diff --check`, `git status --short`, and inspect staged files. Confirm no production deployment, remote push, Snow edit, credential, database dump, or Docker volume deletion is present.
