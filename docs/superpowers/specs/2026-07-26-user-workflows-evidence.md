# 用户工作流验收证据

**日期：** 2026-07-26  
**最终验收地址：** `http://127.0.0.1:18104`  
**最终运行镜像：** `clove-api:qa-20260726-final3-dockerfile`  
**后端提交：** `9ba61b78`  
**前端提交：** `a51b726d`

## 覆盖范围

- 邀请码生成、拒绝或用尽后的再次申请、邀请链接与注册预填。
- 邀请奖励金额汇总、用户提现申请和管理员审批。
- 单条公告弹窗、HTML 内容、长文本、图片上传与点击放大。
- 个人资料弹窗编辑、昵称冷却提示、头像上传和全局头像同步。
- 第三方登录解绑后保持当前登录态。
- 用户兑换码创建、查看、复制、自领和管理员治理。

## 自动化结果

| 验证层 | 结果 |
| --- | --- |
| Go 全仓 | `go test -p 1 ./... -count=1` 全部通过 |
| MySQL 8.4 | `model/service/controller/router` 全部通过 |
| PostgreSQL 15 | `model/service/controller/router` 全部通过 |
| Default | `416/416`，类型检查、lint、格式化和生产构建通过 |
| Classic | `251/251`，eslint、格式化和生产构建通过 |
| 四角色浏览器验收 | `4/4` 通过 |

## 邀请码与邀请奖励

- 邀请码恢复为短随机码，不再使用冗长标识符。
- 状态为拒绝、已取消或已用尽时，达到冷却条件后可以再次申请；过期的红色拒绝提示不会继续占位。
- 用户同时看到邀请码和邀请链接，注册链接可自动预填并校验邀请码有效性。
- 奖励卡展示已邀请金额、应得金额和可申请金额。
- 用户提交的是聚合提现申请，审批前不能领取；管理员在“邀请码审批”页面审核后再结算。
- 申请、审批、拒绝和结算均有后端测试保护，重复操作保持幂等。

## 公告

- 是否弹窗由每条公告单独决定，不使用一个全局弹窗开关替代单条策略。
- 未登录用户不会弹出需要确认的公告。
- 登录后公告确认与合规声明使用串行门禁，不再互相抢占或产生“公告确认失败”。
- 公告正文不再受 500 字符限制，系统公告支持受控 HTML 展示。
- 管理员可直接上传公告图片到服务器数据卷；图片可点击放大查看。
- 通知与时间线切换保留已加载状态，不再闪烁。
- 媒体目录 `/data/announcement-media` 的属主为 `10001:10001`、权限为 `750`，最终容器用户写入验证通过。

## 个人资料与会话

- 个人资料在顶部身份卡右侧通过小按钮打开编辑弹窗，不再额外占用整块页面空间。
- 摘要只显示有值的信息，例如 `@clove · Admin`；无邀请人时不显示邀请人项。
- 昵称处于 7 天冷却期时，界面显示剩余等待原因，而不是只把按钮置灰。
- 头像由用户上传后保存 URL；保存成功后桌面右上角和移动端头像立即同步。
- 第三方渠道解绑只更新绑定状态，不清空当前会话。

## 用户兑换码

- 用户创建额度按标准钱包额度换算，不再误用旧积分精度。
- 创建失败返回具体字段和边界原因，大额输入不会只显示笼统“参数无效”。
- 用户可在创建按钮旁打开“我的兑换码”弹窗，查看码值、领取记录、使用情况和资金状态。
- 复制按钮使用浏览器剪贴板并提供成功或失败反馈。
- 用户允许领取自己创建的兑换码。
- 管理员可以查看详情、禁用、删除、退款、没收和批量处置所有用户兑换码。

## 浏览器证据

最终四角色生产验收：

```powershell
$env:E2E_BASE_URL='http://127.0.0.1:18104'
$env:E2E_ROOT_STORAGE_STATE='D:\cloveapi\.verification-output\auth\root.json'
$env:E2E_ADMIN_STORAGE_STATE='D:\cloveapi\.verification-output\auth\admin.json'
$env:E2E_USER_STORAGE_STATE='D:\cloveapi\.verification-output\auth\user.json'
bunx playwright test e2e/production-acceptance.spec.ts --project=chromium --reporter=line
```

结果：`4 passed`，总耗时 `27.6s`。

截图：

- `D:\cloveapi\restored-source\.verification-output\screenshots\root-announcements-desktop-chromium.png`
- `D:\cloveapi\restored-source\.verification-output\screenshots\administrator-user-redemptions-tablet-chromium.png`
- `D:\cloveapi\restored-source\.verification-output\screenshots\normal-user-wallet-mobile-chromium.png`
- `D:\cloveapi\restored-source\.verification-output\screenshots\normal-user-avatar-synced-mobile-chromium.png`

## 结论

邀请码、邀请奖励、公告、个人资料、会话和用户兑换码的主要角色流程已在源码测试、三数据库和最终标准镜像浏览器验收中通过。当前未发现阻断交付的工作流回归。
