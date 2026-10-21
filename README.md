# Clove API

Clove API 是基于 New API `v1.0.0-rc.21` 集成的增强版 AI 网关，当前版本为 `v1.0.6`。这份源码已经把上游 RC21 合入，并保留 Clove 侧新增的合规、来源归属、余额查询、注册邀请控制、操练场附件、生产部署和安全加固能力。

仓库定位很直接：作为可私有化部署的多模型 API 网关，统一管理上游模型渠道、API Key、用户、额度、订阅、日志、合规确认和前端控制台。

## 当前主线

- 版本：`v1.0.6`
- 上游基线：New API `v1.0.0-rc.21`
- 主分支：`main`
- 默认服务端口：`3000`
- 许可证：AGPL-3.0，详见 `LICENSE`

## 主要能力

### 多模型网关

- OpenAI 兼容 Chat Completions、Responses、Realtime、图片和多模态请求。
- Claude Messages、Google Gemini、Rerank、AWS、Azure、DeepSeek、Qwen 等上游渠道适配。
- 支持渠道分组、模型映射、倍率、价格、上游同步、渠道健康检查和请求日志。
- 支持按用户、分组、模型、Token、订阅和钱包余额进行额度控制与计费。

### Clove 增强功能

- 公共 API Key 余额查询：新增 `/token-balance` 页面和 `POST /api/token-balance` 接口，无需登录即可用 API Key 查询当前可用额度、已用额度、受限额度、分组限制、模型限制和单次请求上限。查询过程会脱敏 Key，禁用缓存，并使用匿名请求限流。
- 合规声明系统：新增 `/api/compliance/*` 能力，支持单条或多条合规声明、版本号、内容哈希、勾选确认、短语确认、分段确认、管理员确认记录查看，以及 Token API 强制确认。
- 来源归属系统：新增 `/api/source-attribution/*` 能力，支持管理员配置来源选项、用户确认来源、可选描述、确认记录查看，以及 Token API 强制来源归属。
- 注册与邀请限制：支持开启注册邀请码必填，支持系统级默认邀请人数限制，也支持用户级自定义邀请限制。普通用户按成功邀请人数受限，管理员和 root 用户作为邀请人时免限。
- OAuth 与账号绑定：保留 GitHub、Discord、OIDC、LinuxDO、微信、Telegram 等登录能力，并增强用户侧账号绑定、解绑和自定义 OAuth provider 入口。
- 操练场附件：默认前端操练场支持图片、文本、PDF、Word、Excel 和 OpenDocument 表格等附件读取，文本内容会作为上下文拼入请求，图片会按 OpenAI 兼容多模态格式发送。附件大小、数量和本地存储都有保护限制。
- Clove 品牌兜底：Default 和 Classic 前端的静态 HTML 初始标题改为 `Clove API`。管理员配置的系统名称仍会在 `/api/status` 加载后覆盖标题。

### 管理端与前端

- Default 前端位于 `web/default`，使用 React、TanStack Router、TanStack Query、Rsbuild 和 Tailwind 体系。
- Classic 前端位于 `web/classic`，保留 Semi Design 旧版控制台和公共页面。
- 两套前端都覆盖余额查询、合规确认、来源归属、注册邀请、用户管理、使用日志和多语言文案。
- 后台设置增加站点合规、来源归属、注册限制、邮箱、OAuth、渠道、模型、价格、倍率和系统维护相关入口。
- 用户侧增加 API Key 余额查询、账号绑定、邀请限制提示、钱包/订阅展示和操练场附件交互。

### 安全与生产部署

- 提供生产用 `docker-compose.yml` 和 `.env.production.example`。
- 默认使用 PostgreSQL、Redis、只读容器文件系统、非特权运行、`no-new-privileges`、最小 Linux capabilities、健康检查和日志滚动。
- 生产环境要求显式配置 `SESSION_SECRET`、`FRONTEND_BASE_URL`、`SESSION_COOKIE_TRUSTED_URL`、`CORS_ALLOW_ORIGINS` 和 `TRUSTED_PROXIES`。
- 会话 Cookie、CORS、请求体大小、关键接口限流、响应缓存、TLS/SMTP 不安全跳过开关均有明确生产默认值。
- 日志侧增加敏感字段脱敏，避免 API Key 等凭据进入 SQL 或错误日志。

## 快速部署

### Docker Compose 生产部署

完整的密钥生成、发布前门禁、备份恢复、回滚和监控流程见 [`docs/production-deployment.md`](docs/production-deployment.md)。

```bash
cp .env.production.example .env.production
# 编辑 .env.production，设置所有 replace_with_* 和域名/代理配置

docker compose --env-file .env.production config
docker compose --env-file .env.production up -d --build
```

部署完成后访问：

```text
https://你的域名/
```

如果只是本机验证，默认应用端口为 `3000`。生产环境建议在可信反向代理后面终止 HTTPS，并保持应用只绑定内网或本机地址。

### 本地开发

后端直接运行：

```bash
go run main.go
```

启动 Docker 开发依赖：

```bash
make dev-api
```

启动 Default 前端：

```bash
make dev-web
```

启动 Classic 前端：

```bash
make dev-web-classic
```

构建两套前端：

```bash
make build-all-web
```

## 常用入口

| 入口 | 说明 |
| --- | --- |
| `/` | Default 前端控制台 |
| `/token-balance` | 公共 API Key 余额查询页 |
| `/api/status` | 站点状态与公共配置 |
| `/api/token-balance` | API Key 余额查询接口 |
| `/api/compliance/statement` | 合规声明读取 |
| `/api/compliance/confirm` | 合规声明确认 |
| `/api/source-attribution/status` | 来源归属状态 |
| `/api/source-attribution/confirm` | 来源归属确认 |
| `/api/registration/settings` | 注册与邀请限制设置 |

## 配置说明

生产部署至少需要设置：

- `POSTGRES_PASSWORD`
- `REDIS_PASSWORD`
- `SESSION_SECRET`
- `CRYPTO_SECRET`
- `TOKEN_HMAC_KEY`
- `TOKEN_ENCRYPTION_KEYS`
- `FRONTEND_BASE_URL`
- `SESSION_COOKIE_TRUSTED_URL`
- `CORS_ALLOW_ORIGINS`
- `TRUSTED_PROXIES`
- `CLOVE_API_IMAGE_TAG`

Compose 默认把应用数据库连接池限制为 80 个打开连接和 20 个空闲连接，为 PostgreSQL 健康检查、迁移、备份与人工运维保留余量。多节点部署时应按 PostgreSQL `max_connections` 重新分配每个节点的上限。

示例见 `.env.production.example`。不要提交填好真实密钥的 `.env.production`。

## 源码结构

| 路径 | 说明 |
| --- | --- |
| `common` | 通用配置、日志、安全、请求限制、工具函数 |
| `controller` | HTTP 控制器，包含合规、来源归属、注册、余额查询等接口 |
| `middleware` | 鉴权、限流、CORS、合规/来源归属拦截、请求体限制 |
| `model` | 数据模型、用户、Token、日志、邀请限制和确认记录 |
| `router` | API、Web、Token 余额等路由注册 |
| `service` | 计费、额度、渠道、文件、订阅和余额快照业务逻辑 |
| `setting` | 系统设置、合规设置、来源归属设置、注册设置 |
| `web/default` | 新版 Default 前端 |
| `web/classic` | Classic 前端 |
| `electron` | 桌面打包入口与安全配置 |
| `docs/superpowers` | 设计记录与实现计划 |

## 验证重点

本版本包含 Go 单元测试、路由测试、前端工具测试、i18n 检查和构建配置。修改后建议至少按影响范围运行：

```bash
go test -p 1 ./... -count=1
go vet ./...
```

Default 前端：

```bash
cd web/default
bun run format:check
bun run lint
bun run typecheck
bun run test
bun run build
```

Classic 前端：

```bash
cd web/classic
bun run lint
bun run eslint
bun run test
bun run build
```

## 合规提示

本项目仅用于合法授权的 AI API 网关、组织级认证、多模型管理、用量统计、成本核算和私有化部署场景。运营方需要自行确保上游 API Key、模型服务、支付、用户数据、内容安全、备案、许可、日志留存、税务和当地监管要求均已合规。
