# Vendored Snow Home

This directory vendors the built Snow root-page assets used when `HomePageMode=snow`.

## Contents

- `dist/` — production build output served from `/snow-assets/`
- Entry: `dist/index.html`
- Host integration: `router/web-router.go` mounts `/snow-assets` and theme home pages embed/link it

## Update instructions

1. Obtain a fresh production build of Snow.
2. Replace `web/snow/dist` atomically.
3. Run `node scripts/patch-snow-branding.mjs`.
4. Run `node scripts/verify-snow-assets.mjs`.
5. Confirm host routes still serve theme consoles for non-root pages.
6. Commit only tracked vendored assets and verification docs.

## Verification

```powershell
node scripts/verify-snow-assets.mjs
```

The script enumerates files/bytes, checks index asset references, and fails on missing assets.
