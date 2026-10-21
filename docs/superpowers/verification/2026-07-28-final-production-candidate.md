# 2026-07-28 最终生产候选验收

## 验收对象

- 源码：`D:\cloveapi\restored-source`
- 分支：`recovery/production-restore-20260719`
- 交付提交：包含本文档的最终提交
- QA 地址：`http://127.0.0.1:18128`
- QA 应用：`cloveapi-invitation-browser-qa`
- QA 镜像：`clove-api:production-candidate-20260728-responsive-5`
- QA MySQL：`cloveapi-invitation-mysql-qa`，版本 8.4.10
- QA 数据库：`cloveapi_browser_qa_20260728`
- 最终主题：`default`

本轮验收以恢复后的完整源码为准。运行镜像同时包含 Default、Classic
两套前端和同一份 Go 后端。提交后使用最终提交重新构建正式镜像，并通过
`scripts/verify-runtime-source.ps1` 核对提交、两套前端源码指纹和运行镜像标签。

验收期间 Docker Desktop 曾未启动。按本机备忘录使用管理员权限恢复
`docker_data.vhdx` 挂载及 Desktop 后，Docker Engine 29.4.3 / Desktop 4.73.0
恢复正常。当前 QA 应用 `RestartCount=0`、`OOMKilled=false`，`/api/status`
返回 HTTP 200、`version=v1.0.0-rc.21-Clove API`、`theme=default`。

## 已完成范围

1. 修复 Security & IP Audit 的历史数据回填、批次推进、风险排序、详情权限、
   IPv6 展示和使用日志精确跳转。风险口径以共享注册/API/认证 IP 和 401
   为主，不把普通上游错误率当作风险。
2. 修复邀请注册、邀请码申请重试、邀请链接、奖励审批与提现状态，区分邀请人
   奖励和被邀请人自动奖励。
3. 修复用户兑换码的钱包单位、创建校验、领取、本人领取、管理员治理、退款/
   不退款、批量结算、已删除状态展示和错误提示。
4. 修复 Playground 分组模型联动、模型搜索、联网搜索能力判断、图片模式取消、
   推理显示控制和两套前端响应式布局。聊天模式保留分组内全部模型。
5. 修复自定义 API Key 的无前缀输入、自动生成 `sk-` 前缀、碰撞隔离、删除后
   重建、完整状态展示和快捷切换分组。
6. 修复个人资料弹窗、昵称冷却提示、头像上传与即时刷新、第三方解绑会话隔离、
   Passkey 中文相对时间和资料头身份文案。
7. 修复系统公告的逐条弹窗配置、HTML、长内容、服务器图片上传、图片放大、
   登录后确认和通知/时间线切换稳定性。
8. 修复匿名状态启动、认证会话隔离、退出行为、Gate 加载错误和 OAuth
   回调，避免单个失败请求把整站导航到 500 页面。
9. 修复系统设置翻译、Select 样式、用户名校验、分层计费边界和两套 Compose
   的生产默认值。
10. 加固公告媒体读取：使用受目录约束的文件 API，拒绝符号链接、目录外路径和
    非普通文件；写权限探针使用随机临时文件，避免并发实例相互覆盖。
11. 修复公告编辑器产生的缩进 HTML 被 Markdown 当作代码块展示；Default、
    Classic 的通知、时间线、详情和登录后弹窗先规范化 HTML 块，再经过原有
    DOMPurify 清洗。
12. 修复 Default API Key 快捷切换分组弹层过窄的问题；桌面、平板和手机均按
    可用视口展开，分组名称、说明和倍率完整换行显示。
13. 新增邀请奖励审批历史、申请详情、冻结和解除冻结。冻结允许整笔冲正后余额
    为负，但不改变其他钱包扣款的余额不足保护；每次动作写钱包流水、递增修订号、
    不可变动作记录、业务日志和审计 outbox。
14. 完成 Default / Classic 邀请审批移动卡片与桌面表格，并修复 320px 分页、
    768px 渠道工具栏与 Playground 控件、Classic 紧凑顶栏以及 1365px 审批历史
    长操作 ID/时间列重叠。

## 本轮新增发现与修复

### 邀请奖励冻结和审批历史

- 提现申请新增 `frozen` 状态，审批历史服务端筛选固定包含 `paid`、`rejected`、
  `frozen`，不会被分页或前端默认筛选遗漏。
- 冻结与解除在单个事务内锁定操作者、申请、目标用户、申请明细和奖励记录；
  钱包、奖励状态、申请状态、动作表和审计 outbox 同步提交。
- 专用负余额调整只用于撤销已入账业务价值；普通钱包扣款仍拒绝余额不足。
- 详情 API 返回奖励来源账号、注册 IP、邀请码、审批人、钱包操作 ID 和完整动作历史。
- Classic 新增审批文案已补齐全部 locale 键和插值占位符；法语、日语、越南语中
  本轮残留的英文回退也已补译。

### 响应式回归

- Default 审批历史桌面表格设置稳定最小宽度，长操作 ID 使用可断行规则，
  1365x900 下不再与状态、提交时间和审核时间重叠；右侧操作列通过表格内部的
  轻量横向滚动访问。
- Default 在窄屏使用审批卡片，分页和页面操作区可换行；渠道工具栏、API Key
  分组浮层和 Playground 控件均限制在可用视口内。
- Classic 在平板宽度切换为紧凑导航菜单，渠道主操作允许换行；审批待处理、
  历史、详情、冻结和解除在移动端均完整可达。

### 既有 MySQL 回归

隔离 MySQL 回归曾发现公告确认的幂等返回值在 `clientFoundRows=true` 时错误：
MySQL 将 GORM 的 `ON DUPLICATE KEY UPDATE id=id` 视为命中一行，重复确认被误报
为首次确认。修复后 MySQL 使用原子的 `INSERT IGNORE`，SQLite/PostgreSQL 继续
使用 `ON CONFLICT DO NOTHING`，三种数据库的重复确认均返回 `created=false`。

## 自动化门禁

| 门禁 | 结果 |
| --- | --- |
| `go test -p 1 ./... -count=1 -timeout 45m` | 通过，447.1s |
| `go vet ./...` | 通过 |
| `go mod verify` | 通过，全部模块校验正确 |
| `gofmt` 差异扫描 | 通过 |
| 隔离 MySQL 8.4.10 `model` | 通过，20.551s |
| 隔离 MySQL 8.4.10 `service` | 通过，23.359s |
| 隔离 MySQL 8.4.10 `controller` | 通过，10.855s |
| Default `bun run test -- --reporter=dot` | 通过，461/461，15259 assertions |
| Default typecheck、lint、format、source audit、copyright | 通过；lint 仅有既有 warning |
| Default 生产构建 | 通过 |
| Classic `bun run test` | 通过，283/283，4992 assertions |
| Classic ESLint、Prettier、i18n lint | 通过；i18n lint 仅有既有 warning |
| Classic 生产构建 | 通过 |
| Default 邀请生产 E2E 基线 | 通过，2/2；最终响应式轮次因已有奖励 fixture 未重复执行 |
| `git diff --check` | 通过 |
| 最终差异凭据扫描 | 通过；命中项仅为测试夹具和日志字段名 |

Default 测试必须使用 `bun run test -- --reporter=dot`；直接运行无路径的
`bun test` 会把 Playwright E2E 当作 Bun 单元测试加载。Classic `i18n:status`
仍报告仓库既有的 379 个 zh-CN 缺失键，因此不是本轮正式门禁，也未记录为通过。

MySQL 门禁使用独立空库 `cloveapi_final_candidate_20260728`，测试完成后已删除，
没有复用 QA 或业务数据库。

## 真实浏览器验收

Default 和 Classic 均覆盖以下四个真实视口：

| 视口 | Default | Classic |
| --- | --- | --- |
| `320x568` | 通过 | 通过 |
| `390x844` | 通过 | 通过 |
| `768x1024` | 通过 | 通过 |
| `1365x900` | 通过 | 通过 |

两套前端各生成 32 张最终截图。自动证据均为 `errors=0`，没有控制台错误、
页面异常、HTTP 500、全局横向滚动或可见元素越界。

人工复核重点：

- Default `1365x900` 审批历史的操作 ID、状态、提交时间和审核时间不再重叠。
- Default `320x568` 待审批卡片完整，`768x1024` 详情和冻结历史正常。
- Classic `768x1024` 顶栏使用紧凑导航菜单且无重叠。
- Classic `320x568` 渠道三个主操作完整换行，`1365x900` 宽屏导航完整。
- 两套前端均可完成待审批/历史切换、详情、冻结和解除；最终主题恢复为 `default`。

## QA 数据终态与审计

最终 Default 浏览器轮次后保留以下可复核 fixture：

| 申请 | 状态 | 金额 | 冻结修订号 |
| --- | --- | ---: | ---: |
| 2 | `frozen` | 500000 | 3 |
| 3 | `paid` | 600000 | 3 |
| 4 | `pending` | 700000 | 0 |

- `invite_reward_payout_actions`：11 条冻结/解除动作。
- `logs`：按 `event_id` 精确关联 11/11。
- `audit_outbox_events`：11/11，全部已投递，全部首次尝试成功。
- 应用容器运行期间无重启、无 OOM、无 500、panic、fatal 或迁移失败。

## 正式镜像与清理顺序

以下步骤必须在本文档所在提交完成并推送后执行：

1. 计算 `web/default`、`web/classic` 的已跟踪源码指纹。
2. 使用最终提交重建正式镜像；`org.opencontainers.image.revision` 必须精确等于
   最终提交，并写入两套前端 SHA-256 标签。
3. 验证镜像 manifest、标签、`/api/status`、Display Version 和最终 `default` 主题。
4. 仅删除本轮 QA 应用、QA MySQL、QA 网络、QA 数据库、`.verification-output`
   中的登录状态/脚本/截图/证据，以及 `production-candidate-20260728-*` 临时镜像。
5. 定向清理 BuildKit 可回收缓存并停止 `cloveapi-docker-daemon-proxy`。

当前清理尚未执行。正式镜像构建前磁盘余量约为 C 盘 17.08 GB、D 盘
10.46 GB；Docker 镜像 26.8 GB，Build Cache 15.7 GB（13.4 GB 可回收），
Local Volumes 1.451 GB（1.412 GB 可回收）。在正式镜像完成前保留 BuildKit 缓存。

清理必须保留原有镜像、所有命名卷和业务数据，并保留
`grok-register-local-privoxy`、`grok-register-local-warp-proxy`。

## 明确延期与环境条件

- 关于页按产品决定延期，不作为本轮阻断项。
- 本地支付跳转缺少真实公网回调环境，按产品决定不作为本轮失败；支付业务逻辑
  由后端测试和本地模拟链路覆盖，正式发布仍需使用真实网关做小额回调验收。
- 已确认不可用的上游模型保持现状；网关负责正常返回上游错误和退款，不伪造
  成功结果。
- 本机候选不等于公网发布。正式上线前仍需目标域名 HTTPS、可信代理、生产
  MySQL/Redis 凭据、稳定加密密钥、异机备份恢复演练、支付回调和镜像漏洞扫描。

## 交付结论

除上述明确延期和外部环境条件外，本轮定制功能、数据库兼容、两套前端、角色
权限和核心业务链路均已通过自动化与真实浏览器验收。最终交付以提交后重新构建、
带精确 revision 与两套前端指纹标签的正式镜像为准。
