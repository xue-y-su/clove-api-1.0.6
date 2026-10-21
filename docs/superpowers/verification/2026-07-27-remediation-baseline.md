# 2026-07-27 修复基线

## 源码

- 仓库：`D:/cloveapi/restored-source`
- 设计提交：`443ce4c8`
- 实施计划提交：`5680b58f`
- 本轮开始时工作区：干净

## 运行环境

- 验收地址：`http://127.0.0.1:18104`
- 应用容器：`cloveapi-current-source-mysql-qa`
- 开始时镜像：`clove-api:qa-20260726-invite-reward`
- 数据卷：`cloveapi-current-source-mysql-qa-data`
- 数据库容器：`cloveapi-remote-copy-mysql`
- Redis 容器：`cloveapi-remote-copy-redis`
- 隔离网络：`cloveapi-remote-copy-isolated-net`

开始时镜像没有当前提交与两套前端源码指纹标签，因此浏览器现象不能直接证明当前工作区源码仍有同一缺陷。后续每次最终验收前必须运行 `scripts/verify-runtime-source.ps1`。

## 磁盘

| 盘符 | 已用 | 可用 |
| --- | ---: | ---: |
| C | 165.55 GB | 14.55 GB |
| D | 271.88 GB | 24.65 GB |

构建前后复查可用空间。测试证据和源码继续写入 D 盘；不删除现有 MySQL、Redis、应用数据卷或其他项目容器。
