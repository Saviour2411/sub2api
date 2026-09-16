# 压缩流响应体收尾修复

日期：2026-09-16。基于远端主线 `132bbc90f` 开始修复。本记录不代表生产部署或客户历史请求的最终归因。

## 问题

流式响应已经提供最终用量与完整终止事件时，调用方应能关闭响应体，不必继续等待上游 HTTP EOF。

原 `decompressedBody.Close` 先关闭解压器，再关闭网络响应体。zstd 解压器的关闭可能等待内部读取结束；网络流仍然打开时，会导致解压关闭与网络关闭互相等待。`ForwardResult.Duration` 在延迟关闭前计算，因此可能出现用量耗时很短、整个请求生命周期却持续数分钟的差异。

真实 HTTP/1.1 和 HTTP/2 本地服务均可复现：发送完整终止事件并刷新压缩流后保持连接打开，修复前 zstd 关闭超过测试期限，主动关闭原始网络响应体后解除阻塞。

## 修改

- 先关闭原始网络响应体，解除阻塞中的网络读取。
- 用互斥锁协调解压读取与解压器关闭，避免解码器在仍被读取时被释放。
- 用 `sync.Once` 保证并发、重复关闭只释放一次，并向所有调用者保留原始网络关闭错误。
- 网络关闭返回错误时，仍执行解压器资源释放；保持原有忽略解压器关闭错误的返回语义。

不修改终止事件判定，不伪造结束帧，不增加模型重试，不修改超时和计费规则。修复位于通用上游解压响应体封装，不仅适用于 Claude。

## 验证

`backend/internal/repository/decompress_response_stream_close_test.go` 覆盖：

- HTTP/1.1、HTTP/2，identity、gzip、br、deflate、zstd。
- 完整最终用量与终止事件交付后，上游保持连接打开，关闭仍及时返回。
- 读取下一段数据时并发关闭，读取协程可以退出。
- 两个并发关闭调用只释放一次网络响应体和解压器。
- 网络关闭失败时继续清理解压器，并保留网络错误。

现有解压完整正文、错误压缩数据及用量解析测试继续执行。CI 的 `stream-race` 工作加入上述回归测试和 repository 包，持续检查并发读取/关闭。

主要验证命令（使用项目现有 Go 1.27 工具链与项目内缓存）：

```sh
go test ./internal/repository -run '^TestDecompressResponseBody|^TestDecompressedBodyClose' -count=1
go test -race -tags=unit ./internal/repository -run '^TestDecompressResponseBody|^TestDecompressedBodyClose' -count=10
go test -tags=unit ./...
```

## 上线边界

这是已复现的代码缺陷修复，不证明每一条历史 `client_gone` 都使用了 zstd；历史请求缺少压缩响应头与收尾分段日志时，应保留归因不确定性。

本次只提交代码并验证 CI，不创建发布标签、不重启服务、不修改生产配置。上线后仍需对照完整流终止时间、请求完成时间及下游取消情况验收；真实上游空闲超时和缺结束事件仍应正常报错。
