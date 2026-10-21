# Final Delivery Evidence

- Date: 2026-07-16 (Asia/Shanghai)
- Branch: `feature/user-governance-wallet-audit-playground`
- HEAD (code): `3142fd7a` (wallet ledger convergence tip)
- Workspace: `D:\restore-replay-fuzzy-20260719\extracted-source`
- Disk free (approx): C≈15.7GB, D≈11.0GB

## Delivery commits (newest first)

```
3142fd7a fix(wallet): route remaining user-quota paths through ledger
6369ac0a test(controller): migrate wallet ledger in manage-user fixtures
93c81f2c fix(wallet): route topup and admin quota through ledger
050b2b75 fix(user-redemptions): enforce list auth in service and audit cancels
c88b4b8d docs(delivery): record source-audit fix wave for redemptions
185ac344 fix(user-redemptions): admin cancel, finalize counting, creator cache
71e72f0d test(delivery): refresh evidence for MySQL smoke and source audit
31975a55 fix(user-redemptions): allow any admin to list and inspect all codes
5adadb42 fix(user-redemptions): commit expiry refund and scope admin list
fae30139 test(delivery): record dual-theme source completion gates
b27e07d7 feat(user-redemptions+email): complete dual-theme UI components and tests
```

## Feature plan Tasks 1–13

| Task | Status | Notes |
|---|---|---|
| 1 Workspace baseline | Done | earlier commit |
| 2 OwnProfile / governance | Done | earlier commit |
| 3 WalletLedger | Done | ledger hub + **full Δquota convergence** (topup/admin/redemption/invite/checkin/relay/task/mj) |
| 4 Audit outbox | Done | earlier commit |
| 5 Invitation codes | Done | rewards via ledger op-ids |
| 6 Token encryption/reveal | Done | earlier commit |
| 7 Wallet/token views | Done | earlier commit |
| 8 Playground parity | Done | earlier commit |
| 9 IP audit / records | Done | earlier commit |
| 10 User redemptions | Done | backend + dual-theme UI + admin-all policy + cancel audit |
| 11 Email templates | Done | Root APIs + Default/Classic settings UI |
| 12 Snow home | Done | vendored dist embed + verify script |
| 13 Cross-DB/theme delivery | Partial | SQLite packages + MySQL empty/API smoke green; PG/CH unavailable here |

## Product policy notes (user-confirmed)

- **Admin user-redemption visibility:** any Admin/Root may list and inspect all user redemption codes. Hierarchy filtering is **not** applied to admin redemption list/detail. Router `AdminAuth` is the gate.
- **Ordinary users:** only own codes (create/list/detail/cancel).
- **Admin cancel:** any Admin/Root may cancel any active user redemption code (`POST /api/user-redemption/admin/:id/cancel`).
- **Expiry refund:** claim of an already-expired code must **commit** the refund, then return expired error (do not roll back refund).

## Wallet ledger convergence (2026-07-16)

All major user-wallet Δquota paths go through `model.AdjustWalletWithTx` / `AdjustWallet` / `CreditTopUpWithTx` (`model/wallet_adjust.go`).

Covered:

- Topup (Stripe/Creem/Waffo/Pancake/Epay/admin complete)
- Admin add/sub/override quota
- User redemptions (reserve/refund), invitation rewards
- Checkin, admin redemption redeem, subscription balance purchase
- Relay wallet funding, task billing, midjourney refund
- Legacy invitee gift

Residual:

- `IncreaseUserQuota` / `DecreaseUserQuota` definitions remain; business callers largely cleared; decrease hardened with `WHERE quota >= ?`
- New-user `QuotaForNewUser` may still set initial `user.Quota` at insert (not a secondary Δ)
- Historical pre-ledger balances not backfilled (`history_complete` may stay false)

## Gate matrix

| Gate | Result | Evidence |
|---|---|---|
| `go test ./model ./service ./controller` (SQLite, post-ledger fixtures) | **PASS** | manage-user fixtures migrate `wallet_ledger_entries` + `audit_outbox_events` |
| MySQL 8.4 empty migrate + API smoke | **PASS** | this turn: `cloveapi_wallet_smoke` + `api-smoke.ps1` |
| MySQL wallet ledger rows | **PASS** | 3 ledger rows: admin_adjust / reserve / refund |
| PostgreSQL empty/upgrade | **SKIP** | no local PG / Docker |
| ClickHouse LOG_SQL_DSN | **SKIP** | ports/Docker unavailable |
| Full dual-theme browser E2E | **DEFERRED** | user scope: source + audit/fix; no screenshot matrix |
| Production secret scan (non-test) | **No live secrets** | only examples/fixtures |

## MySQL 8.4 acceptance detail (this turn)

- Server: local MySQL 8.4.9 (`MySQL84`), root TCP empty password on `127.0.0.1:3306`
- Smoke DB: `cloveapi_wallet_smoke`
- Binary: `D:\restore-replay-fuzzy-20260719\.tmp\wallet-mysql-smoke\cloveapi.exe` on port **18092**
- Env: `SQL_DSN=root@tcp(127.0.0.1:3306)/cloveapi_wallet_smoke?parseTime=true&charset=utf8mb4&loc=Local&allowNativePasswords=true`
- Migrated tables: **46** including `wallet_ledger_entries`, `user_redemption_codes`, `user_redemption_claims`, `email_templates`, `audit_outbox_events`
- Script: `D:\restore-replay-fuzzy-20260719\.tmp\wallet-mysql-smoke\api-smoke.ps1`
- Report: `D:\restore-replay-fuzzy-20260719\.tmp\wallet-mysql-smoke\api-smoke-report.txt`

### API smoke results

```
PASS status ready
SETUP already done
PASS login root id=1 role=100
PASS self id=1 quota=100000000 role=100
MANAGE_QUOTA success=True
PASS ledger total=1 items=1 first_cat=admin_adjust
PASS create redemption id=1 status=active
PASS cancel status=cancelled
PASS ledger_after cats=[user_redemption_refund,user_redemption_reserve,admin_adjust] total=3
PASS email_templates success=True count=1
PASS admin_list path=/api/user-redemption/admin?page=1&page_size=20 total=1
```

### DB verification after smoke

| Artifact | Result |
|---|---|
| `users` | root id=1 role=100 quota=**100100000** (setup 1e8 + admin +1e5; reserve/refund net 0) |
| `wallet_ledger_entries` | 3 rows |
| row1 | `admin_adjust` delta=+100000 balance_after=100100000 ref=`admin_quota` |
| row2 | `user_redemption_reserve` delta=-10000 balance_after=100090000 op=`user-redemption:1:reserve` |
| row3 | `user_redemption_refund` delta=+10000 balance_after=100100000 op=`user-redemption:1:refund` |
| `user_redemption_codes` | id=1 status=`cancelled` total_quota=10000 creator=1 |
| `audit_outbox_events` | 9 rows |

Smoke notes:

- Setup JSON field is **`confirmPassword`** (camelCase), not `confirm_password`
- Authenticated dashboard APIs require header **`New-Api-User`** matching session user id

## Source audit (ledger + redemptions)

### Fixed this arc (ledger convergence)

1. **Topup / admin quota** → `93c81f2c` routes through ledger
2. **Remaining user-quota paths** → `3142fd7a` (invite/checkin/relay/task/mj/etc.)
3. **Controller fixtures** → `6369ac0a` migrate wallet ledger tables for manage-user tests
4. **User redemption list auth + cancel audit** → `050b2b75`

### Fixed earlier (still valid)

1. Admin cancel API + finalize counting + creator cache → `185ac344`
2. Expired claim commits refund → `5adadb42`
3. Admin list any Admin/Root (no hierarchy) → `31975a55`

### Residual notes (non-blocking)

- Admin cancel is API-complete; dual-theme UI still primarily personal cancel
- PG / ClickHouse matrix and full browser E2E remain environment/scope deferred
- Full `go test ./...` not re-run in last wave; `./model ./service ./controller` green after ledger fixtures

### Intentionally untracked leftovers

```
?? .task9-acceptance/
?? docs/superpowers/plans/2026-07-08-playground-attachments-invite-limits.md
?? docs/superpowers/plans/2026-07-13-user-governance-wallet-audit-playground.md
```

Do not commit acceptance artifacts or draft plans. Leave `.tmp/wallet-mysql-smoke` untracked (local evidence only).

## Remaining for absolute production sign-off

1. PostgreSQL empty/upgrade/repeat-start matrix (needs local PG or working Docker)
2. Optional ClickHouse LOG_SQL_DSN event_id/TTL checks
3. Full dual-theme browser matrix (if product still requires UI visual sign-off)
4. Optional prune of large smoke binaries under `D:\restore-replay-fuzzy-20260719\.tmp\*` after review (~185MB exe under wallet-mysql-smoke)

## Conclusion

Functional delivery of planned Tasks 1–12 is implemented. Wallet Δquota paths converge on the ledger. Task 13 is **partially** complete: MySQL 8.4 empty-migration + live API smoke (**admin adjust / redemption reserve-refund / ledger categories / email templates / admin list**) are green on this machine. PG/CH and full browser matrix remain blocked/deferred.

Admin redemption policy matches confirmed product rule (any admin manages all codes, including cancel). No new production credential leaks found in this turn.
