# Clove API Initial Page Title Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make both frontend HTML entry points display `Clove API` before JavaScript or `/api/status` loads while preserving administrator-configured runtime titles.

**Architecture:** Treat the HTML `<title>` as a static branding fallback only. Existing Default and Classic runtime code remains responsible for replacing it with `system_name` from cache or `/api/status`.

**Tech Stack:** HTML, Bun test, Rsbuild, React, Go embed

---

### Task 1: Add Static Branding Regression Coverage

**Files:**
- Create: `web/initial-title.test.ts`

- [x] **Step 1: Write the failing test**

```ts
import { describe, expect, test } from 'bun:test'

const read = (path: string) => Bun.file(new URL(path, import.meta.url)).text()

describe('initial page title', () => {
  test('uses Clove API before the default frontend boots', async () => {
    const html = await read('./default/index.html')
    expect(html).toContain('<title>Clove API</title>')
    expect(html).toContain('<meta name="title" content="Clove API" />')
  })

  test('uses Clove API before the classic frontend boots', async () => {
    const html = await read('./classic/index.html')
    expect(html).toContain('<title>Clove API</title>')
  })
})
```

- [x] **Step 2: Run the test and verify RED**

Run: `cd web; bun test initial-title.test.ts`

Expected: both tests fail because the source HTML contains `<title>New API</title>`.

### Task 2: Change Only the Static Fallback

**Files:**
- Modify: `web/default/index.html`
- Modify: `web/classic/index.html`

- [x] **Step 1: Update the Default HTML title and metadata**

```html
<title>Clove API</title>
<meta name="title" content="Clove API" />
```

- [x] **Step 2: Update the Classic HTML title**

```html
<title>Clove API</title>
```

- [x] **Step 3: Run the regression test and verify GREEN**

Run: `cd web; bun test initial-title.test.ts`

Expected: 3 pass, 0 fail, including the runtime override assertion.

- [x] **Step 4: Confirm runtime title overrides remain present**

Run: `rg -n "document.title = (name|systemName)" web/default/src/main.tsx web/classic/src/components/layout/PageLayout.jsx`

Expected: both existing assignments remain unchanged.

### Task 3: Build and Runtime Verification

**Files:**
- Generated: `web/default/dist/**`
- Generated: `web/classic/dist/**`
- Generated: `D:/cloveapi/artifacts/verification/rc21-build/clove-api-v1.0.0-rc.21.clove-enhanced.exe`

- [x] **Step 1: Run focused and full frontend tests**

Run the root title test, Default suite, and Classic suite with `scripts/use-d-drive-cache.ps1` loaded.

Expected: all tests pass.

- [x] **Step 2: Build both frontends**

Run `bun run build` in `web/default` and `web/classic` with D-drive cache variables.

Expected: both production builds succeed and both generated `dist/index.html` files contain `<title>Clove API</title>`.

- [x] **Step 3: Build and replace the embedded Go executable**

Build with:

```text
-X github.com/QuantumNous/new-api/common.Version=v1.0.0-rc.21.clove-enhanced
```

Expected: `--version` returns `v1.0.0-rc.21.clove-enhanced`.

- [x] **Step 4: Restart against the existing SQLite database**

Back up the database, stop only the verified current executable, and restart on port `3231` with an explicit `SQLITE_PATH`.

Expected: `/api/status` is healthy and existing data remains available.

- [x] **Step 5: Verify initial and runtime titles**

Check the raw `/` response and a browser load with scripts blocked.

Expected: the initial title is `Clove API`; after normal JavaScript boot, an administrator-configured `system_name` can still replace it.

### Task 4: Commit and Refresh Delivery Artifacts

**Files:**
- Update: RC21 source ZIP, hashes, delivery manifest, audit supplement, and Desktop copy outside the Git tree.

- [x] **Step 1: Commit the tested source change**

```powershell
git add web/initial-title.test.ts web/default/index.html web/classic/index.html
git commit -m "fix(branding): show Clove API during initial load"
```

- [x] **Step 2: Push the verified branch**

Push `upgrade/rc21-integration` without changing unrelated remote history.

- [x] **Step 3: Regenerate delivery artifacts from the final commit**

Use `git archive`, compare all exported files against `git ls-tree`, scan for forbidden runtime/private files, run Gitleaks, and update SHA-256 records.

Expected: source tree difference 0, forbidden entries 0, and Desktop/source ZIP hashes match.
