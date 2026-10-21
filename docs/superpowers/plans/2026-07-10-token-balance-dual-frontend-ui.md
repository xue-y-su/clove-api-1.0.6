# Token Balance Dual Frontend UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the default frontend's oversized token-balance result with the approved native data panel and add the approved Semi Design balance page to the Classic frontend.

**Architecture:** The existing `POST /api/token-balance` response remains the only billing truth. Each frontend owns a small presentation layer for formatting and progress segments, but neither frontend recalculates subscription eligibility or funding participation. Both API clients preserve the existing privacy contract and add only `Retry-After` response metadata for rate-limit guidance.

**Tech Stack:** React 19, TypeScript, TanStack Router, Tailwind/Shadcn, JavaScript, React Router, Semi UI, i18next, Bun tests, Rsbuild.

---

## File Map

- Modify `web/default/src/features/token-balance/api.ts` and `types.ts`: expose rate-limit retry metadata without changing backend payload fields.
- Modify `web/default/src/features/token-balance/api.test.ts`: verify `Retry-After`, invalid JSON fallback, and credential omission.
- Modify `web/default/src/features/token-balance/lib/balance.ts` and `balance.test.ts`: keep deterministic finite/unlimited display segments.
- Create `web/default/src/features/token-balance/components/balance-metrics.tsx`: render finite and unlimited three-column metric sets.
- Create `web/default/src/features/token-balance/components/balance-result.tsx`: compose identity, metrics, progress, and details.
- Create `web/default/src/features/token-balance/components/balance-query-error.tsx`: render the four approved query-error states.
- Modify `web/default/src/features/token-balance/components/balance-progress.tsx`, `balance-restrictions.tsx`, and `index.tsx`: implement A2/A3/A4 with native theme tokens.
- Modify `web/default/src/lib/nav-modules.ts` and `hooks/use-top-nav-links.ts`: make the balance link configurable with a backward-compatible default of enabled.
- Create `web/classic/src/pages/TokenBalance/api.js`: privacy-preserving Classic API client.
- Create `web/classic/src/pages/TokenBalance/balance.js` and `balance.test.js`: Classic formatting and progress presentation helpers.
- Create `web/classic/src/pages/TokenBalance/components/BalanceMetrics.jsx`, `BalanceProgress.jsx`, `BalanceDetails.jsx`, and `BalanceQueryError.jsx`: focused Semi UI result units.
- Create `web/classic/src/pages/TokenBalance/index.jsx`: public Classic page using the approved standard Semi query panel.
- Modify `web/classic/src/App.jsx`, `hooks/common/useNavigation.js`, and `pages/Setting/Operation/SettingsHeaderNavModules.jsx`: register and configure the public route/link.
- Modify all seven locale JSON files under both `web/default/src/i18n/locales/` and `web/classic/src/i18n/locales/`: add exact user-facing strings.

### Task 1: API Error Metadata And Presentation Contracts

**Files:**
- Modify: `web/default/src/features/token-balance/types.ts`
- Modify: `web/default/src/features/token-balance/api.ts`
- Modify: `web/default/src/features/token-balance/api.test.ts`
- Create: `web/classic/src/pages/TokenBalance/api.js`
- Create: `web/classic/src/pages/TokenBalance/api.test.js`

- [ ] **Step 1: Add failing default-client tests for `Retry-After` and malformed error bodies**

```ts
test('returns retry metadata from a 429 response', async () => {
  globalThis.fetch = mock(async () =>
    new Response(JSON.stringify({ success: false, code: 'rate_limited' }), {
      status: 429,
      headers: { 'Content-Type': 'application/json', 'Retry-After': '60' },
    })
  ) as typeof fetch

  expect(await queryTokenBalance('sk-test')).toMatchObject({
    code: 'rate_limited',
    retry_after_seconds: 60,
  })
})
```

- [ ] **Step 2: Run the focused test and verify RED**

Run: `cd web/default && bun test src/features/token-balance/api.test.ts`

Expected: FAIL because `retry_after_seconds` is absent.

- [ ] **Step 3: Add the response metadata type and robust client parsing**

```ts
export type TokenBalanceResponse = {
  success: boolean
  code?: string
  message?: string
  retry_after_seconds?: number
  data?: TokenBalanceData
}

const retryAfter = Number(response.headers.get('Retry-After'))
return {
  ...payload,
  code: response.status === 429 ? payload.code || 'rate_limited' : payload.code,
  retry_after_seconds:
    response.status === 429 && Number.isFinite(retryAfter) && retryAfter > 0
      ? retryAfter
      : undefined,
}
```

When `response.json()` fails, return `{ success: false, code: 'query_failed' }`; never include the submitted Key.

- [ ] **Step 4: Add the Classic fetch client and tests**

```js
export async function queryTokenBalance(key) {
  const response = await fetch('/api/token-balance', {
    method: 'POST',
    credentials: 'omit',
    cache: 'no-store',
    headers: { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' },
    body: JSON.stringify({ key }),
  });
  let payload;
  try {
    payload = await response.json();
  } catch {
    return { success: false, code: 'query_failed' };
  }
  const retryAfter = Number(response.headers.get('Retry-After'));
  return {
    ...payload,
    code:
      response.status === 429
        ? payload.code || 'rate_limited'
        : payload.code,
    retry_after_seconds:
      response.status === 429 && Number.isFinite(retryAfter) && retryAfter > 0
        ? retryAfter
        : undefined,
  };
}
```

Run: `cd web/classic && bun test src/pages/TokenBalance/api.test.js`

Expected: PASS after the implementation.

- [ ] **Step 5: Commit the API contract change**

```bash
git add web/default/src/features/token-balance web/classic/src/pages/TokenBalance/api.js web/classic/src/pages/TokenBalance/api.test.js
git commit -m "fix(balance): expose safe retry metadata"
```

### Task 2: Default Frontend Native Result Panel

**Files:**
- Modify: `web/default/src/features/token-balance/index.tsx`
- Create: `web/default/src/features/token-balance/components/balance-metrics.tsx`
- Create: `web/default/src/features/token-balance/components/balance-result.tsx`
- Create: `web/default/src/features/token-balance/components/balance-query-error.tsx`
- Modify: `web/default/src/features/token-balance/components/balance-progress.tsx`
- Modify: `web/default/src/features/token-balance/components/balance-restrictions.tsx`
- Modify: `web/default/src/features/token-balance/lib/balance.ts`
- Modify: `web/default/src/features/token-balance/lib/balance.test.ts`

- [ ] **Step 1: Extend failing helper tests for finite and unlimited metric modes**

```ts
expect(getBalanceProgressSegments(finiteBalance)).toEqual({
  available: 40,
  used: 100 / 3,
  restricted: 80 / 3,
})
expect(getBalanceProgressSegments(unlimitedBalance)).toEqual({
  available: 60,
  used: 40,
  restricted: 0,
})
```

Also assert that unlimited funding returns `{ available: 100, used: 0, restricted: 0 }`.

- [ ] **Step 2: Run the default balance tests**

Run: `cd web/default && bun test src/features/token-balance/lib/balance.test.ts`

Expected: existing arithmetic tests pass; any added presentation helper test fails before implementation.

- [ ] **Step 3: Implement the approved three-column metrics**

`BalanceMetrics` must render:

```tsx
const thirdMetric = balance.token_unlimited
  ? { label: t('Key Limit'), value: t('Unlimited'), tone: 'neutral' }
  : {
      label: t('Unavailable Quota'),
      value: formatValue(balance.restricted),
      tone: balance.restricted > 0 ? 'destructive' : 'neutral',
    }
```

The outer container uses the wallet stats pattern: `grid grid-cols-1 divide-y sm:grid-cols-3 sm:divide-x sm:divide-y-0`. Values use the existing monospace/tabular number stack.

- [ ] **Step 4: Implement the native result composition**

`BalanceResult` composes one `Card` in this order: masked identity and status, `BalanceMetrics`, segmented `BalanceProgress`, then two-column `BalanceRestrictions`. Remove the centered blue `text-5xl` amount and duplicate supporting metrics from `index.tsx`.

Use `bg-foreground` for current available, `bg-muted-foreground/45` for used, and `bg-destructive` for unavailable. Non-active Keys keep their masked result and show `$0.00` with the exact disabled/expired/exhausted/user-disabled status.

- [ ] **Step 5: Implement the four query-error presentations**

```ts
type BalanceQueryErrorCode =
  | 'invalid_request'
  | 'invalid_api_key'
  | 'rate_limited'
  | 'query_failed'
```

`BalanceQueryError` uses the existing `Alert`; rate limits include `t('Try again in {{seconds}} seconds', { seconds })`. Invalid Keys use one generic message. Network and malformed responses map to `query_failed`.

- [ ] **Step 6: Rebuild the page state flow**

Keep the raw Key only in component state. Clear it on success and on server-side failure after the request has been sent; retain focus for empty local validation. Never store the Key in URL, Web Storage, logs, toasts, or error objects.

- [ ] **Step 7: Run focused default frontend checks**

Run:

```bash
cd web/default
bun test src/features/token-balance
pnpm typecheck
pnpm lint
```

Expected: PASS.

- [ ] **Step 8: Commit the default frontend redesign**

```bash
git add web/default/src/features/token-balance
git commit -m "feat(balance): redesign default balance results"
```

### Task 3: Classic Presentation Helpers And Semi Components

**Files:**
- Create: `web/classic/src/pages/TokenBalance/balance.js`
- Create: `web/classic/src/pages/TokenBalance/balance.test.js`
- Create: `web/classic/src/pages/TokenBalance/components/BalanceMetrics.jsx`
- Create: `web/classic/src/pages/TokenBalance/components/BalanceProgress.jsx`
- Create: `web/classic/src/pages/TokenBalance/components/BalanceDetails.jsx`
- Create: `web/classic/src/pages/TokenBalance/components/BalanceQueryError.jsx`
- Create: `web/classic/src/pages/TokenBalance/components/BalanceResult.jsx`

- [ ] **Step 1: Write failing Classic presentation tests**

```js
test('finite segments preserve the key total', () => {
  expect(getBalanceProgressSegments(finiteBalance)).toEqual({
    available: 40,
    used: 100 / 3,
    restricted: 80 / 3,
  });
});

test('formats unlimited without a numeric sentinel', () => {
  expect(formatBalanceQuota({ unlimited: true, value: 0 }, display, '无限制'))
    .toBe('无限制');
});
```

- [ ] **Step 2: Run the test and verify RED**

Run: `cd web/classic && bun test src/pages/TokenBalance/balance.test.js`

Expected: FAIL because `balance.js` does not exist.

- [ ] **Step 3: Implement deterministic presentation helpers**

Port only `safeQuota`, `toPercent`, `getBalanceProgressSegments`, and `formatBalanceQuota` from the default presentation helper. Do not inspect plans, subscriptions, groups, or billing preferences in these functions.

- [ ] **Step 4: Implement focused Semi components**

- `BalanceMetrics`: Semi-style three-column grid; primary theme color for available, grey for used, danger for unavailable.
- `BalanceProgress`: accessible segmented bar plus text/amount legend.
- `BalanceDetails`: two responsive columns for funding composition and Key restrictions; use `Tooltip` for model names.
- `BalanceQueryError`: Semi `Banner` with danger, warning, or neutral treatment based on the four codes.
- `BalanceResult`: Semi `Card` with masked identity `Tag`s and the above components.

Every component accepts response data through props and has no fetch or storage access.

- [ ] **Step 5: Run Classic helper tests**

Run: `cd web/classic && bun test src/pages/TokenBalance`

Expected: PASS.

- [ ] **Step 6: Commit Classic presentation units**

```bash
git add web/classic/src/pages/TokenBalance
git commit -m "feat(classic): add balance presentation components"
```

### Task 4: Classic Public Page, Route, Navigation, And Configuration

**Files:**
- Create: `web/classic/src/pages/TokenBalance/index.jsx`
- Modify: `web/classic/src/App.jsx`
- Modify: `web/classic/src/hooks/common/useNavigation.js`
- Modify: `web/classic/src/pages/Setting/Operation/SettingsHeaderNavModules.jsx`
- Modify: `web/default/src/lib/nav-modules.ts`
- Modify: `web/default/src/hooks/use-top-nav-links.ts`

- [ ] **Step 1: Add failing navigation parser coverage**

Create `web/default/src/lib/nav-modules.test.ts`, asserting missing `balance` defaults to `true` and explicit `false` hides it.

```ts
expect(parseHeaderNavModules('{}').balance).toBe(true)
expect(parseHeaderNavModules('{"balance":false}').balance).toBe(false)
```

- [ ] **Step 2: Implement backward-compatible balance navigation configuration**

Add `balance: true` to both default module maps. In both navigation hooks, insert the public link only when `modules.balance !== false`. Add a “余额查询” switch to Classic `SettingsHeaderNavModules`; old saved JSON without the key remains enabled.

- [ ] **Step 3: Implement the Classic public page**

`TokenBalance` uses `Card`, `Input`, `Button`, `Tag`, `Banner`, `Spin`, and Semi icons. It renders under the existing `PageLayout`, so the global Classic maintenance banner remains visible. The content container is `max-w-4xl mx-auto px-3 py-5 sm:px-5` and does not create a second page shell.

The submit flow matches the default frontend privacy behavior and renders `BalanceResult` or `BalanceQueryError` inline.

- [ ] **Step 4: Register the public route**

```jsx
<Route
  path='/token-balance'
  element={
    <Suspense fallback={<Loading />} key={location.pathname}>
      <TokenBalance />
    </Suspense>
  }
/>
```

The route is outside `PrivateRoute` and `AuthRedirect`.

- [ ] **Step 5: Run navigation and Classic lint checks**

Run:

```bash
cd web/default && bun test src/lib/nav-modules.test.ts
cd ../classic && pnpm eslint
```

Expected: PASS.

- [ ] **Step 6: Commit route and navigation integration**

```bash
git add web/default/src/lib/nav-modules.ts web/default/src/hooks/use-top-nav-links.ts web/classic/src/App.jsx web/classic/src/hooks/common/useNavigation.js web/classic/src/pages/Setting/Operation/SettingsHeaderNavModules.jsx web/classic/src/pages/TokenBalance/index.jsx
git commit -m "feat(balance): expose public query in both frontends"
```

### Task 5: Seven-Language Copy In Both Frontends

**Files:**
- Modify: `web/default/src/i18n/locales/{en,fr,ja,ru,vi,zh,zh-TW}.json`
- Modify: `web/classic/src/i18n/locales/{en,fr,ja,ru,vi,zh-CN,zh-TW}.json`

- [ ] **Step 1: Define the exact shared key set**

Include metric, detail, state, and error strings used by both frontends: `Key Limit`, `Unavailable Quota`, `Key Has No Independent Limit`, all four Key status explanations, `Try again in {{seconds}} seconds`, generic invalid-Key guidance, and service-unavailable guidance.

- [ ] **Step 2: Add translations in all seven locale files per frontend**

Use human translations, preserve interpolation names exactly, and keep JSON key ordering compatible with each frontend's formatter. Do not leave English values in non-English files except product/API terms such as `API Key` and `default`.

- [ ] **Step 3: Run i18n and formatting checks**

Run:

```bash
cd web/default
pnpm i18n:sync
pnpm format:check
cd ../classic
pnpm i18n:status
pnpm lint
```

Expected: no missing or extra token-balance keys and no formatting differences.

- [ ] **Step 4: Commit translations**

```bash
git add web/default/src/i18n web/classic/src/i18n
git commit -m "feat(i18n): translate balance states for both frontends"
```

### Task 6: Builds, Security Regression, And Browser Acceptance

**Files:**
- Test the files listed in Tasks 1-5; if a check fails, modify only the file that owns the failed behavior.
- Write local evidence under ignored `.verification-output/`.

- [ ] **Step 1: Run both frontend test and build suites**

```bash
cd web/default
bun test src/features/token-balance src/lib/nav-modules.test.ts
pnpm typecheck
pnpm lint
pnpm build
cd ../classic
bun test src/pages/TokenBalance
pnpm eslint
pnpm lint
pnpm build
```

Expected: PASS and both `dist` directories generated.

- [ ] **Step 2: Run backend regression tests without changing billing behavior**

Run: `go test ./service ./controller ./middleware ./router -count=1`

Expected: PASS.

- [ ] **Step 3: Scan for credential persistence or accidental private fields**

```bash
rg -n "localStorage|sessionStorage|console\.|user_id|username|email|subscription_id" \
  web/default/src/features/token-balance web/classic/src/pages/TokenBalance
```

Expected: no Key persistence/logging and no private response fields.

- [ ] **Step 4: Start production-like builds on unused ports**

Keep the existing MySQL-backed service untouched at `http://127.0.0.1:3003`. Start both frontend development servers with explicit proxy targets:

```powershell
$env:VITE_REACT_APP_SERVER_URL='http://127.0.0.1:3003'
pnpm --dir web/default dev -- --port 3005
pnpm --dir web/classic dev -- --port 3006
```

Expected: default frontend at `http://127.0.0.1:3005/token-balance` and Classic at `http://127.0.0.1:3006/token-balance`, both proxying `/api/token-balance` to port 3003.

- [ ] **Step 5: Verify all approved states in both frontends**

At desktop, 390px, and 320px widths, verify:

- finite `$150 = $60 available + $50 used + $40 unavailable`;
- unlimited Key with no red segment;
- subscription-only hides wallet data;
- wallet-only hides subscription data;
- invalid, rate-limited, and service-error presentations;
- disabled, expired, exhausted, and user-disabled recognized results;
- dark mode, seven languages, no overlap, no horizontal scroll, and no console errors;
- refresh clears the Key and the Key never appears in URL or Web Storage.

- [ ] **Step 6: Final repository checks**

Run:

```bash
git diff --check
git status --short
```

Expected: only intended source/doc changes plus explicitly retained local visual artifacts.
