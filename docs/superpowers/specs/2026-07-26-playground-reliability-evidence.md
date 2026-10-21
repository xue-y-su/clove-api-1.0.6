# 游乐场可靠性验收证据

**日期：** 2026-07-26  
**最终运行镜像：** `clove-api:qa-20260726-final3-dockerfile`  
**镜像摘要：** `sha256:2dd42202aa302384caf1af266b1e88cfa73a3f05d9b1f7d42792f7d2d10e6ca6`  
**验收地址：** `http://127.0.0.1:18104`  
**实现提交：** `9ba61b78`、`a51b726d`  
**浏览器门禁提交：** `585d7201`

## 验收范围

- Default 游乐场聊天与图片模式快速切换。
- 图片分组切换、模型搜索、模型能力选项和响应展示。
- 图片请求成功与 HTTP 503 失败时保留表单状态。
- 联网搜索空来源、成功来源和上游失败。
- 关闭思考时隐藏 `reasoning_content` 与 `<think>` 内容。
- 桌面 `1440x900` 和移动端 `390x844` 的控件边界与横向溢出。
- 本地验收数据库中的 rightcode 实际图片渠道。

## 最终浏览器结果

运行目录：`web/default`

```powershell
$env:E2E_BASE_URL='http://127.0.0.1:18104'
$env:E2E_STORAGE_STATE='D:\cloveapi\.verification-output\auth\user.json'
$env:PLAYWRIGHT_CHANNEL='msedge'
bunx playwright test e2e/playground-reliability.spec.ts --project=chromium --reporter=line
```

结果：`9 passed`，总耗时 `21.8s`。

| 场景 | 结果 |
| --- | --- |
| 聊天与图片模式快速切换 | 不显示 `canceled`，不产生页面异常 |
| 图片分组与模型搜索 | 仅展示当前分组模型，切组后不残留旧模型 |
| 图片 503 | 展示具体上游原因并保留提示词、分组和模型 |
| 图片成功响应 | 渲染图片预览，不向用户裸露输出 URL |
| 联网搜索空来源 | 正常结束，不因来源为空崩溃 |
| 联网搜索成功来源 | 渲染并可查看来源 |
| 联网搜索失败 | 留在游乐场并显示可理解的错误 |
| 桌面布局 | 无横向溢出，控件完整可见 |
| 移动端布局 | 无横向溢出，控件完整可见 |

## 源码测试结果

后端聚焦测试：

```powershell
go test ./service ./controller ./relay/helper ./relay/channel/openai -run 'Test.*Playground' -count=1
```

结果：4 个包全部通过。

Default 聚焦测试与类型检查：

```powershell
bun test src/lib/api-deduplication.test.ts src/features/playground
bun run typecheck
```

结果：`52 passed`，类型检查通过。

Classic 聚焦测试：

```powershell
bun test src/components/playground src/hooks/playground src/helpers/playground.test.js
```

结果：`32 passed`。

完整回归中 Default `416/416`、Classic `251/251`、Go 全仓测试均通过。

## rightcode 实际渠道

验收分组：`Select`  
模型：`gpt-image-2`

| 场景 | 参数摘要 | HTTP | request ID | 耗时 | 图片 |
| --- | --- | ---: | --- | ---: | --- |
| 默认参数 | `n=1`、`1024x1024`，质量与背景由上游默认处理 | 200 | `202607252354029044815938268d9d6mHFyWRHb` | 41562 ms | 已返回 |
| 非默认参数 | `n=1`、`1024x1024`、`quality=high`、`background=transparent` | 200 | `202607252354599679180398268d9d6DyGyHamm` | 48828 ms | 已返回 |

验收记录不包含提示词、用户密钥、渠道密钥、图片地址或上游响应正文。

## 结论

游乐场的请求取消隔离、联网搜索容错、分组模型刷新、图片参数透传、错误保留和响应式布局均通过自动化与实际渠道验证。最终标准 Dockerfile 镜像上未发现阻断性回归。
