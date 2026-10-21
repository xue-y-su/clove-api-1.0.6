# 性能与发布验收 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 降低首屏资源和全局等待时间，消除门禁永久 Spinner 与生产调试浮层，统一新增界面和翻译，并完成三数据库、双前端、浏览器及 Docker 的生产交付验收。

**Architecture:** 路由在开发和生产环境统一拆包，模型品牌图标使用显式异步映射并在未识别时立即回退。合规与来源归属状态登录后并行查询，展示顺序仍由单一门禁状态机控制；失败进入可重试错误页而不是解析成 loading。最终验收只使用隔离数据库和可删除的临时容器，证据不包含凭据。

**Tech Stack:** Rsbuild/Rspack、TanStack Router/Query、React 19、React 18、Bun、Playwright Chromium、Go 1.22、SQLite/MySQL/PostgreSQL、Docker Compose、PowerShell。

---

## 文件职责

- `web/default/rsbuild.config.ts`：路由拆包和产物优化。
- `web/default/src/lib/lobe-icon.tsx`、`web/default/src/lib/lobe-icon-loaders.ts`：异步图标解析和缓存。
- `web/default/src/features/auth/lib/authenticated-gates.ts`：门禁状态机。
- `web/default/src/components/layout/components/authenticated-layout.tsx`：并行预取、顺序展示和错误重试。
- `web/default/src/features/compliance/compliance-gate.tsx`、`web/default/src/features/source-attribution/source-attribution-gate.tsx`：独立查询/显示状态。
- `web/default/src/routes/__root.tsx`、`web/default/src/components/devtools.tsx`：开发工具隔离。
- `web/default/src/i18n`、`web/classic/src/i18n`：新增功能的七语言契约。
- `web/default/src/components/ui/select.tsx` 和各 feature 页面：统一 Select、固定尺寸和防溢出。
- `scripts/verify-production-bundles.mjs`：生产产物体积和调试字符串断言。
- `docs/superpowers/specs/2026-07-26-production-release-evidence.md`：最终可复查证据。

### Task 1: 开发与生产统一路由拆包

**Files:**
- Modify: `web/default/rsbuild.config.ts`
- Modify: `web/default/rsbuild.config.test.ts`
- Create: `scripts/verify-production-bundles.mjs`

- [x] **Step 1: Write the failing config test**

Extract or inspect the TanStack Router plugin options and assert `autoCodeSplitting` is enabled in both modes. Add a bundle verifier that fails when initial JS exceeds limits or contains production devtool markers.

```ts
test.each(['development', 'production'])('splits routes in %s', async (mode) => {
  const config = await createConfig({ envMode: mode } as never)
  expect(readRouterSplitSetting(config)).toBe(true)
})
```

- [x] **Step 2: Run test to verify it fails**

Run from `web/default`: `bun test rsbuild.config.test.ts`

Expected: FAIL for development because `autoCodeSplitting` currently equals `isProd`.

- [x] **Step 3: Write the minimal implementation**

Set `autoCodeSplitting: true` and keep existing vendor cache groups. The verifier reads `dist/manifest.json` or emitted HTML entry scripts, recursively sums only initial JS, calculates gzip bytes with Node `zlib`, and checks:

```js
assert(initialRawBytes < 10 * 1024 * 1024)
assert(initialGzipBytes < 1024 * 1024)
assert(!bundleText.includes('TanStackRouterDevtools'))
assert(!bundleText.includes('ReactQueryDevtools'))
```

Do not count lazy route chunks as initial assets and do not delete source maps or license files to make the metric pass.

- [x] **Step 4: Run test to verify it passes**

Run from `web/default`: `bun test rsbuild.config.test.ts && bun run build && node ../../scripts/verify-production-bundles.mjs dist`

Expected: PASS; development routes are split and the production initial gzip total is below 1 MiB.

- [x] **Step 5: Commit**

```bash
git add web/default/rsbuild.config.ts web/default/rsbuild.config.test.ts scripts/verify-production-bundles.mjs
git commit -m "perf(web): enable route splitting in every mode"
```

### Task 2: 移除首屏整包 Lobe 图标导入

**Files:**
- Create: `web/default/src/lib/lobe-icon-loaders.ts`
- Modify: `web/default/src/lib/lobe-icon.tsx`
- Create: `web/default/src/lib/lobe-icon.test.tsx`
- Modify: `web/default/src/components/provider-badge.tsx`

- [x] **Step 1: Write the failing loader tests**

Assert source no longer contains `import * as LobeIcons`，已知图标只加载对应模块，重复渲染复用 Promise，未知图标立即显示首字母且不发起导入。

```ts
expect(lobeIconSource).not.toContain("import * as LobeIcons")
expect(resolveLobeIconLoader('OpenAI')).toBe(iconLoaders.OpenAI)
expect(resolveLobeIconLoader('UnknownVendor')).toBeUndefined()
```

- [x] **Step 2: Run tests to verify they fail**

Run from `web/default`: `bun test src/lib/lobe-icon.test.tsx`

Expected: FAIL because the current module imports the full package synchronously.

- [x] **Step 3: Write the minimal implementation**

Create an explicit loader map for provider keys already produced by `getChannelTypeIcon` and current vendor fixtures; each loader targets a package submodule:

```ts
export const iconLoaders = {
  OpenAI: () => import('@lobehub/icons/es/OpenAI'),
  Anthropic: () => import('@lobehub/icons/es/Anthropic'),
  Gemini: () => import('@lobehub/icons/es/Gemini'),
  Azure: () => import('@lobehub/icons/es/Azure'),
} satisfies Record<string, () => Promise<Record<string, unknown>>>
```

`getLobeIcon()` returns an `AsyncLobeIcon` component that parses `.Color/.Avatar` and props exactly as today, caches module promises by base key, and renders the existing fixed-size letter fallback during loading/error. Unknown database-provided icon keys stay usable through fallback and never block the page.

- [x] **Step 4: Run tests to verify they pass**

Run from `web/default`: `bun test src/lib/lobe-icon.test.tsx && bun run typecheck && bun run build && node ../../scripts/verify-production-bundles.mjs dist`

Expected: PASS and `@lobehub/icons` is absent from initial entry chunks.

- [x] **Step 5: Commit**

```bash
git add web/default/src/lib/lobe-icon-loaders.ts web/default/src/lib/lobe-icon.tsx web/default/src/lib/lobe-icon.test.tsx web/default/src/components/provider-badge.tsx
git commit -m "perf(web): lazy load provider icons"
```

### Task 3: 让登录门禁失败可恢复

**Files:**
- Modify: `web/default/src/features/auth/lib/authenticated-gates.ts`
- Modify: `web/default/src/features/auth/lib/authenticated-gates.test.ts`
- Modify: `web/default/src/features/compliance/compliance-gate.tsx`
- Modify: `web/default/src/features/source-attribution/source-attribution-gate.tsx`
- Modify: `web/default/src/components/layout/components/authenticated-layout.tsx`
- Create: `web/default/src/components/layout/components/gate-load-error.tsx`
- Create: `web/default/src/components/layout/components/authenticated-layout.test.tsx`
- Modify: `web/classic/src/components/layout/authenticated-gates.js`
- Modify: `web/classic/src/components/layout/authenticated-gates.test.js`
- Modify: `web/classic/src/components/layout/AuthenticatedGateStack.jsx`

- [x] **Step 1: Write the failing state-machine tests**

Extend resolution to `loading|required|satisfied|error`. Assert compliance/source queries start together after login, source dialog waits for compliance, announcement waits for both, and either query error renders a retry state within the request timeout.

```ts
expect(getAuthenticatedGateAccess({
  signedIn: true,
  compliance: 'error',
  sourceAttribution: 'satisfied',
})).toEqual({
  compliance: true,
  sourceAttribution: false,
  announcement: false,
  content: false,
  failed: 'compliance',
})
```

- [x] **Step 2: Run tests to verify they fail**

Run from `web/default`: `bun test src/features/auth/lib/authenticated-gates.test.ts src/components/layout/components/authenticated-layout.test.tsx`

Run from `web/classic`: `bun test src/components/layout/authenticated-gates.test.js`

Expected: FAIL because query errors currently resolve to `loading` forever.

- [x] **Step 3: Write the minimal implementation**

Add separate `queryEnabled` and `displayEnabled` props to compliance/source gates. Once signed in, both queries run with a 10-second Axios timeout; only the active gate may open a dialog. Report `error` through `onResolutionChange`. In `AuthenticatedLayout`, replace the fallback-only Spinner with:

```tsx
if (gateAccess.failed) {
  return (
    <GateLoadError
      gate={gateAccess.failed}
      onRetry={() => {
        void queryClient.invalidateQueries({ queryKey: gateQueryKey(gateAccess.failed) })
      }}
    />
  )
}
```

The error page names the failed check in user terms, shows request ID when available and provides one retry button. It must never bypass compliance/source requirements. Announcement query remains disabled until both checks are satisfied.

- [x] **Step 4: Run tests to verify they pass**

Run from `web/default`: `bun test src/features/auth/lib/authenticated-gates.test.ts src/components/layout/components/authenticated-layout.test.tsx src/features/compliance src/features/source-attribution && bun run typecheck`

Run from `web/classic`: `bun test src/components/layout`

Expected: PASS; network failure exits Spinner into a retryable state no later than 10 seconds.

- [x] **Step 5: Commit**

```bash
git add web/default/src/features/auth/lib/authenticated-gates.ts web/default/src/features/auth/lib/authenticated-gates.test.ts web/default/src/features/compliance/compliance-gate.tsx web/default/src/features/source-attribution/source-attribution-gate.tsx web/default/src/components/layout/components web/classic/src/components/layout
git commit -m "fix(auth): make authenticated gates retryable"
```

### Task 4: 从生产产物移除 TanStack Devtools

**Files:**
- Create: `web/default/src/components/devtools.tsx`
- Modify: `web/default/src/routes/__root.tsx`
- Create: `web/default/src/routes/root-devtools.test.ts`
- Modify: `scripts/verify-production-bundles.mjs`

- [x] **Step 1: Write the failing source and bundle tests**

Assert root route has no static devtool import, development mode can still render both tools, and production JS/HTML contains neither package name nor `TANSTACK` UI marker.

- [x] **Step 2: Run tests to verify they fail**

Run from `web/default`: `bun test src/routes/root-devtools.test.ts && bun run build && node ../../scripts/verify-production-bundles.mjs dist`

Expected: FAIL because `__root.tsx` statically imports both devtool packages.

- [x] **Step 3: Write the minimal implementation**

Move imports into a development-only lazy module:

```tsx
const DevelopmentTools = import.meta.env.DEV
  ? lazy(() => import('@/components/devtools'))
  : null

{DevelopmentTools ? (
  <Suspense fallback={null}><DevelopmentTools /></Suspense>
) : null}
```

`devtools.tsx` exports the two current components. Keep the build verifier as the production guard; do not hide the floating button with CSS while shipping its code.

- [x] **Step 4: Run tests to verify they pass**

Run from `web/default`: `bun test src/routes/root-devtools.test.ts && bun run build && node ../../scripts/verify-production-bundles.mjs dist`

Expected: PASS and no `TANSTACK` control appears in a production preview.

- [x] **Step 5: Commit**

```bash
git add web/default/src/components/devtools.tsx web/default/src/routes/__root.tsx web/default/src/routes/root-devtools.test.ts scripts/verify-production-bundles.mjs
git commit -m "fix(web): exclude devtools from production"
```

### Task 5: 统一新增界面、Select 与七语言

**Files:**
- Modify: `web/default/src/i18n/static-keys.ts`
- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/zh-TW.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/vi.json`
- Modify: `web/default/src/i18n/production-ui-locales.test.ts`
- Modify: `web/classic/src/i18n/locales/en.json`
- Modify: `web/classic/src/i18n/locales/zh.json`
- Modify: `web/classic/src/i18n/locales/zh-TW.json`
- Modify: `web/classic/src/i18n/locales/fr.json`
- Modify: `web/classic/src/i18n/locales/ja.json`
- Modify: `web/classic/src/i18n/locales/ru.json`
- Modify: `web/classic/src/i18n/locales/vi.json`
- Modify: `web/classic/src/i18n/production-ui-i18n.test.js`
- Modify: `web/default/src/features/system-settings/integrations/email-templates-section.tsx`
- Modify: `web/default/src/features/system-settings/maintenance/update-checker-section.tsx`
- Modify: `web/default/src/features/system-settings/operations/section-registry.tsx`

- [x] **Step 1: Write the failing locale/UI contract tests**

List every new string from the previous three plans in locale contract tests. Add source assertions that new enum/mode inputs import the native Default `Select` or Classic Semi `Select`, status badges use `shrink-0 whitespace-nowrap`, and the system-maintenance update-check button no longer exists.

- [x] **Step 2: Run tests to verify they fail**

Run from `web/default`: `bun run audit:source && bun test src/i18n`

Run from `web/classic`: `bun test src/i18n`

Expected: FAIL listing exact missing keys or non-standard controls.

- [x] **Step 3: Write the minimal implementation**

Add human translations for `en`, `zh`, `zh-TW`, `fr`, `ja`, `ru`, `vi`; do not copy English placeholders into non-English files. Cover IP reasons/views, invitation payout states, announcement errors, playground failures, Key states, redemption dispositions and gate retry text. Complete SMTP template labels/help text and remove the system-maintenance “检查更新” command while retaining version display.

Replace only new feature option sets with existing Select components. Keep binary options as checkbox/switch, commands as buttons, colors as swatches and icon commands as Lucide icons with tooltips.

- [x] **Step 4: Run tests to verify they pass**

Run from `web/default`: `bun run i18n:sync && bun run audit:source && bun test src/i18n && bun run typecheck`

Run from `web/classic`: `bun run i18n:sync && bun test src/i18n`

Expected: PASS with no untranslated keys in the changed surfaces and no truncated status labels.

- [x] **Step 5: Commit**

```bash
git add web/default/src/i18n web/classic/src/i18n web/default/src/features/system-settings
git commit -m "fix(i18n): complete production workflow translations"
```

### Task 6: 稳定用户错误与日志边界

**Files:**
- Modify: `common/gin.go`
- Create: `common/gin_api_response_test.go`
- Modify: `controller/invitation.go`
- Modify: `controller/user_redemption.go`
- Modify: `controller/token.go`
- Modify: `controller/announcement.go`
- Modify: `controller/playground.go`
- Modify: `web/default/src/lib/handle-server-error.ts`
- Create: `web/default/src/lib/handle-server-error.test.ts`
- Modify: `web/classic/src/helpers/utils.jsx`
- Create: `web/classic/src/helpers/server-error.test.js`

- [x] **Step 1: Write the failing response tests**

Assert changed endpoints return `success=false`, a stable `code`, localized `message` and `request_id`; public messages never contain channel names, SQL text, absolute paths, stack traces or upstream credentials. Assert the frontend displays the message and request ID once.

```go
require.Equal(t, false, body["success"])
require.Equal(t, "user_redemption_balance_insufficient", body["code"])
require.Equal(t, "req-visible", body["request_id"])
require.NotContains(t, body["message"], "Duplicate entry")
```

- [x] **Step 2: Run tests to verify they fail**

Run: `go test ./common ./controller -run 'Test.*(APIError|Invitation|UserRedemption|Token|Announcement|Playground).*Response' -count=1`

Run frontend error-helper tests in both themes.

Expected: FAIL because generic helpers currently omit `code/request_id` or the frontend discards the server message.

- [x] **Step 3: Write the minimal implementation**

Add a narrow helper without turning usage logs into a diagnostic store:

```go
func ApiErrorCode(c *gin.Context, code, message string) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"code": strings.TrimSpace(code),
		"message": strings.TrimSpace(message),
		"request_id": c.GetString(RequestIdKey),
	})
}
```

Use it in the changed controllers with feature-specific codes. Technical details go to existing `logger.LogError`/stdout only for final unexpected failures; expected validation/business rejection and successful retries do not emit error logs. A relay request with internal retries logs only the final failed attempt. `RecordErrorLog` and the existing usage-log schema remain unchanged. Frontends render `message` plus a deduplicated `request_id`, never channel metadata.

- [x] **Step 4: Run tests to verify they pass**

Run: `go test ./common ./controller ./service -run 'Test.*(APIError|Invitation|UserRedemption|Token|Announcement|Playground).*Response' -count=1`

Run from `web/default`: `bun test src/lib/handle-server-error.test.ts && bun run typecheck`.

Run from `web/classic`: `bun test src/helpers/server-error.test.js`.

Expected: PASS; normal errors are useful to users, internal diagnostics remain server-side, and usage logs retain their existing granularity.

- [x] **Step 5: Commit**

```bash
git add common/gin.go common/gin_api_response_test.go controller/invitation.go controller/user_redemption.go controller/token.go controller/announcement.go controller/playground.go web/default/src/lib/handle-server-error.ts web/default/src/lib/handle-server-error.test.ts web/classic/src/helpers/utils.jsx web/classic/src/helpers/server-error.test.js
git commit -m "fix(errors): standardize workflow error responses"
```

### Task 7: 浏览器检查布局与性能

**Files:**
- Create: `web/default/e2e/production-acceptance.spec.ts`
- Create: `docs/superpowers/specs/2026-07-26-ui-performance-evidence.md`

- [x] **Step 1: Write the browser acceptance scenarios**

Use Root, administrator, normal user and anonymous contexts. Cover homepage, profile, IP audit, user redemptions, invitation approval, announcements, playground chat/image, API keys and wallet. At `1440x900`, `1024x768`, `390x844`, assert no horizontal document overflow, overlapping text, truncated status, permanent Spinner or unexpected route 500.

- [x] **Step 2: Start the local validation stack**

First run `Get-PSDrive C,D` and use `scripts/use-d-drive-cache.ps1` for build caches. Rebuild only the application service:

```powershell
docker compose -f docker-compose.dev.yml up -d postgres redis
docker compose -f docker-compose.dev.yml up -d --build new-api
```

Expected: healthy API/PostgreSQL/Redis and no extra duplicate application containers.

- [x] **Step 3: Run Playwright and capture evidence**

Run from `web/default`: `bunx playwright test e2e/production-acceptance.spec.ts --project=chromium`.

Expected: PASS. Capture screenshots only for the listed key pages and failures; do not generate unbounded videos/traces. Record navigation timing, initial transferred JS and gate resolution time in the evidence document.

- [x] **Step 4: Verify performance budgets**

Run production build verifier and use browser performance entries to assert initial raw JS < 10 MiB, initial gzip JS < 1 MiB, no route-long-task caused by loading all icons, and authenticated gate success/error resolves within 10 seconds.

- [x] **Step 5: Commit**

```bash
git add web/default/e2e/production-acceptance.spec.ts docs/superpowers/specs/2026-07-26-ui-performance-evidence.md
git commit -m "test(web): verify responsive production workflows"
```

### Task 8: 执行完整生产发布门禁

**Files:**
- Create: `docs/superpowers/specs/2026-07-26-production-release-evidence.md`

- [x] **Step 1: Run full backend and database tests**

SQLite: `go test ./... -count=1`.

MySQL/PostgreSQL:

```powershell
if (-not $env:TEST_MYSQL_DSN) { throw 'TEST_MYSQL_DSN must point to an isolated test database' }
if (-not $env:TEST_POSTGRES_DSN) { throw 'TEST_POSTGRES_DSN must point to an isolated test database' }
go test ./model ./service ./controller ./router -run 'Test.*(MainDatabase|IPSecurity|Token|UserRedemption|Invitation|Announcement)' -count=1
```

Expected: PASS on all three; skipped integration tests are acceptable only when their DSN is intentionally absent and must be recorded.

- [x] **Step 2: Run both frontend release suites**

Default:

```powershell
Set-Location web/default
bun run format:check
bun run lint
bun run typecheck
bun run audit:source
bun test
bun run build
node ../../../scripts/verify-production-bundles.mjs dist
```

Classic:

```powershell
Set-Location web/classic
bun run lint
bun run eslint
bun test
bun run build
```

Expected: every command exits `0`; do not suppress unrelated failures in the evidence.

- [x] **Step 3: Rebuild Docker from current Git source**

Build with `docker compose -f docker-compose.dev.yml build new-api`, start against the copied acceptance database, verify migrations, `/data/announcement-media`, Redis/session recovery and health endpoints, then restart the application container once and repeat login/announcement/profile checks.

Expected: no source bind to an old container image, no permission error and no startup migration loop.

- [x] **Step 4: Audit final changes and disk usage**

Run `git diff --check`, `git status --short`, `git diff --stat`, and secret-pattern scan over changed files. Run `docker system df` and remove only stopped ephemeral test containers/build cache created by this plan; retain the active validation database and named volumes. Record free space on C: and D: before/after without deleting user files.

- [x] **Step 5: Record evidence and commit**

Fill the release evidence with commit SHA, database versions, exact commands, pass counts, browser roles/viewports, Docker image ID, known external-upstream limitations and zero unresolved blocking failures.

```bash
git add docs/superpowers/specs/2026-07-26-production-release-evidence.md
git commit -m "docs(release): record production acceptance evidence"
```
