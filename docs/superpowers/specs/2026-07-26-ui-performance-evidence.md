# UI 与性能验收证据

**日期：** 2026-07-26  
**前端：** Default React 19 / Rsbuild，Classic React 18 / Vite  
**最终验收地址：** `http://127.0.0.1:18104`  
**实现提交：** `a51b726d`  
**发布门禁提交：** `585d7201`

## 静态门禁

### Default

- 格式化检查通过。
- lint 通过。
- TypeScript 类型检查通过。
- 源码审计通过。
- 单元与组件测试 `416/416` 通过。
- production build 通过。

### Classic

- Prettier 检查通过。
- lint 与 eslint 通过；`0 error`，保留 6 条不阻断的版权头规则 warning。
- 单元与组件测试 `251/251` 通过。
- production build 通过。

## 首屏产物

生产 bundle 扫描结果：

| 指标 | 数值 |
| --- | ---: |
| 首屏脚本数量 | 5 |
| 原始体积 | 2,291,512 bytes |
| gzip 体积 | 650,098 bytes |

`scripts/verify-production-bundles.mjs` 会检查生产产物不存在 TanStack Devtools，并验证首屏脚本数量和体积预算。Lobe 图标改为按需加载，不再把整套图标包塞进首屏。

## 浏览器路由性能

最终标准镜像上的 Root 桌面验收指标：

| 路由 | 门禁完成 | 导航耗时 | 传输脚本 |
| --- | ---: | ---: | ---: |
| `/profile` | 1495ms | 331ms | 2,817,517 bytes |
| `/security-audit` | 409ms | 159ms | 10,114 bytes |
| `/user-redemptions` | 398ms | 123ms | 8,504 bytes |
| `/invitations` | 408ms | 64ms | 5,465 bytes |
| `/invitation-rewards` | 288ms | 57ms | 3,894 bytes |
| `/system-settings/content/announcements` | 282ms | 56ms | 2,522 bytes |
| `/playground` | 584ms | 47ms | 571,810 bytes |
| `/keys` | 472ms | 67ms | 15,598 bytes |
| `/wallet` | 753ms | 127ms | 24,043 bytes |

匿名首页：门禁完成 `647ms`，导航 `264ms`，传输脚本 `839,164 bytes`。

这些数值来自 Playwright 浏览器资源时间线，同一次验收同时监控浏览器控制台错误、页面异常和 HTTP 500 响应。

## 布局验收

- Root 桌面宽度：个人资料、安全审计、用户兑换码、邀请码、邀请奖励、公告、游乐场、密钥和钱包均通过。
- 管理员平板宽度：用户兑换码治理通过。
- 普通用户移动宽度：钱包与头像同步通过。
- 游乐场桌面 `1440x900` 和移动端 `390x844` 均无横向溢出。
- 长错误信息、IPv6、状态标签和下拉选项使用换行、截断与详情展示，避免挤压表格。

截图保存在：

`D:\cloveapi\restored-source\.verification-output\screenshots`

## 结论

双前端静态门禁和构建全部通过，生产产物不存在开发工具残留；路由拆包、图标按需加载、可恢复门禁和响应式布局已在最终标准镜像中验证。
