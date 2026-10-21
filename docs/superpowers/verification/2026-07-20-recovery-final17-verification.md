# Recovery Final17 Delivery Verification

- Date: 2026-07-20 (Asia/Shanghai)
- Source tree: `D:\cloveapi\restored-source`
- Branch: `recovery/production-restore-20260719`
- Baseline commit: `87bb61d6`
- Runtime URL: `http://127.0.0.1:18088`
- Runtime image: `clove-api:qa-20260719-audit8`

## Scope

This pass continues the recovered-source delivery audit. It closes a real MySQL
configuration persistence defect, reruns the database matrix and full source
gates, verifies the previously reported user workflows against the running
application, and replaces the local audit container with a binary built from
the current source tree.

## Fixed Defect

`loadOptionsFromDatabase` normalized legacy SMTP values such as `"<nil>"` in
memory, but its persistence repair used the mutable package-level
`commonKeyCol`. A previous PostgreSQL connection can leave that value as
`"key"`; subsequent MySQL updates then match zero rows without returning an
error.

The repair now uses GORM's `Where(&Option{Key: key})` condition. GORM derives
the correct identifier quoting from the transaction's actual database dialect.
`TestLoadOptionsFromDatabaseRepairsLegacySMTPValueOnMySQL` now deliberately
sets the PostgreSQL-style column reference before connecting to MySQL. It was
observed failing with the stored `"<nil>"` value, then passed after the change.

## Automated Verification

| Gate | Result |
| --- | --- |
| Deterministic MySQL regression test | PASS after RED/GREEN cycle |
| MySQL/PostgreSQL targeted matrix | PASS: 8/8 tests |
| `go test -p 1 ./... -count=1` | PASS: all packages |
| `go vet ./...` | PASS |
| Default `bun run audit:source` | PASS |
| Default `bun test` | PASS: 264 tests, 4,158 assertions |
| Default typecheck, format check, production build | PASS |
| Default lint | PASS with the existing warning-only baseline, 0 errors |
| Classic `bun test` | PASS: 148 tests, 2,311 assertions |
| Classic ESLint, Prettier, i18n lint, production build | PASS |
| `git diff --check` | PASS |

The targeted database matrix ran in disposable MySQL and PostgreSQL containers
on `127.0.0.1:13306` and `127.0.0.1:15432`. Both containers were removed after
the suite completed. It covered full and idempotent migration, MySQL 5.7-safe
IP projection, PostgreSQL IP rollup, the legacy SMTP repair, long setup
versions, source-attribution migration, and the reserved `key` column query.

## Live Runtime Verification

The new container started with the existing controlled MySQL/Redis audit
configuration. `/api/status` returned `success: true` and version
`v1.0.0-rc.21.clove-enhanced`; startup logs show successful migration and no
panic, fatal error, or HTTP 5xx.

Authenticated browser checks confirmed the recovered behaviours that had
previously failed:

- Profile editing is a compact dialog, not an expanded card body.
- Playground keeps the reasoning control available and displays a clear,
  disabled image-model selector only when the selected group has no models.
- Wallet contains invitation application, code redemption, and user redemption
  creation; it does not expose the removed wallet ledger UI.
- User redemption creation explains wallet-unit amounts, one-time and red-packet
  modes, refund-on-expiry/cancel semantics, and administrator settlement.
- Administrator redemption audit exposes creator, copy, detail, disable,
  terminate, delete, claim history, and refund information.
- Invitation administration displays application, issued-code, copy, status,
  and revocation states without duplicate copy controls.
- IP audit rendered a real `172.19.0.1` record with registration/login activity
  and no browser console error after the application-container replacement.
- At `390x844`, wallet actions remain visible and the redemption audit table
  has a horizontal `overflow-x: auto` container rather than clipped columns.

## Runtime Image Method

The Dockerfile's upstream base images were unavailable because Docker Hub OAuth
requests repeatedly timed out from this host. Two native `docker build`
attempts failed before any source build stage began. This is an external network
condition, not a source or Docker daemon failure.

To keep the local audit deployment current, the image was produced from the
already verified scratch runtime base by replacing `/new-api` and `/healthcheck`
with static Linux binaries compiled from this source tree after both fresh
frontend production builds. `main.go` embeds those freshly built Default,
Classic, and tracked Snow assets, so the running `audit8` image contains the
current backend and frontend artifacts. The original audit application was
replaced only after a separate `18089` candidate container passed `/api/status`.

## Deployment Conditions

The local audit stack intentionally uses HTTP and reports an insecure-cookie
warning. A public deployment still requires the HTTPS origin, secure cookies,
trusted proxy CIDRs, stable session/crypto/token secrets, and production MySQL
and Redis values documented in `.env.production.example`.
