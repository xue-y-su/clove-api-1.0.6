# rc33 第一阶段生产加固选择性整合设计

## 状态与范围

- 日期：2026-09-06
- Clove 基线：`v1.0.5-beta`，当前整合分支 `integration/rc25`
- 上游参考：New API `v1.0.0-rc.33`，提交 `eb99ab1b40343c3317bb47981cccdbb2b159a5fa`
- 本阶段目标：移植可独立验证、与 Clove 业务定制低耦合的生产修复
- 交付范围：本地源码、自动化测试、本地 PostgreSQL/SQLite 迁移演练、本地 Docker 验证、Git 提交
- 明确排除：生产服务器操作、远端推送、整包合并 rc33、任务插件系统、RelayKit 重构、模型名修饰符、Token Key 官方迁移

上游 `rc25..rc33` 共修改 597 个文件，其中任务插件、RelayKit 和新前端目录形成了新的架构边界。Clove 同期维护钱包与订阅统一结算、用户兑换码、邀请奖励、安全与 IP 审计、自定义 Key、模型广场可见性及 `web/default` 前端，因此本阶段只移植边界清晰的修复，不以合并提交或目录覆盖方式升级。

## 已确认方案

采用分阶段选择性整合：

1. 第一阶段先合入数据库稳定性、遗留约束迁移、统计正确性、时间计费规则和响应头超时配置。
2. 第二阶段另行设计 RelayKit 协议转换、权威 usage、模型修饰符与 `gpt-6-astra` 表达式计费，并适配 Clove 统一结算。
3. 第三阶段在独立实验分支评估任务插件系统，不直接进入生产主线；进入该阶段前先冻结视频模型价格与任务回调样本。

第一阶段不改变用户余额、订阅、API Key、分组、渠道、兑换码、邀请奖励和日志记录的业务语义。

## 回滚与数据保护

源码回滚使用两个既有锚点：

- `snapshot-pre-rc33-phase1-20260906`：当前已提交源码。
- `snapshot-pre-rc33-working-20260906`：包含当前 tracked 未提交修改的工作树快照对象。

在首次让新代码连接本地 PostgreSQL 前，使用 `pg_dump --format=custom` 生成逻辑备份，并记录数据库表数、关键业务表行数与 schema 摘要。迁移后再次记录同一组指标；除预期索引和 schema 变化外，关键业务表行数必须一致。随后在独立临时数据库执行一次 `pg_restore --clean --if-exists`，确认备份可恢复，而不只验证备份文件存在。

本阶段不连接生产服务器。Docker 重建只使用当前本地 `new-api-dev`、`new-api-dev-pg` 和 `new-api-dev-redis` 实验环境。

## 数据库连接稳定性

### SQLite

将默认 DSN 从无效的 `_busy_timeout=30000` 改为：

```text
one-api.db?_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)&_txlock=immediate
```

三个参数分别保证写事务最多等待 30 秒、读写通过 WAL 并发、读后写事务在开始时取得写锁，避免 `SQLITE_BUSY_SNAPSHOT`。测试必须实际打开 SQLite，读取 `journal_mode` 与 `busy_timeout`，并执行竞争写入，不只比较字符串。

### PostgreSQL

PostgreSQL 同时保持 pgx `PreferSimpleProtocol: true` 并关闭 GORM `PrepareStmt`。这使连接可通过 PgBouncer、Neon 或 Supabase 的事务池使用，避免命名 prepared statement 在复用连接上产生 `08P01`/`42P05`。

GORM 升级到官方验证过的 `v1.25.12`，PostgreSQL 驱动同步到 `v1.5.9`；MySQL 和 SQLite 继续保留语句缓存。数据库错误脱敏继续只暴露稳定错误码；对于 `08P01` 和 `42P05` 增加不含原始 SQL 或参数的运维提示。

关闭 prepared statements 后，pgx simple protocol 会把 `driver.Valuer` 返回的 `[]byte` 编码为 `bytea`，直接写入 JSON 列会触发 `22P02`。因此 `ChannelInfo`、`Properties`、`TaskPrivateData` 和 `JSONValue` 的 Valuer 必须返回 `string` 或 `nil`，对应 Scanner 同时接受 `[]byte` 与 `string`。该适配只改变驱动传输类型，不改变数据库 JSON 内容或对外 JSON 结构。

## Prefill Group 遗留唯一约束迁移

`PrefillGroup.Name` 当前使用 `uk_prefill_name WHERE deleted_at IS NULL`，但旧数据库可能仍保留全局唯一对象 `idx_prefill_groups_name`。GORM AutoMigrate 在旧对象存在时可能失败或保留错误语义。

启动迁移在 AutoMigrate 前执行，仅对 PostgreSQL 生效：

1. 查询 `pg_catalog`，识别 `prefill_groups.name` 上的全局唯一约束和独立索引。
2. 只自动处理已知遗留名称 `idx_prefill_groups_name`。
3. 遇到其他全局唯一对象时中止并报告对象名，不猜测、不删除。
4. 在事务内对表取得 `ACCESS EXCLUSIVE` 锁。
5. 确保 `deleted_at` 和目标部分唯一索引存在且定义正确。
6. 删除已知旧约束或索引。
7. 重复执行应无副作用，现有记录内容和软删除语义必须保留。

迁移不修改 Prefill Group 行数据。

## 用量统计正确性

`SumUsedQuotaWithIP` 先把额度合计 Scan 到 `Stat`，随后又把仅含 RPM/TPM 的查询 Scan 到同一个结构体。在部分数据库驱动和 GORM 版本中，第二次 Scan 会把此前的 `Quota` 清零。

修复方式是将 RPM/TPM 查询 Scan 到局部结构，只把 `Rpm` 和 `Tpm` 显式回填到最终 `Stat`。过滤条件、60 秒窗口、ClickHouse/主日志库选择和 API 返回结构均保持不变。

## 时间计费表达式

当前可视化编辑器对所有时间范围都生成：

```text
hour(tz) >= start || hour(tz) < end
```

当 `start <= end` 时该表达式几乎恒真，使日内倍率全天生效。修复规则：

- 日内范围 `start <= end` 使用 `&&`。
- 跨午夜范围 `start > end` 使用 `||`。
- 相等边界生成恒假的 `&&` 范围，不变成全天倍率。
- `hour/minute/weekday/month/day` 只接受各自定义域内的整数。
- 解析器兼容已有合法 `||` 跨午夜规则，也能把 `&&` 的两个相邻时间边界恢复为单个范围控件。
- 无效旧表达式保持原始模式并提示管理员处理，不能在可视化重建时静默丢失。

修复只位于 `web/default` 当前生产前端，不引入官方已经迁移到 `web/src` 的目录结构。

## 上游响应头超时

Clove 已有固定 5 分钟 `ResponseHeaderTimeout`，因此不存在官方修复前的无限等待问题。本阶段保留 5 分钟行为作为默认值，并增加 `RELAY_RESPONSE_HEADER_TIMEOUT` 配置：

- 默认 `300` 秒，保持现有 Clove 行为。
- `0` 表示关闭响应头超时。
- 负数按关闭处理。
- 超过 `time.Duration` 可表达范围的值在转换前截断。
- 只限制等待响应头，不限制响应正文读取，不截断正常 SSE 流。
- 直连、受保护拉取和渠道代理产生的 relay transport 使用相同配置。

该设计吸收 rc33 的可配置性和溢出保护，同时避免把 Clove 当前 5 分钟默认值静默改为官方 30 分钟。

## 非目标与冲突隔离

以下内容本阶段不动：

- `model/token.go`、`model/token_test.go` 和 `main.go` 中正在进行的自定义 Key 展示迁移。
- 钱包、订阅、Token 额度、结算重试和消费日志的原子事务。
- `relaykit/`、`plugins/` 和官方 `web/src/` 目录。
- `@thinking`、`@effort`、`@temperature`、`@topp` 模型修饰符。
- `gpt-6-astra` 内置价格及 rc33 表达式计费。
- 视频任务插件和视频模型价格迁移。

这些能力分别需要独立设计，因为直接移植会跨越 Clove 的资金来源、渠道路由和前端目录边界。

## 验收条件

自动化验收必须同时满足：

1. SQLite 实际启用 WAL、30 秒 busy timeout 和 immediate transaction lock，竞争写入测试通过。
2. PostgreSQL 连接禁用 GORM prepared statements，JSON Valuer/Scanner 兼容 simple protocol，`08P01`/`42P05` 日志提示不泄露原始消息。
3. Prefill Group 迁移在 fresh、已知旧约束、已知旧索引和重复执行场景通过；未知对象保持不变并中止。
4. 日志统计在第二次 Scan 后仍保留额度合计，RPM/TPM 正确。
5. 时间规则生成、解析、混合条件与定义域测试通过。
6. 响应头超时默认值、关闭、正常自定义和溢出值测试通过。
7. `go test ./...`、`go vet ./...`、前端测试、类型检查和构建通过；已知部署配置断言若仍失败，必须单独说明并统一配置或测试。
8. 本地 PostgreSQL 迁移前后关键业务表行数一致，逻辑备份在独立临时数据库恢复成功。
9. 本地 Docker 重建后健康检查、登录、API Key 列表、模型目录、钱包、订阅、用户兑换码和日志查询冒烟通过。
10. 生产服务器在本阶段保持未访问、未修改状态。
