# 第三方软件声明

## Infinite Canvas

- 上游项目：[basketikun/infinite-canvas](https://github.com/basketikun/infinite-canvas)
- 固定来源提交：`b66936d891b82c2b51c1ed05e1a6eae3e31d4ca3`
- 许可证：MIT License
- 上游版权：Copyright (c) 2026 basketikun
- 完整许可文本：[`LICENSE`](./LICENSE)

本目录保留了上游 React 无限画布的核心实现，并为 Sub2API 做了裁剪和适配。主要变化包括：

- 路由基址调整为 `/canvas-app/`，由 Sub2API Vue 主站以同源 iframe 嵌入；
- 接入 Sub2API 用户、分组、模型、余额和 API Key 体系；
- 删除用户自填 API Key、WebDAV、本地 Agent/MCP、远程插件、远程提示词脚本、赞助推广和 GitHub 入口；
- 将提示词来源限制为内置静态内容和浏览器本地自定义内容；
- 按 Sub2API 用户 ID 隔离 IndexedDB 数据，并将运行时 API Key 限制在内存中；
- 新增受操作白名单和用户确认约束的站内画布助手。

除上述修改外，原项目的 MIT 许可权利与免责声明保持不变。
