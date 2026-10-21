# Clove API RC20 源码与生产就绪审计报告

审计日期：2026-07-11
审计源码：`D:\cloveapi\extracted-source`
当前版本：`v1.0.0-rc.20.clove-enhanced`
当前提交：`9b6a0115479951d3629a6d3206861d330e3e56af`（含未提交的本地功能与修复）
官方基线：New API RC20 `90fa6fe6b6452912af395898ceba7c1fd917e066`

## 执行摘要

本轮对 Go/Gin/GORM 后端、React/TypeScript 默认前端、React/Semi UI 经典前端、数据库迁移和本地运行实例进行了静态与动态联合审计。未发现由本轮 Clove 扩展引入的 Critical 或 High 级已确认漏洞，也未发现已确认的计费、支付、订阅、定价或倍率行为偏离官方 RC20。

支付、充值、订阅扣费、定价、倍率、签到和资金来源相关敏感路径已逐文件与官方 RC20 比对，均一致。Clove 新功能集中在邀请上限、来源归属、合规确认、用户角色/第三方绑定管理、SMTP 验证邮件模板、操练场附件、通用日志和匿名 API Key 余额查询；相关后端测试、前端测试、类型检查、构建和三种数据库迁移均已通过。

本报告不把“测试通过”等同于“绝对没有问题”。当前仍有两个需要生产部署方明确接受或缓解的官方 RC20 遗留风险：支付 webhook 会记录完整签名和正文；管理员可显式开启 TLS 证书校验跳过。由于支付敏感源码要求以官方 RC20 为准，本轮没有私自改变这些行为。

## 官方 RC20 一致性

### AUD-RC20-001：敏感业务路径保持官方实现

状态：通过
严重级别：无发现

逐文件 `git diff --quiet` 对比官方 RC20 后，以下路径一致：

- `controller/billing.go`
- `controller/channel-billing.go`
- `controller/checkin.go`
- `controller/pricing.go`
- `controller/ratio_sync.go`
- `controller/subscription_payment_stripe.go`
- `controller/topup.go`
- `controller/topup_creem.go`
- `controller/topup_stripe.go`
- `controller/topup_waffo.go`
- `controller/topup_waffo_pancake.go`
- `model/checkin.go`
- `model/subscription.go`
- `service/billing_session.go`
- `service/funding_source.go`
- `setting/ratio_setting/model_ratio.go`
- 默认前端 pricing、subscriptions、wallet 充值/支付、倍率编辑相关路径

结论：当前版本的支付、充值、订阅扣费、钱包/订阅资金来源选择、模型定价、倍率、阶梯定价和签到额度逻辑以官方 RC20 为准。匿名 Key 余额查询只读取并解释这些结果，不修改余额、订阅、订单或扣费状态。

## 已验证的 Clove 扩展

### AUD-EXT-001：Root 角色和第三方绑定管理

状态：通过
位置：`controller/user.go:1307-1353`、`controller/custom_oauth.go:515-585`

- 只有用户 ID 1 且当前角色为 Root 才能提升或降低其他用户的 Root 角色。
- 用户 ID 1 不能把自己降级，避免系统失去主 Root。
- 后端按目标角色再次鉴权，前端菜单隐藏不构成唯一安全边界。
- 内置和自定义 OAuth 解绑会阻止移除用户最后一种登录方式。
- 角色变更后，过期 session 会在后续 Root 接口访问时被拒绝。

### AUD-EXT-002：邀请上限、邀请注册和来源归属

状态：通过
位置：`model/invite_limit.go:69-139`、`controller/registration.go:33-61`、`controller/user.go:763-772`

- `0` 表示可邀请零人，不代表无限制；无限制由独立模式表示。
- 普通用户继承系统上限；管理员和 Root 默认豁免。
- 给管理员或 Root 设置个人 `limited` 上限后，个人设置优先，默认豁免不再覆盖它。
- 成功注册才计入邀请人数，软删除的已注册用户仍计入，避免通过删除账号重复释放名额。
- 来源归属确认不会发放邀请奖励，也不会消耗邀请名额。
- 密码注册、后台创建和 OAuth 首次注册均有测试覆盖。

### AUD-EXT-003：合规确认、日志和 IP

状态：通过
位置：`middleware/compliance.go:25-191`、`model/compliance.go`、`model/log.go`

- 合规确认按用户、声明版本和内容哈希记录，更新协议内容会发布新快照。
- 普通用户不会获得管理员专用的完整确认记录。
- 邀请注册、跳过邀请码原因、来源归属和合规确认写入任务/审计日志。
- 通用日志保留可审计 IP；本机 IPv6 回环地址规范化显示，避免只出现难理解的 `::1`。
- 任务失败日志和 Redis 日志摘要不会打印原始敏感值。

### AUD-EXT-004：SMTP 验证邮件模板

状态：通过
位置：`controller/misc.go:71-101`、`common/email.go`

- 管理员可配置验证邮件正文模板。
- 模板替换值经过 HTML 转义；未知占位符不会被执行。
- SMTP 支持显式 STARTTLS、SSL、无认证和 NTLM 兼容路径，并有本地 SMTP 测试覆盖。

### AUD-EXT-005：操练场附件

状态：通过
位置：`service/file_service.go:138-188`、`web/default/src/features/playground/lib/attachments/`

- 图片、截图和文件会进入请求 payload，不根据模型能力声明预先拦截图片。
- 远程文件读取有大小上限，避免无界下载。
- 默认前端桌面和 390px 移动视口已检查附件界面，无横向溢出。

### AUD-EXT-006：匿名 API Key 余额查询

状态：通过
位置：`controller/token_balance.go:12-64`、`service/token_balance.go`、`router/api-router.go:33`

- 查询接口为匿名 POST，并有独立 IP 限流、请求体限制和禁止缓存响应头。
- 无效 Key 返回统一错误，不回显 Key，不区分不存在、禁用或用户禁用等内部原因。
- 响应不返回用户名、用户 ID、完整 Key、订单或内部订阅标识。
- 有限 Key、无限 Key、钱包优先、订阅优先、仅钱包、严格订阅回退和账号余额限制均有单元测试。
- 订阅展示遵循官方 RC20 的实际资金来源选择，不虚构“订阅按 Key 分组隔离”的计费规则。
- 默认和经典前端均有成功/失败状态；默认前端七种语言的动态状态和错误文案已补齐。

## 安全加固状态

以下旧报告问题已在当前代码中修复或已有明确控制：

- HTTP server 设置 `ReadHeaderTimeout`、`ReadTimeout`、`IdleTimeout` 和 `MaxHeaderBytes`：`main.go:270-278`。
- pprof 默认关闭，启用时默认只监听 `127.0.0.1:8005` 且使用独立 mux：`main.go:156-162`、`main.go:281-301`。
- 可信代理由 `TRUSTED_PROXIES` 显式配置，默认不信任转发代理：`main.go:246-267`。
- panic 对客户端只返回通用信息，详细错误留在服务端日志：`middleware/recover.go:26-32`。
- `/api` 有统一请求体限制，匿名关键接口使用更小限制：`router/api-router.go:15-21`、`middleware/request_body_limit.go`。
- CORS wildcard 模式不允许凭据；allowlist 模式只对可信 Origin 开启凭据，并兼容 provider headers 与 PATCH：`middleware/cors.go:12-30`。
- 浏览器安全响应头集中设置：`middleware/cors.go:56-66`。
- session cookie 为 HttpOnly、SameSite=Strict；HTTPS 部署可自动/显式启用 Secure：`main.go:185-194`、`common/session_cookie.go:10-44`。
- classic 富文本和 Markdown 在进入 `dangerouslySetInnerHTML` 前使用 DOMPurify：`web/classic/src/helpers/sanitize.js:1-66`。
- OAuth state 一次性消费、绑定用户状态和最后登录方式保护均有测试。
- `govulncheck ./...` 未发现可达漏洞；两个 Bun workspace 的 `bun audit` 均未发现漏洞。

## 剩余风险

### AUD-RES-001：官方支付 webhook 日志包含完整签名和正文

规则：GO-CONFIG-001
严重级别：Medium
状态：官方 RC20 遗留，未改动

位置：

- `controller/topup_stripe.go:155-164`
- `controller/topup_creem.go:246-268`
- `controller/topup_waffo.go:357-369`
- `controller/topup_waffo_pancake.go:453-457`

影响：支付回调正文可能含订单、邮箱或客户元数据，签名 header 也会进入日志系统。日志平台、客服截图或较宽的日志权限会扩大敏感信息暴露面。

处理建议：由于这些文件要求与官方 RC20 一致，生产侧应限制日志访问、缩短保留期并在采集器中对 `signature`、`body`、`payload` 做脱敏。若将来允许偏离官方，应只记录 provider、event ID、订单号、验签结果、长度和 payload hash。

### AUD-RES-002：可配置跳过 TLS 证书校验

规则：GO-HTTPCLIENT-001
严重级别：Medium
状态：默认安全，误配置风险

位置：`common/init.go:85-87`、`common/tls_config.go:5-17`、`common/email.go:37-41`

影响：生产环境启用 `TLS_INSECURE_SKIP_VERIFY=true` 或 `SMTP_INSECURE_SKIP_VERIFY=true` 后，上游 relay、同步或 SMTP 可能接受伪造证书，存在中间人读取或篡改风险。

处理建议：生产配置固定为 `false`；自签证书场景优先安装专用 CA。当前实现已经避免修改 `http.DefaultTransport`，风险只作用于显式使用兼容配置的 client。

### AUD-RES-003：完整 CSP 依赖部署策略

规则：GO-HTTP-004 / REACT-CSP-001
严重级别：Low
状态：部分缓解

位置：`middleware/cors.go:56-66`

当前应用设置 `frame-ancestors 'self'`、`X-Frame-Options: DENY`、`nosniff`、Referrer-Policy 和 Permissions-Policy，但没有在应用层设置完整的 `script-src` CSP。New API 支持站点自定义内容和分析脚本，直接收紧可能破坏现有部署。

处理建议：在实际域名和第三方脚本清单确定后，从 CDN/反向代理部署 `Content-Security-Policy-Report-Only`，观察后再转为强制策略。

### AUD-RES-004：静态规则基线仍有质量债

严重级别：Low
状态：非发布阻断，但应持续治理

- `staticcheck ./...` 仍报告官方 RC20 和存量代码中的 dead code、风格及 API 使用警告。
- 默认前端完整 `bun run lint` 会命中官方恢复的 pricing/payment/subscription 文件和存量规则问题。
- classic `bun run i18n:lint` 报告 106 个存量项，主要是品牌名、技术字符串和旧组件。

这些结果不等于对应功能运行失败，也不是本轮新增回归。敏感路径不能仅为通过 lint 而改变官方业务行为；后续应单独建立基线并逐批清理。

## 验证证据

本轮已通过：

- `go test ./... -count=1`
- `go vet ./...`
- `go mod verify`
- `govulncheck ./...`：可达漏洞 0
- `go test -race ./middleware ./controller ./model ./service -count=1`
- 默认前端：86 个测试、162 个断言、typecheck、生产构建、新功能 i18n 检查
- 经典前端：19 个测试、生产构建
- 默认和经典前端：`bun audit` 均为 0 个漏洞
- SQLite、MySQL 8.4、PostgreSQL 16：启动迁移和关键 Clove 数据模型/查询链路通过
- 动态 CORS：wildcard、可信 allowlist、不可信 Origin 和 PATCH/provider headers 场景通过
- 动态余额页：有限 Key、无限 Key、无效 Key、桌面和 390x844 移动视口通过，无横向溢出或控制台错误
- 用户页：Root 操作菜单与后端权限一致，无页面横向溢出或控制台错误

## 未覆盖边界

- 未使用真实商户生产密钥触发 Stripe、Creem、Waffo、Waffo Pancake 或易支付的真实扣款、退款和 webhook 全链路。
- 未使用所有模型供应商的真实 API Key 穷举每一种 relay 协议；操练场附件验证覆盖本地 payload 生成和界面流程。
- 未执行长期压测、故障注入、跨地域网络、真实 CDN/WAF 或高并发生产流量测试。
- SQLite、MySQL 和 PostgreSQL 的数据结构兼容不代表它们可以共享同一份物理数据库文件；迁移时仍需使用数据库导出/导入和行数、金额、订单状态校验。

## 发布判断

当前源码具备进入受控生产发布流程的条件：敏感计费路径与官方 RC20 一致，已确认的 Clove 功能回归测试通过，未发现可达依赖漏洞。上线前必须确认生产环境不启用 TLS 跳过校验、日志平台对支付 webhook 内容做脱敏，并完成数据库备份和灰度发布。真实支付和模型供应商链路仍应在沙箱/小额环境完成最终验收。
