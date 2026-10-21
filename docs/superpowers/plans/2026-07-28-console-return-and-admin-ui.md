# Console Return and Admin UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix cached-gate dashboard returns and align the redemption audit and wallet preset layouts with the existing New API frontend.

**Architecture:** Keep all current API and domain behavior. Correct the authenticated layout's mount lifecycle, replace page-local native controls with shared UI primitives, and make wallet preset columns respond to the named content container.

**Tech Stack:** React 19, TypeScript, TanStack Router and Query, Base UI, Tailwind CSS v4 container queries, Bun tests, Rsbuild, Docker

---

### Task 1: Cached Gate Return Regression

**Files:**
- Modify: `web/default/src/components/layout/components/authenticated-layout.test.tsx`
- Modify: `web/default/src/components/layout/components/authenticated-layout.tsx`

- [ ] **Step 1: Write the failing lifecycle assertions**

Add source assertions proving the layout tracks the mounted user and skips the
initial reset:

```ts
expect(layoutSource).toContain(
  "import { useCallback, useEffect, useRef, useState } from 'react'"
)
expect(layoutSource).toContain('const resolvedUserId = useRef(userId)')
expect(layoutSource).toContain(
  'if (resolvedUserId.current === userId) return'
)
expect(layoutSource).toContain('resolvedUserId.current = userId')
```

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```powershell
bun test src/components/layout/components/authenticated-layout.test.tsx
```

Expected: FAIL because the layout still resets both gate resolutions on mount.

- [ ] **Step 3: Skip the mount reset and retain real user changes**

Import `useRef` and replace the unconditional effect body with:

```ts
const resolvedUserId = useRef(userId)

useEffect(() => {
  if (resolvedUserId.current === userId) return
  resolvedUserId.current = userId
  setComplianceResolution('loading')
  setSourceAttributionResolution('loading')
  setComplianceError(null)
  setSourceAttributionError(null)
}, [userId])
```

- [ ] **Step 4: Run the focused test and verify GREEN**

Run the Step 2 command. Expected: PASS.

### Task 2: Standard Redemption Audit Controls

**Files:**
- Modify: `web/default/src/features/user-redemptions/user-redemptions.state.test.ts`
- Modify: `web/default/src/features/user-redemptions/index.tsx`
- Modify: `web/default/src/features/user-redemptions/components/user-redemptions-table.tsx`

- [ ] **Step 1: Write failing page-contract assertions**

Extend the existing source test with:

```ts
expect(pageSource).toContain("import { SectionPageLayout } from '@/components/layout'")
expect(pageSource).toContain("from '@/components/ui/select'")
expect(pageSource).not.toContain('<select')
expect(tableSource).toContain("import { Checkbox } from '@/components/ui/checkbox'")
expect(tableSource).not.toContain("type='checkbox'")
expect(tableSource.match(/rounded-full/g)?.length).toBe(3)
```

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```powershell
bun test src/features/user-redemptions/user-redemptions.state.test.ts
```

Expected: FAIL on the layout, shared select, and shared checkbox assertions.

- [ ] **Step 3: Move the page into `SectionPageLayout`**

Use the standard title/actions/content slots:

```tsx
<SectionPageLayout>
  <SectionPageLayout.Title>{t('User Redemption Codes')}</SectionPageLayout.Title>
  <SectionPageLayout.Actions>{/* total and batch settlement */}</SectionPageLayout.Actions>
  <SectionPageLayout.Content>
    <div className='space-y-4'>{/* description, filters, table, pagination */}</div>
  </SectionPageLayout.Content>
</SectionPageLayout>
```

Keep all dialog state and callbacks outside or after the layout in the same
component fragment.

- [ ] **Step 4: Replace all three native selects**

Represent the empty kind and status choices with the sentinel value `all` and
map it back to the existing empty-string filter state:

```tsx
<Select
  value={draftFilters.kind || 'all'}
  onValueChange={(value) =>
    setDraftFilters((current) => ({
      ...current,
      kind: value === 'all' ? '' : (value as AdminFilters['kind']),
    }))
  }
>
  <SelectTrigger className='w-full' aria-label={t('Kind')}>
    <SelectValue />
  </SelectTrigger>
  <SelectContent alignItemWithTrigger={false}>
    <SelectItem value='all'>{t('All modes')}</SelectItem>
    <SelectItem value='single'>{t('Single')}</SelectItem>
    <SelectItem value='random'>{t('Red packet')}</SelectItem>
  </SelectContent>
</Select>
```

Apply the same mapping to status; keep deletion values `all`, `false`, and
`true` directly.

- [ ] **Step 5: Replace mobile, header, and row checkbox inputs**

Use the shared primitive in all three locations:

```tsx
<Checkbox
  className='rounded-full'
  checked={selectedIds.includes(item.id)}
  disabled={!canSettleUserRedemption(item)}
  aria-label={t('Select redemption code {{code}}', { code: item.code })}
  onCheckedChange={(checked) =>
    toggleItemSelection(item, Boolean(checked))
  }
/>
```

The header checkbox calls `toggleAllSelection(Boolean(checked))`; the mobile
checkbox additionally retains `mt-1`.

- [ ] **Step 6: Run the focused test and verify GREEN**

Run the Step 2 command. Expected: PASS.

### Task 3: Wallet Preset Readability

**Files:**
- Modify: `web/default/src/features/wallet/wallet-composition.test.ts`
- Modify: `web/default/src/features/wallet/components/recharge-form-card.tsx`

- [ ] **Step 1: Write failing responsive-layout assertions**

Read `recharge-form-card.tsx` in the composition test and assert:

```ts
expect(rechargeFormSource).toContain('@lg/content:grid-cols-3')
expect(rechargeFormSource).toContain('@3xl/content:grid-cols-4')
expect(rechargeFormSource).toContain('h-auto min-h-20')
expect(rechargeFormSource).not.toContain("{' • '}")
```

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```powershell
bun test src/features/wallet/wallet-composition.test.ts
```

Expected: FAIL because presets still use viewport breakpoints and inline bullet
wrapping.

- [ ] **Step 3: Use content-container columns and stable metadata lines**

Apply the same grid classes to the loading skeleton and real presets:

```tsx
className='grid grid-cols-2 gap-1.5 @lg/content:grid-cols-3 @3xl/content:grid-cols-4'
```

Use `h-auto min-h-20 justify-start` on amount buttons. Keep the amount and
discount in a gap-aware top row, then render payment and savings as separate
block lines without a bullet separator.

- [ ] **Step 4: Run the focused test and verify GREEN**

Run the Step 2 command. Expected: PASS.

### Task 4: Frontend Validation and Local Replacement

**Files:**
- Verify: `web/default/src/**`
- Use: `scripts/rebuild-local-mysql-qa.ps1`
- Use: `scripts/verify-runtime-source.ps1`

- [ ] **Step 1: Run focused tests together**

```powershell
bun test src/components/layout/components/authenticated-layout.test.tsx src/features/user-redemptions/user-redemptions.state.test.ts src/features/wallet/wallet-composition.test.ts
```

Expected: PASS.

- [ ] **Step 2: Run the full default frontend checks**

```powershell
bun test ./src ./rsbuild.config.test.ts
bun run typecheck
bun run lint
bun run format:check
bun run build
```

Expected: every command exits 0.

- [ ] **Step 3: Commit the implementation**

```powershell
git add web/default/src/components/layout/components/authenticated-layout.tsx web/default/src/components/layout/components/authenticated-layout.test.tsx web/default/src/features/user-redemptions web/default/src/features/wallet
git commit -m "fix(web): align admin controls and restore dashboard returns"
```

- [ ] **Step 4: Build and replace only the app container**

```powershell
$short = git rev-parse --short=8 HEAD
pwsh -File scripts/rebuild-local-mysql-qa.ps1 -ContainerName cloveapi-current-source-mysql-qa -ImageTag "clove-api:production-20260728-$short" -HostPort 18104
pwsh -File scripts/verify-runtime-source.ps1 -ContainerName cloveapi-current-source-mysql-qa -ExpectedCommit (git rev-parse HEAD)
```

Expected: HTTP 200 at `http://127.0.0.1:18104`, the existing `/data` volume,
MySQL, Redis, and isolated network remain attached, and image labels match HEAD.

- [ ] **Step 5: Browser acceptance**

Verify the authenticated SPA flow `dashboard -> home -> dashboard` without a
hard reload. At 795x912 verify shared selects and round checkboxes on
`/user-redemptions`. At 834x912 verify wallet presets use three columns and no
metadata touches the button edge. Also check 390x844 and 1365x900 for horizontal
overflow, console errors, and persistent loading indicators.
