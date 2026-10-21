# 游乐场可靠性 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复游乐场跨页面取消、联网搜索崩页、分组模型错配和虚假图片参数，使 Default 与 Classic 的聊天和生图流程可独立取消、可重试并真实发送所选参数。

**Architecture:** 带 `AbortSignal` 的 GET 由调用组件独占，不进入全局 Promise 去重；无 signal 的幂等 GET 保留去重。联网搜索后端返回稳定结果/失败元数据，前端只消费归一化来源数组。图片模型由已有 `/api/user/models?group=` 单一事实源提供，控件是否可见与请求构造器发送字段使用同一能力表。

**Tech Stack:** Go 1.22、Gin、GORM、OpenAI-compatible relay、React 19/TypeScript/Bun、React 18/JavaScript/Bun、Axios、AbortController、Base UI、Semi UI。

---

## 文件职责

- `web/default/src/lib/api.ts`：Axios GET 去重与取消隔离。
- `service/playground_search.go`：Tavily/原生搜索判定、来源归一化和安全错误。
- `controller/playground.go`、`controller/playground_metadata_writer.go`：游乐场选项与响应元数据。
- `web/default/src/features/playground/hooks/use-chat-handler.ts`：搜索来源更新和局部错误状态。
- `web/default/src/features/playground/lib/streaming`：SSE 元数据与错误归一化。
- `web/default/src/features/playground/components/image/playground-image-panel.tsx`：分组、模型搜索、参数与重试状态。
- `web/default/src/features/playground/lib/image/image-generation.ts`：真实图片请求字段和响应解析。
- `web/classic/src/components/playground/PlaygroundImagePanel.jsx`、`web/classic/src/hooks/playground/useApiRequest.jsx`：Classic 契约对齐。
- `relay/channel/openai`、`relay/helper`：图片参数和思考输出的协议边界。

### Task 1: 隔离带 signal 的 GET 请求

**Files:**
- Modify: `web/default/src/lib/api.ts`
- Create: `web/default/src/lib/api-deduplication.test.ts`
- Modify: `web/default/src/features/playground/api.ts`

- [x] **Step 1: Write the failing test**

Mock `originalGet`，以相同 URL 发出两个带不同 signal 的请求；取消第一个不得取消或复用第二个。两个无 signal 请求仍应复用同一个 Promise。

```ts
test('never deduplicates requests carrying AbortSignal', async () => {
  const first = new AbortController()
  const second = new AbortController()
  const one = api.get('/api/user/models', { signal: first.signal })
  const two = api.get('/api/user/models', { signal: second.signal })
  expect(mockAxiosGet).toHaveBeenCalledTimes(2)
  first.abort()
  await expect(two).resolves.toMatchObject({ data: { success: true } })
})
```

- [x] **Step 2: Run test to verify it fails**

Run from `web/default`: `bun test src/lib/api-deduplication.test.ts`

Expected: FAIL because both calls currently share `inFlightGet` by URL/config key.

- [x] **Step 3: Write the minimal implementation**

Change the early-return condition only:

```ts
api.get = ((url: string, config: ApiRequestConfig = {}) => {
  const disableDuplicate = config.disableDuplicate === true
  if (disableDuplicate || config.signal) return originalGet(url, config)
  // existing key calculation and inFlightGet logic remain unchanged
}) as typeof api.get
```

Keep model and generation controllers local to their components. Component cleanup aborts only the request created by that component; cancellation must not show an error toast.

- [x] **Step 4: Run test to verify it passes**

Run from `web/default`: `bun test src/lib/api-deduplication.test.ts src/features/playground && bun run typecheck`

Expected: PASS; switching chat/image pages no longer surfaces `canceled` from another page's request.

- [x] **Step 5: Commit**

```bash
git add web/default/src/lib/api.ts web/default/src/lib/api-deduplication.test.ts web/default/src/features/playground/api.ts
git commit -m "fix(web): isolate abortable get requests"
```

### Task 2: 规范联网搜索后端结果

**Files:**
- Modify: `service/playground_search.go`
- Modify: `service/playground_search_test.go`
- Modify: `controller/playground.go`
- Modify: `controller/playground_test.go`
- Modify: `controller/playground_metadata_writer.go`
- Modify: `controller/playground_metadata_writer_test.go`

- [x] **Step 1: Write the failing backend tests**

覆盖空结果、上游失败、超时、畸形来源、成功重试和敏感错误脱敏。无来源时必须返回空数组而非 `null`，失败只产生一个最终运行日志事件和一个稳定用户错误码。

```go
require.Equal(t, []PlaygroundSearchResult{}, outcome.Results)
require.Equal(t, PlaygroundSearchOutcomeFailed, outcome.Status)
require.Equal(t, "playground_search_failed", outcome.Code)
require.NotContains(t, outcome.Error, "tvly-secret")
```

- [x] **Step 2: Run tests to verify they fail**

Run: `go test ./service ./controller -run 'Test.*Playground.*Search' -count=1`

Expected: FAIL on empty/nil source normalization or incomplete failure metadata.

- [x] **Step 3: Write the minimal implementation**

Replace direct JSON calls in `service/playground_search.go` with project wrappers:

```go
if err := common.DecodeJson(response.Body, &payload); err != nil {
	return nil, fmt.Errorf("%w: invalid provider response", ErrPlaygroundSearchFailed)
}
if payload.Results == nil {
	payload.Results = make([]PlaygroundSearchResult, 0)
}
```

`PlaygroundSearchOutcome` must contain `Status`, stable `Code`, normalized `Results`, and a safe `Error`; never include API keys, channel names or upstream bodies. Emit response metadata once after final search resolution. Preserve native search tools only when the selected route advertises native capability; otherwise use configured fallback or return unavailable without injecting fake results.

- [x] **Step 4: Run tests to verify they pass**

Run: `go test ./service ./controller -run 'Test.*Playground.*(Search|Metadata)' -count=1`

Expected: PASS; cancellation propagates as cancellation, while provider failure returns safe metadata and does not panic.

- [x] **Step 5: Commit**

```bash
git add service/playground_search.go service/playground_search_test.go controller/playground.go controller/playground_test.go controller/playground_metadata_writer.go controller/playground_metadata_writer_test.go
git commit -m "fix(playground): normalize web search outcomes"
```

### Task 3: 防止联网搜索错误崩溃聊天页

**Files:**
- Modify: `web/default/src/features/playground/hooks/use-chat-handler.ts`
- Modify: `web/default/src/features/playground/lib/streaming/stream-utils.ts`
- Modify: `web/default/src/features/playground/lib/streaming/stream-utils.test.ts`
- Modify: `web/default/src/features/playground/lib/streaming/request-error-utils.ts`
- Modify: `web/default/src/features/playground/lib/streaming/request-error-utils.test.ts`
- Modify: `web/default/src/features/playground/components/message/message-metadata.tsx`
- Modify: `web/classic/src/hooks/playground/useApiRequest.jsx`
- Modify: `web/classic/src/components/playground/MessageContent.jsx`
- Create: `web/classic/src/components/playground/web-search-results.test.js`

- [x] **Step 1: Write the failing frontend tests**

Feed `sources` as missing, `null`, object, mixed malformed array and valid array. Assert the conversation remains mounted, invalid entries are discarded, the request error includes request ID, and retry retains the user's message/settings.

```ts
expect(normalizeSearchSources(undefined)).toEqual([])
expect(normalizeSearchSources({ url: 'bad-shape' })).toEqual([])
expect(normalizeSearchSources([{ title: 'A', url: 'https://a.example' }])).toHaveLength(1)
```

- [x] **Step 2: Run tests to verify they fail**

Run from `web/default`: `bun test src/features/playground/lib/streaming`

Run from `web/classic`: `bun test src/components/playground/web-search-results.test.js`

Expected: FAIL where `metadata.sources.map` or equivalent assumes an array.

- [x] **Step 3: Write the minimal implementation**

Define one boundary normalizer per frontend and call it before state updates/rendering:

```ts
export function normalizeSearchSources(value: unknown): SearchSource[] {
  if (!Array.isArray(value)) return []
  return value.filter((item): item is SearchSource => {
    if (!item || typeof item !== 'object') return false
    const source = item as Record<string, unknown>
    return typeof source.title === 'string' && typeof source.url === 'string'
  })
}
```

On request failure, append/update only the current assistant message with a concise localized error and request ID. Do not throw from render, clear the conversation, navigate away or show channel details. Abort errors remain silent.

- [x] **Step 4: Run tests to verify they pass**

Run from `web/default`: `bun test src/features/playground && bun run typecheck`

Run from `web/classic`: `bun test src/components/playground src/hooks/playground`

Expected: PASS; malformed search metadata cannot produce a page-level 500.

- [x] **Step 5: Commit**

```bash
git add web/default/src/features/playground web/classic/src/hooks/playground/useApiRequest.jsx web/classic/src/components/playground/MessageContent.jsx web/classic/src/components/playground/web-search-results.test.js
git commit -m "fix(playground): contain web search failures"
```

### Task 4: 对齐分组模型和图片参数能力

**Files:**
- Modify: `model/playground_image_models_test.go`
- Modify: `controller/playground.go`
- Modify: `controller/playground_test.go`
- Modify: `web/default/src/features/playground/api.ts`
- Modify: `web/default/src/features/playground/lib/model/model-capability-utils.ts`
- Modify: `web/default/src/features/playground/lib/model/model-capability-utils.test.ts`
- Modify: `web/default/src/features/playground/lib/image/image-generation.ts`
- Modify: `web/default/src/features/playground/lib/image/image-generation.test.ts`
- Modify: `web/default/src/features/playground/components/image/playground-image-panel.tsx`
- Modify: `web/classic/src/helpers/playground.js`
- Modify: `web/classic/src/helpers/playground.test.js`
- Modify: `web/classic/src/components/playground/PlaygroundImagePanel.jsx`

- [x] **Step 1: Write the failing tests**

Backend test proves `/api/user/models?group=Select` returns every truly available model in `Select` and none only available elsewhere. Frontend table tests prove group change clears stale model, model search filters locally, hidden capabilities do not enter payload, and visible non-default controls do enter payload.

```ts
expect(buildImageRequest({ ...base, quality: 'hd' })).toMatchObject({ quality: 'hd' })
expect(buildImageRequest({ ...base, background: 'transparent' })).toMatchObject({ background: 'transparent' })
expect(buildImageRequest({ ...base, quality: 'unsupported' })).not.toHaveProperty('quality')
```

- [x] **Step 2: Run tests to verify they fail**

Run: `go test ./model ./controller -run 'Test.*Playground.*Models' -count=1`

Run image/helper tests in both frontends.

Expected: FAIL on at least stale cross-group selection or a control/payload mismatch.

- [x] **Step 3: Write the minimal implementation**

Keep `getUserImageModels(group, signal)` backed by `/api/user/models?group=<group>` and do not add `endpoint_type` filtering: the user explicitly wants all models available to that group. On group change:

```ts
setModel('')
setModels([])
setModelsError('')
modelAbortRef.current?.abort()
```

The model picker must be searchable and disabled when the list is empty, not replaced by a fake “no image models” option. Remove any visible “Output URL” control; `response_format: 'url'` may remain an internal protocol field. Derive quality/style/background controls and request construction from the same capability object.

- [x] **Step 4: Run tests to verify they pass**

Run: `go test ./model ./controller -run 'Test.*Playground.*Models' -count=1`

Run from `web/default`: `bun test src/features/playground/lib/model src/features/playground/lib/image src/features/playground/components/image && bun run typecheck`

Run from `web/classic`: `bun test src/helpers/playground.test.js src/components/playground`

Expected: PASS; selected group, visible model list and submitted `group/model` always agree.

- [x] **Step 5: Commit**

```bash
git add model/playground_image_models_test.go controller/playground.go controller/playground_test.go web/default/src/features/playground web/classic/src/helpers/playground.js web/classic/src/helpers/playground.test.js web/classic/src/components/playground/PlaygroundImagePanel.jsx
git commit -m "fix(playground): align image models and capabilities"
```

### Task 5: 验证图片参数穿过中继

**Files:**
- Modify: `relay/helper/openai_image_request_test.go`
- Modify: `relay/channel/openai/relay_image.go`
- Create: `relay/channel/openai/playground_image_request_test.go`
- Modify: `dto/playground.go`
- Modify: `controller/playground_test.go`

- [x] **Step 1: Write the failing relay test**

Start an `httptest.Server` that captures the exact rightcode-style OpenAI image JSON. Send a non-default request and assert `model`, `prompt`, `n`, `size`, `quality`, `background`, `style` and internal `response_format` values. Also assert unsupported values return 400 before billing/relay.

```go
require.Equal(t, "gpt-image-2", upstream.Model)
require.Equal(t, "hd", upstream.Quality)
require.Equal(t, "transparent", upstream.Background)
require.Equal(t, 2, upstream.N)
```

- [x] **Step 2: Run tests to verify they fail**

Run: `go test ./relay/helper ./relay/channel/openai ./controller -run 'Test.*Playground.*Image' -count=1`

Expected: FAIL if any visible control is dropped, rewritten or bypasses bounds.

- [x] **Step 3: Write the minimal implementation**

Use pointer optional scalars in request DTOs so explicit `false/0` semantics are preserved where valid. Reuse `dto.MaxImageN` and existing image validators. Do not invent provider-specific fields; for OpenAI-compatible rightcode, forward only fields supported by the selected capability map and omit absent values.

```go
type PlaygroundImageRequest struct {
	Model      string  `json:"model"`
	Prompt     string  `json:"prompt"`
	N          *int    `json:"n,omitempty"`
	Size       string  `json:"size,omitempty"`
	Quality    string  `json:"quality,omitempty"`
	Background string  `json:"background,omitempty"`
	Style      string  `json:"style,omitempty"`
}
```

Map upstream `400/401/429/5xx/timeout` to normal request-level errors with request ID. Users must not see channel names, internal URL or upstream credentials; server logs may include channel ID and final status once.

- [x] **Step 4: Run tests to verify they pass**

Run: `go test ./relay/helper ./relay/channel/openai ./controller -run 'Test.*(Playground|OpenAI).*Image' -count=1`

Expected: PASS; all visible non-default parameters are observed by the stub exactly once.

- [x] **Step 5: Commit**

```bash
git add relay/helper/openai_image_request_test.go relay/channel/openai/relay_image.go relay/channel/openai/playground_image_request_test.go dto/playground.go controller/playground_test.go
git commit -m "fix(relay): preserve playground image parameters"
```

### Task 6: 完成游乐场端到端验收

**Files:**
- Create: `docs/superpowers/specs/2026-07-26-playground-reliability-evidence.md`
- Create: `web/default/e2e/playground-reliability.spec.ts`

- [x] **Step 1: Write the failing browser scenarios**

Add Playwright scenarios for: chat↔image rapid switch, group/model search, image failure preserving form, web search empty/failure/success sources, reasoning hidden when toggle关闭, and mobile controls without overlap. Use local mocked upstreams for deterministic `401/429/503/timeout` and successful images.

- [x] **Step 2: Run focused suites**

Run backend: `go test ./service ./controller ./relay/helper ./relay/channel/openai -run 'Test.*Playground' -count=1`.

Run Default: `bun test src/lib/api-deduplication.test.ts src/features/playground && bun run typecheck`.

Run Classic: `bun test src/components/playground src/hooks/playground src/helpers/playground.test.js`.

Expected: PASS before browser execution.

- [x] **Step 3: Run browser scenarios**

Run from `web/default`: `bunx playwright test e2e/playground-reliability.spec.ts --project=chromium`.

Expected: PASS at desktop `1440x900` and mobile `390x844`; no uncaught exception, page 500, false cancellation toast or stale model.

- [x] **Step 4: Validate real rightcode channel**

Against the local copied database and configured rightcode channel, submit one default and one non-default `gpt-image-2` request. Record request ID, model, group, selected parameters, HTTP status and whether an image was returned. Do not record prompt text, API key, channel secret or upstream body. A genuine upstream rejection is acceptable only when it is specific, contained to the request and traceable by request ID.

- [x] **Step 5: Commit**

```bash
git add web/default/e2e/playground-reliability.spec.ts docs/superpowers/specs/2026-07-26-playground-reliability-evidence.md
git commit -m "test(playground): verify search image and cancellation flows"
```
