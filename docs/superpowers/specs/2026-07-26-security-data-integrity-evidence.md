# 安全与数据完整性验证证据

**日期：** 2026-07-26  
**实现提交：** `9ba61b78`（后端契约）、`a51b726d`（双前端）  
**范围：** IP 审计 401 风险投影、Token 撤销墓碑与密钥迁移、用户兑换码状态机和资金并发约束。

## 验证环境

| 项目 | 值 |
| --- | --- |
| Go | `go1.26.5 windows/amd64` |
| SQLite | `github.com/glebarez/sqlite v1.9.0`，`modernc.org/sqlite v1.40.1` |
| MySQL | `8.4.10`，隔离测试数据库 |
| PostgreSQL | `15.18`，隔离测试数据库 |
| 最终应用镜像 | `clove-api:qa-20260726-final3-dockerfile` |
| 镜像摘要 | `sha256:2dd42202aa302384caf1af266b1e88cfa73a3f05d9b1f7d42792f7d2d10e6ca6` |

MySQL 和 PostgreSQL 测试只使用可删除的隔离数据库。本文件和 Git 历史不记录 DSN、密码或生产连接信息；需要空库的测试会重建数据库或 PostgreSQL `public` schema。

## 自动化结果

| 验证项 | 结果 |
| --- | --- |
| Go 全仓 `go test -p 1 ./... -count=1` | 全部通过，最终复验耗时 `362.1s` |
| SQLite `model/service/controller/router` | 全部通过 |
| MySQL 8.4 `model/service/controller/router` | 全部通过 |
| PostgreSQL 15 `model/service/controller/router` | 全部通过 |
| IP 投影和完整迁移 | PostgreSQL rollup、MySQL 兼容迁移、两种全量迁移均实际运行并通过 |
| 兑换码并发资金约束 | MySQL 与 PostgreSQL 均只有一次创建成功，另一次稳定返回活动数量限制 |
| Token 旧列迁移 | MySQL 与 PostgreSQL 的 `char(48)` 均迁移到 `varchar(128)` 并可写入较长密钥 |

跨库命令通过以下隔离环境变量执行：

```powershell
$env:TEST_MYSQL_DSN = '<isolated-mysql-dsn>'
$env:TEST_POSTGRES_DSN = '<isolated-postgres-url>'
go test -p 1 ./model ./service ./controller ./router -count=1
```

## 生产数据核验

验收数据库中共有 `839,157` 条带 IP 的调用日志和 `26,107` 条 IP 汇总记录。

| 查询 | 结果 |
| --- | --- |
| 全部 IP 活动 | `4,580` 个地址，首屏约 `333ms` |
| 单 IP 详情 | 约 `62ms` |
| 单 IP 关联用户 | `7` 个 |
| 单 IP 安全时间线 | `148` 条 |

默认风险页为空时表示没有地址达到当前风险阈值，不表示数据未连接。风险排序以可解释的注册重合、调用重合和 `401` 异常活动为依据；普通上游错误不再被误判为安全风险。

## 浏览器证据

四角色生产验收中的 Root 安全审计路由通过，桌面截图位于：

`D:\cloveapi\restored-source\.verification-output\screenshots\root-security-audit-desktop-chromium.png`

浏览器确认：

- 风险指标只保留 `Unauthorized (401)`，普通 `Errors` 已从风险摘要移除。
- IP 列表和详情表头无重叠，IPv6 可完整换行或缩略显示。
- 风险记录可进入详情查看关联用户、注册事实、调用事实和安全时间线。
- 安全时间线可跳转到对应使用日志。
- IP 后提供小型外部查询按钮，不在站内伪造第三方画像结果。

## 覆盖的不变量

- `401` 是 HTTP 错误风险计数的唯一事实；`429`、`503` 等上游错误仍可记录，但不作为异常活动计分。
- IP 事件、日汇总和生命周期汇总在 SQLite、MySQL、PostgreSQL 上均可迁移、可幂等写入。
- 已泄露 Token 的指纹永久记录在撤销墓碑中；普通删除不保留原始自定义密钥，历史删除记录不会触发唯一索引冲突。
- 自然生成的 Token 保留标准前缀，自定义 Token 不强制添加 `sk-`。
- 兑换码并发创建遵守每用户活动数量上限；退款或没收由管理员明确选择，过期任务不会自动修改钱包。
- 管理员可以管理所有用户兑换码，用户可以领取自己创建的兑换码。

## 结论

安全与数据完整性相关实现已通过三数据库、并发、迁移、浏览器和生产数据查询验证。当前未发现会导致越权、重复结算、唯一索引冲突或错误风险告警的阻断问题。
