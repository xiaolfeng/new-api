# bamboo 知识库

## 概述

对话中继的协议归一化内核。用 `bamboo-messages` 把入口协议解析成与供应商无关的中间表示，再经 provider 打上游、用 codec 序列化回入口格式，用来替代各 handler 里 `Convert → DoRequest → DoResponse` 的原生三段式。

## 目录结构

```text
relay/bamboo/
├── bridge.go              # ChatRelay：入口格式映射 → codec 解析 → provider 调用 → 序列化回写
├── provider_factory.go    # 按 info.ApiType 构造 bamboo provider
├── host_relay.go          # 宿主侧中继衔接（与 new-api RelayInfo / 计费对接）
├── client_stream.go       # 客户端流式写出
├── codec_map.go           # types.RelayFormat ↔ bamboo codec FormatType
├── errors.go              # ErrUnsupportedProvider 等；调用方据此 fallback 原生链路
├── timing.go / usage.go   # 耗时与 usage 归集
├── visible_box.go         # 调试可见框
├── hosttool/              # 宿主工具循环（search / fetch / MCP / HTML→MD 等）
└── imagerec/              # 图像识别改写（与 image_recognize_hop 衔接）
```

## 导航指南

| 任务 | 位置 | 说明 |
|------|------|------|
| 理解 bamboo 主链路 | `bridge.go` `ChatRelay` | 入口格式 → `codec.ParseRequest` → `provider.Chat/Complete` → 序列化回入口协议 |
| 新增/对接一种入口协议 | `codec_map.go` | 在 `relayFormatToCodec` 登记 `types.RelayFormat` |
| 新增/对接一种上游供应商 | `provider_factory.go` | 按 `info.ApiType` 返回 bamboo provider；不支持则返回 `ErrUnsupportedProvider` |
| 宿主工具（搜索/抓取/MCP） | `hosttool/` | `registry.go` 注册，`loop.go` / `execute.go` 执行 |
| 图像识别改写 | `imagerec/rewrite.go` | 与 `relay/image_recognize_hop.go` 配合 |
| 流式写出失败 | `client_stream.go` | 客户端 SSE/event 写出 |

## 约定

- **失败必须可回退**：`ChatRelay` 在供应商不受支持时返回 `ErrUnsupportedProvider`，调用方回退到 `relay/channel` 原生 adaptor 链路。不要把「未实现」伪装成成功。
- **调用方只传入口格式**：外部传入 `types.RelayFormat` 与原始 request body；格式映射、codec 选择留在 bridge 内部。
- **上游由 ApiType 决定**：`info.ApiType`（经 ChannelMeta 嵌入）决定用哪个 bamboo provider，不要在 bridge 里再按渠道名硬编码分支。
- **codec 子包必须空白 import**：`bridge.go` 顶部的 `_ "…/codec/anthropic"` 等会触发 `init()` 注册。删掉会导致 `codec.Get` 返回 nil。
- **响应体有硬上限**：`info.ResponseBody` 与 debug 流式帧分别截断到 50000 字节，避免撑爆日志库。改上限要同时评估 DB 与排查需求。
- **JSON 走 `common.*`**：本目录属于 root module，marshal/unmarshal 用 `common.Marshal` / `common.Unmarshal`，不要直接调 `encoding/json`。
- **计费仍走宿主**：bamboo 只做协议与上游 IO；预扣/结算仍由 `service` 的 `BillingSession` 负责，usage 经 `usage.go` 交回 handler。

## 反模式

- ❌ 在 bamboo 里写用户鉴权或直接改配额——这里只做协议归一化与上游 IO。
- ❌ 吞掉 `ErrUnsupportedProvider`——调用方依赖它回退原生 adaptor。
- ❌ 按渠道目录互相调用 `relay/channel/<provider>` 做转换——bamboo 路径不应再走 adaptor 的 `Convert*Request`。
- ❌ 去掉 codec 空白 import——注册表会空。
- ❌ 把超长响应体完整写入 `info.ResponseBody`——会撑爆日志表。

## 调试路径

1. 请求走了原生 adaptor 而不是 bamboo → `ChatRelay` 是否返回 `ErrUnsupportedProvider`；检查 `provider_factory.go` 是否覆盖该 `ApiType`，以及 handler 的 fallback 分支。
2. 入口协议解析失败 → `codec_map.go` 的 `RelayFormat` 映射，再看 `codec.ParseRequest` 对应 codec 子包。
3. 上游打到错误供应商 → `provider_factory.go` 与 `info.ApiType`。
4. 流式中断 / 客户端收不到增量 → `client_stream.go` 与 `maxBambooDebugStreamLen` 截断。
5. 宿主工具没执行 → `hosttool/registry.go` 是否注册，`loop.go` / `execute.go` 的工具循环条件。
6. usage 为 0 或计费偏差 → `usage.go` 归集是否写回 handler，再转到 `service` 计费会话。
