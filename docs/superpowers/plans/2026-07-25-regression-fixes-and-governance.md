# 回归修复与治理实施计划

> **供执行代理使用：** 必须逐项执行本计划。每个步骤使用复选框追踪，不可跳过失败用例、数据库验证或双前端验收。

**目标：** 修复游乐场、个人资料、安全与 IP 审计、兑换码、公告、邀请码奖励、API Key、系统设置和测试数据问题，同时不削弱隐私、资金账本和上游错误处理。

**架构：** 保持现有 `Router -> Controller -> Service -> Model` 分层。用户看到的失败信息必须来自稳定错误码；上游诊断信息只进入管理员日志；Default 与 Classic 前端必须使用同一后端契约。邀请码奖励审批必须独立于邀请码申请审批，不能把发码和资金提现混在同一状态机中。

**技术栈：** Go 1.22、Gin、GORM、MySQL/SQLite/PostgreSQL、React 19 + TypeScript + Bun、React 18 + JavaScript、TanStack Query、Base UI、Semi UI。

---

## 已定位的问题

1. **联网搜索没有实际检索提供方。** 运行中的 canary 数据库没有非敏感的 `PlaygroundTavily*` 配置，因此 `service/playground_search.go` 会走“不可用”分支，不会得到真实搜索结果。另有设计缺陷：`controller/playground.go` 把所有 OpenAI 兼容渠道都视为原生搜索能力渠道；一旦配置了宽泛的模型规则，就可能把不支持的工具请求发送给上游。
2. **rightcode 生图错误是上游错误被泛化。** `rightcode` 是已启用的 OpenAI 兼容渠道，位于 `Select` 分组，包含 `gpt-image-2` 等模型。`do_request_failed` 是中继层的通用包装，当前日志缺少足够的脱敏结构化上下文，无法区分请求参数不兼容、渠道配置错误和真实 503 故障。
3. **个人资料副标题不符合目标样式。** `profile-header.tsx` 当前主动渲染 `Username:`、`User Group:`、`Inviter:`，并且无邀请人时显示 `No Inviter`；应改为只展示值。
4. **管理员看得到 IP 风险数，却点不开详情。** 摘要接口是管理员可用，但 `router/api-router.go` 与 `service/ip_audit.go` 将详情限制为 Root；`canShowSensitiveIPAuditDetails` 因而把普通管理员的展开控件完全隐藏。表格同时没有明确说明一条 IP 到底为什么有风险。
5. **兑换失败被刻意压成同一句错误。** `controller/user.go` 对所有钱包兑换错误统一返回 `MsgRedeemFailed`，真实原因只写服务器日志。用户创建的兑换码在过期、已领、自己领取、余额事务失败等情形都无法区分。
6. **删除后的自定义 Key 不能重新创建。** `model.Token` 使用软删除，`tokens.key` 仍保留唯一值；自定义 Key 碰撞检查不查询软删除行，随后插入触发 MySQL 1062。泄露 Key 在认证层必须保持“禁用”，但控制台需要以“已泄露/疑似泄露”展示其业务状态。
7. **公告弹窗和内容能力不符合需求。** 弹窗由全局 `PopupEnabled` 控制，不是单条公告属性。Classic 两个公告编辑入口仍有 `maxCount={500}`。Default 默认按 Markdown 渲染，HTML 不是明确且可测试的契约；现有公告图片也不能点击放大。
8. **邀请码奖励没有“审批后提现”生命周期。** 现有邀请码申请审批已经存在，但 `service/invitation_reward_worker.go` 直接发放奖励，不能满足“管理员审批后用户才可提现”。
9. **系统维护和 SMTP 翻译尚未收口。** Default 的系统维护注册了带“检查更新”按钮的 `UpdateCheckerSection`。邮件模板页面的字符串分散在两套前端本地化文件中，需要通过契约检查，而不是零散补翻译。
10. **用户 ID 跳到 9400037 是 canary 测试数据污染。** 活跃 canary 中只有 2,871 行用户，最大 ID 却是 `9400037`，下一个自增值是 `9400038`；高 ID 行是测试样式账号。这不是数百万正常用户，但清理或重置序列前必须备份并取得明确确认。

## 涉及文件

- `controller/playground.go`、`service/playground_search.go`：联网搜索能力与错误上下文。
- 中继错误 DTO/错误写入点：rightcode 等上游失败的脱敏可观测性。
- `controller/user.go`、两套个人资料组件：紧凑资料副标题。
- `router/api-router.go`、`controller/ip_audit.go`、`service/ip_audit.go`、`model/ip_security_query.go`：管理员 IP 详情和风险原因。
- `service/wallet_redemption.go`、`service/user_redemption*.go`、钱包与用户兑换码前端：兑换错误与治理完整性。
- `model/token.go`、`controller/token.go`、Token 前端：Key 重建、泄露状态、点击分组修改。
- `service/announcement.go`、公告控制器/设置、两套公告页面：单条弹窗、安全 HTML、图片放大。
- 新建邀请码奖励支付模型/服务/控制器/页面：审批和提现。
- 两套 locale、邮件模板页面、维护设置页面：翻译和移除检查更新。
- 新建只读数据报告与需确认的清理运行手册：隔离 canary 测试数据。

## 实施任务

## 当前实施状态（2026-07-25）

本计划是本轮治理的完整范围说明，不以复选框状态替代验证证据。当前已完成的邀请码奖励审批改造如下；其余条目仍按后续回归计划逐项核验。

### 已完成：邀请码奖励审批与提现

1. **独立资金状态机。** 新增 `invite_reward_payouts`，以 `(invite_code_use_id, recipient_kind)` 保证每个邀请使用记录的邀请人、被邀请人各只有一笔奖励事实。状态仅使用 `pending_review`、`approved`、`rejected`、`withdrawn`、`skipped`，不与邀请码申请审批共用状态。
2. **注册不再自动入账。** 注册交易只创建待审批奖励；历史 worker 只负责补建缺失的奖励记录，不能把待审批奖励直接写入用户钱包。
3. **管理员审批。** 管理员可筛选奖励、查看邀请码、邀请人、注册用户、收款人、金额、审核人和理由，并对待审记录执行通过或拒绝。角色范围沿用邀请码管理的管理员层级规则。
4. **用户主动提现。** 钱包只显示待审、已通过、已拒绝和已提现汇总；仅已通过金额可提现。提现在同一数据库事务中锁定奖励、写钱包账本、更新奖励状态和邀请码奖励状态，并写审计事件。
5. **审计与兼容性。** 审核事件为 `invitation.reward_review`，提现事件为 `invitation.reward_withdraw`；模型迁移覆盖 SQLite、MySQL 与 PostgreSQL，历史已完成奖励归档为已提现，避免二次入账。

### 已交付接口

| 访问方 | 接口 | 行为 |
| --- | --- | --- |
| 已登录用户 | `GET /api/invitation/rewards` | 查询本人待审、已通过、已拒绝、已提现汇总。 |
| 已登录用户 | `POST /api/invitation/rewards/withdraw` | 将本人全部已通过奖励一次性记入钱包。 |
| 管理员 | `GET /api/invitation/admin/rewards` | 分页查询奖励审批记录，支持状态与关键词筛选。 |
| 管理员 | `POST /api/invitation/admin/rewards/:id/review` | 对一笔待审奖励通过或拒绝；拒绝必须填写理由。 |

### 前端交付

- Default 在钱包奖励卡显示真实状态，并在“用户兑换码”之后提供“邀请码审批”页面及路由 `/invitation-rewards`。
- Classic 在钱包奖励卡显示真实状态，并提供“邀请码审批”页面及路由 `/console/invitation-rewards`。
- 两套侧栏模块开关均增加 `invitation_reward`，关闭后不再展示审批入口。
- 状态码均映射为可翻译文案，不直接向用户显示内部 `pending_review` 等状态值。

### 任务 1：先建立回归契约

**文件：**
- 修改：`controller/playground_test.go`、`service/playground_search_test.go`、`controller/user_redemption_test.go`、`controller/token_test.go`、`service/announcement_test.go`、`service/invitation_registration_test.go`
- 修改：对应 Default/Classic 前端测试
- 新建：`docs/superpowers/specs/2026-07-25-regression-evidence.md`

- [ ] **步骤 1：记录脱敏复现证据**

在隔离数据库中复现联网搜索、rightcode 生图、兑换、公告、IP 详情、Key 重建和邀请奖励流程。证据只记录请求 ID、路由、角色、渠道 ID、模型、稳定错误码和 HTTP 状态；不得记录 API Key、提示词、完整兑换码或上游授权头。

- [ ] **步骤 2：先写失败的后端契约测试**

为以下外部行为建立表驱动测试：搜索未配置、显式原生搜索渠道、rightcode 风格 503、管理员 IP 详情、紧凑资料信息、单条公告弹窗、奖励审批/提现幂等，以及兑换错误映射。

```go
require.Equal(t, "USER_REDEMPTION_ALREADY_CLAIMED", response.Body.Code)
require.Equal(t, "TOKEN_KEY_REUSABLE_AFTER_DELETE", response.Body.Code)
```

- [ ] **步骤 3：先写失败的前端行为测试**

```ts
expect(screen.getByText('@clove')).toBeVisible()
expect(screen.queryByText(/No Inviter/i)).toBeNull()
expect(screen.getByRole('button', { name: /view IP details/i })).toBeEnabled()
expect(screen.getByRole('button', { name: /withdraw invitation reward/i })).toBeDisabled()
```

- [ ] **步骤 4：运行定向测试并确认失败原因正确**

只运行新增的 Go 包和前端测试文件。把失败断言写入证据文档后，才开始改生产实现。

### 任务 2：修复联网搜索与 rightcode 诊断

**文件：**
- 修改：`controller/playground.go`、`service/playground_search.go`、现有游乐场设置注册表
- 修改：中继错误 DTO/写入点、两套游乐场前端
- 测试：游乐场、搜索服务、中继错误、双前端游乐场测试

- [ ] **步骤 1：建立显式搜索能力配置**

配置契约包含提供方模式（`tavily`、`native`、`disabled`）、允许的渠道 ID 和模型模式。只有渠道和模型都被显式标记为原生搜索能力时，才允许把搜索工具透传给上游。

- [ ] **步骤 2：在中继前处理不可用状态**

无搜索提供方时移除搜索工具，返回结构化 `PLAYGROUND_SEARCH_UNAVAILABLE` 能力状态，并禁用搜索开关且说明原因。不得把伪原生工具发送给任意 OpenAI 兼容渠道，也不得让业务失败演变成整页 500。

- [ ] **步骤 3：保留 rightcode 的安全诊断信息**

管理员日志写入 `channel_id`、渠道名、模型、中继格式、上游 HTTP 状态、是否可重试和请求 ID，以及脱敏后的错误类别。5xx 给用户的提示为“图像生成服务暂时不可用，请稍后重试或切换模型”；不得暴露上游响应体和密钥。

- [ ] **步骤 4：验证真实能力矩阵**

使用 Tavily stub、显式原生渠道、不支持工具的 OpenAI 兼容渠道，以及 rightcode 风格的 200/400/503/超时/畸形 JSON 响应。503 只能影响当前请求，且管理员能从日志定位渠道和错误类别。

### 任务 3：修正个人资料副标题

**文件：**
- 修改：`web/default/src/features/profile/components/profile-header.tsx`
- 修改：Classic 中命中 `inviter_display_name` 或 `inviter_info` 的资料头组件
- 测试：Default Profile 与 Classic Profile Identity 测试

- [ ] **步骤 1：渲染只含值的元信息**

```text
@clove • Admin
@clove • Admin • @inviter
```

不展示 `Username:`、`User Group:`、`Inviter:`、邮箱、角色说明、邀请码和“无邀请人”占位。

- [ ] **步骤 2：保持后端返回语义字段**

`controller/user.go` 继续独立返回 `username`、`group_name`、`inviter_user_id`、`inviter_display_name`；不得重新拼接展示字符串。

- [ ] **步骤 3：验证响应式布局**

测试有/无邀请人、长用户名、桌面和移动端。文字必须可换行且不重叠，资料编辑入口仍位于资料头操作区。

### 任务 4：让安全与 IP 审计真正可操作

**文件：**
- 修改：`router/api-router.go`、`controller/ip_audit.go`、`service/ip_audit.go`、`model/ip_security_query.go`
- 修改：Default `features/usage-logs` IP 审计 API/类型/控件；Classic 对应页面与 API
- 测试：`router/ip_audit_route_test.go`、`service/ip_audit_test.go`、`model/ip_security_query_test.go`、双前端审计测试

- [ ] **步骤 1：分离“可看详情”和 Root 范围**

摘要响应新增 `can_view_details`。详情路由与服务允许管理员访问，同时保留 Root 的特权数据范围。不得用 `is_root` 代替权限位：`is_root` 仅表达范围，`can_view_details` 决定前端是否显示详情操作。

- [ ] **步骤 2：返回明确风险原因与来源事实**

每条 IP 返回稳定、有序的 `risk_reasons`，仅包含实际命中的 `shared_registration_ip`、`shared_authentication_ip`、`shared_api_ip`、`private_or_loopback_source`、`proxy_configuration_warning` 等。注册、认证、API 调用计数必须分别返回；代理污染必须显式标记，不能当作真实用户关联。

- [ ] **步骤 3：在界面上可见、可点、可解释**

按风险等级、受影响账号数、最近活动排序。将纯箭头换为带文字的“查看 IP 详情”操作，行内展示风险徽标，展开区展示关联账号和注册/认证/API 时间线。

- [ ] **步骤 4：验证数据正确性与性能**

构造不同注册、登录、API 事件，共享 IPv4、共享 IPv6、回环/代理污染和多页用户数据。验证不重复账号、不丢分页、管理员可看允许范围内详情，摘要查询走投影而非全量原始日志扫描。

### 任务 5：修复钱包兑换并补全用户兑换码治理

**文件：**
- 修改：`controller/user.go`、`service/wallet_redemption.go`、`service/user_redemption.go`、`service/user_redemption_governance.go`
- 修改：Default 钱包 hook/用户兑换码页面；Classic 钱包/用户兑换码组件
- 测试：兑换控制器、服务、并发和前端钱包测试

- [ ] **步骤 1：为已登录兑换者映射安全错误码**

替换统一 `MsgRedeemFailed`。区分格式错误、未知或不可用、领取自己的码、已领取、过期、领完、旧系统码冲突和可重试内部失败；内部失败附请求 ID。仅需要防枚举的公共接口保留模糊错误。

- [ ] **步骤 2：保持领取账务原子性**

`ClaimUserRedemptionCode` 继续在一个事务内完成锁码、创建领取记录、记钱包、更新次数/状态、写审计。验证钱包调整失败会回滚领取记录和计数；过期退款只提交一次且可幂等重试。

- [ ] **步骤 3：补全管理员治理详情**

列表与详情展示创建者、码、模式、冻结总额、已领/已退/没收/剩余、领取次数、过期、状态原因、删除原因、最后操作。详情含领取历史和相关审计事件。保留禁用、启用、终止、删除、退款、没收，所有资金性操作必须填写理由。

- [ ] **步骤 4：验证用户与管理员完整流程**

覆盖单人码、红包码、他人领取、自己领取拒绝、并发重复领取、过期退款、管理员退款/没收、软删除、详情、复制按钮和两套中文错误提示。

### 任务 6：修复 API Key 生命周期与点击分组修改

**文件：**
- 修改：`model/token.go`、`controller/token.go`、Token Secret 查询/迁移辅助代码
- 修改：Default `features/keys`；Classic Token 页面与 hook
- 测试：Token 模型、控制器、Secret、Default/Classic Key 测试

- [ ] **步骤 1：删除时释放密钥占位**

删除事务先写审计，再清空软删除行的 Key、指纹、密文、前缀和后缀，使相同自定义 Key 可以安全重建。为历史软删除行增加清理迁移，先产出仅含计数的报告，再清理旧密钥数据。

- [ ] **步骤 2：区分认证状态与业务状态**

认证层继续禁用泄露 Key；控制台在 `status_reason == custom_key_collision_suspected_compromise` 时显示“已泄露/疑似泄露”及说明。请求者收到稳定的冲突提示，不能再看到 MySQL 1062。

- [ ] **步骤 3：实现点击分组直接编辑**

将分组单元格变为无障碍按钮，点击后打开现有分组组合框。只提交最小合法 Token 更新并刷新该行；候选项仅包含当前用户可选分组。`auto` 分组保留说明，但也可从同一对话框修改。

- [ ] **步骤 4：验证边界条件**

覆盖自动 `sk-` Key、无前缀自定义 Key、启用 Key 碰撞、泄露 Key 碰撞、删除后重建、历史清理、缓存失效、分组切换和旧 Key 认证。

### 任务 7：公告改为单条弹窗、安全 HTML 与图片放大

**文件：**
- 修改：`service/announcement.go`、公告控制器、公告设置
- 修改：Default 公告、系统内容设置、仪表盘公告组件
- 修改：Classic 公告设置、全局公告、仪表盘公告、清洗辅助代码
- 测试：服务/控制器公告测试和双前端公告测试

- [ ] **步骤 1：新增单条公告展示字段**

JSON 公告 DTO 新增 `popup` 和 `content_format`（`markdown` 或 `html`）。历史公告缺失字段时归一化为 `popup: false`、`content_format: markdown`。全局公告启用仅保留为紧急总开关；从两套后台删除全局弹窗开关。

- [ ] **步骤 2：仅选择勾选弹窗的公告**

`GetAnnouncementPopupState` 只选择已发布且 `popup: true` 的最新公告，签名包含 `popup` 和 `content_format`。不弹窗的公告仍在公告列表中显示，但永远不阻塞已登录用户。

- [ ] **步骤 3：解除 500 字限制但保留容量保护**

移除 Classic 两处 `maxCount={500}` 和对应 Default 限制。保存时验证文档化的 `256 KiB` 字节上限，确保配置、渲染和审计有边界；超限返回字段级、可读错误。

- [ ] **步骤 4：把安全 HTML 做成明确模式**

Markdown 使用既有 Markdown 清洗器，HTML 使用 DOMPurify 支持的 HTML 渲染器。允许安全格式和链接，移除脚本、事件属性、`srcdoc`、危险协议及危险 iframe 行为。测试 HTML 样式生效、XSS 不保留也不执行。

- [ ] **步骤 5：所有公告图片支持放大**

将受管图片做成带 `aria-label="查看公告图片"` 的按钮，打开独立预览对话框，限制视口、支持键盘关闭、避免布局抖动。覆盖全局弹窗、仪表盘详情及 Classic 对应位置。

- [ ] **步骤 6：验证公告顺序与持久化**

覆盖游客、登录确认、合规/来源/公告顺序、弹窗与非弹窗公告、HTML、501 字、256 KiB 边界、图片上传/放大、容器重启后的媒体持久化和确认幂等。

### 任务 8：新增邀请码奖励审批与用户提现

**文件：**
- 新建：`model/invitation_reward_payout.go`、`service/invitation_reward_payout.go`、`controller/invitation_reward_payout.go`
- 修改：`model/main.go`、`router/api-router.go`、邀请码注册/奖励 worker/查询 DTO
- 新建：Default `features/invitation-reward-approvals/*`
- 修改：两套钱包邀请奖励卡、后台侧栏和邀请码组件
- 测试：模型、服务、控制器、Default/Classic 奖励测试

- [ ] **步骤 1：建立独立的资金状态机**

新建 `invite_reward_payouts`，唯一键为 `(invite_code_use_id, recipient_kind)`，包含收款用户、金额、状态、审批/拒绝信息、提现操作 ID 和时间。状态为 `pending_review`、`approved`、`rejected`、`withdrawn`、`skipped`、`failed`；绝不复用邀请码申请状态。

- [ ] **步骤 2：安全迁移旧奖励事实**

历史已完成奖励迁移为 `withdrawn`，不二次入账；未支付的 pending 迁移为 `pending_review`；保留 skipped/failed。迁移先输出计数报告，并在 SQLite、MySQL、PostgreSQL 夹具中演练。

- [ ] **步骤 3：停止自动入账并提供后台审批**

注册只创建待审奖励记录，worker 可以重试建记录但不得对需要审批的奖励直接记钱包。管理员侧栏在“用户兑换码”下新增“邀请码审批”，支持筛选、查看、通过/拒绝理由、收款人、邀请信息、金额与审计历史。

- [ ] **步骤 4：提供用户提现**

钱包邀请奖励卡展示待审、已通过、已拒绝、已提现金额。只有 `approved` 奖励启用“提现”；提现锁定记录，以幂等操作 ID 记入钱包、标记 `withdrawn`、刷新余额并写审计。

- [ ] **步骤 5：验证资金不变量**

覆盖管理员通过/拒绝、并发提现、钱包失败后重试、历史已发奖励回填、审批前零入账、审批后恰好一次入账、角色隔离和双前端本地化。

### 任务 9：收口系统设置、维护页和翻译

**文件：**
- 修改：`web/default/src/features/system-settings/operations/section-registry.tsx`、`maintenance/update-checker-section.tsx`
- 修改：两套邮件模板页面和 locale 文件
- 修改：前端 i18n 契约测试

- [ ] **步骤 1：移除检查更新操作**

保留版本和运行时间信息，删除 GitHub 请求、更新对话框、按钮和相关字符串；不得影响其他系统维护功能。

- [ ] **步骤 2：补齐 SMTP 邮件模板翻译**

枚举邮件模板页所有 `t()` key 和 API fallback。补齐两套前端的中文翻译，将可见英文 fallback 换成翻译 key，同时保留后端错误细节。

- [ ] **步骤 3：强制翻译一致性**

扩展 locale 测试：邮件模板、IP 审计、兑换、奖励审批和公告任一新增 key 缺失中文时 CI 必须失败。执行 Default i18n 同步/检查和 Classic i18n lint。

### 任务 10：隔离测试数据并安全修复 canary 序列

**文件：**
- 新建：只读数据报告工具、`docs/operations/canary-test-data-cleanup.md`
- 新建：带 `--confirm-canary-test-cleanup` 显式确认参数的清理命令
- 测试：可丢弃 MySQL 库中的清理计划测试

- [ ] **步骤 1：只读生成依赖报告**

按用户 ID 报告候选测试用户及其 Token、日志、钱包、邀请、兑换、会话和外键引用。默认只输出计数和 ID，用户名与密钥均脱敏。

- [ ] **步骤 2：清理必须有明确白名单确认**

命令仅接受报告中已确认的精确用户 ID，先将受影响行备份到应用数据库外的带时间戳 SQL 工件，再验证不存在 Root/受保护账号；依赖与预览不一致时中止。未得到用户确认前绝不执行删除。

- [ ] **步骤 3：清理后才重置自增值**

删除和计数核验成功后，才将 canary MySQL `users.AUTO_INCREMENT` 设为 `MAX(id) + 1`。用一次性测试用户验证连续 ID，再通过同一审计清理路径删除该用户。

- [ ] **步骤 4：阻止再次污染**

浏览器和集成测试只能使用隔离数据库。测试 seed 命令若检测到 canary DSN 必须拒绝启动。

### 任务 11：发布前核验

**文件：**
- 修改：`docs/superpowers/specs/2026-07-25-regression-evidence.md`
- 必要时修改部署/回滚文档

- [ ] **步骤 1：运行定向回归测试**

先运行变更涉及的 Go 包、两套前端测试、翻译检查和格式化；只修复本计划引入的失败。

- [ ] **步骤 2：运行三数据库验证**

在 SQLite、MySQL 8.4、PostgreSQL 隔离库验证奖励迁移、Token Secret 清理、兑换事务原子性和公告 JSON 兼容性。

- [ ] **步骤 3：构建并执行角色浏览器回归**

构建 Default 与 Classic，以游客、普通用户、管理员、Root 验证：联网搜索能力状态、rightcode 503、资料头、IP 展开、兑换与治理、Key 重建和分组、单条公告/HTML/图片、奖励审批/提现、邮件模板翻译。

- [ ] **步骤 4：记录发布与回滚事实**

记录提交、镜像摘要、迁移版本、变更设置、数据库备份和回滚行为。只有三数据库与浏览器回归全部通过后，才可以声明具备本轮生产交付条件。
