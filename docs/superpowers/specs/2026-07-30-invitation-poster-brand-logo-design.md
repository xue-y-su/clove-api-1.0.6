# Invitation Poster Brand Logo and Verification Closure Design

## Status

- Date: 2026-07-30
- Scope: local source and local QA runtime only
- Production deployment, remote push, and GitHub Release: excluded
- Confirmed failure behavior: keep the initial-letter brand fallback and allow PNG download when the configured logo cannot be loaded

## Problem

The invitation poster receives the configured `Logo` URL from both frontends. The shared renderer loads that URL with `crossOrigin = "anonymous"` so the canvas remains exportable. The currently configured WebP image is served without `Access-Control-Allow-Origin`, so the browser rejects it and the renderer silently draws the initial letter `C` instead.

Previous verification proved poster dimensions, QR decoding, nickname toggling, PNG generation, download events, and visual layout. It did not prove that the configured brand image was present in the canvas. The broader test audit also found browser-level gaps for subscription redemption, locked model catalog presentation, and Chinese username registration. Invitation commission trigger rules have strong backend coverage, while the browser suite covers payout review rather than a real external payment provider.

## Decision

Use a fixed same-origin endpoint for cross-origin poster logos.

The endpoint will be `GET /api/invitation/poster-logo` as a public route covered by the API group's global read limit. A browser `Image` request cannot carry the custom `New-Api-User` header required by `UserAuth`, so placing it inside that group would always return 401. The configured Logo URL is already public through `/api/status`; this endpoint reads only the same server-owned `common.Logo` and callers cannot supply a target URL. This avoids creating a general-purpose proxy while allowing a canvas-safe response even when the configured image host does not support CORS.

Alternative approaches were rejected:

- Removing `crossOrigin` would taint the canvas and make `toBlob` fail.
- Requiring every image host to enable CORS would leave the current configuration broken and make deployments depend on third-party header policy.
- Importing every configured logo into managed storage would change the system-setting lifecycle and add storage migration and cleanup concerns beyond this bug.

## Backend Design

Add a focused branding-logo service and controller with these rules:

1. Read a snapshot of the current configured `Logo` under the existing option lock.
2. Accept only absolute HTTP(S) URLs for proxying. Relative and same-origin assets remain direct browser loads and do not use the endpoint.
3. Validate the initial URL and every redirect with the existing SSRF-protected HTTP client. DNS resolution is context-aware and runs inside the same bounded `singleflight` operation as cache lookup and fetching.
4. Apply a five-second total operation timeout and require a successful 2xx response.
5. Read through a hard byte limit and reject oversized bodies before returning them.
6. Validate actual image bytes and dimensions, supporting PNG, JPEG, GIF, and WebP. Do not trust the upstream `Content-Type` alone.
7. Return the validated image with its detected content type, `X-Content-Type-Options: nosniff`, a bounded public cache policy, and an ETag derived from the configured URL and image bytes.
8. Cache a successful result in memory by configured URL for five minutes, matching the browser cache lifetime. Revalidate the URL against the current SSRF policy inside the bounded operation before every cache hit, and expire the entry so an image replaced at the same URL becomes visible without restarting the process.
9. Coalesce concurrent validation, cache lookup, and fetch work for the same URL with `singleflight`. A failed fetch starts a 15-second retry backoff so an unavailable image host cannot create a request storm; failures themselves are not served as images.
10. Return a non-image response on any failure. The frontend uses the existing initial-letter fallback and never bypasses a size, format, timeout, or SSRF rejection by loading the original cross-origin URL directly.
11. Exclude the fixed image path from API gzip so already-compressed image bytes and their strong ETag retain one representation.

The service will not log every successful cache hit or poster render. Errors will be concise and will not expose response bodies or credentials.

## Frontend Design

Add a pure URL helper to `web/shared/invitation-poster.js` and its declaration file:

- Empty, `data:`, `blob:`, relative, and same-origin logo URLs remain unchanged.
- Cross-origin HTTP(S) logo URLs resolve to `/api/invitation/poster-logo`.
- Unsupported schemes resolve to an empty logo URL and therefore use fallback branding.

Default and Classic invitation dialogs call the helper before invoking the shared renderer. The renderer keeps `crossOrigin = "anonymous"`, waits six seconds so the five-second backend operation can finish, and retains the initial-letter fallback when loading fails or times out. Each dialog passes an `AbortSignal`, so closing it or changing poster settings clears the in-flight browser image request.

No visual layout, copy, poster dimensions, or filename behavior changes.

## Error Behavior

- Proxy success: draw the configured brand image and allow PNG download.
- Proxy timeout, unsupported or invalid image, oversized image, blocked URL, or upstream failure: draw the site-name initial and allow PNG download.
- QR or canvas failure: retain the existing render-error state and disable download.
- A logo failure must not be reported as a successful brand-image verification merely because the fallback poster is exportable.

## Test-First Plan

Production code will follow failing tests in this order:

1. Shared resolver tests for cross-origin mapping, backslash-normalization edge cases, and unchanged same-origin, relative, data, blob, empty, and unsupported-scheme inputs.
2. Shared renderer tests proving the loaded logo object is passed to `drawImage`, a proxy rejection goes directly to the initial-letter fallback, the browser deadline exceeds the backend deadline, and cancellation clears the request.
3. Backend service tests for a valid no-CORS WebP, byte and dimension limits, invalid bytes, unsupported content, non-2xx status, timeout, protected redirects, cache expiry, retry backoff, cancellable policy validation, SSRF-policy revalidation, and concurrent operation coalescing.
4. Controller and route tests proving the fixed public route is read-limited, excluded from gzip, and callers cannot select an arbitrary upstream URL.
5. Default and Classic wiring tests proving both dialogs use the shared resolver.

Each new test must be observed failing for the intended reason before implementation.

## Browser and Integration Verification

After targeted tests pass, rebuild the local image and verify against the preserved local database:

1. Open Default and Classic posters with the real configured no-CORS WebP.
2. Confirm the browser requests the same-origin poster-logo endpoint.
3. Compare the logo-region pixels against the decoded configured image and assert the region is not the initial-letter fallback.
4. Repeat `4:5`, `3:4`, and `9:16`; verify exact dimensions, nonblank pixels, QR decoding, origin-clean `toBlob`, and real download completion.
5. Force the logo endpoint to fail once and verify both themes still produce downloadable initial-letter posters.
6. Add browser coverage for purchasing and claiming a subscription redemption code and observing the resulting subscription, with complete balance and fixture cleanup.
7. Add browser coverage showing a user without a locked group can see that group, its model, and its price on the model marketplace.
8. Add browser coverage for registering and signing in with a Chinese username, with the temporary account removed afterward.
9. Preserve the existing invitation commission backend trigger, idempotency, reversal, and payout tests; browser verification will cover source evidence and the payout approval flow. External payment-provider execution is represented by deterministic signed/stubbed integration tests rather than real funds.
10. Run all Go tests and vet, both frontend suites and production builds, database migration tests, the complete Playwright suite, database baseline comparison, HTTP probes, and error-log checks.

## Data and Delivery Safety

- Do not reset or recreate the existing local database volume.
- All browser fixtures must have explicit cleanup and a final baseline assertion.
- Restore `theme.frontend=default` and any temporarily activated invitation code.
- Keep the local service at `http://127.0.0.1:18104` after verification.
- Commit the implementation only after the complete verification set passes.
- Rebuild the local source ZIP, checksum, and delivery manifest at the new clean HEAD.
