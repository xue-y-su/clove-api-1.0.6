# 2026-07-25 回归证据

## 邀请码奖励审批与提现

### 需求契约

1. 用户完成邀请码注册后，奖励只能进入待审批状态，注册和历史 worker 都不得自动记入钱包。
2. 管理员可对每笔奖励通过或拒绝；拒绝必须记录原因和审核人。
3. 用户只能提现本人已通过奖励；待审批、已拒绝、已提现奖励均不可重复提现。
4. 提现必须原子地完成钱包账本写入、奖励状态更新和审计事件写入；任一步失败都不得造成部分入账。
5. 历史已经发放的奖励只能迁移为已提现事实，禁止在迁移或 worker 重试时二次入账。

### 已完成实现证据

| 范围 | 实现位置 | 已验证契约 |
| --- | --- | --- |
| 奖励事实表与跨数据库迁移 | `model/invitation.go`、`model/main.go` | SQLite 测试夹具可迁移 `InviteRewardPayout`；生产迁移按 GORM 模型在 MySQL、PostgreSQL 同样建表。 |
| 注册与历史补建 | `service/invitation_registration.go`、`service/invitation_reward_worker.go` | 新注册产生两个 `pending_review` 记录；worker 仅补建记录且不增加用户额度。 |
| 审批与提现 | `service/invitation_reward_payout.go` | 通过前提现返回不可用；通过后仅首次提现增加钱包额度，重复提现被拒绝。 |
| HTTP 契约 | `controller/invitation.go`、`router/api-router.go` | 用户汇总/提现和管理员列表/审核接口均已注册，审核接口沿用管理员权限。 |
| Default 前端 | `web/default/src/features/invitations/` | 钱包状态、提现按钮、审批页面、路由和侧栏入口已有定向测试。 |
| Classic 前端 | `web/classic/src/components/invitations/`、`web/classic/src/pages/InvitationRewards/` | 钱包状态、提现按钮、审批页面和侧栏入口已有定向测试。 |

### 已执行验证

| 命令 | 结果 | 说明 |
| --- | --- | --- |
| `go test ./service -count=1` | 通过 | 覆盖注册、历史补建、审批、提现及钱包事务回归。 |
| Default 奖励相关 Bun 测试 | 通过 | 覆盖 API、状态映射、钱包卡与审批页面的用户可见行为。 |
| Classic 奖励相关 Bun 测试 | 通过 | 覆盖钱包状态、提现条件、审批页面和侧栏入口。 |
| `bun run build`（Default） | 通过 | 新增 `/invitation-rewards` 路由已写入 `routeTree.gen.ts`。 |
| `TEST_MYSQL_DSN=... TEST_POSTGRES_DSN=... GOMAXPROCS=4 go test -p 1 ./model ./service ./controller -count=1 -timeout 30m` | 通过 | 隔离 MySQL 8.4 与 PostgreSQL 16 数据库均参与执行；覆盖迁移、邀请码奖励表、兑换事务、IP 审计、Token 与控制器契约。 |
| `GOMAXPROCS=4 go test ./... -count=1 -p 1` | 通过 | 全部 Go 包通过。Windows 本机在默认并行链接 Minimax 时会耗尽内存，使用单包并行避免环境资源误报。 |
| `bun test --reporter=dot`（Default） | 通过，385 项 | 包含静态键、公告、IP 审计、游乐场、兑换码和邀请码审批回归。 |
| `bun run audit:source && bun run typecheck && bun run lint && bun run format:check && bun run build`（Default） | 通过 | i18n 源码审计、类型、lint、版权头安全格式检查和生产构建均通过；lint 仅输出既有警告。 |
| `bun test --reporter=dot`（Classic） | 通过，236 项 | 包含邀请码审批、钱包、公告、游乐场、IP 审计和 API Key 回归。 |
| `bun run i18n:sync && bun run i18n:lint && bun run lint && bun run build`（Classic） | 通过 | 翻译同步、翻译检查、Prettier 和生产构建均通过。 |
| `git diff --check` | 通过 | 未发现差异中的空白错误。 |

### 翻译回归防护

Default 的同步器此前只重新排序已有语言包，未将 `STATIC_I18N_KEYS` 写入英文基准，导致“源码已登记、语言包缺失”的文案在运行时回退。现已通过 TypeScript AST 读取静态键，在同步前注入英文基准并传播到全部语言包；新的静态键测试会验证七套生产语言包都具备每一个静态键。邀请码奖励审批、公告 HTML/弹窗、公告图片预览和两条邀请奖励审计模板均已补齐简体、繁体中文。

### 发布环境仍需执行

1. 在目标发布环境演练数据库备份、迁移、回滚及构建产物部署；本机隔离数据库与浏览器验证不能替代真实发布演练。

### 风险与防护

- 审核使用状态条件更新，必须核验受影响行数；否则并发审核时可能把“未更新任何行”误判为审核成功。
- 提现采用行锁、状态条件更新和钱包操作 ID 三层保护；后续每次改动都必须保持“恰好一次入账”不变量。
- 审核理由和审计参数属于管理员可见业务数据，前端不得向普通用户暴露审核人的内部信息。

## IP 审计详情权限

### 复现范围

- 角色：普通管理员。
- 路由：`GET /api/log/ip-audit/detail`。
- 数据：一个普通用户在单个公开 IPv4 上产生认证事件；管理员列表视图只接收掩码地址。
- 禁止记录：用户完整 IP、请求正文、密钥、兑换码或任何上游授权信息。

### 修复前的失败证据

| 契约 | 定向测试 | 修复前失败 |
| --- | --- | --- |
| 管理员可查掩码 IP 的详情 | `go test ./model -run TestQueryIPAuditProjectionDetailAllowsAdminActors -count=1` | `permission denied for IP audit details` |
| 管理员路由可查详情 | `go test ./controller -run TestIPAuditDetailAllowsAdminActors -count=1` | `ip_audit_forbidden` |
| Default 显示详情操作 | `bun test src/features/usage-logs/lib/ip-audit.test.ts` | `can_view_details: true` 仍返回 `false` |

### 目标契约

1. `is_root` 仅代表完整 IP 投影；`can_view_details` 独立代表管理员能否展开详情。
2. Root 使用完整 IP 作为详情键；管理员使用掩码 IP 作为详情键，查询不得返回完整地址。
3. 审计事件记录实际的 `full_ip_access`，管理员详情读取必须为 `false`。

### 已完成实现

1. 后端列表稳定返回 `risk_reasons`，前端不再根据展示文案或临时统计自行猜测风险。当前稳定原因包括共享注册 IP、共享 API IP、共享认证 IP、异常活动以及私网或回环来源。
2. Default 与 Classic 均将风险原因直接展示在列表中，并提供明确的“查看/收起 IP 详情”操作；同一时间只展开一行，避免长列表同时请求详情。
3. Root 列表展示完整 IP，并可查看全部相关账号；普通管理员只接收掩码 IP，详情只包含低权限账号。
4. Default 使用 `is_root` 决定“Root 视图”或“Admin 掩码视图”，使用 `can_view_details` 决定是否允许展开。两者不再混用，普通管理员不会再被错误标记为 Root。
5. IPv6 单元格允许自然换行并启用任意位置断行，长地址不会覆盖相邻列；移动端改用不产生页面横向溢出的紧凑布局。

### 自动化回归

| 范围 | 结果 | 覆盖内容 |
| --- | --- | --- |
| 后端 IP 审计定向测试 | 通过 | Root 完整 IP、管理员掩码详情、低角色过滤、读取审计和稳定风险原因。 |
| Default IP 审计与静态键测试 | 通过，32 项 | 风险原因映射、Root/Admin 标签、详情权限和七套语言包静态键。 |
| Default Usage Logs 回归 | 通过，69 项 | IP 审计与使用日志格式、筛选和详情行为。 |
| Classic IP 审计定向测试 | 通过 | 风险原因、详情操作、单行展开和角色权限。 |

### 浏览器验收

| 场景 | 验收结果 | 本机证据文件 |
| --- | --- | --- |
| Root 桌面视图 | 风险徽标正确；同一时间仅一行展开；详情可见 Root、Admin 和普通用户关联。 | `default-root-ip-audit-expanded.png` |
| Root IPv6 视图 | 完整 IPv6 地址正常换行，文本未越过单元格。 | `default-root-ip-audit-ipv6.png` |
| Admin 桌面视图 | 页面显示“Admin 掩码视图”；地址为 `127.0.0.*`；详情仅包含低角色用户。 | `default-admin-ip-audit-masked.png` |
| 移动端视图 | `390x844` 下页面宽度与可视宽度一致，无横向溢出，详情可正常展开。 | `default-mobile-ip-audit-expanded.png` |

上述截图保存在本机 `D:\cloveapi\.tmp\current-source-qa-20260725`，不纳入 Git，避免把测试账号关联详情长期写入仓库。

### 数据口径说明

- 风险视图会排除已识别的代理基础设施地址，避免在未配置可信代理时把反向代理统一来源误报为大量用户共用 IP。
- 被排除的基础设施地址仍会出现在“全部 IP 活动”中，并标记为“基础设施 IP”，因此风险总览为空不代表原始活动丢失。
- 浏览器 IPv6 验收事件属于指定时间窗口投影；验证时使用“30 天 + 全部 IP 活动”，不以终身汇总是否已重建作为展示前提。
