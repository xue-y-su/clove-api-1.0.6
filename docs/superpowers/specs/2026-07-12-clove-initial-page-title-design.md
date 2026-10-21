# Clove API 初始页面标题设计

日期：2026-07-12

## 目标

浏览器收到静态 HTML 后、JavaScript 和 `/api/status` 尚未加载时，标签页立即显示 `Clove API`，不再短暂显示 `New API`。

## 范围

- 将 Default 前端 `index.html` 的 `<title>` 和 `meta[name="title"]` 兜底值改为 `Clove API`。
- 将 Classic 前端 `index.html` 的 `<title>` 兜底值改为 `Clove API`。
- 保留现有动态品牌逻辑：缓存或 `/api/status` 返回 `system_name` 后，继续用管理员配置的站点名称覆盖静态兜底。
- 不替换关于页、协议说明、渠道类型、翻译词条等指向官方项目或产品概念的 `New API` 文本。

## 数据流

1. 浏览器解析服务端返回的静态 HTML，立即显示 `Clove API`。
2. 前端脚本启动后读取本地缓存的站点状态；存在 `system_name` 时覆盖标题。
3. `/api/status` 返回后，以最新 `system_name` 再次覆盖标题。

因此，`Clove API` 只作为加载前和状态不可用时的品牌兜底，不会阻止管理员自定义站点名。

## 验证

- 静态回归测试检查 Default 和 Classic 的源 HTML 均以 `Clove API` 为标题。
- 测试同时确认 Default 的 `meta[name="title"]` 为 `Clove API`。
- 保留并检查现有 `document.title = systemName` 动态覆盖路径。
- 重新构建两套前端和正式 Windows 程序。
- 在禁用缓存且延迟 `/api/status` 的浏览器条件下，检查响应 HTML 和首个可见标题均为 `Clove API`，最终标题仍可被系统设置覆盖。
