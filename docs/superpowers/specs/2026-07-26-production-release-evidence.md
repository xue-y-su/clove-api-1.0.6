# 生产发布验收证据

**日期：** 2026-07-26  
**分支：** `recovery/production-restore-20260719`  
**最终镜像：** `clove-api:qa-20260726-final3-dockerfile`  
**镜像摘要：** `sha256:2dd42202aa302384caf1af266b1e88cfa73a3f05d9b1f7d42792f7d2d10e6ca6`

## 实现提交

| 提交 | 内容 |
| --- | --- |
| `9ba61b78` | 后端安全、Token、邀请码、兑换码和错误契约 |
| `a51b726d` | Default 与 Classic 工作流、IP 审计、密钥治理、性能和国际化 |
| `585d7201` | Docker 构建参数、bundle 审计和 Playwright 生产门禁 |

## 源码门禁

| 门禁 | 结果 |
| --- | --- |
| Go 全仓 | `go test -p 1 ./... -count=1` 全部通过，最终复验 `362.1s` |
| SQLite | `model/service/controller/router` 全部通过 |
| MySQL 8.4 | `model/service/controller/router` 全部通过 |
| PostgreSQL 15 | `model/service/controller/router` 全部通过 |
| Default 测试 | `416/416` 通过 |
| Default format/lint/typecheck | 全部通过 |
| Default production build | 通过 |
| Classic 测试 | `251/251` 通过 |
| Classic Prettier/lint/eslint | `0 error`，6 条不阻断的版权头规则 warning |
| Classic production build | 通过 |
| Git whitespace | `git diff --check` 通过 |

## Docker 构建

标准仓库 `Dockerfile` 从当前源码完成构建。`GOPROXY` 支持构建参数覆盖，默认值仍为：

```text
https://proxy.golang.org,direct
```

本地构建使用：

```text
https://goproxy.cn,direct
```

该覆盖只影响依赖下载，不改变运行时配置。最终镜像大小约 `63.7 MB`，Docker 列表显示占用约 `233 MB`。

## 运行时复验

应用容器：`cloveapi-current-source-mysql-qa`  
访问地址：`http://127.0.0.1:18104`  
重启策略：`unless-stopped`  
运行用户：`10001:10001`

| 检查 | 结果 |
| --- | --- |
| `/api/status` | `success=true`，版本 `v1.0.0-rc.21.clove-enhanced` |
| `/healthcheck` | HTTP 200 |
| Root 会话 | ID `1`，用户 `clove`，有效 |
| 管理员会话 | ID `9400039`，用户 `qaadmin26`，有效 |
| 普通用户会话 | ID `9400040`，用户 `qauser26`，有效 |
| 公告媒体目录 | UID `10001` 实际创建并删除测试文件成功 |
| 数据卷 | `cloveapi-current-source-mysql-qa-data` 保留 |
| 网络 | 隔离数据库网络和 `bridge` 均保留 |

## 浏览器门禁

四角色生产验收：`4/4`，耗时 `27.6s`。

```powershell
bunx playwright test e2e/production-acceptance.spec.ts --project=chromium --reporter=line
```

游乐场可靠性：`9/9`，耗时 `21.8s`。

```powershell
bunx playwright test e2e/playground-reliability.spec.ts --project=chromium --reporter=line
```

浏览器测试会失败于任何页面异常、控制台错误、HTTP 500、门禁长时间旋转、横向溢出或关键交互不可用。

## 数据与回滚

- 应用数据使用命名卷，不随镜像切换删除。
- 镜像切换脚本在状态检查失败时自动恢复预编译回滚容器。
- 标准镜像运行和浏览器验收通过后，已删除两个已停止的 QA 回滚容器、临时预编译镜像和旧 QA 镜像。
- 本地 Playwright Root、管理员和普通用户会话状态文件已删除，避免遗留可复用登录 Cookie。
- 不执行 `docker volume prune`，避免删除未知业务卷。

## 磁盘清理

| 指标 | 清理前 | 清理后 |
| --- | ---: | ---: |
| C 盘可用空间 | 16.32 GB | 16.32 GB |
| D 盘可用空间 | 1.60 GB | 1.60 GB |
| Docker Build Cache | 9.253 GB | 0 B |
| Docker 容器 | 12 | 10 |
| Docker 镜像 | 25 | 23 |

`docker buildx prune --all --force` 已回收全部 BuildKit 缓存。Docker Desktop 的 VHDX 未离线压缩，因此宿主 D 盘没有立即增长；为保持最终应用和验收数据库在线，本次不停止 Docker，也不执行 `Optimize-VHD`。应用、MySQL、Redis、最终镜像和命名数据卷均保留。

## 发布结论

当前源码已通过后端、三数据库、双前端、Docker、运行时和浏览器验收。完成最终 Git 提交、推送和非业务缓存清理后，可作为本轮生产候选版本交付。
