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

## shadcn 静态样式

- 来源：`shadcn@4.18.0` npm 包的 `dist/tailwind.css`。
- 保留文件：`src/styles/vendor/shadcn.css`，仅翻译注释，样式规则未改动。
- 原文件 SHA-256：`bc7d83425702955b4cb67cb14ede9d603f9d912376d57a2d81d661094d2a782a`。
- 许可证：MIT License；版权：Copyright (c) 2023 shadcn。
- 完整许可文本：`src/styles/vendor/LICENSE.shadcn`，保留上游原文。

Canvas 只引用该静态样式，不调用 shadcn CLI、MCP 或源码生成工具。为移除
CLI 间接引入的高危 braces 依赖，构建改用固定的本地样式副本，不再安装
shadcn 包；未将该依赖移动到开发依赖或添加安全豁免。更新副本时须复核许可、
来源摘要和生产构建的样式差异。
