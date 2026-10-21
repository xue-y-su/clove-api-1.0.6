# CloveAPI r36 Production Verification

- Date: 2026-07-23 (Asia/Shanghai)
- Source tree: `D:\cloveapi\restored-source`
- Branch: `recovery/production-restore-20260719`
- Baseline commit: `b49b07fa`
- Runtime URL: `http://127.0.0.1:18096`
- Runtime image: `clove-api:qa-20260723-r36`
- Runtime container: `cloveapi-r36-canary`
- Rollback image/container: `clove-api:qa-20260723-r34` / running `cloveapi-r34-canary`
- Active theme after verification: `default`

## Scope

This pass verifies the seven customized production feature groups requested for
CloveAPI: playground image and reasoning controls, custom API keys, invitation
applications, profile editing and provider unlinking, global popup
announcements with managed media, Security & IP Audit, and system-level feature
controls. It also covers the coupled user-redemption, wallet, subscription, and
source-attribution behavior changed during the same recovery effort.

## Closed Defects

- Playground image mode lists the complete user model set. It sends quality,
  background, style, and response options only when the selected model supports
  them; the obsolete output-URL selector is absent. Reasoning controls remain
  available for every model, and the off state suppresses visible reasoning even
  if an upstream model still reasons internally. Relay requests use a stable,
  neutral gateway user agent instead of Go's default user agent, while explicit
  channel header overrides retain final priority.
- Custom API keys accept complete non-`sk-` values, keep the feature in advanced
  settings, explain length and character validation failures, quarantine
  collisions without leaking the conflicting key, and render the complete
  disabled status and remediation text.
- Invitation rejection state includes the review reason and exact reapply time.
  The rejected alert expires when the cooldown ends, application/revocation
  reasons are collected in dialogs, and the invitation reward section remains
  content-sized.
- Profile editing opens from the profile header. Nickname cooldown is stated
  explicitly without disabling avatar changes, uploaded avatar URLs are
  validated against configured hosts, and unlinking one provider preserves the
  current session.
- Administrators can upload optional announcement images to managed server
  storage. Published announcements open once for guests and once per user with
  durable acknowledgement and audit records. Media writes are synchronized,
  referenced files are validated, and orphan cleanup uses a 24-hour grace period
  plus a six-hour leased background task.
- Security & IP Audit is a standalone feature, associates events with the real
  user rather than Root, formats IPv4 and IPv6 safely, exposes explicit date and
  retention controls, and keeps projection/backfill queries bounded.
- User-redemption accounting uses wallet units rather than the legacy point
  scale. Active limits exclude already-expired records, random packet recipients
  are normalized before minimum-amount validation, and SQLite creation is
  serialized while MySQL/PostgreSQL retain row locking. Users create and inspect
  codes from Wallet; administrators can audit, inspect, disable, terminate,
  delete, refund, or forfeit all user-created codes.
- Subscription eligibility is restricted to the purchased group, source
  attribution binds a configured user ID, and the customized production
  features expose translated, atomic, server-enforced system controls.
- Classic Telegram login removes stale widget-generated iframe content when its
  component parameters change.

## Automated Verification

| Gate | Result |
| --- | --- |
| `go test -p 1 ./... -count=1` | PASS: all Go packages |
| `go vet -p 1 ./...` | PASS |
| MySQL `go test -p 1 ./model ./service ./controller -count=1` | PASS |
| PostgreSQL `go test -p 1 ./model ./service ./controller -count=1` | PASS |
| Default frontend tests | PASS: 342/342, 5,163 assertions |
| Classic frontend tests | PASS: 203/203, 3,013 assertions |
| Default typecheck, lint, format, source audit, and copyright | PASS |
| Default and Classic production builds | PASS |
| Classic format, ESLint, and `i18n:lint` | PASS |
| `git diff --check` | PASS before delivery |

MySQL and PostgreSQL tests used the dedicated local database containers, not
SQLite substitutes. Together with the full SQLite-backed suite, they cover
migrations, atomic option writes, announcement media, acknowledgement behavior,
invitation cooldowns, user-redemption accounting and concurrency, subscription
group eligibility, audit outbox projection, IP retention, and date-window
queries across all three supported databases.

Classic `i18n:status` is not treated as a passing gate because the upstream
classic locale catalog has historical gaps. On this source tree it reports
80-86% coverage: zh-TW 3322/4170, en 3617/4190, fr 3574/4210, ru 3585/4230,
ja 3551/4170, and vi 3566/4170. The new production controls and every locale key
added by this delivery are covered by dedicated seven/eight-locale tests and
`i18n:lint`, all of which pass.

## IP Audit Performance

All approximately 840,000 historical audit events were already projected. The
former slow path scanned approximately 208,000 raw events for the first partial
date bucket. UTC calendar-day boundaries now avoid that scan:

| Measurement | Result |
| --- | --- |
| Previous cold request | 16.73 seconds |
| Optimized cold request | 1.64 seconds |
| Optimized warm request | 307 milliseconds |
| Final 30-day result count | 4,868 |
| Browser-verified page size | 20 rows |

The 7-, 30-, and 90-day filters mean today plus the previous 6, 29, or 89 UTC
days. Both themes keep loading distinct from empty and error states.

## Live Runtime Verification

- r36 starts against MySQL and Redis, `/api/status` and `/` return HTTP 200, and
  the embedded default assets including `/static/js/index.e25a7f162b.js` load.
- Docker Hub authentication timed out during the final full-image rebuild. The
  local r36 image therefore overlays the current cross-compiled Go binary onto
  the already verified r34 application image. The running `/new-api` binary and
  the r36 build artifact have the same SHA-256 digest.
- Root, ordinary-user, and anonymous sessions were exercised in isolated browser
  origins. Ordinary users have no administrator navigation or endpoints.
- Default image mode lists text and image models as required. Selecting
  `gpt-image-2` reveals real quality and background controls; selecting a generic
  text model does not present unsupported controls. Reasoning off is selectable.
- ID 1 exercised all three rightcode models through
  `/pg/images/generations`. The two GPT image requests included real
  `quality=low` and `background=transparent` parameters. Each successful result
  returned a URL that independently responded HTTP 200 with `image/png`:

  | Model | Gateway result | Elapsed | Image verification |
  | --- | --- | ---: | --- |
  | `gpt-image-2` | HTTP 200 | 81.2 seconds | HTTP 200, `image/png` |
  | `gpt-image-2-vip` | HTTP 200 | 80.8 seconds | HTTP 200, `image/png` |
  | `nano-banana-2` | HTTP 200 | 64.9 seconds | HTTP 200, `image/png` |

  One earlier `nano-banana-2` attempt ended when rightcode closed the connection
  with `unexpected EOF`; the gateway refunded the full pre-charge and a single
  manual retry passed. Automatic retries remain disabled for non-idempotent image
  creation to avoid duplicate generations and upstream charges.
- Custom-key enabled/disabled states, optional non-`sk-` advanced input,
  collision remediation, invitation application presentation, wallet creation
  and history, subscription fallback, and user/admin redemption views rendered
  successfully.
- Profile editing opens as a header dialog and states the exact nickname wait.
  Avatar controls remain enabled. Provider unlink session preservation is also
  covered by deterministic frontend and backend regression tests.
- An isolated guest received the global announcement modal and acknowledged it.
  The administrator announcement composer exposes the square managed-image
  upload control and server-backed popup/max-size settings.
- Security & IP Audit loaded 4,868 30-day rows with explicit filters, Root-only
  associations, pagination, IPv4, and a full IPv6 value without overlap.
- Classic rendered its home, administrator navigation, announcement UI, chat
  reasoning selector, and image mode from the same verified frontend assets. The server theme
  was restored to `default` after verification.
- Browser console inspection found no errors. Application logs contain no panic,
  fatal, `wallet adjustment is invalid`, or database-error entries for the final
  verification window. The observed rightcode 502 and connection EOF were both
  surfaced to the caller and refunded rather than corrupting wallet state.
- ID 1 remained enabled as Root and authenticated with its pre-existing
  credential. No credential update was performed.

## Delivery Conditions

r36 is a local production-like canary, not a public production deployment. A
public rollout still requires the target HTTPS origin, secure cookies, trusted
proxy CIDRs, durable application secrets, production MySQL/Redis credentials,
backup and restore verification, and real external payment gateway credentials.
Those environment-specific controls are intentionally not embedded in source or
this report.
