# Clove API Full Browser Audit Remediation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 2026-07-27 全浏览器审计中确认仍成立的问题，同步 Default 与 Classic 两套前端，并用当前源码重建本地运行环境后完成第二轮多角色、多设备验收。

**Architecture:** 后端以稳定业务错误、事务状态机和聚合查询作为单一事实源；两套前端只负责一致的展示与交互，不复制业务判断。实施按数据事务、游乐场、安全审计、前端响应式、内容一致性和部署验收分批提交，每批先写或更新回归测试，再修改最小实现。

**Tech Stack:** Go 1.22、Gin、GORM、SQLite/MySQL/PostgreSQL、React 19 + TypeScript + Base UI、React 18 + Semi UI、Bun、Vitest/Bun Test、Playwright、Docker Compose。

---

## 约束

- 不修改 `/about`；关于页延后。
- 不修改当前正常的首页数字逻辑，只回归检查不出现 `0+`。
- 不修复或隐藏 `grok-imagine-image-lite`。
- 聊天和图片模式继续展示全部模型，切换分组保留当前模型。
- 分组变化必须真实影响请求路由、订阅资格、计费和日志。
- 两套前端都要同步修复并构建。
- 本地支付只验订单前链路，不要求第三方支付完成页成功。

### Task 1: 固定当前基线与运行版本一致性

**Files:**
- Create: `docs/superpowers/verification/2026-07-27-remediation-baseline.md`
- Modify: `scripts/verify-runtime-source.ps1`
- Create: `scripts/rebuild-local-mysql-qa.ps1`
- Test: `scripts/verify-runtime-source.Tests.ps1`

- [ ] **Step 1: 写运行版本校验失败测试**

校验脚本必须比较 `git rev-parse HEAD`、镜像标签/创建时间、容器启动时间和前端静态资源指纹；任一证据缺失时退出非零，不能把旧容器当成当前源码。

```powershell
$result = & .\scripts\verify-runtime-source.ps1 -ContainerName 'cloveapi-local' -ExpectedCommit (git rev-parse HEAD)
if ($LASTEXITCODE -eq 0 -and -not $result.SourceMatches) { throw 'runtime mismatch was not detected' }
```

- [ ] **Step 2: 运行测试并确认旧运行态会失败**

Run: `pwsh -File scripts/verify-runtime-source.Tests.ps1`  
Expected: 若 `18104` 仍来自旧镜像，测试以“runtime source mismatch”失败；若已匹配，记录精确提交和指纹。

- [ ] **Step 3: 实现稳定的运行版本证据**

脚本输出 JSON，至少包含：

```json
{"expected_commit":"COMMIT","image_id":"IMAGE","container_id":"CONTAINER","started_at":"RFC3339","source_matches":true}
```

Docker 构建通过镜像标签和 OCI revision label 记录提交；`rebuild-local-mysql-qa.ps1` 从现有 `cloveapi-current-source-mysql-qa` 的 inspect 结果读取环境、数据卷、端口与网络，构建新镜像后只替换应用容器。脚本不得把环境值打印或写入磁盘，也不得删除 `cloveapi-current-source-mysql-qa-data`、`cloveapi-remote-copy-mysql`、Redis 或隔离网络。

- [ ] **Step 4: 记录磁盘基线**

Run: `Get-PSDrive C,D | Select-Object Name,Used,Free`  
Expected: 文档记录 C/D 盘可用空间、Docker 数据位置和本轮证据目录，后续每次大构建前复查。

- [ ] **Step 5: 提交**

```bash
git add scripts/verify-runtime-source.ps1 scripts/verify-runtime-source.Tests.ps1 scripts/rebuild-local-mysql-qa.ps1 docs/superpowers/verification/2026-07-27-remediation-baseline.md
git commit -m "test(runtime): verify local image matches source"
```

### Task 2: 修复注册随机归属码碰撞

**Files:**
- Modify: `model/user.go`
- Modify: `model/errors.go`
- Test: `model/user_update_test.go`
- Test: `controller/registration_aff_test.go`

- [ ] **Step 1: 写随机码碰撞回归测试**

测试注入可预测生成器，使第一次生成已有 `AffCode`、第二次生成唯一值；注册必须成功且响应不含 `1062`、索引名或 SQL。

```go
require.NoError(t, user.Insert(0))
assert.Equal(t, "next", user.AffCode)
assert.NotContains(t, recorder.Body.String(), "idx_users_aff_code")
```

- [ ] **Step 2: 运行定向测试确认失败**

Run: `go test ./model ./controller -run 'Test.*AffCode.*Collision|TestRegister.*DatabaseError' -count=1`  
Expected: 当前一次性 `tx.Create` 路径不能从碰撞恢复。

- [ ] **Step 3: 实现有限重试**

在模型层为归属码创建建立稳定边界：仅识别 `users.aff_code` 唯一约束，生成并重试最多固定次数；用户名、邮箱和其他错误原样返回上层。`Insert` 与 `InsertWithTx` 复用同一实现，OAuth 与密码注册行为一致。

```go
for attempt := 0; attempt < maxAffCodeCreateAttempts; attempt++ {
    user.AffCode = generateAffCode()
    if err := tx.Create(user).Error; !isAffCodeConflict(err) { return err }
}
return ErrAffCodeGenerationExhausted
```

- [ ] **Step 4: 验证 SQLite 与 MySQL 错误识别**

Run: `go test ./model ./controller -run 'AffCode|Register' -count=1`  
Expected: SQLite 约束文本与 MySQL `1062` 都只在目标列冲突时重试。

- [ ] **Step 5: 提交**

```bash
git add model/user.go model/errors.go model/user_update_test.go controller/registration_aff_test.go
git commit -m "fix(registration): retry generated affiliation collisions"
```

### Task 3: 完成自定义 API Key 生命周期闭环

**Files:**
- Modify: `controller/token.go`
- Modify: `model/token.go`
- Modify: `model/token_secret.go`
- Modify: `service/token_secret.go`
- Modify: `service/token_secret_lookup.go`
- Test: `controller/token_test.go`
- Test: `controller/token_key_outbox_test.go`
- Test: `model/token_revocation_test.go`
- Test: `service/token_secret_test.go`

- [ ] **Step 1: 写删除重建与跨用户碰撞测试**

覆盖：自定义 Key 创建后删除、同用户同值重建成功；用户 B 提交用户 A 的 Key 后 A 状态为 `leaked` 且认证失败；B 收到稳定中文错误；响应不含 `idx_tokens_key`。

```go
assert.Equal(t, common.TokenStatusLeaked, original.Status)
assert.ErrorIs(t, authErr, service.ErrTokenRevoked)
assert.NotContains(t, response, "Duplicate entry")
```

- [ ] **Step 2: 运行定向测试确认真实失败点**

Run: `go test ./controller ./model ./service -run 'Custom.*Token|Token.*Collision|Delete.*Recreate|Leaked' -count=1`  
Expected: 至少一个浏览器复现契约失败；若源码测试已通过，记录运行容器版本差异并保留测试。

- [ ] **Step 3: 原子释放删除 Key 材料**

删除事务先将 `key` 替换为不可认证且全局唯一的墓碑值，清空指纹/密文/前后缀，再软删除。创建前只清理历史旧数据；所有路径使用 GORM，兼容三种数据库。

- [ ] **Step 4: 原子标记跨用户泄露**

碰撞处理锁定命中 Token，在同一事务将状态设为 `leaked`、写入原因并撤销缓存/认证，再拒绝新建。审计只写一次，普通用户错误为“该自定义密钥不可用，请更换”。

- [ ] **Step 5: 统一状态优先级与错误码**

API 返回 `token_custom_key_unavailable`、`token_key_leaked`、`token_key_revoked` 等稳定码；自动 Key 继续带 `sk-`，自定义 Key 保持原值。

- [ ] **Step 6: 运行测试并提交**

Run: `go test ./controller ./model ./service -run 'Token|CustomKey' -count=1`  
Expected: PASS。

```bash
git add controller/token.go model/token.go model/token_secret.go service/token_secret.go service/token_secret_lookup.go controller/token_test.go controller/token_key_outbox_test.go model/token_revocation_test.go service/token_secret_test.go
git commit -m "fix(tokens): close custom key lifecycle"
```

### Task 4: 收紧用户兑换码状态与日志

**Files:**
- Modify: `service/user_redemption.go`
- Modify: `service/user_redemption_governance.go`
- Modify: `controller/user_redemption.go`
- Modify: `model/user_redemption.go`
- Modify: `service/audit_outbox_worker.go`
- Test: `service/user_redemption_test.go`
- Test: `service/user_redemption_concurrency_test.go`
- Test: `controller/user_redemption_test.go`
- Test: `service/audit_outbox_test.go`

- [ ] **Step 1: 写状态、处置和日志回归测试**

覆盖创建者自领、禁用提示、过期、耗尽、管理员退款/没收、并发最后一份领取，以及创建/领取各只有一条用户使用日志。

```go
assert.ErrorIs(t, err, service.ErrUserRedemptionDisabled)
assert.Equal(t, int64(1), countUserVisibleLogs(t, operationID))
```

- [ ] **Step 2: 运行定向测试**

Run: `go test ./service ./controller -run 'UserRedemption|Redemption.*Audit' -count=1`  
Expected: 明确暴露状态映射或重复日志失败。

- [ ] **Step 3: 统一事务状态机**

服务层在事务内锁定兑换码、领取记录和钱包，状态错误映射为稳定错误类型；管理员终止必须携带 `refund` 或 `forfeit` 和原因。成功结果返回最终状态，删除返回明确标记。

- [ ] **Step 4: 聚合用户可见日志**

一次业务操作只投影一条简洁中文摘要，金额使用两位小数。钱包内部前后值和实体 ID 只留管理员内部数据，不再生成额外普通日志。

- [ ] **Step 5: 运行测试并提交**

Run: `go test ./model ./service ./controller -run 'UserRedemption|Wallet.*Redemption' -count=1`  
Expected: PASS。

```bash
git add service/user_redemption.go service/user_redemption_governance.go controller/user_redemption.go model/user_redemption.go service/audit_outbox_worker.go service/user_redemption_test.go service/user_redemption_concurrency_test.go controller/user_redemption_test.go service/audit_outbox_test.go
git commit -m "fix(redemptions): align state errors and audit summaries"
```

### Task 5: 修复游乐场分组与联网能力契约

**Files:**
- Modify: `controller/playground.go`
- Modify: `controller/option.go`
- Modify: `service/playground_search.go`
- Modify: `relay/common/relay_info.go`
- Test: `controller/playground_test.go`
- Test: `service/playground_search_test.go`
- Test: `controller/playground_metadata_writer_test.go`

- [ ] **Step 1: 写分组单一事实源测试**

请求体和上下文同时存在旧、新分组时，控制器必须以当前 UI 请求分组完成 `GenRelayInfo`、订阅资格和审计；重试沿用原请求快照。

```go
assert.Equal(t, "cheap", capturedRelayInfo.UsingGroup)
assert.Equal(t, "cheap", capturedAudit.Group)
```

- [ ] **Step 2: 写联网可用性测试**

覆盖显式原生渠道+模型、仅模型匹配但渠道未授权、Tavily 已配置、未配置、空结果、超时、取消和畸形响应。未配置时选项 API 返回不可用原因，不发送“假联网”请求。

- [ ] **Step 3: 运行定向测试**

Run: `go test ./controller ./service ./relay/... -run 'Playground|Tavily|WebSearch' -count=1`  
Expected: 分组或未配置降级测试先失败。

- [ ] **Step 4: 实现真实能力判定与降级**

原生联网要求模型规则和渠道 ID/类型同时允许；否则仅在 Tavily 配置完整时注入检索结果。公开选项返回：

```json
{"web_search":{"available":false,"native":false,"tavily":false,"reason":"not_configured"}}
```

用户响应只包含稳定中文提示和一个请求 ID，服务端记录渠道诊断但不下发。

- [ ] **Step 5: 运行测试并提交**

Run: `go test ./controller ./service ./relay/... -run 'Playground|Tavily|WebSearch' -count=1`  
Expected: PASS。

```bash
git add controller/playground.go controller/option.go service/playground_search.go relay/common/relay_info.go controller/playground_test.go service/playground_search_test.go controller/playground_metadata_writer_test.go
git commit -m "fix(playground): bind groups and search capability"
```

### Task 6: 修复安全与 IP 审计口径和查询

**Files:**
- Modify: `model/ip_security_query.go`
- Modify: `model/ip_security_event.go`
- Modify: `service/ip_audit.go`
- Modify: `service/ip_security_projection_worker.go`
- Modify: `controller/ip_audit.go`
- Modify: `controller/log.go`
- Test: `model/ip_security_query_test.go`
- Test: `service/ip_audit_test.go`
- Test: `service/ip_security_projection_worker_test.go`
- Test: `controller/ip_audit_outbox_test.go`

- [ ] **Step 1: 写风险口径和完整分页测试**

两个账号共享注册或调用 IP 即为风险；429/500/超时不增加异常；401 增加未授权；详情账号超过页大小时 `total` 准确并可翻页。

```go
assert.True(t, page.Items[0].Risky)
assert.Equal(t, 0, page.Items[0].AnomalyCountForStatus(429))
assert.Equal(t, 2, detail.Accounts.Total)
```

- [ ] **Step 2: 写低噪声审计测试**

切时间范围、视图、分页和打开详情不得写普通使用日志；导出或完整 IP 解密继续写高价值审计。

- [ ] **Step 3: 运行定向测试**

Run: `go test ./model ./service ./controller -run 'IPAudit|IPSecurity|Unauthorized' -count=1`  
Expected: 普通错误统计或读取审计测试失败。

- [ ] **Step 4: 拆分聚合列表与按需详情**

列表只读 rollup/聚合字段；详情按标签和游标分页读取账号、注册、API、401、登录和时间线。删除普通查看事件投影，保留敏感导出/解密事件。

- [ ] **Step 5: 运行测试并提交**

Run: `go test ./model ./service ./controller -run 'IPAudit|IPSecurity|Unauthorized' -count=1`  
Expected: PASS。

```bash
git add model/ip_security_query.go model/ip_security_event.go service/ip_audit.go service/ip_security_projection_worker.go controller/ip_audit.go controller/log.go model/ip_security_query_test.go service/ip_audit_test.go service/ip_security_projection_worker_test.go controller/ip_audit_outbox_test.go
git commit -m "fix(security): narrow IP risk and page details"
```

### Task 7: 修复 Default 的 Key、兑换码、审计和游乐场布局

**Files:**
- Modify: `web/default/src/features/keys/components/api-key-group-combobox.tsx`
- Modify: `web/default/src/features/keys/components/api-key-group-editor.tsx`
- Modify: `web/default/src/features/keys/components/api-keys-table.tsx`
- Modify: `web/default/src/features/keys/components/api-keys-cells.tsx`
- Modify: `web/default/src/features/user-redemptions/index.tsx`
- Modify: `web/default/src/features/user-redemptions/components/user-redemptions-table.tsx`
- Modify: `web/default/src/features/user-redemptions/components/create-dialog.tsx`
- Modify: `web/default/src/features/user-redemptions/components/admin-governance-dialog.tsx`
- Modify: `web/default/src/features/security-audit/index.tsx`
- Modify: `web/default/src/features/usage-logs/components/ip-audit-table.tsx`
- Modify: `web/default/src/features/playground/hooks/use-playground-state.ts`
- Modify: `web/default/src/features/playground/components/input/playground-input-tools.tsx`
- Modify: `web/default/src/features/playground/components/chat/playground-chat.tsx`
- Test: adjacent `*.test.ts` / `*.test.tsx` files in these feature directories

- [ ] **Step 1: 写交互回归测试**

覆盖：Key 分组浮层挂到 Portal、长列表可滚动、手机卡片可切组、`leaked` 优先展示；兑换码复制回退、成功后弹窗关闭、手机主内容可滚动；IP 详情在独立面板可见；游乐场切组保留模型但 payload 使用新组；搜索未配置时禁用；消息区保留底部空间。

- [ ] **Step 2: 运行定向测试确认失败**

Run: `bun test src/features/keys src/features/user-redemptions src/features/security-audit src/features/usage-logs/lib/ip-audit.test.ts src/features/playground`  
Expected: 当前布局或产品决定冲突测试失败。

- [ ] **Step 3: 实现响应式与状态修复**

使用 Base UI 的 Portal/Positioner，设置 collision avoidance、最大可视高度和滚动；`768px` 及以下切换不重叠布局。安全详情移出宽表横向滚动上下文。兑换码页面恢复单一纵向滚动所有者。游乐场 payload 读取当前 group store，模型值不随切组清空。

- [ ] **Step 4: 运行 Default 验证**

Run: `bun test src/features/keys src/features/user-redemptions src/features/security-audit src/features/playground && bun run typecheck && bun run lint && bun run build`  
Expected: 全部 PASS，生产构建成功。

- [ ] **Step 5: 提交**

```bash
git add web/default/src/features/keys web/default/src/features/user-redemptions web/default/src/features/security-audit web/default/src/features/usage-logs web/default/src/features/playground
git commit -m "fix(default-ui): restore responsive admin workflows"
```

### Task 8: 同步修复 Classic 的关键交互

**Files:**
- Modify: `web/classic/src/components/table/tokens/TokensColumnDefs.jsx`
- Modify: `web/classic/src/components/table/tokens/TokensTable.jsx`
- Modify: `web/classic/src/hooks/tokens/useTokensData.jsx`
- Modify: `web/classic/src/components/user-redemptions/UserRedemptionsTable.jsx`
- Modify: `web/classic/src/components/user-redemptions/CreateUserRedemptionModal.jsx`
- Modify: `web/classic/src/components/user-redemptions/AdminUserRedemptionGovernanceModal.jsx`
- Modify: `web/classic/src/components/playground/SettingsPanel.jsx`
- Modify: `web/classic/src/components/playground/ChatArea.jsx`
- Modify: `web/classic/src/hooks/playground/usePlaygroundState.js`
- Modify: `web/classic/src/pages/Log/index.jsx`
- Test: adjacent Classic tests

- [ ] **Step 1: 写与 Default 同契约的 Classic 测试**

断言手机 Key 可切组、状态完整；兑换码可复制和滚动；切组保留模型但请求使用新组；搜索不可用时禁用；安全详情和消息操作可达。

- [ ] **Step 2: 运行定向测试确认失败**

Run: `bun test src/components/table/tokens src/components/user-redemptions src/components/playground src/hooks/playground`  
Expected: 至少一项旧交互失败。

- [ ] **Step 3: 使用 Semi Portal/Dropdown 修复布局**

浮层挂到 `document.body` 并限制视口高度；窄屏使用卡片/操作菜单，避免固定操作列覆盖分组。Classic 请求构造与 Default 使用相同后端 group/search 契约。

- [ ] **Step 4: 运行 Classic 验证并提交**

Run: `bun test && bun run eslint && bun run build`  
Expected: PASS。

```bash
git add web/classic/src/components/table/tokens web/classic/src/hooks/tokens web/classic/src/components/user-redemptions web/classic/src/components/playground web/classic/src/hooks/playground web/classic/src/pages/Log
git commit -m "fix(classic-ui): align responsive workflow behavior"
```

### Task 9: 完成公告逐条发布与媒体链路

**Files:**
- Modify: `setting/announcement_setting/config.go`
- Modify: `service/announcement.go`
- Modify: `service/announcement_media.go`
- Modify: `controller/announcement.go`
- Modify: `controller/announcement_setting.go`
- Modify: `router/api-router.go`
- Modify: `web/default/src/features/system-settings/content/announcements-section.tsx`
- Modify: `web/default/src/features/announcements/global-announcement-dialog.tsx`
- Modify: `web/default/src/features/announcements/announcement-image-preview.tsx`
- Modify: `web/classic/src/pages/Setting/Dashboard/SettingsAnnouncements.jsx`
- Modify: `web/classic/src/components/layout/GlobalAnnouncementModal.jsx`
- Modify: `docker-compose.yml`
- Modify: `docker-compose.dev.yml`
- Test: existing announcement tests in backend and both frontends

- [ ] **Step 1: 写逐条公告契约测试**

每条公告独立 `popup`、发布状态、HTML 和图片；匿名用户不请求/确认；确认与合规声明分离；媒体目录启动可写、上传原子保存、未引用旧图可清理。

- [ ] **Step 2: 运行定向测试**

Run: `go test ./setting/announcement_setting ./model ./service ./controller ./router -run 'Announcement' -count=1`  
Expected: 全局 popup 依赖或列表编辑路径失败。

- [ ] **Step 3: 移除全局弹窗开关依赖并统一公告模型**

旧 `PopupEnabledKey` 保持可读取但不参与运行；弹窗只看公告项 `popup`。服务端保存前清洗 HTML、验证媒体引用和发布字段。管理端提供列表及新增/编辑弹窗，不再只显示单一文本框。

- [ ] **Step 4: 修复媒体目录与图片灯箱**

启动探针验证 `/data/announcement-media` 可写；Compose init 只为运行 UID 设置该目录。两套前端使用方形加号上传位，图片可预览、删除和点击放大。

- [ ] **Step 5: 运行双前端与 Docker 媒体验证并提交**

Run: `go test ./setting/announcement_setting ./model ./service ./controller ./router -run 'Announcement' -count=1`  
Run Default: `bun test src/features/announcements src/features/system-settings/content/announcements-section.test.ts`  
Run Classic: `bun test src/components/layout/global-announcement.test.js`  
Expected: PASS。

```bash
git add setting/announcement_setting service/announcement.go service/announcement_media.go controller/announcement.go controller/announcement_setting.go router/api-router.go web/default/src/features/system-settings/content web/default/src/features/announcements web/classic/src/pages/Setting/Dashboard/SettingsAnnouncements.jsx web/classic/src/components/layout/GlobalAnnouncementModal.jsx docker-compose.yml docker-compose.dev.yml
git commit -m "fix(announcements): complete per-item publishing"
```

### Task 10: 统一邀请、日志、SMTP、折扣、订阅币种和维护页

**Files:**
- Modify: `controller/audit.go`
- Modify: `model/log.go`
- Modify: `service/email_template.go`
- Modify: `web/default/src/features/usage-logs/lib/format.ts`
- Modify: `web/default/src/features/invitations/components/admin-invitations-page.tsx`
- Modify: `web/default/src/features/system-settings/integrations/email-templates-section.tsx`
- Modify: `web/default/src/features/system-settings/maintenance/update-checker-section.tsx`
- Modify: `web/default/src/features/wallet/components/subscription-plans-card.tsx`
- Modify: `web/default/src/features/subscriptions/lib/format.ts`
- Modify: corresponding Classic invitation, email, maintenance and subscription components
- Modify: locale files under both frontends
- Test: related backend and frontend localization tests

- [ ] **Step 1: 写业务文案和金额测试**

断言 `password`、奖励状态、Token 状态码和 `token.group_update` 显示中文业务语义；SMTP 默认模板为中文；折扣显示优惠而非关闭；订阅全部为 `¥`；维护页没有检查更新操作。

- [ ] **Step 2: 运行定向测试**

Run backend: `go test ./controller ./model ./service -run 'Audit|EmailTemplate|Subscription' -count=1`  
Run Default: `bun test src/features/usage-logs src/features/invitations src/features/subscriptions src/i18n`  
Expected: 英文枚举或货币混用测试失败。

- [ ] **Step 3: 建立集中映射并移除失效入口**

后端审计事件输出结构化前后值；前端集中翻译枚举，不解析自由文本。SMTP 新建模板默认值使用中文。删除 Default 与 Classic 的“检查更新”按钮和调用，不删除版本展示。

- [ ] **Step 4: 同步翻译并验证两套构建**

Run Default: `bun run i18n:sync && bun run audit:source && bun run typecheck && bun run build`  
Run Classic: `bun run i18n:sync && bun run build`  
Expected: 无新增未翻译用户文案，构建成功。

- [ ] **Step 5: 提交**

```bash
git add controller/audit.go model/log.go service/email_template.go web/default/src web/classic/src
git commit -m "fix(content): align audit copy and currency"
```

### Task 11: 数据库、自动化与静态审计

**Files:**
- Modify: regression tests discovered during Tasks 2-10
- Create: `docs/superpowers/verification/2026-07-27-remediation-automated-results.md`

- [ ] **Step 1: 运行后端定向测试**

Run: `go test ./model ./service ./controller ./router -run 'AffCode|Token|UserRedemption|Playground|Tavily|IPAudit|Announcement|EmailTemplate|Subscription' -count=1`  
Expected: PASS。

- [ ] **Step 2: 运行后端全量测试**

Run: `go test ./... -count=1`  
Expected: PASS；与本轮无关的失败单独记录，不掩盖相关失败。

- [ ] **Step 3: 运行真实 MySQL 集成**

使用本地 MySQL 容器和独立测试库运行带 MySQL 条件的注册、Token、兑换码、公告与审计测试；测试数据使用唯一前缀并在完成后清理。

Run: `go test ./model ./service ./controller -run 'MySQL|MainDatabase|AffCode|Token|UserRedemption' -count=1`  
Expected: PASS，响应不泄露 MySQL 错误。

- [ ] **Step 4: 运行双前端全量验证**

Run Default: `bun test && bun run audit:source && bun run typecheck && bun run lint && bun run format:check && bun run build`  
Run Classic: `bun test && bun run eslint && bun run lint && bun run build`  
Expected: PASS。

- [ ] **Step 5: 静态审计**

Run: `git diff --check`  
Run: `rg -n 'Duplicate entry|idx_tokens_key|distributor\)|token secret is unavailable|\$[0-9]' web/default/src web/classic/src controller service model`  
Expected: 用户可见路径不包含原始数据库/渠道错误和订阅美元符号。

- [ ] **Step 6: 提交自动化证据**

```bash
git add docs/superpowers/verification/2026-07-27-remediation-automated-results.md
git commit -m "test(remediation): record automated validation"
```

### Task 12: 重建运行环境并完成第二轮全浏览器审计

**Files:**
- Create: `qa-reports/2026-07-27-remediation-browser-audit/report.md`
- Create: screenshots and evidence under `qa-reports/2026-07-27-remediation-browser-audit/`
- Create: `docs/superpowers/verification/2026-07-27-remediation-delivery.md`

- [ ] **Step 1: 构建当前源码并验证版本匹配**

Run: `pwsh -File scripts/rebuild-local-mysql-qa.ps1 -ContainerName cloveapi-current-source-mysql-qa -ImageTag clove-api:qa-20260727-remediation -HostPort 18104`  
Run: `pwsh -File scripts/verify-runtime-source.ps1 -ContainerName cloveapi-current-source-mysql-qa -ExpectedCommit (git rev-parse HEAD)`  
Expected: `source_matches=true`，健康检查通过，公告目录可写。

- [ ] **Step 2: 多角色功能回归**

匿名用户验证注册错误不泄露 SQL、未登录不弹公告、首页数字正常；普通用户验证 Key、游乐场、订阅、兑换码、邀请和日志；管理员验证安全审计、用户兑换码、公告、SMTP、支付设置和维护页。

- [ ] **Step 3: 多设备兼容回归**

使用 `1440 x 900`、`1280 x 720`、`768 x 1024`、`390 x 844`。对每个核心页面检查横纵滚动、下拉位置、弹窗边界、固定工具栏、IPv6、长分组名和长错误；使用布局边界断言确认交互元素不相交。

- [ ] **Step 4: 故障链路回归**

通过本地模拟服务验证 Tavily、图片和 SMTP 的成功、401、429、503、超时、取消和畸形响应。用户只看正常中文错误和一个请求 ID，服务端日志可按该 ID 定位且不重复刷屏。

- [ ] **Step 5: 逐项复核处置矩阵**

对设计文档中的 29 条 ISSUE 和新增 Key 下拉问题逐项给出当前源码、自动化或浏览器证据；ISSUE-001、004、012、017、029 按已确认产品决定判定，不重新解释范围。

- [ ] **Step 6: 最终源码审计与提交**

Run: `git status --short && git diff --check && git log --oneline -15`  
Expected: 只有计划内证据文件待提交，源码工作区干净。

```bash
git add qa-reports/2026-07-27-remediation-browser-audit docs/superpowers/verification/2026-07-27-remediation-delivery.md
git commit -m "docs(delivery): record full remediation audit"
```

## 完成标准

只有以下证据全部成立才标记目标完成：

1. 设计处置矩阵每项都有直接证据；
2. 后端定向和全量测试通过；
3. SQLite 与真实 MySQL 关键事务通过；
4. Default 与 Classic 测试、检查和生产构建通过；
5. 当前运行容器明确来自最终提交；
6. 三角色、四视口真实浏览器复审通过；
7. 用户错误不泄露渠道/SQL，使用日志无重复刷屏；
8. C/D 盘空间在交付时记录且未被无用缓存耗尽；
9. 最终 Git 工作区干净，全部修复和证据已提交。
