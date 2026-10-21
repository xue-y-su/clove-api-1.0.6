# Production Readiness Verification

- Date: 2026-07-19 02:35 +08:00
- Branch: `feature/user-governance-wallet-audit-playground`
- HEAD: `7dc3654a` plus the current uncommitted delivery worktree
- QA URL: `http://127.0.0.1:18084`
- Runtime image: `clove-api:qa-20260719-final13`
- Image ID: `sha256:9bffca9f45ca0260f95c37e50377b052945548ff96a5c8e03f24b39eefb5c698`
- Embedded binary SHA-256: `745fab8f858b3321cdd4c7e59bb8a71eebc04c257f1d119cbfbb2d4bcfb4c1f5`

## Result

The implemented source and local MySQL QA runtime satisfy the tested release gates. No application defect found in this verification remains open. The local QA container intentionally uses HTTP development settings; production deployment still requires the HTTPS, secure-cookie, trusted-proxy, database, Redis, and stable secret values documented in `.env.production.example`.

The QA relay timeout now matches the production example: `RELAY_TIMEOUT=0` and `STREAMING_TIMEOUT=300`. The prior inherited 30-second total relay timeout was too short for synchronous image generation and was removed before final acceptance.

## Runtime Acceptance

| Area | Result | Evidence |
|---|---|---|
| User invitations | PASS | Application and owner revocation use compact dialogs; administrator issue/revoke actions and audit events are available. |
| User redemption creation | PASS | Single and red-packet modes use wallet display units; one-hour expiry creates successfully; creation and list copy paths use the shared fallback. |
| Redemption governance | PASS | User cancel refunds the unused balance and immediately refreshes the wallet; Admin can view, disable, enable, terminate with refund/forfeit, delete, and inspect claims. |
| Redemption localization | PASS | Sidebar entry, statuses, filters, settlement labels, and governance dialog titles render in Chinese. |
| Profile identity | PASS | The profile card is compact, editing opens in a dialog, the browser uploads a selected image directly to the configured image host, no avatar URL field is exposed, nickname cooldown behavior is covered, and the avatar label renders as `头像`. |
| Playground reasoning | PASS | The control remains available for every model; `off` was exercised against `deepseek-r1`; visible response and raw response contained only `OK`, with no reasoning output. |
| Playground images | PASS | Image mode exposes every selected-group model. A real `gpt-image-2` request used `playground-国模`, routed to channel `#16`, returned HTTP 200 in 60.6 seconds, rendered the image, and copied a valid URL. |
| Custom API Key | PASS | The optional custom key is under Advanced Settings; a short value shows the exact 16-128 character requirement; a valid custom key was created, revealed, and copied byte-for-byte without an added `sk-`; the temporary key was deleted. |
| Task logs | PASS | Task log page loads without 500 or indefinite loading. |
| IP audit | PASS | Thirty-day Root view returned 450 rows; a local-IP row expanded into its event timeline without failure. |
| Usage audit | PASS | Playground consumption, IP audit reads, wallet adjustment, and `user.redemption.cancel` events appear in the common log UI. |
| Subscription group isolation | PASS | Real MySQL/HTTP checks returned only the matching subscription: `pro=900`, `standard=8000`; wallet funding was absent under `subscription_only`. |
| API Key balance UI | PASS | Public page rendered an active fixed `pro` Key with subscription-only availability of `900.00` in the configured wallet display unit. |

All temporary subscription/balance fixtures were removed after verification.

## Automated Gates

| Gate | Result |
|---|---|
| `go test ./... -count=1` | PASS, all packages |
| Default `bun test` | PASS, 256 tests / 4142 assertions |
| Classic `bun test` | PASS, 136 tests / 2096 assertions |
| Default typecheck | PASS |
| Default oxlint | PASS with warning-only repository baseline, 0 errors |
| Default protected-header format check | PASS |
| Default source/i18n audit | PASS |
| Default production build | PASS |
| Classic ESLint | PASS, 0 errors; 4 warning-only pre-existing missing-header findings |
| Classic i18n lint | PASS |
| Classic Prettier check | PASS |
| Classic production build | PASS |
| `git diff --check` | PASS; only Git LF-to-CRLF notices |
| Known runtime credential scan | PASS, no supplied or temporary credentials found in source |

## MySQL 8.4 Gates

Tests ran against an isolated, no-volume MySQL 8.4.10 container on `127.0.0.1:13307`. The container and database were destroyed after completion.

| Package | Tests | Result |
|---|---|---|
| `model` | Full migration/idempotency, IP projection compatibility, setup version, source attribution migration, duplicate username mapping | PASS, 5/5 |
| `service` | Audit outbox `clientFoundRows` canonical conflict handling | PASS, 1/1 |
| `controller` | Token Key column migration from `CHAR(48)` to `VARCHAR(128)` on a clean database | PASS, 1/1 |

The running QA MySQL database separately passed live redemption reserve/refund, audit projection, subscription group filtering, and public token-balance requests.

## Runtime Health

- Container `cloveapi-qa-final13b-app`: running, healthy, restart count 0.
- Since startup: 75 parsed API requests, all HTTP 2xx.
- No 4xx/5xx responses, application errors, panic, fatal error, or migration failure found in container logs.
- `final13`, `final12`, and `final11` containers remain stopped as rollback points; `final13` and `final13b` use the same image, with `final13b` carrying the production-aligned timeout environment.
- The running image label matches the final embedded binary SHA-256 above.

## Production Configuration Gate

Before exposing this build publicly:

1. Use `DEPLOYMENT_MODE=production` behind HTTPS.
2. Set `FRONTEND_BASE_URL`, `SESSION_COOKIE_TRUSTED_URL`, and `CORS_ALLOW_ORIGINS` to the real HTTPS origin.
3. Keep secure cookies enabled and provide stable, distinct session, crypto, Token HMAC, and Token encryption secrets.
4. Set `TRUSTED_PROXIES` to only the actual reverse-proxy IPs/CIDRs so IP audit data is authoritative.
5. Run one master migration/worker node per the documented `NODE_TYPE` and projection-worker settings.

The HTTP QA warning for an insecure session cookie and its trusted-proxy reliability banner are expected local configuration signals, not unresolved source defects.
