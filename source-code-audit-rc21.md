# Clove API RC21 源码审计报告

审计日期：2026-07-12，最终复核：2026-07-17（Asia/Shanghai）
审计源码：当前仓库工作树
当前版本：`v1.0.0-rc.21.clove-enhanced`
审计基线：`9b2d76b8bb44f4c98db2f2c93c5912b41be46275`（私人仓库 `main`）
交付分支：`feature/user-governance-wallet-audit-playground`（代码提交 `babf0c3e`、`e5d2fc37`、`af68e9b6`、`746a9434`）
审计范围：Go/Gin/GORM 后端全量源码 + React/TypeScript 双前端 + AGENTS.md 规则合规 + 安全参考（Go/React）
审计方法：静态扫描 + 源码逐文件审查 + 规则匹配 + 并行 agent 深度审计

---

## 执行摘要

本轮对 Clove API RC21 后端进行了系统源码审计，覆盖 AGENTS.md 工程约束合规、OWASP/Go
安全最佳实践、计费安全不变量、认证授权、Clove 定制功能正确性。

**未发现 Critical 级已确认漏洞。** 安全基础设施设计成熟：Session secret 生产强制 ≥32
字节、密码用 bcrypt DefaultCost、API Key 生成用 crypto/rand、CORS 通配模式不启用
credentials、TLS insecure 默认关闭且强制 TLS 1.2、HTTP server 有明确超时、pprof 仅
内部监听、所有 auth 端点有 CriticalRateLimit + TurnstileCheck。前端 XSS 全量经
DOMPurify 清毒、localStorage 不持久化敏感数据、OAuth 重定向有同源校验。认证授权
无角色提权/会话篡改/Root 删除/最后登录方式锁定/OAuth 接管/邀请限制绕过路径。

**发现 3 个 MEDIUM 和 8 个 LOW 级问题**：3 个 MEDIUM 分别为 billing 路径裸 int() 转换
（2 处 AGENTS.md 合规偏差）和 HardDeleteUserById 不清理缓存（删除用户后 Token 在
缓存窗口内仍可用）；8 个 LOW 涵盖 JSON wrapper 绕过、CSRF 设计说明、前端 new Function()
求值、审计日志缺失、邮箱枚举、过期 Token 只读访问、2FA 状态一致性、session cookie
Secure 默认值。均不构成即时可利用漏洞，但应修正以符合工程约束和安全最佳实践。另有
3 项 RC20 已知残余（承袭自上游）。

---

## 发现列表

### AUD-RC21-001 [MEDIUM] Billing 路径使用裸 int(math.Round(...)) 而非 common.QuotaRound

**规则**：AGENTS.md 计费安全不变量 / GO billing safety
**位置**：
- `controller/channel-test.go:545-546`
- `service/quota.go:275`

**证据**：
```go
// controller/channel-test.go:545-546 — settleTestQuota()
quota = usage.PromptTokens + int(math.Round(float64(usage.CompletionTokens)*priceData.CompletionRatio))
quota = int(math.Round(float64(quota) * priceData.ModelRatio))

// service/quota.go:275 — CalcOpenRouterCacheCreateTokens()
return int(math.Round((cost -
    totalPromptTokens*quotaPrice +
    promptCacheReadTokens*(quotaPrice-promptCacheReadPrice) -
    completionTokens*completionPrice) /
    (promptCacheCreatePrice - quotaPrice)))
```

**影响**：AGENTS.md 要求所有 quota 转换必须使用 `common.QuotaRound` / `common.QuotaFromFloat`，
以获得 int32 饱和保护和审计标记。裸 `int(math.Round(...))` 在极端输入下可能溢出 int32 范围
（quota 列是 32 位整数），理论上可产生负费用或回绕。channel-test 路径风险较低（上游测试
响应），但 CalcOpenRouterCacheCreateTokens 的 `cost` 来自上游 `usage.Cost.(float64)`，
若上游返回操纵值，计算结果可能溢出。

**修复**：将上述 `int(math.Round(...))` 替换为 `common.QuotaRound(...)`。对于
CalcOpenRouterCacheCreateTokens，考虑使用 `common.QuotaRoundChecked` 并将
`*QuotaClamp` 记录到日志中。

---

### AUD-RC21-002 [MEDIUM] Billing 邻近路径使用裸 int() 转换

**规则**：AGENTS.md 计费安全不变量
**位置**：
- `relay/claude_handler.go:97`
- `service/relayconvert/internal/oai_chat/to_claude_messages_req.go:167`
- `service/token_counter.go:143,147,157,158,166,167`

**证据**：
```go
// relay/claude_handler.go:97
BudgetTokens: common.GetPointer[int](int(float64(*request.MaxTokens) * model_setting.GetClaudeSettings().ThinkingAdapterBudgetTokensPercentage))

// service/token_counter.go:143
return int(math.Round(float64(imageTokens) * multiplier)), nil
```

**影响**：这些路径计算的是 token 数量而非直接 quota，但 token 数量后续进入计费链路。
`MaxTokens` 受 `maxTokensLimit` 约束，image token 受 1536 上限约束，实际溢出风险低。
但仍偏离 AGENTS.md 集中转换要求。

**修复**：对于进入 quota 计算的 token 计数，使用 `common.QuotaRound`；对于纯 token 数量
展示，添加注释说明非计费用途。

---

### AUD-RC21-003 [LOW] encoding/json 直接调用违反 AGENTS.md JSON wrapper 规则

**规则**：AGENTS.md — "All JSON marshal/unmarshal operations MUST use the wrapper functions in common/json.go"
**位置**：~60 处 `json.Marshal/Unmarshal/NewDecoder` 直接调用，分布在：
- `controller/channel-billing.go`（12 处）、`channel.go`、`console_migrate.go`、`deployment.go`、`misc.go`、`model_meta.go`、`model_sync.go`、`topup_creem.go`、`uptime_kuma.go`、`user.go`
- `middleware/jimeng_adapter.go`、`kling_adapter.go`、`turnstile-check.go`

**证据**：
```go
// controller/channel-billing.go:177
err = json.Unmarshal(body, &response)

// controller/misc.go:354
err := json.NewDecoder(c.Request.Body).Decode(&req)

// controller/topup_creem.go:405
jsonData, err := json.Marshal(requestData)
```

**影响**：`common/json.go` 的 wrapper 当前是薄透传（`common.Marshal` = `json.Marshal`），
实际功能无差异。但 AGENTS.md 要求用 wrapper 以便未来集中加日志、校验或自定义序列化。
controller 层直接调用 `json.Unmarshal` 解析上游响应或用户请求体，绕过了 wrapper。
DTO 层实现 `MarshalJSON`/`UnmarshalJSON` 接口的方法中使用 `json.Marshal` 属可接受场景。
`common/` 包内部调用属可接受场景。

**修复**：将 controller/ 和 middleware/ 中的 `json.Marshal` → `common.Marshal`，
`json.Unmarshal` → `common.Unmarshal`，`json.NewDecoder(r.Body).Decode` → `common.DecodeJson`。

---

### AUD-RC21-004 [LOW] 无 CSRF Token 中间件，依赖 SameSite=Strict Cookie

**规则**：GO-HTTP-006 — Cookie-authenticated state-changing endpoints MUST be CSRF-protected
**位置**：`main.go:192-198`（session cookie 配置），全量 `/api/*` 路由

**证据**：
```go
// main.go:192-198
store.Options(sessions.Options{
    Path: "/", MaxAge: 2592000, HttpOnly: true,
    Secure:   common.SessionCookieSecure,
    SameSite: http.SameSiteStrictMode,  // 最强 SameSite 防御
})
```

**影响**：管理 API（`/api/user/*`、`/api/option/*` 等）使用 session cookie 认证，
存在 CSRF 理论风险。`SameSite=Strict` 是除 token 外最强 CSRF 防御，实际风险低。
Relay API（`/v1/*`）使用 bearer token（`Authorization` 头），无 CSRF 风险。

**说明**：这是上游 New API 的既有设计，非 Clove 引入。`SameSite=Strict` 已提供合理
防御深度。如需进一步增强，可加 Origin/Referer 校验或自定义头要求。

**建议**：记录为已知设计选择，不强制修改（属于上游行为，保持一致）。

---

## 安全控制确认（通过项）

以下安全控制经源码审查确认设计正确且有效：

| 控制项 | 实现位置 | 状态 |
|--------|---------|------|
| Session secret 生产强制 | `common/production_config.go:28-39` | ✅ ≥32 字节，拒绝 placeholder |
| Session secret 默认随机 | `common/constants.go:76` | ✅ `uuid.New().String()` |
| 密码哈希 bcrypt | `common/crypto.go:25,30` | ✅ DefaultCost |
| API Key 生成 crypto/rand | `common/utils.go:226-238` | ✅ `crand.Int(crand.Reader, ...)` |
| CORS 通配不启用 credentials | `middleware/cors.go:24-27` | ✅ 注释明确说明 |
| TLS insecure 默认关闭 | `common/init.go:88`, `tls_config.go:16-17` | ✅ `GetEnvOrDefaultBool("TLS_INSECURE_SKIP_VERIFY", false)` |
| TLS MinVersion 1.2 | `common/tls_config.go:8,21` | ✅ `tls.VersionTLS12` |
| HTTP server 超时 | `main.go:283-291` | ✅ ReadHeaderTimeout/ReadTimeout/IdleTimeout/MaxHeaderBytes |
| pprof 仅内部监听 | `main.go:295` | ✅ `127.0.0.1:8005` |
| Rate limiting auth 端点 | `router/api-router.go` | ✅ CriticalRateLimit + TurnstileCheck 全覆盖 |
| 安全响应头 | `middleware/cors.go:56-67` | ✅ nosniff/DENY/Referrer-Policy/Permissions-Policy/CSP |
| SQL 参数化查询 | `model/user.go:409` | ✅ `?` 参数化 |
| 方言分支锁 | `model/user.go:402-413` | ✅ PostgreSQL/MySQL 分支 |
| SSRF 保护 | `controller/video_proxy.go:72` | ✅ `GetSSRFProtectedHTTPClient()` |
| 无路径遍历 | `controller/performance.go` | ✅ 文件名来自目录列举 |
| 无命令注入 | `common/utils.go:28-42` | ✅ OpenBrowser 无调用方 |
| 无硬编码密钥 | 全量扫描 | ✅ 无 sk-/ghp_/AKIA/PEM 硬编码 |
| 无 text/template | 全量扫描 | ✅ 无 SSTI 风险 |
| 无 cgo | 全量扫描 | ✅ |
| unsafe 仅用于零拷贝 | `common/str.go:103-105` | ✅ string→[]byte 优化 |
| 合规 enforcement | `middleware/compliance.go` | ✅ root 豁免，Token API 返回 403，5min 缓存 |
| Token balance 脱敏 | `service/token_balance.go:445` | ✅ `"sk-" + token.GetMaskedKey()` |
| math/rand 非安全用途 | `common/utils.go:254-257` | ✅ `#nosec G404` 注释，仅 load distribution |
| .env.production.example | 项目根 | ✅ placeholder，loopback 绑定，显式 CORS |

---

### AUD-RC21-005 [LOW] 前端 pricing 表达式使用 new Function() 动态求值

**规则**：GO-XSS-001 / 前端安全 — 动态代码执行
**位置**：
- `web/default/src/features/pricing/lib/tier-expr.ts:305`
- `web/classic/src/pages/Setting/Ratio/components/TieredPricingEditor.jsx:982`

**证据**：
```javascript
// web/default/src/features/pricing/lib/tier-expr.ts:305
new Function(...envKeys, "\"use strict\"; return (${exprStr});")
```

**影响**：两端均使用 `new Function()` 求值 admin 配置的分层定价表达式字符串。
`exprStr` 是管理员配置的模型倍率/定价内容，非终端用户输入，不构成经典 XSS。
但在多管理员部署中，能编辑定价倍率的管理员可在其他管理员会话中执行任意 JS
（存储型代码执行 / 提权向量）。Default 版本还会将 `error.message` 回显到管理 UI。

**修复**：将 `new Function` 替换为沙箱化表达式求值器（小型数学解析器/AST 解释器），
或将求值移至服务端（后端已有 `pkg/billingexpr` 表达式引擎，可复用）。

---

## 前端安全确认（通过项）

以下前端安全控制经深度审查确认正确：

| 控制项 | 实现位置 | 状态 |
|--------|---------|------|
| XSS / dangerouslySetInnerHTML | Default: `html-content.tsx`, `markdown.tsx`; Classic: `DocumentRenderer`, `AnnouncementsPanel`, `FaqPanel`, `NoticeModal`, `Footer`, `About`, `Home`, `OtherSetting` | ✅ 全部经 DOMPurify `sanitizeHtmlContent`/`sanitizeRichHtml` 清毒 |
| Classic CodeViewer | `playground/CodeViewer.jsx:350` | ✅ `escapeHtml`/`highlightJson` 全转义 + `linkifyHtml` 仅包裹已转义 URL |
| Classic MarkdownRenderer | ComplianceGate 使用 | ✅ react-markdown AST，无 raw HTML 执行 |
| localStorage 敏感数据 | playground storage, token-balance, auth-store | ✅ playground `sanitizeAttachmentsForStorage` 剥离 dataUrl/text；token-balance 不持久化 Key；AuthUser 无 access_token/password 字段 |
| sessionStorage | 全量扫描 | ✅ 无 `sessionStorage.setItem` 使用 |
| 硬编码密钥 | 全量扫描 | ✅ 无 sk-/ghp_/AKIA/xox 硬编码 |
| 开放重定向 | Default: `oauth/$provider.tsx`; Classic: `OAuth2Callback.jsx` | ✅ Default 用 `sanitizeRedirectTarget`（要求 `/` 前缀，拒绝 `//`，同源校验）；Classic 用固定路由 |
| Token 日志泄露 | 全量扫描 | ✅ 无 console.log/error/warn 记录 key/token/secret |
| 认证方式 | Default: `lib/api.ts`; Classic: `helpers/api.js` | ✅ `withCredentials: true`（cookie session）+ `New-Api-User` 头仅含用户 ID |

---

### AUD-RC21-006 [MEDIUM] HardDeleteUserById 不清理用户/Token 缓存，删除用户后 API Token 在缓存窗口内仍可用

**规则**：认证授权完整性 / 禁用用户旧 Key 应立即失效
**位置**：`model/user.go:555-565`（HardDeleteUserById），调用方 `controller/user.go:1088`（`DELETE /api/user/:id`）

**证据**：`HardDeleteUserById` 仅执行 `tx.Unscoped().Delete(&User{})` 并移除 OAuth 绑定，**未调用** `InvalidateUserCache`（`model/user_cache.go:63`）或 `InvalidateUserTokensCache`（`model/token.go:520`），也**未删除用户的 Token**。当 Redis 启用时，被删用户的 `UserBase`（status=enabled）和 `Token`（status=enabled）在缓存中保留 `RedisKeyCacheSeconds()` = `SyncFrequency`（默认 60s，可配更高）。在此窗口内，`TokenAuth`（`middleware/auth.go:423` ValidateUserToken → 缓存命中；`:458` GetUserCache → 缓存命中，status=enabled）允许被删用户的 Token 继续发起完整 API 调用。

对比：软删除路径 `ManageUser` "delete"（`controller/user.go:1289-1305`）正确调用了 `user.Delete()` → `invalidateUserCache` + `InvalidateUserTokensCache`。硬删除路径是缺口。

**影响**：管理员删除用户后，该用户的 API Token 在缓存 TTL 窗口内仍可用于 API 调用。孤儿 Token 在 DB 中永久存在（缓存过期后 `GetUserCache` 重读 DB 返回 "record not found" → HTTP 500）。

**修复**：在 `HardDeleteUserById` 内添加缓存失效（`InvalidateUserCache` + `InvalidateUserTokensCache`）并删除用户的 Token。

---

### AUD-RC21-007 [LOW] ManageUser "delete" 动作 fall-through 导致返回错误且跳过审计日志

**规则**：审计完整性
**位置**：`controller/user.go:1289-1305, 1419-1424, 1437`

**证据**：`case "delete"`（line 1289）软删除用户并失效 Token 缓存，但成功路径**无 `return`**，导致执行 fall-through 到 `user.Update(false)`（line 1420）。由于记录刚被软删除，`UpdateWithTx` → `tx.First(&current, user.Id)` 返回 `ErrRecordNotFound`，handler 返回错误响应（尽管删除已成功），且 `recordManageAuditFor`（line 1437）永远不会执行。

**影响**：通过 manage 动作删除用户不在管理员审计追踪中记录（问责缺口），管理员收到误导性 "失败" 响应。缓存失效不受影响（在 delete case 内完成）。

**修复**：在 `case "delete"` 成功路径添加 `return`。

---

### AUD-RC21-008 [LOW] 邮箱注册枚举预言

**规则**：用户枚举防护
**位置**：`controller/misc.go:299-302`

**证据**：`SendEmailVerification` 在 `model.IsEmailAlreadyTaken(email)` 为 true 时返回 "email already taken"，直接泄露邮箱是否已注册。受 `EmailVerificationRateLimit` + `TurnstileCheck` 限制（`router/api-router.go:65`），但不消除预言。

注意：密码重置端点 `SendPasswordResetEmail`（`controller/misc.go:319-345`）实现正确——无论邮箱是否存在都返回统一 `success:true`，不是枚举向量。

**修复**：将 `SendEmailVerification` 改为统一返回成功（无论邮箱是否已注册），在后台静默处理已注册邮箱。

---

### AUD-RC21-009 [LOW] 过期/耗尽 Token 保留只读访问

**规则**：Token 生命周期管理
**位置**：`middleware/auth.go:288-365`（TokenAuthReadOnly）

**证据**：`TokenAuthReadOnly` 仅拒绝 `TokenStatusDisabled`（line 326）；过期和耗尽的 Token 被显式放行至只读端点（如 Token 用量日志）。代码注释中有说明。

**影响**：泄露的过期/耗尽 Token 仍可读取所属用户的数据。Disabled Token 被正确拒绝。

**建议**：根据安全策略决定是否收紧。如保留只读访问为有意设计（方便用户查看历史用量），记录为已知设计。

---

### AUD-RC21-010 [LOW] 2FA 完成时不重新校验用户是否已禁用

**规则**：认证状态一致性
**位置**：`controller/twofa.go:427-434`

**证据**：`Verify2FALogin` 通过 `model.GetUserById` 加载待定用户，但在 `setupLogin`（line 485）前**未检查** `user.Status == common.UserStatusEnabled`。若管理员在密码验证（`Login`，`controller/user.go:80`）和 2FA 完成之间禁用用户，被禁用用户仍可完成登录。

**影响**：实际影响有限——`setupLogin` 将 `status=disabled` 写入 session，后续所有请求被 `currentSessionUser`（`middleware/auth.go:43`）阻断；但 `recordLoginAudit` 记录了成功登录，`UpdateUserLastLoginAt` 更新了被禁用账户的 `last_login_at`。

**修复**：在 `Verify2FALogin` 的 `setupLogin` 调用前添加 `user.Status` 检查。

---

### AUD-RC21-011 [LOW] Session cookie Secure 默认 false

**规则**：GO-HTTP-005 — Cookies MUST use secure attributes in production
**位置**：`common/session_cookie.go:15`, `main.go:192-198`

**证据**：Session cookie 创建时 `Secure: common.SessionCookieSecure`，默认为 `false`，除非 `SESSION_COOKIE_SECURE=true` 或 `FRONTEND_BASE_URL` 是 HTTPS URL（`common/session_cookie.go:18-21` 自动启用）。对于未设置这两项的 HTTPS 部署，session cookie 通过 HTTP 传输。

**影响**：若部署在 HTTPS 但未配置 `SESSION_COOKIE_SECURE` 或 `FRONTEND_BASE_URL`，cookie 可能通过 HTTP 传输被中间人截获。生产 secret 强度校验正确（`common/production_config.go`：`DEPLOYMENT_MODE=production` 时 `SESSION_SECRET` 必需、≥32 字节、拒绝 placeholder）。

**修复**：在 `DEPLOYMENT_MODE=production` 时强制 `SessionCookieSecure=true`，或部署文档中明确要求设置。

---

## 认证授权确认（通过项）

以下认证授权控制经深度审查确认正确：

| 控制项 | 实现位置 | 状态 |
|--------|---------|------|
| Auth middleware 角色校验 | `middleware/auth.go` authHelper → currentSessionUser/ValidateAccessToken | ✅ 每次从 DB 重新加载 role/status，`role < minRole` + `status==disabled` 拦截 |
| New-Api-User 头校验 | `middleware/auth.go:128` | ✅ 必须等于认证 id，session 签名不可篡改 |
| Root 提权限制 | `controller/user.go:1317,1337` | ✅ 仅 ID 1 且 role=root 可提权，`promote_root` 受限 |
| 角色变更防护 | `controller/user.go:758-762,1152,263` | ✅ UpdateUser 拒绝 role 变更，CreateUser 拒绝 `user.Role >= myRole`，Register 硬编码 RoleCommonUser |
| Root 删除防护 | `controller/user.go:1084,1283,1290,1108` | ✅ 全路径阻断 root 删除/禁用/自删 |
| 最后登录方式保护 | `model/user.go:889` EnsureCanRemoveLoginBinding | ✅ 统计 password + OAuth 绑定 + passkey，拒绝移除最后一种 |
| OAuth CSRF | `controller/oauth.go:75-86` | ✅ state 校验 session 存储，使用后删除，服务端生成 |
| OAuth 绑定防接管 | `controller/oauth.go:192-202,295-300` | ✅ 需登录 session，拒绝已绑定 provider，拒绝邮箱碰撞 |
| 邀请码/限制 | `controller/registration.go:24-50`, `model/invite_limit.go:135-138` | ✅ AffRequired 强制，limit 校验，`lockForUpdate` 序列化并发 |
| 密码重置端点 | `controller/misc.go:319-345` | ✅ 统一返回 success:true，无枚举向量 |

---

## RC20 承袭残余（保持不变）

以下残余在 RC20 审计中已记录，RC21 保持不变，属于上游设计：

1. **支付 webhook 记录完整签名和正文** — 上游官方行为，保持一致
2. **管理员可开启 TLS InsecureSkipVerify** — 默认关闭，admin-controlled，`#nosec G402`
3. **签到奖励使用 math/rand** — `model/checkin.go:73`，受 admin MinQuota/MaxQuota 约束和每日原子成功

---

## RC21 修复状态总结

审计日期：2026-07-12，修复执行：2026-07-12

### 已修复（7 项）

| 编号 | 级别 | 修复内容 |
|------|------|----------|
| AUD-RC21-001 | MEDIUM | `channel-test.go:544-545,552` + `service/quota.go:275` — 裸 `int(math.Round(...))` 替换为 `common.QuotaRound` / `common.QuotaFromFloat`，移除未使用的 `math` 导入 |
| AUD-RC21-002 | MEDIUM | `token_counter.go:143,147,157,158,166,167` + `claude_handler.go:97` + `to_claude_messages_req.go:167` — 裸 `int(math.Round(...))` / `int()` 替换为 `common.QuotaRound`（一致性修复，输入有界） |
| AUD-RC21-006 | MEDIUM | `model/user.go` `HardDeleteUserById` + `HardDelete` — 事务前 `InvalidateUserTokensCache`，事务内删除用户令牌行，事务后 `InvalidateUserCache` |
| AUD-RC21-007 | LOW | `controller/user.go:1289-1310` — `ManageUser` "delete" case 添加 `recordManageAuditFor` + 成功响应 + `return`，防止 fall-through 到 `user.Update(false)` |
| AUD-RC21-008 | LOW | `controller/misc.go:299-302` — `SendEmailVerification` 移除邮箱枚举预言，已注册邮箱返回统一 `success:true` |
| AUD-RC21-010 | LOW | `controller/twofa.go:434-444` — `Verify2FALogin` 在 `setupLogin` 前添加 `user.Status` 检查，镜像 `ValidateAndFill` 的状态门控 |
| AUD-RC21-011 | LOW | `common/production_config.go` + `common/init.go:69-71` — 生产环境强制 `SessionCookieSecure=true`，`DEPLOYMENT_MODE=production` 时启动失败并给出配置指引 |

### 不修复（4 项，已记录为接受残留）

| 编号 | 级别 | 不修复原因 |
|------|------|------------|
| AUD-RC21-003 | LOW | 用户决定"没有问题就不要动"，`common/json.go` wrapper 是薄透传，无功能差异 |
| AUD-RC21-004 | LOW | `SameSite=Strict` 已是 CSRF 防御，上游设计选择，保持一致 |
| AUD-RC21-005 | LOW | 用户选择"仅记录风险"，管理员上下文，两端定价表达式求值器保持原样 |
| AUD-RC21-009 | LOW | 用户选择"保持当前行为"，过期/耗尽 Token 保留只读访问记录为有意设计 |

### 额外修复（审计外发现）

| 位置 | 内容 |
|------|------|
| `middleware/compliance.go:127-145` | 旧版合规确认回退（snapshot_id=0）从仅单快照扩展到多快照遍历，修复单/多快照路径不对称的 LOW 边界 case |
| `relay/channel/adapter.go`、19 个渠道适配器、`unsupported_adaptor_test.go` | 不支持 rerank 的转换器不再返回 `(nil, nil)` 并继续发送 JSON `null`，统一返回 `ErrUnsupportedRerankRequest`，表驱动回归覆盖全部受影响渠道 |

### 供应商能力边界

`channel.Adaptor` 是覆盖聊天、Claude、Gemini、音频、图像、Embedding、Rerank 和 Responses 的胖接口。部分供应商只支持其中一部分协议，因此适配器中返回明确错误的转换方法表示“该渠道不支持此端点组合”，不是成功路径，也不应伪造上游能力。生产缺陷的判定标准是：不支持的能力必须明确失败，不能 panic，也不能返回 `(nil, nil)` 后继续请求。现有 Claude 不支持路径和本次 rerank 路径均有回归测试保护。

OpenAPI 中明确标记未实现的 Files/Fine-tunes 路由属于已知产品边界，且本轮任务约束明确禁止路由开发，不纳入本次源码修复范围。

### 验证

- `go build ./...` — ✅ 编译通过，无错误
- `go test ./...` — ✅ 全部测试通过，无失败/panic

---

## 审计结论

```
PASS WITH DOCUMENTED RESIDUALS
```

在已执行审计范围内，未发现确认可利用的 Critical/High 级安全问题。安全基础设施设计成熟，
覆盖了 OWASP Top 10、Go/React 安全最佳实践和 AGENTS.md 工程约束的主要控制项。原审计
发现 3 个 MEDIUM 和 8 个 LOW 级问题，本次修复 7 项（3 MEDIUM + 4 LOW），4 项 LOW 经
用户确认为接受残留。另有 1 项审计外合规边界 case 一并修复。所有修改通过编译和测试验证。
整体安全 posture 良好，不构成阻断发布的问题。
