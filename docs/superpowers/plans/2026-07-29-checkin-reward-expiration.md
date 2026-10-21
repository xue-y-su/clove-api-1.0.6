# Check-in Reward Expiration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an opt-in policy that expires only the unused portion of newly issued daily check-in rewards at the next server-local midnight.

**Architecture:** Persist each reward's remaining amount and expiry on its existing `checkins` row. Integrate debit consumption and overdue settlement into the central transactional wallet boundary, then add a quiet master-only minute worker for eventual settlement. Expose lifecycle data through the existing check-in API and add the switch and current-reward notice to both frontends.

**Tech Stack:** Go, GORM, SQLite/MySQL/PostgreSQL, React/TypeScript, React Hook Form/Zod, Semi UI, Bun tests, i18next.

---

### Task 1: Setting, Schema, and Expiry Timestamp

**Files:**
- Modify: `setting/operation_setting/checkin_setting.go`
- Modify: `model/checkin.go`
- Test: `model/checkin_reward_test.go`

- [ ] **Step 1: Write failing tests for the default setting and next-midnight timestamp**

Add assertions that `GetCheckinSetting().ExpireUnusedQuota` defaults to false and that `nextLocalMidnight(time.Date(2026, 7, 29, 13, 5, 0, 0, location))` returns midnight on July 30 in the same location, including a non-UTC fixed zone.

```go
func TestCheckinRewardExpirationDefaultsDisabled(t *testing.T) {
    require.False(t, operation_setting.GetCheckinSetting().ExpireUnusedQuota)
}

func TestNextLocalMidnight(t *testing.T) {
    location := time.FixedZone("UTC+8", 8*60*60)
    now := time.Date(2026, 7, 29, 13, 5, 0, 0, location)
    require.Equal(t, time.Date(2026, 7, 30, 0, 0, 0, 0, location).Unix(), nextLocalMidnight(now))
}
```

- [ ] **Step 2: Run the focused tests and verify RED**

Run: `go test ./model -run 'Test(CheckinRewardExpirationDefaultsDisabled|NextLocalMidnight)$' -count=1`

Expected: compile failure because the setting field and helper do not exist.

- [ ] **Step 3: Add the setting and persisted lifecycle columns**

Add `ExpireUnusedQuota bool \`json:"expire_unused_quota"\`` to `CheckinSetting`, default it to false, and add the following fields to `Checkin`:

```go
QuotaRemaining int64 `json:"quota_remaining" gorm:"not null;default:0"`
ExpiresAt      int64 `json:"expires_at" gorm:"not null;default:0;index:idx_checkin_expiry,priority:1"`
ReclaimedQuota int64 `json:"reclaimed_quota" gorm:"not null;default:0"`
SettledAt      int64 `json:"settled_at" gorm:"not null;default:0;index:idx_checkin_expiry,priority:2"`
```

Implement:

```go
func nextLocalMidnight(now time.Time) int64 {
    year, month, day := now.In(time.Local).Date()
    return time.Date(year, month, day+1, 0, 0, 0, 0, time.Local).Unix()
}
```

Tests temporarily set and restore `time.Local` so server-local behavior is deterministic.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run: `go test ./model -run 'Test(CheckinRewardExpirationDefaultsDisabled|NextLocalMidnight)$' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add setting/operation_setting/checkin_setting.go model/checkin.go model/checkin_reward_test.go
git commit -m "feat(checkin): add reward expiration metadata"
```

### Task 2: Reward Issuance Captures the Policy

**Files:**
- Modify: `model/checkin.go`
- Test: `model/checkin_reward_test.go`

- [ ] **Step 1: Write failing issuance tests**

Build check-ins using a new pure helper and assert these two cases:

```go
permanent := newCheckinReward(7, "2026-07-29", 1200, now, false)
require.Zero(t, permanent.QuotaRemaining)
require.Zero(t, permanent.ExpiresAt)

expiring := newCheckinReward(7, "2026-07-29", 1200, now, true)
require.EqualValues(t, 1200, expiring.QuotaRemaining)
require.Equal(t, expectedMidnight, expiring.ExpiresAt)
require.Zero(t, expiring.SettledAt)
```

- [ ] **Step 2: Run and verify RED**

Run: `go test ./model -run TestNewCheckinReward -count=1`

Expected: compile failure because `newCheckinReward` is missing.

- [ ] **Step 3: Implement policy capture at issuance**

Create `newCheckinReward(userID, date string, quota int, now time.Time, expiring bool) *Checkin`, setting lifecycle fields only when `expiring` is true. Make `UserCheckin` capture one `now := time.Now()` and use the helper with `setting.ExpireUnusedQuota`.

Remove the SQLite manual-compensation branch and use the same `DB.Transaction` path on every supported database; GORM transactions work on SQLite and this preserves atomic reward metadata plus wallet credit.

- [ ] **Step 4: Run focused and package tests**

Run: `go test ./model -run 'Test(NewCheckinReward|RandomCheckinQuota)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add model/checkin.go model/checkin_reward_test.go
git commit -m "feat(checkin): capture expiry policy on reward issuance"
```

### Task 3: Wallet Debits Consume Active Rewards

**Files:**
- Modify: `model/wallet_adjust.go`
- Modify: `model/checkin.go`
- Modify: `model/wallet_topup_adjust_test.go`
- Test: `model/checkin_expiration_test.go`

- [ ] **Step 1: Write failing debit and credit tests**

Use a fresh in-memory SQLite database migrated with `User`, `Checkin`, `WalletLedgerEntry`, and `AuditOutboxEvent`. Assert a `-30` debit changes an active reward from 100 to 70, a later `-80` debit settles it at zero, and a positive credit leaves the remainder unchanged. Repeat the negative assertion with representative categories `relay`, `task`, `user_redemption_reserve`, and `admin_adjust` because the behavior must be category-independent.

- [ ] **Step 2: Run and verify RED**

Run: `go test ./model -run 'TestWallet(De|Cre)bit.*CheckinReward' -count=1`

Expected: tests fail because `quota_remaining` is unchanged.

- [ ] **Step 3: Implement consumption after the durable debit**

Add:

```go
func consumeActiveCheckinRewardWithTx(tx *gorm.DB, userID int, debit, now int64) error
```

It selects the single row with `user_id = ? AND expires_at > ? AND settled_at = 0 AND quota_remaining > 0`, locks it after the user row is already locked, decrements by `min(debit, quota_remaining)`, and sets `settled_at = now` only when fully consumed. Call it from `adjustWalletWithTx` after the balance update and only when `delta < 0`; use an internal options struct with `skipCheckinLifecycle` for the expiry debit introduced in Task 4.

Update `setupTopupWalletTestDB` to migrate `Checkin` so all existing wallet tests exercise the integrated boundary.

- [ ] **Step 4: Run focused and wallet regression tests**

Run: `go test ./model -run 'Test(AdjustWallet|Wallet|OverrideWallet)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add model/checkin.go model/wallet_adjust.go model/wallet_topup_adjust_test.go model/checkin_expiration_test.go
git commit -m "feat(wallet): consume expiring check-in rewards on debits"
```

### Task 4: Transactional and Idempotent Expiration

**Files:**
- Modify: `model/checkin.go`
- Modify: `model/wallet_adjust.go`
- Test: `model/checkin_expiration_test.go`

- [ ] **Step 1: Write failing settlement tests**

Cover: partial remainder reclaimed from sufficient balance, reclaim clamped to a smaller non-negative balance, zero remainder after full use, repeated settlement producing one ledger row, and a debit after midnight settling the expired reward before applying its own delta. Assert operation ID `checkin-expiry:<id>`, category `checkin_expiry`, reference type `checkin`, reference ID equal to `checkin_date`, and context `unused_daily_reward`.

- [ ] **Step 2: Run and verify RED**

Run: `go test ./model -run 'Test(Settle|Expire|MidnightDebit)' -count=1`

Expected: compile failure because settlement entry points do not exist.

- [ ] **Step 3: Implement settlement with one lock order**

Add:

```go
func SettleDueCheckinRewards(limit int) (int, error)
func settleDueCheckinRewardsForUserWithTx(tx *gorm.DB, userID int, now int64) (bool, error)
func settleCheckinRewardWithLockedUser(tx *gorm.DB, userID int, checkinID int, now int64) (bool, error)
```

Every path locks `users` first and `checkins` second. Re-read the check-in under lock, calculate `reclaim := min(quota_remaining, max(user.quota, 0))`, write a wallet adjustment with the stable operation ID through the internal skip-lifecycle path when `reclaim > 0`, then atomically set `quota_remaining = 0`, `reclaimed_quota = reclaim`, and `settled_at = now`. Mark the row settled even when reclaim is zero. The due batch query selects only IDs where `expires_at > 0 AND expires_at <= now AND settled_at = 0`, ordered by ID and limited to the caller's bound.

Call per-user overdue settlement immediately after the user row lock and before computing the requested adjustment balance. Refresh the locked user's quota after settlement.

- [ ] **Step 4: Run focused tests twice to prove idempotency**

Run: `go test ./model -run 'Test(Settle|Expire|MidnightDebit)' -count=2`

Expected: PASS with exactly one expiry ledger row in repeated-settlement tests.

- [ ] **Step 5: Commit**

```bash
git add model/checkin.go model/wallet_adjust.go model/checkin_expiration_test.go
git commit -m "feat(checkin): reclaim unused daily rewards transactionally"
```

### Task 5: Quiet Master-Only Worker

**Files:**
- Create: `service/checkin_expiration_task.go`
- Create: `service/checkin_expiration_task_test.go`
- Modify: `main.go`

- [ ] **Step 1: Write failing worker eligibility tests**

Extract `checkinExpirationTaskEnabled() bool` and assert it follows `common.IsMasterNode`. Add a runner test by replacing a package-level settlement function with a counting fake and verify one invocation per `runCheckinExpirationOnce`, guarded by an atomic running flag.

- [ ] **Step 2: Run and verify RED**

Run: `go test ./service -run TestCheckinExpiration -count=1`

Expected: compile failure because the worker functions are absent.

- [ ] **Step 3: Implement and wire the worker**

Follow `subscription_reset_task.go`: `sync.Once`, `atomic.Bool`, batch size 300, startup run, then a one-minute ticker. Return immediately on non-master nodes. Loop bounded batches through `model.SettleDueCheckinRewards`; emit only one warning when a batch errors and no routine startup or success log. Call `service.StartCheckinExpirationTask()` beside the subscription task in `main.go`.

- [ ] **Step 4: Run focused tests**

Run: `go test ./service -run TestCheckinExpiration -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add service/checkin_expiration_task.go service/checkin_expiration_task_test.go main.go
git commit -m "feat(checkin): schedule unused reward expiration"
```

### Task 6: API Lifecycle Projection

**Files:**
- Modify: `model/checkin.go`
- Test: `model/checkin_reward_test.go`

- [ ] **Step 1: Write a failing projection test**

Create an expiring check-in record and assert `GetUserCheckinStats` includes `quota_remaining`, `expires_at`, and `reclaimed_quota` in the matching `CheckinRecord`, while permanent historical rows return zero values.

- [ ] **Step 2: Run and verify RED**

Run: `go test ./model -run TestGetUserCheckinStatsProjectsRewardLifecycle -count=1`

Expected: compile or assertion failure because the fields are absent.

- [ ] **Step 3: Extend the response projection**

Add the three JSON fields to `CheckinRecord` and copy them from each persisted record. Do not expose internal `settled_at` or database IDs.

- [ ] **Step 4: Run focused tests**

Run: `go test ./model -run 'Test(GetUserCheckinStats|NewCheckinReward)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add model/checkin.go model/checkin_reward_test.go
git commit -m "feat(checkin): expose reward expiration status"
```

### Task 7: Default Frontend Switch and Notice

**Files:**
- Modify: `web/default/src/features/system-settings/types.ts`
- Modify: `web/default/src/features/system-settings/billing/index.tsx`
- Modify: `web/default/src/features/system-settings/billing/section-registry.tsx`
- Modify: `web/default/src/features/system-settings/general/checkin-settings-section.tsx`
- Modify: `web/default/src/features/profile/types.ts`
- Modify: `web/default/src/features/profile/components/checkin-calendar-card.tsx`
- Create: `web/default/src/features/system-settings/general/checkin-settings-section.test.tsx`
- Modify: `web/default/src/i18n/static-keys.test.ts`
- Modify: `web/default/src/i18n/locales/{en,fr,ja,ru,vi,zh,zh-TW}.json`

- [ ] **Step 1: Write failing form/default/locale tests**

Assert the typed default is false, the switch emits `checkin_setting.expire_unused_quota`, and every locale contains `Expire unused check-in rewards daily`, `New check-in rewards expire at the next midnight. Wallet spending uses the reward first.`, and `Unused check-in reward expires at midnight`.

- [ ] **Step 2: Run and verify RED**

Run: `bun test src/features/system-settings/general/checkin-settings-section.test.tsx src/i18n/static-keys.test.ts` from `web/default`.

Expected: missing-field and missing-translation failures.

- [ ] **Step 3: Implement the default UI**

Add `expireUnusedQuota` to the Zod schema, defaults, dirty update list, settings type, global billing defaults, and section registry. Render a `Switch` under the enabled check-in section. Extend `CheckinRecord` with the lifecycle fields and show the compact notice only when today's record has `expires_at > 0 && quota_remaining > 0`; use existing layout primitives and quota formatting.

- [ ] **Step 4: Add all seven translations and run checks**

Run: `bun test src/features/system-settings/general/checkin-settings-section.test.tsx src/i18n/static-keys.test.ts && bun run typecheck` from `web/default`.

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/default/src
git commit -m "feat(default-ui): configure check-in reward expiration"
```

### Task 8: Classic Frontend Switch and Notice

**Files:**
- Modify: `web/classic/src/components/settings/OperationSetting.jsx`
- Modify: `web/classic/src/pages/Setting/Operation/SettingsCheckin.jsx`
- Modify: `web/classic/src/components/settings/personal/cards/CheckinCalendar.jsx`
- Create: `web/classic/src/pages/Setting/Operation/SettingsCheckin.test.jsx`
- Modify: `web/classic/src/i18n/production-controls-locales.test.js`
- Modify: `web/classic/src/i18n/locales/{en,fr,ja,ru,vi,zh,zh-CN,zh-TW}.json`

- [ ] **Step 1: Write failing form and locale tests**

Assert the new option defaults false, saves under `checkin_setting.expire_unused_quota`, and the classic locale dictionaries contain the switch label, description, and current-reward notice.

- [ ] **Step 2: Run and verify RED**

Run: `bun test src/pages/Setting/Operation/SettingsCheckin.test.jsx src/i18n/production-controls-locales.test.js` from `web/classic`.

Expected: missing-option and missing-translation failures.

- [ ] **Step 3: Implement the classic UI**

Add the option to both settings default maps, render an independent `Form.Switch` disabled only when the main check-in feature is off, include it in the existing save operation, and show the current reward notice in the calendar only when today's record is expirable and has a positive remainder.

- [ ] **Step 4: Add all eight classic translations and run checks**

Run: `bun test src/pages/Setting/Operation/SettingsCheckin.test.jsx src/i18n/production-controls-locales.test.js && bun run build` from `web/classic`.

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/classic/src
git commit -m "feat(classic-ui): configure check-in reward expiration"
```

### Task 9: Full Verification, Packaging, and Deployment Readiness

**Files:**
- Modify only files needed to fix failures introduced by Tasks 1-8
- Create: release source archive outside the Git working tree

- [ ] **Step 1: Format and run all backend tests**

Run: `gofmt -w model/checkin.go model/checkin_reward_test.go model/checkin_expiration_test.go model/wallet_adjust.go model/wallet_topup_adjust_test.go service/checkin_expiration_task.go service/checkin_expiration_task_test.go main.go`

Run: `go test ./... -count=1`

Expected: PASS.

- [ ] **Step 2: Run both frontend suites and builds**

Run from `web/default`: `bun test ./src ./rsbuild.config.test.ts && bun run lint && bun run format:check && bun run build:check`

Run from `web/classic`: `bun test && bun run lint && bun run build`

Expected: PASS.

- [ ] **Step 3: Verify migration on the local MySQL production-copy database**

Start the existing local Docker stack without deleting volumes, build the application image, and confirm `checkins` gains only the four additive columns plus `idx_checkin_expiry`. With the option off, check-in remains permanent. With it on, create a test reward, debit it partially, advance/seed an overdue timestamp, run settlement, and verify wallet balance, ledger operation ID, remainder, reclaimed quota, and settled timestamp.

- [ ] **Step 4: Browser-check both themes**

At desktop and mobile widths, verify the admin switch saves and reloads, the current-day expiration notice fits without overlap, disabled state is clear, and ordinary check-in still works. Capture screenshots for the default and classic settings/profile views.

- [ ] **Step 5: Commit final fixes, push, and package**

```bash
git status --short
git add setting/operation_setting/checkin_setting.go model/checkin.go model/checkin_reward_test.go model/checkin_expiration_test.go model/wallet_adjust.go model/wallet_topup_adjust_test.go service/checkin_expiration_task.go service/checkin_expiration_task_test.go main.go web/default/src web/classic/src docs/superpowers/plans/2026-07-29-checkin-reward-expiration.md
git commit -m "test(checkin): verify daily reward expiration"
git push origin recovery/production-restore-20260719
git archive --format=zip --output ../clove-api-v1.0.0-beta-source.zip HEAD
```

Before any production deployment, stop only the application service, take a fresh MySQL dump, preserve Compose and `.env`, replace only the application image, leave MySQL/Redis/data/log volumes intact, start the application, verify migration and baseline counts, and keep the new option disabled until smoke checks pass.
