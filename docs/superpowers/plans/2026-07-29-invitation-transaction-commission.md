# Invitation Transaction Commission Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the existing invitation reward approval lifecycle so registration rewards, real-cash payments, administrator balance codes, and administrator subscription codes can produce configurable, auditable, reversible commission rewards.

**Architecture:** Generalize `InviteRewardPayout` into a source-keyed reward fact and keep the existing payout request, review, freeze, and wallet ledger paths. A model-layer transactional commission entry point is called by every canonical payment or redemption completion path; settings are centralized in one typed configuration and both frontends consume the same public policy summary. Immutable reversal rows preserve partial/full refund evidence without duplicating wallet debits.

**Tech Stack:** Go 1.22, GORM, SQLite/MySQL/PostgreSQL, shopspring/decimal, React/TypeScript Default UI, React/Semi Classic UI, Vitest, Playwright.

---

## Dependency And Execution Order

Execute `docs/superpowers/plans/2026-07-29-subscription-redemption-codes.md` before Task 6 of this plan. That plan creates subscription-code entitlements and claims used as official-code commission sources. After this plan passes, continue with `2026-07-29-model-catalog-visibility.md` and `2026-07-29-invitation-poster-export.md`. All four features remain local-only until the combined acceptance run.

## File Map

- Create `setting/operation_setting/invitation_incentive_setting.go`: typed switches, percentage parsing, public policy view.
- Create `setting/operation_setting/invitation_incentive_setting_test.go`: defaults, round trips, rates above 100%, validation.
- Modify `model/invitation.go`: source-keyed reward facts, reversal rows, request cancellation state and lock helpers.
- Create `model/invitation_commission.go`: exact calculation, source qualification, idempotent reward creation and reversal.
- Create `model/invitation_commission_test.go`: source, concurrency, logging and reversal coverage.
- Modify `model/main.go` and `model/main_database_integration_test.go`: additive schema and cross-dialect migration.
- Modify `service/invitation_registration.go` and its tests: registration switches and stable registration trigger.
- Modify `model/topup.go`, `model/subscription.go`, `model/redemption.go`: canonical cash/admin-code hooks.
- Modify subscription-redemption files from the prerequisite plan: official-issuer snapshot and subscription claim hook.
- Modify `service/invitation_reward_payout.go` and tests: net amounts, cancellation, partial reversals and freeze interaction.
- Modify `controller/invitation.go` and tests: policy summary and extended administrator detail.
- Modify Default settings, invitation types, wallet card and approval page.
- Modify Classic settings, invitation API, wallet card and approval page.

### Task 1: Typed Invitation Incentive Settings

**Files:**
- Create: `setting/operation_setting/invitation_incentive_setting.go`
- Create: `setting/operation_setting/invitation_incentive_setting_test.go`
- Modify: `controller/feature_setting_test.go`

- [ ] **Step 1: Write the failing settings tests**

```go
func TestInvitationIncentiveDefaultsPreserveRegistrationAndDisableNewCommission(t *testing.T) {
    got := DefaultInvitationIncentiveSetting()
    require.True(t, got.Enabled)
    require.True(t, got.InviterRegistrationRewardEnabled)
    require.True(t, got.InviteeRegistrationGiftEnabled)
    require.False(t, got.CashCommissionEnabled)
    require.Equal(t, "0.00", got.CashCommissionRate)
}

func TestParseInvitationCommissionRateAllowsMoreThanOneHundredPercent(t *testing.T) {
    bps, normalized, err := ParseInvitationCommissionRate("1250.75")
    require.NoError(t, err)
    require.Equal(t, int64(125075), bps)
    require.Equal(t, "1250.75", normalized)
}
```

Add table cases for `0`, `5.25`, `200`, negative input, three fractional digits, non-numeric input and the technical integer-overflow boundary. Add a global-config round-trip test for every key.

- [ ] **Step 2: Run the focused tests and verify RED**

Run: `go test ./setting/operation_setting ./controller -run 'InvitationIncentive|InvitationCommissionRate' -count=1`

Expected: FAIL because the setting type, parser and registered keys do not exist.

- [ ] **Step 3: Implement the typed setting and public policy**

```go
type InvitationIncentiveSetting struct {
    Enabled                                      bool   `json:"enabled"`
    InviterRegistrationRewardEnabled            bool   `json:"inviter_registration_reward_enabled"`
    InviteeRegistrationGiftEnabled               bool   `json:"invitee_registration_gift_enabled"`
    CashCommissionEnabled                       bool   `json:"cash_commission_enabled"`
    CashCommissionRate                          string `json:"cash_commission_rate"`
    AdminBalanceRedemptionCommissionEnabled     bool   `json:"admin_balance_redemption_commission_enabled"`
    AdminBalanceRedemptionCommissionRate        string `json:"admin_balance_redemption_commission_rate"`
    AdminSubscriptionRedemptionCommissionEnabled bool  `json:"admin_subscription_redemption_commission_enabled"`
    AdminSubscriptionRedemptionCommissionRate   string `json:"admin_subscription_redemption_commission_rate"`
}
```

Register it under `invitation_incentive_setting`. Parse percent strings with `decimal.NewFromString`, require non-negative values and at most two fractional digits, multiply by 100 and convert exactly to `int64`. Do not add a 100% maximum. Return a public policy containing only the master/cash enabled flags and normalized current cash rate for the wallet card.

- [ ] **Step 4: Run focused and package tests**

Run: `go test ./setting/operation_setting ./controller -run 'InvitationIncentive|InvitationCommissionRate|FeatureSetting' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add setting/operation_setting/invitation_incentive_setting.go setting/operation_setting/invitation_incentive_setting_test.go controller/feature_setting_test.go
git commit -m "feat(invitation): add incentive policy settings"
```

### Task 2: Source-Keyed Reward Schema And Migration

**Files:**
- Modify: `model/invitation.go`
- Modify: `model/invitation_test.go`
- Modify: `model/main.go`
- Modify: `model/main_database_integration_test.go`

- [ ] **Step 1: Write failing model and migration tests**

```go
func TestInvitationRewardFactsAllowMultipleTransactionSourcesPerInviteUse(t *testing.T) {
    first := InviteRewardPayout{InviteCodeUseID: 7, RecipientKind: InviteRewardRecipientInviter,
        RecipientUserID: 3, TriggerType: InviteRewardTriggerCashTopUp, TriggerID: "pay-1", Amount: 10}
    second := first
    second.TriggerID = "pay-2"
    require.NoError(t, DB.Create(&first).Error)
    require.NoError(t, DB.Create(&second).Error)
    require.Error(t, DB.Create(&first).Error)
}

func TestInvitationRewardMigrationBackfillsLegacyRegistrationSources(t *testing.T) {
    legacyID := seedLegacyInvitationReward(t, InviteRewardRecipientInviter)
    require.NoError(t, MigrateInvitationRewardFacts(DB))
    var reward InviteRewardPayout
    require.NoError(t, DB.First(&reward, legacyID).Error)
    require.Equal(t, InviteRewardTriggerRegistrationInviter, reward.TriggerType)
    require.Equal(t, strconv.FormatInt(reward.InviteCodeUseID, 10), reward.TriggerID)
}
```

Assert the migration preserves paid, frozen, rejected and pending request items and creates `invite_reward_reversals` with the refund-source unique index on all configured dialects.

- [ ] **Step 2: Run the focused tests and verify RED**

Run: `go test ./model -run 'InvitationReward(Facts|Migration|Reversal)' -count=1`

Expected: FAIL because trigger fields, reversal models and migration do not exist.

- [ ] **Step 3: Add reward source and reversal fields**

```go
const (
    InviteRewardTriggerRegistrationInviter      = "registration_inviter"
    InviteRewardTriggerCashTopUp                = "cash_topup"
    InviteRewardTriggerCashSubscription         = "cash_subscription"
    InviteRewardTriggerAdminBalanceRedemption   = "admin_balance_redemption"
    InviteRewardTriggerAdminSubscriptionClaim   = "admin_subscription_redemption"
    InviteRewardReversalNone                    = "none"
    InviteRewardReversalPartial                 = "partial"
    InviteRewardReversalFull                    = "full"
    InviteRewardPayoutRequestStatusCancelled    = "cancelled"
)

type InviteRewardReversal struct {
    ID               int64  `gorm:"primaryKey;autoIncrement"`
    RewardID         int64  `gorm:"not null;uniqueIndex:uniq_invite_reward_refund,priority:1"`
    RefundSourceType string `gorm:"type:varchar(32);not null;uniqueIndex:uniq_invite_reward_refund,priority:2"`
    RefundSourceID   string `gorm:"type:varchar(191);not null;uniqueIndex:uniq_invite_reward_refund,priority:3"`
    RefundedBase     int64  `gorm:"type:bigint;not null"`
    Amount           int64  `gorm:"type:bigint;not null"`
    WalletOperationID string `gorm:"type:varchar(191);not null;default:'';index"`
    CreatedAt        int64  `gorm:"bigint;autoCreateTime"`
}
```

Change reward monetary fields to `int64`; add `TriggerType`, `TriggerID`, `EligibleQuota`, `RateBasisPoints`, `ReversedAmount` and `ReversalState`. Implement an idempotent migration that backfills old inviter/invitee registration facts before replacing the old uniqueness constraint.

- [ ] **Step 4: Register and verify migrations**

Run: `go test ./model -run 'InvitationReward(Facts|Migration|Reversal)|MainDatabase.*Invitation' -count=1`

Expected: PASS on SQLite and on configured integration dialects.

- [ ] **Step 5: Commit**

```bash
git add model/invitation.go model/invitation_test.go model/main.go model/main_database_integration_test.go
git commit -m "feat(invitation): generalize reward source records"
```

### Task 3: Registration Reward Switches

**Files:**
- Modify: `service/invitation_registration.go`
- Modify: `service/invitation_registration_test.go`
- Modify: `service/invitation_reward_worker.go`
- Modify: `service/invitation_reward_worker_test.go`

- [ ] **Step 1: Write failing registration behavior tests**

```go
func TestInvitationRegistrationSwitchesRewardsIndependently(t *testing.T) {
    setting := operation_setting.GetInvitationIncentiveSetting()
    setting.Enabled = true
    setting.InviterRegistrationRewardEnabled = false
    setting.InviteeRegistrationGiftEnabled = true
    result := registerInvitationFixture(t)
    require.Zero(t, loadInviterAvailableReward(t, result.InviterID))
    require.Equal(t, int64(common.QuotaForInvitee), loadWallet(t, result.UserID))
    require.NotZero(t, result.InviteUseID)
}
```

Add cases for master-off, inviter-only, invitee-only, both-on, payment-compliance-off, retries and worker backfill. Always assert the invite use and count survive when rewards are disabled.

- [ ] **Step 2: Run and verify RED**

Run: `go test ./service -run 'InvitationRegistration.*Switch|InvitationRewardWorker.*Switch' -count=1`

Expected: FAIL because registration ignores the new policy.

- [ ] **Step 3: Apply settings and stable registration triggers**

Read one policy snapshot inside the registration transaction. Create the inviter fact with trigger `registration_inviter` and trigger ID equal to the invite-use ID only when enabled and positive. Credit the invitee gift only when its switch is enabled. Update worker existence queries to use the trigger identity rather than assuming one inviter fact per invite use.

- [ ] **Step 4: Run service regression tests**

Run: `go test ./service -run 'Invitation(Registration|RewardWorker|RewardSubmit|RewardFreeze)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add service/invitation_registration.go service/invitation_registration_test.go service/invitation_reward_worker.go service/invitation_reward_worker_test.go
git commit -m "feat(invitation): make registration rewards configurable"
```

### Task 4: Transactional Commission Engine And Quiet Logs

**Files:**
- Create: `model/invitation_commission.go`
- Create: `model/invitation_commission_test.go`
- Modify: `model/log.go`
- Modify: `service/audit_outbox.go`
- Modify: `service/audit_outbox_test.go`

- [ ] **Step 1: Write failing calculation, source and log tests**

```go
func TestCreateInvitationCommissionAllowsTwoHundredPercentExactlyOnce(t *testing.T) {
    input := InvitationCommissionInput{
        InviteeUserID: 20, TriggerType: InviteRewardTriggerCashTopUp,
        TriggerID: "cash-20", EligibleQuota: 900, RateBasisPoints: 20000,
    }
    reward, created, err := CreateInvitationCommissionWithTx(DB, input)
    require.NoError(t, err)
    require.True(t, created)
    require.Equal(t, int64(1800), reward.Amount)
    _, created, err = CreateInvitationCommissionWithTx(DB, input)
    require.NoError(t, err)
    require.False(t, created)
}
```

Add cases for `5.25%`, floor rounding, no inviter, source-attribution inviter, disabled policy, zero rate, concurrent duplicates, unknown trigger, and exactly one concise outbox-backed usage log. Assert skipped paths produce no log.

- [ ] **Step 2: Run and verify RED**

Run: `go test ./model ./service -run 'InvitationCommission|InvitationRewardLog' -count=1`

Expected: FAIL because the commission API and log projection do not exist.

- [ ] **Step 3: Implement the single commission entry point**

```go
type InvitationCommissionInput struct {
    InviteeUserID  int
    TriggerType    string
    TriggerID      string
    EligibleQuota  int64
    RateBasisPoints int64
}

func CreateInvitationCommissionWithTx(tx *gorm.DB, input InvitationCommissionInput) (*InviteRewardPayout, bool, error)
```

Validate the known trigger set, lock the invitee and `InviteCodeUse`, require `InviterSourceInviteCode`, calculate with `big.Int` or `decimal.Decimal`, and insert via the source unique key. Insert a pending audit outbox event with a stable event ID only when a positive reward is newly created. Project one short user log containing source class, amount and rate; exclude full order/code identifiers.

- [ ] **Step 4: Verify engine and outbox regression**

Run: `go test ./model ./service -run 'InvitationCommission|InvitationRewardLog|AuditOutbox' -count=1`

Expected: PASS with one reward and one log per source.

- [ ] **Step 5: Commit**

```bash
git add model/invitation_commission.go model/invitation_commission_test.go model/log.go service/audit_outbox.go service/audit_outbox_test.go
git commit -m "feat(invitation): create transactional commission rewards"
```

### Task 5: Cash Top-Up Commission Hooks

**Files:**
- Modify: `model/topup.go`
- Modify: `model/wallet_topup_adjust_test.go`
- Modify: `controller/topup.go`
- Modify: `controller/topup_epay_integration_test.go`
- Modify: `controller/stripe_wallet_payment_test.go`
- Create: `controller/topup_waffo_test.go`
- Modify: `controller/topup_waffo_pancake_test.go`

- [ ] **Step 1: Write failing actual-payment and provider parity tests**

```go
func TestCashTopUpCommissionUsesPaidAmountNotCreditedQuota(t *testing.T) {
    seedInvitedTopUp(t, "discounted", 1000, 900)
    require.NoError(t, completeTopUp("discounted"))
    reward := loadRewardByTrigger(t, InviteRewardTriggerCashTopUp, "discounted")
    require.Equal(t, int64(900), reward.EligibleQuota)
    require.Equal(t, int64(90), reward.Amount)
}
```

Run the same assertion for Epay, Stripe, Creem, Waffo and Waffo Pancake. Assert administrator balance adjustment and a raw manual credit do not create commission. Assert duplicate callbacks do not duplicate rewards or logs.

- [ ] **Step 2: Run and verify RED**

Run: `go test ./model ./controller -run '(TopUp|Recharge).*Commission' -count=1`

Expected: FAIL because top-up completion does not persist an eligible paid basis or call the engine.

- [ ] **Step 3: Persist the paid basis and call one helper**

Add `EligibleQuota int64`, `PaidAmount string` and `PaidCurrency string` snapshots to `TopUp`. Set them when the checkout/order is created, preferring verified provider settlement amount where available. Add one `completeCashTopUpWithCommissionTx` helper used after `CreditTopUpWithTx`; exclude `PaymentProviderBalance` and the special `admin` adjustment source.

- [ ] **Step 4: Run payment regression**

Run: `go test ./model ./controller -run '(TopUp|Recharge|Stripe|Epay|Creem|Waffo).*' -count=1`

Expected: PASS for every provider and discount path.

- [ ] **Step 5: Commit**

```bash
git add model/topup.go model/wallet_topup_adjust_test.go controller/topup.go controller/topup_epay_integration_test.go controller/stripe_wallet_payment_test.go controller/topup_waffo_test.go controller/topup_waffo_pancake_test.go
git commit -m "feat(invitation): reward eligible cash topups"
```

### Task 6: Subscription And Official Redemption Hooks

**Prerequisite:** Complete Tasks 1-7 in `2026-07-29-subscription-redemption-codes.md` first.

**Files:**
- Modify: `model/subscription.go`
- Modify: `model/payment_method_guard_test.go`
- Modify: `model/redemption.go`
- Modify: `model/redemption_test.go`
- Modify: `model/user_redemption.go`
- Modify: `service/user_redemption.go`
- Modify: `service/user_redemption_test.go`
- Modify: `controller/user_redemption.go`

- [ ] **Step 1: Write failing source qualification tests**

```go
func TestInvitationCommissionSourceMatrix(t *testing.T) {
    requireReward(t, completeCashSubscription(t), InviteRewardTriggerCashSubscription)
    requireNoReward(t, purchaseSubscriptionWithBalance(t))
    requireReward(t, redeemAdminBalanceCode(t), InviteRewardTriggerAdminBalanceRedemption)
    requireNoReward(t, claimUserBalanceCode(t))
    requireReward(t, claimOfficialAdminSubscriptionCode(t), InviteRewardTriggerAdminSubscriptionClaim)
    requireNoReward(t, claimUserSubscriptionCode(t))
}
```

Add tests that an official code remains eligible after issuer demotion and a user code remains ineligible after creator promotion. Multi-unit claims must create one reward per successful claim.

- [ ] **Step 2: Run and verify RED**

Run: `go test ./model ./service -run 'InvitationCommissionSourceMatrix|OfficialSubscriptionCommission' -count=1`

Expected: FAIL because official issuance and completion hooks do not exist.

- [ ] **Step 3: Add immutable official issuance and canonical hooks**

Persist `OfficialIssuer bool` and `IssuedBy int` on subscription redemption entitlements when and only when the administrator route creates the code. Cash `CompleteSubscriptionOrder` uses the order paid-basis snapshot and excludes `PaymentProviderBalance`. `Redeem` for traditional administrator codes calls the engine with redeemed quota. Subscription claim calls it with `UnitPaidQuota` and claim ID after the subscription and claim rows exist in the same transaction.

- [ ] **Step 4: Run subscription and redemption regression**

Run: `go test ./model ./service ./controller -run '(Subscription|Redemption|InvitationCommission).*' -count=1`

Expected: PASS without changing purchase limits, snapshots, refunds or user-created code behavior.

- [ ] **Step 5: Commit**

```bash
git add model/subscription.go model/payment_method_guard_test.go model/redemption.go model/redemption_test.go model/user_redemption.go service/user_redemption.go service/user_redemption_test.go controller/user_redemption.go
git commit -m "feat(invitation): reward official subscription and code sources"
```

### Task 7: Refund Reversals And Freeze Coordination

**Files:**
- Modify: `model/invitation_commission.go`
- Modify: `model/invitation_commission_test.go`
- Modify: `service/invitation_reward_payout.go`
- Modify: `service/invitation_reward_payout_request_test.go`

- [ ] **Step 1: Write failing state-matrix tests**

```go
func TestInvitationCommissionReversalStateMatrix(t *testing.T) {
    for _, state := range []string{
        model.InviteRewardStatusAvailable,
        model.InviteRewardStatusPendingPayout,
        model.InviteRewardStatusPaid,
        model.InviteRewardStatusFrozen,
    } {
        t.Run(state, func(t *testing.T) { verifyPartialThenFullReversal(t, state) })
    }
}
```

Assert pending request net amounts shrink and become `cancelled` at zero; paid rewards create one negative wallet entry; frozen rewards create no second debit; unfreeze restores only unreversed net; duplicate refund IDs are no-ops; balance may become negative.

- [ ] **Step 2: Run and verify RED**

Run: `go test ./model ./service -run 'InvitationCommissionReversal|InvitationReward.*(Refund|Freeze)' -count=1`

Expected: FAIL because reversal accounting is absent.

- [ ] **Step 3: Implement immutable partial/full reversal**

```go
type InvitationCommissionReversalInput struct {
    TriggerType      string
    TriggerID        string
    RefundSourceType string
    RefundSourceID   string
    RefundedBase     int64
}

func ReverseInvitationCommissionWithTx(tx *gorm.DB, input InvitationCommissionReversalInput) (*InviteRewardReversal, bool, error)
```

Lock user, request, request items, reward and existing reversal in the documented order. Calculate the incremental reversal with the original rate snapshot and clamp cumulative reversal to original reward amount. For paid rewards call `AdjustWalletWithTx` with operation ID `invitation-commission-reversal:<reward>:<refund-source>` and allow negative balance. For frozen rewards only reduce future unfreeze net.

- [ ] **Step 4: Run all reward lifecycle tests**

Run: `go test ./model ./service -run 'Invitation(Commission|Reward)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add model/invitation_commission.go model/invitation_commission_test.go service/invitation_reward_payout.go service/invitation_reward_payout_request_test.go
git commit -m "feat(invitation): reverse refunded commissions"
```

### Task 8: API Summaries And Administrator Evidence

**Files:**
- Modify: `service/invitation_reward_payout.go`
- Modify: `service/invitation_query.go`
- Modify: `service/invitation_query_test.go`
- Modify: `controller/invitation.go`
- Modify: `controller/invitation_test.go`

- [ ] **Step 1: Write failing API tests**

```go
func TestInvitationRewardSummaryReturnsCurrentCashPolicyAndNetAmounts(t *testing.T) {
    summary, err := GetInvitationRewardSummary(context.Background(), inviterID)
    require.NoError(t, err)
    require.True(t, summary.CashCommissionVisible)
    require.Equal(t, "200.00", summary.CashCommissionRate)
    require.Equal(t, int64(75), summary.Available)
}
```

Assert administrator detail includes trigger type/ID, eligible quota, rate, original amount, reversed amount and net amount. Assert the ordinary user response omits other users' full identifiers and provider payload.

- [ ] **Step 2: Run and verify RED**

Run: `go test ./service ./controller -run 'InvitationReward(Summary|Detail|Privacy)' -count=1`

Expected: FAIL because new policy and source evidence fields are missing.

- [ ] **Step 3: Extend summary and detail DTOs**

Use `Amount - ReversedAmount` in every available/pending/paid/frozen aggregate. Return `cash_commission_visible` only when master and cash switches are on and rate is positive; return normalized `cash_commission_rate`. Detail joins source user and invite-use data but never provider payload or full code in non-admin responses.

- [ ] **Step 4: Run API regression**

Run: `go test ./service ./controller ./router -run 'Invitation' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add service/invitation_reward_payout.go service/invitation_query.go service/invitation_query_test.go controller/invitation.go controller/invitation_test.go
git commit -m "feat(invitation): expose commission policy and evidence"
```

### Task 9: Default Frontend Settings And Wallet Card

**Files:**
- Modify: `web/default/src/features/system-settings/types.ts`
- Modify: `web/default/src/features/system-settings/general/quota-settings-section.tsx`
- Modify: `web/default/src/features/system-settings/billing/index.tsx`
- Modify: `web/default/src/features/system-settings/billing/section-registry.tsx`
- Create: `web/default/src/features/system-settings/billing/invitation-incentive-section.tsx`
- Modify: `web/default/src/features/invitations/types.ts`
- Modify: `web/default/src/features/wallet/components/affiliate-rewards-card.tsx`
- Modify: `web/default/src/features/invitations/components/invitation-reward-approvals-page.tsx`
- Modify: `web/default/src/features/invitations/invitation-rewards-ui.test.ts`

- [ ] **Step 1: Write failing UI contract tests**

```ts
expect(source).toContain('invitation_incentive_setting.cash_commission_rate')
expect(source).not.toMatch(/cashCommissionRate.*max=['"]?100/)
expect(renderWallet({ cash_commission_visible: true, cash_commission_rate: '200.00' }))
  .toContain('充值返现 200%')
expect(renderWallet({ cash_commission_visible: false })).not.toContain('充值返现')
```

Assert the old invitation amount controls are absent from the generic quota section, master-off disables but preserves child values, and administrator detail labels every financial field.

- [ ] **Step 2: Run and verify RED**

Run: `pnpm --dir web/default test --run src/features/invitations/invitation-rewards-ui.test.ts`

Expected: FAIL because the unified section and wallet rate are missing.

- [ ] **Step 3: Implement the Default UI**

Build one un-nested settings section with a master switch, two registration switches/amount inputs and three commission switches/rate inputs. Use decimal text/number inputs with `step=0.01`, `min=0` and no maximum. Render exactly one wallet line only when `cash_commission_visible`; keep long source IDs in the detail dialog with wrapping and copy actions.

- [ ] **Step 4: Run Default tests and build**

Run: `pnpm --dir web/default test --run src/features/invitations src/features/system-settings`

Run: `pnpm --dir web/default build`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/default/src/features/system-settings web/default/src/features/invitations web/default/src/features/wallet/components/affiliate-rewards-card.tsx
git commit -m "feat(default-ui): configure invitation commissions"
```

### Task 10: Classic Frontend Settings And Wallet Card

**Files:**
- Modify: `web/classic/src/components/settings/OperationSetting.jsx`
- Modify: `web/classic/src/pages/Setting/Operation/SettingsCreditLimit.jsx`
- Create: `web/classic/src/pages/Setting/Operation/SettingsInvitationIncentive.jsx`
- Modify: `web/classic/src/components/invitations/api.js`
- Modify: `web/classic/src/components/invitations/UserInvitationCard.jsx`
- Modify: `web/classic/src/components/invitations/AdminInvitationRewards.jsx`
- Modify: `web/classic/src/components/invitations/invitation-ui.test.js`

- [ ] **Step 1: Write failing Classic UI tests**

```js
expect(source).toContain('invitation_incentive_setting.cash_commission_rate');
expect(walletSource).toContain("summary.cash_commission_visible");
expect(walletSource).not.toContain('充值返现未开启');
expect(adminSource).toContain('reversed_amount');
expect(adminSource).toContain('eligible_quota');
```

Assert rate inputs have no `max={100}`, registration amount fields exist only in the invitation section, and long source identifiers wrap in mobile cards.

- [ ] **Step 2: Run and verify RED**

Run: `pnpm --dir web/classic test --run src/components/invitations/invitation-ui.test.js`

Expected: FAIL.

- [ ] **Step 3: Implement Classic parity**

Use Semi switches and numeric inputs with the same key names and enable/disable semantics as Default. Display only the active cash rate in `UserInvitationCard`; keep the administrator list compact and place full evidence in the detail modal.

- [ ] **Step 4: Run Classic tests and build**

Run: `pnpm --dir web/classic test --run src/components/invitations/invitation-ui.test.js`

Run: `pnpm --dir web/classic build`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/classic/src/components/settings/OperationSetting.jsx web/classic/src/pages/Setting/Operation web/classic/src/components/invitations
git commit -m "feat(classic-ui): configure invitation commissions"
```

### Task 11: Combined Regression And Local Acceptance

**Files:**
- Modify only files required by defects reproduced in this verification step.
- Create: `docs/superpowers/specs/2026-07-29-local-feature-acceptance-evidence.md`

- [ ] **Step 1: Run formatting and backend tests**

Run: `gofmt -w setting/operation_setting/invitation_incentive_setting.go setting/operation_setting/invitation_incentive_setting_test.go model/invitation.go model/invitation_commission.go model/invitation_commission_test.go service/invitation_registration.go service/invitation_reward_worker.go service/invitation_reward_payout.go controller/invitation.go`

Run: `go test ./model ./service ./controller ./router ./setting/... -count=1`

Run: `go test ./... -count=1`

Expected: PASS.

- [ ] **Step 2: Run both frontend suites and builds**

Run: `pnpm --dir web/default test --run && pnpm --dir web/default build`

Run: `pnpm --dir web/classic test --run && pnpm --dir web/classic build`

Expected: PASS without warnings introduced by these features.

- [ ] **Step 3: Rebuild the local stack without deleting data**

Use the repository's current local Docker compose files and existing volumes. Do not run volume deletion, database recreation, seed reset or destructive migration commands. Confirm the pre-existing user count and wallet ledger count before and after migration, then serve the application at `http://127.0.0.1:18104`.

- [ ] **Step 4: Run browser acceptance**

Verify Default and Classic at 1440x900, 834x912, 390x844 and 320x568:

1. Configure `200%` cash commission and confirm the wallet card shows exactly `充值返现 200%`.
2. Disable cash commission and confirm that line disappears.
3. Complete an invited discounted top-up and confirm the reward uses actual paid amount once.
4. Complete cash and balance subscription purchases and confirm only cash creates commission.
5. Redeem administrator and user-created balance/subscription codes and confirm only official sources create commission.
6. Submit, approve, partially reverse, freeze and unfreeze rewards; reconcile wallet and net amounts.
7. Confirm user logs contain one concise line per real reward/reversal and no duplicate callback noise.
8. Confirm settings, approval detail and mobile layouts have no overlap or horizontal breakage.

- [ ] **Step 5: Record evidence and final commit**

Document commands, pass counts, database before/after counts, browser viewports and screenshot paths. Run `git diff --check` and `git status --short`, then commit only local changes:

```bash
git add docs/superpowers/specs/2026-07-29-local-feature-acceptance-evidence.md
git commit -m "test(local): verify invitation commission lifecycle"
```

Do not push, deploy or create a Release.
