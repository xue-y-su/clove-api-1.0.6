# Recovery Final16 Delivery Verification

- Date: 2026-07-19 (Asia/Shanghai)
- Source tree: `D:\restore-replay-fuzzy-20260719\extracted-source`
- Branch: `recovery/production-restore-20260719`
- Recovery baseline: `9b2d76b8`
- Current runtime image: `clove-api:qa-20260719-final16`
- Current runtime URL: `http://127.0.0.1:18085`

## Scope

This verification follows source recovery. It revalidates the recovered backend,
both frontend themes, the embedded Snow assets, real MySQL/PostgreSQL/ClickHouse
database paths, and a newly built Docker runtime without replacing the existing
final15 QA container.

## Source Recovery Checks

| Check | Result |
| --- | --- |
| Snow embedded assets | PASS: 111 files, 19,910,364 bytes, no missing references |
| Merge-conflict marker scan | PASS: none found outside ignored build dependencies |
| `git diff --check` | PASS: no whitespace errors; Git emitted only CRLF conversion notices |
| Go compile gate | PASS: `go test -run '^$' -count=1 ./...` |
| Go static analysis | PASS: `go vet ./...` |
| Go full regression | PASS: `go test -p 1 ./... -count=1` |

## Frontend Gates

### Default

| Command | Result |
| --- | --- |
| `bun test` | PASS |
| `bun run typecheck` | PASS |
| `bun run lint` | PASS with warning-only repository baseline |
| `bun run format:check` | PASS |
| `bun run audit:source` | PASS |
| `bun run build` | PASS |

### Classic

| Command | Result |
| --- | --- |
| `bun test` | PASS: 137 tests, 2,097 assertions |
| `bun run eslint` | PASS: no errors |
| `bun run lint` | PASS |
| `bun run i18n:lint` | PASS |
| `bun run build` | PASS |

The recovery repair restored the missing user-redemption test import, corrected
IP-audit timestamps to use the active locale, and restored the compact
invitation application/revocation modal flow. Classic formatting and license
headers were normalized with the repository-provided formatters.

## Database Matrix

Each database used a newly created, no-volume temporary container. Containers
were stopped with `--rm` after their tests completed; the existing QA database
and containers were not modified.

| Database | Tests | Result |
| --- | --- | --- |
| PostgreSQL 16 | Full migration/idempotency, IP-security rollup accumulation, Token key `CHAR(48)` to `VARCHAR(128)` migration | PASS |
| MySQL 8.4 | Full migration/idempotency, source-attribution migration, long setup version, MySQL 5.7-compatible IP projection, audit outbox canonical conflict handling, Token key migration | PASS |
| ClickHouse 25.8 | Real log table migration, TTL, duplicate `event_id` deduplication, IP projection, oversized-event quarantine, retry behavior | PASS |

## Docker Runtime

`docker build --pull=false -t clove-api:qa-20260719-final16 .` completed from
the current recovered source. The resulting scratch runtime image is 64.4 MB.

An isolated final16 container was attached to the existing QA backend network on
`127.0.0.1:18085`, with its projection worker disabled to avoid concurrent QA
side effects. It passed all of the following:

- `/healthcheck` exited successfully.
- `/api/status` returned HTTP 200 with `success: true`.
- `/`, `/dashboard/overview`, `/wallet`, `/user-redemptions`, and `/playground`
  returned the SPA shell without a 500 response.
- The container runs as `10001:10001` and has no mounts.
- Startup logs contained no panic, fatal error, migration failure, or HTTP 5xx.

## Browser Smoke

The final16 image was checked in a separate browser session. Desktop and mobile
public home pages rendered without clipping or overlap. The sign-in page exposed
localized labels, password visibility control, agreement confirmation, and
disabled submit state before required input. Unauthenticated dashboard, common
and IP-audit logs, wallet, user-redemption, and playground routes consistently
redirected to sign-in with their original route preserved. Browser error capture
was empty.

Artifacts are stored locally under
`.verification-output/runtime-final16-dogfood/`. The browser pass intentionally
did not submit credentials or mutate QA data. Authenticated workflow coverage is
provided by the prior live runtime verification at
`docs/superpowers/verification/2026-07-19-production-readiness.md` and by the
current automated backend/frontend regressions.

## Release State

The recovered source passes the validated delivery gates above. Production
deployment still requires the HTTPS origin, secure-cookie, trusted-proxy,
database, Redis, and stable-secret configuration documented in
`docs/production-deployment.md` and `.env.production.example`.
