# Invitation Poster Brand Logo Verification Closure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render the configured cross-origin brand image into downloadable invitation posters without tainting Canvas, retain the initial-letter fallback, and close the identified browser-verification gaps without changing production or the preserved QA database baseline.

**Architecture:** Both frontends normalize the configured logo through one shared pure resolver. Cross-origin HTTP(S) logos load from a fixed public same-origin endpoint whose focused service reads only the already-public, server-owned `common.Logo`, fetches through the existing SSRF-protected client, validates bounded image bytes, and keeps a five-minute cache with a short failure backoff. DNS validation, cache lookup, and fetch work are bounded and coalesced, and the current SSRF policy is rechecked before cached data is returned. A browser `Image` cannot provide the custom header required by `UserAuth`, so the route inherits the API group's global read limit and is excluded from gzip. Proxy rejection goes to the initial-letter fallback without a direct cross-origin bypass. Browser acceptance proves actual brand pixels, exportability, the fallback path, and the previously audited business workflows with reversible fixtures.

**Tech Stack:** Go 1.25, Gin, `golang.org/x/image/webp`, Bun tests, React/TypeScript, Classic React/JavaScript, Playwright, Docker, MySQL/PostgreSQL/SQLite migration harnesses, PowerShell.

---

### Task 1: Shared Logo URL Resolution and Positive Canvas Assertion

**Files:**
- Modify: `web/shared/invitation-poster.test.js`
- Modify: `web/shared/invitation-poster.js`
- Modify: `web/shared/invitation-poster.d.ts`

- [ ] **Step 1: Write resolver tests before adding the export**

Add a table-driven test importing `resolveInvitationPosterLogoUrl`. Assert it maps normalized cross-origin HTTP(S), including backslash-confusion forms, to the proxy while same-origin, relative, data, blob, and empty values remain unchanged and unsupported schemes resolve empty. Use `locationOrigin = "https://clove.test"` and assert these primary outcomes:

```js
expect(resolveInvitationPosterLogoUrl("https://img.test/logo.webp", locationOrigin)).toBe(
  "/api/invitation/poster-logo",
);
expect(resolveInvitationPosterLogoUrl("https://clove.test/logo.png", locationOrigin)).toBe(
  "https://clove.test/logo.png",
);
expect(resolveInvitationPosterLogoUrl("/logo.png", locationOrigin)).toBe("/logo.png");
expect(resolveInvitationPosterLogoUrl("data:image/png;base64,AA==", locationOrigin)).toBe(
  "data:image/png;base64,AA==",
);
expect(resolveInvitationPosterLogoUrl("blob:https://clove.test/id", locationOrigin)).toBe(
  "blob:https://clove.test/id",
);
expect(resolveInvitationPosterLogoUrl("", locationOrigin)).toBe("");
expect(resolveInvitationPosterLogoUrl("javascript:alert(1)", locationOrigin)).toBe("");
```

- [ ] **Step 2: Add a renderer test that distinguishes the logo draw from the QR draw**

Use a logo object `{ naturalWidth: 200, naturalHeight: 100 }`, make `loadImage` return that exact object, render with a non-empty `logoUrl`, and assert exactly one `drawImage` call has the logo object as `args[0]` while another has the QR canvas. Also assert the logo branch did not draw the initial letter `C`.

- [ ] **Step 3: Run the shared test and observe the intended red state**

Run:

```powershell
bun test web/shared/invitation-poster.test.js
```

Expected: FAIL because `resolveInvitationPosterLogoUrl` is not exported; after temporarily excluding that import, the positive renderer assertion must already pass and prove the assertion shape is valid.

- [ ] **Step 4: Implement the minimal pure resolver and declaration**

Implement this contract without reading global browser state inside the helper:

```js
export function resolveInvitationPosterLogoUrl(value, locationOrigin = "") {
  const raw = String(value || "").trim();
  if (!raw || raw.startsWith("/") || raw.startsWith("./") || raw.startsWith("../")) return raw;
  let parsed;
  try {
    parsed = new URL(raw, locationOrigin || undefined);
  } catch {
    return "";
  }
  if (parsed.protocol === "data:" || parsed.protocol === "blob:") return raw;
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") return "";
  return locationOrigin && parsed.origin === locationOrigin
    ? raw
    : "/api/invitation/poster-logo";
}
```

Declare the `(value?: string, locationOrigin?: string) => string` helper in the shared `.d.ts` file. Add optional `signal` input, keep `loadBrowserImage` setting `crossOrigin = "anonymous"`, use a six-second browser deadline, and clear the image source on timeout or abort.

- [ ] **Step 5: Run the shared test to green**

Run `bun test web/shared/invitation-poster.test.js` and expect all tests to pass.

### Task 2: Default and Classic Wiring

**Files:**
- Modify: `web/default/src/features/invitations/components/invitation-ui.test.ts`
- Modify: `web/default/src/features/invitations/components/invitation-qr-dialog.tsx`
- Modify: `web/classic/src/components/invitations/invitation-ui.test.js`
- Modify: `web/classic/src/components/invitations/InvitationQRModal.jsx`

- [ ] **Step 1: Write wiring assertions first**

In both source-composition tests, require the dialog source to import both URL helpers and pass:

```ts
logoUrl: resolveInvitationPosterLogoUrl(
  logo,
  typeof window === 'undefined' ? '' : window.location.origin
)
signal: controller.signal
```

Use `props.logo` in Default and `logo` in Classic, matching their current component signatures.

- [ ] **Step 2: Run both focused frontend tests and observe red**

Run:

```powershell
Set-Location web/default; bun test src/features/invitations/components/invitation-ui.test.ts
Set-Location ../classic; bun test src/components/invitations/invitation-ui.test.js
```

Expected: both FAIL because neither dialog currently references the resolver.

- [ ] **Step 3: Wire the resolver into both dialogs**

Import the resolver from `web/shared/invitation-poster.js`, normalize the logo only at the renderer call, and abort the render in effect cleanup. Do not change visible layout, poster copy, size handling, filename behavior, or fallback state.

- [ ] **Step 4: Run both focused tests and shared tests to green**

Run the two commands from Step 2 plus `bun test web/shared/invitation-poster.test.js`; expect all to pass.

### Task 3: Bounded, Validated Poster Logo Service

**Files:**
- Create: `service/invitation_poster_logo.go`
- Create: `service/invitation_poster_logo_test.go`

- [ ] **Step 1: Write the service success and cache tests first**

Define tests in package `service` around an unexported `posterLogoLoader` with injected `*http.Client`, timeout, byte limit, pixel limit, and one-entry cache. Serve valid WebP bytes while deliberately omitting CORS headers. Assert `load(ctx, url)` returns `image/webp`, the exact body, a quoted SHA-256 ETag, decoded non-zero dimensions, and only one upstream request across two calls with the same URL.

- [ ] **Step 2: Write failure tests before implementation**

Cover each behavior independently:

```text
empty or non-HTTP(S) URL -> ErrPosterLogoUnavailable before any request
301/302 ending at a client-rejected private target -> error
404 and 500 -> error
body larger than maxBytes by one byte -> error
random bytes and HTML mislabeled image/png -> error
decoded dimensions exceeding maxPixels -> error
context deadline / slow upstream -> error
PNG, JPEG, GIF, and WebP fixtures -> detected canonical content type
failed responses -> never cached as image data and suppress repeated upstream attempts for 15 seconds
changed configured URL -> fetches and replaces the one-entry cache
successful cache entry -> expires after five minutes
current SSRF policy rejects cached URL -> cache is invalidated before returning bytes
slow DNS/policy validation -> respects cancellation and the five-second operation deadline
concurrent validation, cache lookup, success, or failure -> one bounded operation per URL
```

- [ ] **Step 3: Run service tests and observe red**

Run:

```powershell
go test ./service -run PosterLogo -count=1 -v
```

Expected: FAIL because `posterLogoLoader`, its result type, and sentinel error do not exist.

- [ ] **Step 4: Implement bounded loading through the protected client**

Implement:

```go
type PosterLogo struct {
    Body        []byte
    ContentType string
    ETag        string
    Width       int
    Height      int
}

func FetchInvitationPosterLogo(ctx context.Context, logoURL string) (PosterLogo, error)
```

The production loader must use `GetSSRFProtectedHTTPClient()`, create one five-second operation context covering DNS validation and fetch, accept only 2xx, read with `io.LimitReader(resp.Body, maxBytes+1)`, reject overflow, detect and decode PNG/JPEG/GIF/WebP bytes rather than trusting headers, and reject zero or excessive dimensions. Cache only the most recent successful URL/result for five minutes, apply a 15-second failure backoff, revalidate the current SSRF policy before cache access, and use `singleflight` so concurrent callers share validation/cache/fetch work. Return copied byte slices so callers cannot mutate cache contents. Use a bounded limit of 2 MiB and 16 megapixels.

- [ ] **Step 5: Run service tests to green and check for races**

Run:

```powershell
go test ./service -run PosterLogo -count=1 -v
go test -race ./service -run PosterLogo -count=1
```

Expected: all focused tests pass and the race detector reports no race.

### Task 4: Fixed Public Controller and Read-Limited Route

**Files:**
- Create: `controller/invitation_poster_logo.go`
- Create: `controller/invitation_poster_logo_test.go`
- Modify: `router/api-router.go`
- Modify: `router/invitation_routes_test.go`

- [ ] **Step 1: Write controller response tests first**

Inject the fetch function through a package variable restored with `t.Cleanup`. Assert the controller reads `common.Logo` while holding `common.OptionMapRWMutex` only long enough to copy the string, ignores query parameters such as `?url=https://attacker.test/x`, and returns:

```text
200 with validated bytes and detected Content-Type
ETag from the service
Cache-Control: public, max-age=300
X-Content-Type-Options: nosniff
304 with an empty body when If-None-Match matches
404 with a non-image JSON envelope when the Logo is empty or loading fails
```

- [ ] **Step 2: Extend the route registration test before adding the route**

Add `GET /api/invitation/poster-logo` to the expected route list and a request-level test proving it does not require the custom `New-Api-User` header used by Axios. Assert the fixed route has the repository's public read rate limiter and that adding `?url=https://attacker.test/x` never changes the configured upstream.

- [ ] **Step 3: Run focused controller and router tests and observe red**

Run:

```powershell
go test ./controller -run InvitationPosterLogo -count=1 -v
go test ./router -run InvitationRoutes -count=1 -v
```

Expected: controller symbols are missing and route registration lacks the endpoint.

- [ ] **Step 4: Implement controller and register the fixed GET route**

Register `apiRouter.GET("/invitation/poster-logo", controller.GetInvitationPosterLogo)` before the authenticated invitation group. The route inherits the API group's global read limiter and is excluded from gzip; it must not reuse the much tighter critical-write limiter because browsers behind shared NATs fetch this public asset independently. The handler must never accept a caller-selected URL and must not log successful loads or cache hits. Concurrent callers for the same configured URL must share one bounded operation.

- [ ] **Step 5: Run focused backend tests to green**

Run the two commands from Step 3 and expect all tests to pass.

### Task 5: Browser Acceptance for Real Logo and Fallback

**Files:**
- Modify: `web/default/e2e/production-invitation-flows.spec.ts`
- Store transient evidence only under: `.verification-output/playwright/`

- [ ] **Step 1: Add a public read-limited endpoint and poster assertion before rebuilding**

Add a Playwright test that opens the wallet invitation poster using the real configured no-CORS WebP, records a successful request to `/api/invitation/poster-logo`, and samples the 64x64 brand area from the generated Canvas. Compare its pixel digest with a forced endpoint-failure run and assert the real-logo digest differs from the initial-letter fallback; separately assert `canvas.toBlob` returns a non-empty `image/png` blob.

- [ ] **Step 2: Cover both themes and every size without weakening pixel assertions**

For Default and Classic, iterate `4:5`, `3:4`, `9:16`. Assert exact output dimensions (`1080x1350`, `1080x1440`, `1080x1920`), nonblank pixels, QR decodability, real-logo pixels, origin-clean `toBlob`, and a completed PNG download. Restore `theme.frontend=default` in `finally` even when an assertion fails.

- [ ] **Step 3: Prove the retained fallback path**

Temporarily intercept `/api/invitation/poster-logo` with a non-image 404, assert the brand-region digest equals the established initial-letter fixture, and assert PNG export and download still succeed in both themes.

- [ ] **Step 4: Rebuild the local QA container while preserving its `/data` volume**

Use `scripts/rebuild-local-mysql-qa.ps1` with container `cloveapi-current-source-checkin-expiry-qa`, a new local image tag derived from the current short commit, and port `18104`. Do not delete, recreate, or rename the volume `cloveapi-current-source-mysql-qa-data`; keep the script's automatic rollback container until health succeeds.

- [ ] **Step 5: Run focused invitation Playwright acceptance**

Generate fresh local storage states, set the existing local QA environment variables, and run:

```powershell
bunx playwright test e2e/production-invitation-flows.spec.ts --project=chromium
```

Expected: all invitation tests pass with evidence showing actual brand pixels rather than the fallback letter.

### Task 6: Close Audited Browser Workflow Gaps

**Files:**
- Modify: `web/default/e2e/production-business-flows.spec.ts`
- Modify: `web/default/e2e/production-bootstrap-flows.spec.ts`

- [ ] **Step 1: Add subscription-redemption end-to-end acceptance**

Create a reversible QA fixture for a purchasable subscription plan, buy multiple entitlement-bearing codes through the wallet-balance purchase API/UI, claim one from a second QA user, and assert the resulting active subscription has the selected plan and quantity. Record and restore both users' balances; delete created codes, claims, orders, entitlements, ledger rows, logs, and fixture plan in a transaction in `finally`.

- [ ] **Step 2: Add locked model-catalog visibility acceptance**

Seed a temporary group unavailable to the QA user plus one model and explicit price. Visit the model marketplace and assert the locked group, model name, lock state, and price are visible while execution/purchase controls remain unavailable. Remove only the fixture rows and restore the original group/model configuration in `finally`.

- [ ] **Step 3: Add Chinese username registration and login acceptance**

Use a unique Chinese username through the real registration form, assert successful session creation, sign out, sign back in with the same username, then remove the temporary user and all dependent local QA audit/session rows in `finally`. Retain the existing backend and Default/Classic validation tests.

- [ ] **Step 4: Run the three focused workflows and prove cleanup**

Run the relevant Playwright specs with `E2E_MUTATION_ALLOWED=1` and `E2E_DB_FIXTURE_ALLOWED=1`. After each spec, assert `automated_local_qa_uses=0`, `qa_pending_payouts=0`, `payment_provider_references=0`, and no new entitlement or QA registration fixture remains.

### Task 7: Complete Verification Gates

**Files:**
- Modify only if a reproducible failure requires a test-first fix.
- Record transient logs/evidence under: `.verification-output/`

- [ ] **Step 1: Run backend correctness gates**

```powershell
go test ./... -count=1
go vet ./...
```

- [ ] **Step 2: Run Default full gates**

From `web/default` run:

```powershell
bun test ./src ./rsbuild.config.test.ts
bun run typecheck
bun run lint
bun run audit:source
bun run format:check
bun run copyright:check
bun run build
```

- [ ] **Step 3: Run Classic full gates**

From `web/classic`, run every package script for tests, type checking when present, lint, i18n/source audits, formatting/copyright checks, and production build. Treat warnings promoted by the repo scripts as failures.

- [ ] **Step 4: Run migration matrices**

Run the repository's MySQL, PostgreSQL, and SQLite migration tests against disposable migration databases only. Confirm no migration command points at `cloveapi-current-source-mysql-qa-data`.

- [ ] **Step 5: Run complete Playwright suite**

Run all Default Playwright specs, including anonymous bootstrap, production acceptance, invitation flows, business flows, profile/announcement flows, API-key flows, and playground reliability with the existing local QA storage states.

- [ ] **Step 6: Verify preserved database baseline and invariant rows**

Assert exactly:

```text
users=2878
wallet_ledger=171
wallet_delta_sum=7453337
top_ups=819
subscription_orders=501
user_redemption_codes=38
claims=15
entitlements=0
invite_reward_payouts=7
payment_provider_references=0
payout_request_19=0
automated_local_qa_uses=0
qa_pending_payouts=0
```

Also assert invitation code `id=6` has `used_count=3`, `status=exhausted`, `active_owner_slot=NULL`, and `expires_at=1787668020`; restore `theme.frontend=default`.

- [ ] **Step 7: Check runtime health and log quality**

Probe `/api/status`, public read-limited `/api/invitation/poster-logo`, dashboard navigation, wallet, model marketplace, registration, and both invitation poster themes. Inspect container logs since the rebuilt start for panics, SQL errors, repeated authentication noise, duplicate billing, and unhandled frontend exceptions.

### Task 8: Commit, Rebuild Clean Runtime, and Package Source

**Files:**
- Create outside Git tree: `D:/cloveapi/clove-api-v1.0.1-beta-source.zip`
- Create outside Git tree: `D:/cloveapi/clove-api-v1.0.1-beta-source.zip.sha256`
- Create outside Git tree: `D:/cloveapi/clove-api-v1.0.1-beta-delivery-manifest.json`

- [ ] **Step 1: Review the final diff and ensure all shared-workspace changes remain**

Confirm the six Chinese-username backend and Default/Classic registration files are present in history/current tree, the brand-logo changes are scoped, no credentials or `.verification-output` files are tracked, and `git diff --check` is clean.

- [ ] **Step 2: Commit only after every gate passes**

Stage the plan, shared renderer, both frontends, backend service/controller/router, and acceptance tests, then create a conventional commit such as:

```powershell
git commit -m "fix(invitation): render remote brand logos in posters"
```

- [ ] **Step 3: Rebuild the local image at the clean commit and keep port 18104 healthy**

Run `scripts/rebuild-local-mysql-qa.ps1` again with image tag `clove-api:local-20260730-poster-logo-<shortsha>`. Verify the running image revision label equals `git rev-parse HEAD`, both frontend source labels match `scripts/verify-runtime-source.ps1`, and the preserved volume name is unchanged.

- [ ] **Step 4: Produce deterministic source artifacts from committed HEAD**

Run:

```powershell
git archive --format=zip --output=D:/cloveapi/clove-api-v1.0.1-beta-source.zip HEAD
```

Write the lowercase SHA-256 digest plus filename to `.sha256`. Write a JSON manifest containing version `Clove API v1.0.1-beta`, commit SHA, branch, UTC creation timestamp, archive filename, archive SHA-256, running local image tag/digest, container name, volume name, and the complete verification command/result summary.

- [ ] **Step 5: Final consistency check**

Run `git status --short --branch`, verify the worktree is clean, recompute the archive hash and compare it with both `.sha256` and the manifest, verify `http://127.0.0.1:18104/api/status` returns 200, and verify production/remote state was never changed.
