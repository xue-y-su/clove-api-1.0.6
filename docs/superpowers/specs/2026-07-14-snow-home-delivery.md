# Snow 首页交付说明

**日期：** 2026-07-14

## 目标

Snow 只作为网站根路径首页内容展示，不替换 New API 的 default/classic 前端壳，不接管登录、控制台、后台、文档、模型广场等应用路由。

## 已实现行为

- `HomePageMode=default` 时，首页保持 New API 原首页。
- `HomePageMode=snow` 时，default 前端首页在 `PublicLayout` 内通过 iframe 加载 `/snow-assets/`。
- classic 前端首页同样通过 iframe 加载 `/snow-assets/`，不替换 classic 应用路由。
- `/snow-assets` 和 `/snow-assets/` 返回 Snow 首页入口。
- Snow 静态资源只从 `/snow-assets/assets/...` 加载，不占用 New API 原有 `/assets/...`。
- default 首页仍使用 New API 原 `PublicHeader`；在 Snow iframe 内滚动时同步驱动原导航的 `scrolled` 状态，使导航保持 New API 首页原有的展开/居中收缩行为。
- Snow 原页的循环滚动叙事保留：持续向下滑回到雪屋开场附近时，外层 New API 导航会从收缩态恢复为初始展开态；继续滑出开场区域后再按原逻辑收缩。
- Snow 首页内容从视口顶部开始铺满，不再在固定导航下额外保留 `pt-16` 空白带；iframe 使用 block 布局避免底部/边缘基线空隙。
- New API 的 SPA history fallback 始终返回 default/classic index，不因为 Snow 模式返回 Snow index。
- `/api/status` 返回 `home_page_mode`，前端按该值决定是否嵌入 Snow。
- 系统设置页提供 `HomePageMode` 选项，只切换首页内容，不切换前端主题。

## 关键文件

- `main.go`
- `common/embed-file-system.go`
- `common/constants.go`
- `controller/misc.go`
- `controller/option.go`
- `model/option.go`
- `router/web-router.go`
- `router/web_router_test.go`
- `web/default/src/components/layout/components/public-header.tsx`
- `web/default/src/features/home/index.tsx`
- `web/default/src/features/home/lib/home-page-mode.ts`
- `web/default/src/features/home/lib/snow-header-scroll.ts`
- `web/default/src/features/home/lib/snow-header-scroll.test.ts`
- `web/classic/src/pages/Home/index.jsx`
- `web/classic/src/pages/Home/home-page-mode.js`
- `web/snow/dist/`

## 验证

已完成的针对性验证：

- `go test ./router -run 'TestSnow|TestShould' -count=1`
- `go test ./common -count=1`
- `bun run test -- src/features/home/lib/home-page-mode.test.ts`
- `bun run test -- src/features/home/lib/snow-header-scroll.test.ts src/features/home/lib/home-page-mode.test.ts src/features/home/lib/iframe-origin.test.ts`
- `bun run test -- src/pages/Home/home-page-mode.test.js`
- `bun run typecheck`
- `bun run build`
- `bunx oxlint -c .oxlintrc.json src/features/home/index.tsx src/features/home/lib/snow-header-scroll.ts src/features/home/lib/snow-header-scroll.test.ts src/components/layout/components/public-header.tsx`（仅保留 `public-header.tsx` 既有风格警告）
- 浏览器验收：
  - `/api/status` 返回 `home_page_mode: snow`
  - `/` 渲染 New API 顶部导航，并嵌入 `iframe[src="/snow-assets/"]`
  - `/` 首页主体 `main.top=0`、`iframe.top=0`、`body margin=0`，Snow 内容不再留顶部白边
  - `/` 初始导航为 New API 原展开态（`h-16 px-2`）
  - 在 Snow 区域滚动后，导航切换为 New API 原居中收缩态（`h-12 rounded-2xl`）
  - 在 Snow 区域继续滚过一个 Snow 虚拟滚动周期并回到开场附近后，导航恢复 New API 原展开态（`h-16 px-2`）
  - `/sign-in` 不嵌入 Snow iframe
  - `/dashboard` 未登录时进入 New API 登录页，不嵌入 Snow iframe
  - `/snow-assets/` 返回 Snow 入口，且入口引用 `/snow-assets/assets/...`
  - 验收过程中未发现本机站内 4xx 资源请求

验收截图：

- `D:\cloveapi\artifacts\verification\snow-acceptance\snow-home.png`
- `D:\cloveapi\artifacts\verification\snow-acceptance\snow-home-nav-expanded.png`
- `D:\cloveapi\artifacts\verification\snow-acceptance\snow-home-nav-scrolled.png`
- `D:\cloveapi\artifacts\verification\snow-acceptance\snow-home-no-top-gap.png`
- `D:\cloveapi\artifacts\verification\snow-acceptance\snow-home-no-top-gap-scrolled.png`
- `D:\cloveapi\artifacts\verification\snow-acceptance\snow-home-cycle-initial.png`
- `D:\cloveapi\artifacts\verification\snow-acceptance\snow-home-cycle-scrolled.png`
- `D:\cloveapi\artifacts\verification\snow-acceptance\snow-home-cycle-returned.png`

## 交付注意事项

- `web/snow/dist/**` 是当前交付依赖的静态资产，必须纳入提交或交付包；否则 `main.go` 的 `go:embed web/snow/dist` 会在干净环境构建失败。
- 当前 Snow 包没有源码构建链，后续若要长期维护，应补 Snow 源工程、构建命令或明确将 dist 作为供应商静态资产管理。
- 本地验收二进制与验收数据库位于 `D:\cloveapi\artifacts\verification\snow-acceptance\`，不属于源码仓库提交内容。
