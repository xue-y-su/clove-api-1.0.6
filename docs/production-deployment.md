# 生产部署与运维

本文档适用于仓库根目录的 `docker-compose.yml`，默认部署一个 New API 应用节点、PostgreSQL 16 和 Redis 7。生产流量必须先经过 HTTPS 反向代理，应用端口默认只绑定宿主机回环地址。

生产镜像的最终运行层是 `scratch`，只包含静态 `new-api`、静态 `/healthcheck`、CA 证书、时区数据和运行目录，不包含 shell、包管理器或 Debian 包数据库。Compose 使用 `/healthcheck` 的 exec 健康检查验证 `/api/status` 返回 HTTP 200 且 `success=true`。

## 发布前检查

### 主机与工具

- Docker Engine 24+，Docker Compose v2.20+。
- 至少 4 CPU、8 GiB 内存和独立持久化磁盘；实际容量按日志、附件与数据库增长调整。
- 域名、TLS 证书、反向代理和防火墙已配置。
- PostgreSQL 备份保存到 Docker volume 之外，并有异机或对象存储副本。

### 生成密钥

每个值独立生成，不要复用。下面命令分别生成 URL 安全密码、64 字符会话密钥和 32 字节 Base64 Token 密钥：

```bash
openssl rand -hex 24
openssl rand -hex 32
openssl rand -base64 32
```

复制 `.env.production.example` 为 `.env.production` 后替换所有 `replace_with_*`。以下值必须长期稳定并纳入加密备份：

- `SESSION_SECRET`
- `CRYPTO_SECRET`
- `TOKEN_HMAC_KEY`
- `TOKEN_ENCRYPTION_KEYS`
- PostgreSQL 与 Redis 密码

所有应用节点必须使用相同的会话和 Token 密钥。轮换 `TOKEN_ENCRYPTION_KEYS` 时，把新版本放在第一项用于写入，保留仍被历史密文引用的旧版本；确认回填完成前不得删除旧版本。

### 配置校验

```bash
docker compose --env-file .env.production config --quiet
docker compose --env-file .env.production build new-api
```

发布门禁还必须扫描最终镜像；不要只扫描构建阶段镜像：

```bash
trivy image --scanners vuln --severity HIGH,CRITICAL --exit-code 1 "$CLOVE_API_IMAGE_TAG"
trivy image --scanners vuln --format cyclonedx --output sbom.cdx.json "$CLOVE_API_IMAGE_TAG"
```

发布前执行仓库门禁：

```bash
go test -p 1 ./... -count=1
go vet ./...

cd web/default
bun run format:check
bun run lint
bun run typecheck
bun run test
bun run build

cd ../classic
bun run lint
bun run eslint
bun run test
bun run build
```

确认反向代理实际来源包含在 `TRUSTED_PROXIES` 中，但不得使用 `0.0.0.0/0` 或 `::/0`。`FRONTEND_BASE_URL`、`SESSION_COOKIE_TRUSTED_URL` 与 `CORS_ALLOW_ORIGINS` 必须使用实际 HTTPS 域名。

## 首次部署

```bash
docker compose --env-file .env.production up -d --build --wait
docker compose --env-file .env.production ps
curl -fsS http://127.0.0.1:3000/api/status
```

只有 `/api/status` 返回 `success: true`、PostgreSQL 与 Redis 均为 healthy 后，才允许反向代理接入流量。应用启动时会执行幂等数据库迁移；不要同时启动多个尚未验证的新版本节点。

反向代理至少需要：

- 保留原始 `Host`，设置 `X-Forwarded-Proto=https` 和真实客户端 IP。
- 为普通请求设置合理超时，为流式接口关闭代理缓冲并设置更长读取超时。
- 限制管理后台来源，禁止直接暴露 PostgreSQL、Redis 和 pprof。
- 上传大小不得超过应用的 `MAX_REQUEST_BODY_MB` 与接口级限制。

## 备份与恢复

### 必须备份的内容

1. PostgreSQL 全库逻辑备份。
2. 加密保存的 `.env.production` 或等价密钥管理系统记录。
3. 当前 `VERSION`、`CLOVE_API_IMAGE_TAG` 和镜像摘要。
4. 需要长期保存的 `/app/logs` 外部归档。

Redis 仅用于缓存和短期协调，不应作为业务数据的唯一备份来源。禁止把正在写入的 PostgreSQL volume 目录直接复制为唯一备份。

### PostgreSQL 在线备份

```bash
umask 077
mkdir -p backups
backup="backups/new_api_$(date -u +%Y%m%dT%H%M%SZ).dump"
docker compose --env-file .env.production exec -T postgres \
  pg_dump -U newapi -d new_api --format=custom --no-owner --no-acl > "$backup"
docker compose --env-file .env.production exec -T postgres \
  pg_restore --list < "$backup" > /dev/null
sha256sum "$backup" > "$backup.sha256"
```

定期把备份和校验文件复制到异机存储，并实际演练恢复。仅生成备份但从未恢复验证，不算可用备份。

### 恢复演练或灾难恢复

以下操作会替换目标数据库，必须确认连接的是正确环境：

```bash
backup=backups/new_api_YYYYMMDDTHHMMSSZ.dump
sha256sum -c "$backup.sha256"
docker compose --env-file .env.production stop new-api
docker compose --env-file .env.production exec -T postgres \
  dropdb -U newapi --if-exists --force new_api
docker compose --env-file .env.production exec -T postgres \
  createdb -U newapi -O newapi new_api
docker compose --env-file .env.production exec -T postgres \
  pg_restore -U newapi -d new_api --no-owner --no-acl --exit-on-error < "$backup"
docker compose --env-file .env.production up -d --wait new-api
curl -fsS http://127.0.0.1:3000/api/status
```

恢复后验证管理员登录、Token 查询、一次低额度测试请求、钱包流水、兑换码领取/取消和审计 outbox。密钥丢失时，数据库中的 Token 密文无法恢复，因此密钥备份与数据库备份必须作为同一恢复点管理。

## 发布与回滚

### 发布

1. 为新版本设置唯一且不可变的 `CLOVE_API_IMAGE_TAG`。
2. 完成数据库备份并记录当前镜像摘要。
3. 构建镜像，不覆盖旧标签。
4. 更新应用节点并等待健康检查。
5. 验证核心读写与审计指标，再结束观察窗口。

同时保存镜像摘要、SBOM、HIGH/CRITICAL 扫描结果和二进制校验和；这些文件应与发布标签关联保存。

```bash
docker compose --env-file .env.production build new-api
docker compose --env-file .env.production up -d --wait new-api
docker compose --env-file .env.production logs --since 10m new-api
```

### 仅回滚应用镜像

只有旧版本确认兼容当前数据库结构时，才能只回滚二进制。确保旧镜像仍在本机或镜像仓库中：

```bash
previous_release_tag='v1.0.0-previous'
export CLOVE_API_IMAGE_TAG="$previous_release_tag"
docker compose --env-file .env.production up -d --no-build --wait new-api
```

### 回滚数据库与应用

如果新版本执行了旧版本不兼容的数据迁移，停止应用，按“恢复演练或灾难恢复”恢复发布前备份，再启动对应旧镜像。不要使用 `docker compose down -v` 回滚；该命令会删除数据库、Redis 和应用 volume。

发生失败时保留故障镜像、应用日志、PostgreSQL 日志和备份校验结果，先恢复服务，再进行根因分析。

## 监控与告警

### 健康与可用性

- 每 30 秒请求 `/api/status`；连续 2 分钟失败应告警。
- 监控 `docker compose ps` 中的 health、重启次数和退出码。
- 对真实上游配置一个低成本合成请求，区分“进程健康”和“网关可用”。
- 监控 TLS 到期时间、反向代理 4xx/5xx、请求延迟和流式中断率。

### 应用与审计

管理员可通过 `/api/system-info/instances` 或系统信息页面查看节点资源与以下结构：

- `extra.audit_outbox.pending_count`
- `extra.audit_outbox.oldest_pending_age_seconds`
- `extra.audit_outbox.failure_count`
- `extra.ip_security_projection`

建议在 outbox 最老待投递事件超过 300 秒、失败数大于 0、IP 安全投影进入 failed 状态或长期不推进时告警。日志中重点匹配：

```text
audit outbox delivery failed
audit outbox delivery completed with failures
IP security projection backfill failed
IP security projection retry failed
quota saturation
```

### PostgreSQL、Redis 与主机

- PostgreSQL 活跃连接接近 `SQL_MAX_OPEN_CONNS=80` 或服务端 `max_connections` 的 80% 时告警。
- 监控慢查询、锁等待、事务回滚、数据库大小和备份时长。
- Redis 监控连接数、内存、AOF 错误和驱逐；业务正确性不能依赖缓存永不丢失。
- 磁盘使用率 80% 预警、90% 严重告警；同时监控 inode、内存、负载和容器 OOM。

PostgreSQL 连接检查示例：

```bash
docker compose --env-file .env.production exec -T postgres psql -U newapi -d new_api -c \
  "SELECT count(*) AS used, current_setting('max_connections') AS maximum FROM pg_stat_activity;"
```

## 多节点部署

- 每个节点设置唯一 `NODE_NAME`。
- 只有一个节点设置 `NODE_TYPE=master`；其他节点使用 `slave`，避免重复后台任务。
- 所有节点共享 PostgreSQL、Redis、`SESSION_SECRET`、`CRYPTO_SECRET`、`TOKEN_HMAC_KEY` 和完整 `TOKEN_ENCRYPTION_KEYS`。
- 先升级一个节点并观察，再滚动升级其余节点；迁移阶段禁止并发启动多个全新版本。

## systemd 部署

`new-api.service` 适用于不使用容器的 Linux 主机。安装前创建专用系统用户，把二进制放在 `/opt/new-api/new-api`，生产环境变量放在 `/etc/new-api/new-api.env`：

```bash
sudo useradd --system --home /var/lib/new-api --shell /usr/sbin/nologin new-api
sudo install -d -o root -g root -m 0755 /opt/new-api /etc/new-api
sudo install -o root -g root -m 0755 new-api /opt/new-api/new-api
sudo install -o root -g root -m 0600 .env.production /etc/new-api/new-api.env
sudo install -o root -g root -m 0644 new-api.service /etc/systemd/system/new-api.service
sudo systemctl daemon-reload
sudo systemctl enable --now new-api
sudo systemctl status new-api
```

systemd 环境文件必须直接提供 `SQL_DSN` 和 `REDIS_CONN_STRING`；Compose 专用的 `POSTGRES_PASSWORD`、`REDIS_PASSWORD` 不会自动拼接连接串。
