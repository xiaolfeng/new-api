# relay 知识库

## 概述

AI 中继核心。按请求模式（chat / responses / claude / gemini / embedding / image / audio / rerank / 异步任务）选择 handler，再经 adaptor 或 bamboo 内核打到上游供应商。

## 目录结构

```text
relay/
├── relay_adaptor.go       # GetAdaptor / GetTaskAdaptor：按 apiType 分发
├── relay_task.go          # 异步任务中继入口
├── *_handler.go           # 各模式处理器（claude / gemini / embedding / image / audio / rerank / responses / mjproxy / compatible / alpha_search）
├── websocket.go           # Realtime / 双向流
├── chat_completions_via_responses.go / responses_via_chat_completions.go
├── image_recognize_hop.go / image_recognize_stream.go
├── channel/               # 供应商适配器（见 channel/AGENTS.md）
├── bamboo/                # 协议归一化内核（见 bamboo/AGENTS.md）
├── helper/                # 请求校验、价格、流式扫描、max_tokens 边界
├── common/                # RelayInfo、任务信息、中继工具
├── common_handler/        # handler 共享逻辑
├── clientprofile/         # 客户端画像
└── constant/              # relay 模式常量
```

## 导航指南

| 任务 | 位置 | 说明 |
|------|------|------|
| 看请求如何进到供应商 | `*_handler.go` → `GetAdaptor` 或 `bamboo.ChatRelay` | bamboo 失败则 fallback adaptor |
| 注册新渠道 | `relay_adaptor.go` + `constant/channel.go` | switch 必须覆盖新 `APIType` |
| 校验请求 / max_tokens | `helper/valid_request.go` | 各协议的 max-tokens 与 count 上限从第一天就在这里 |
| 图像 n / 任务时长边界 | `helper/openai_image_request_test.go`、`common/` | 与 `dto.MaxImageN`、`MaxTaskDurationSeconds` 对齐 |
| 价格与 OtherRatios | `helper/price.go` | 乘数必须经 `types.PriceData.AddOtherRatio` |
| 改 RelayInfo | `common/` | 计费会话、QuotaClamp、渠道元数据都挂在这里 |
| 异步任务 | `relay_task.go` + `channel/task/` | 提交 → 轮询 → `TaskAdaptor` 计费钩子 |

## 约定

- **两条中继路径**：优先 `bamboo.ChatRelay`（协议归一化）；`ErrUnsupportedProvider` 时回退 `channel.Adaptor` 的 Convert → DoRequest → DoResponse。新供应商应先评估能否进 bamboo provider，而不是只加 adaptor。
- **分发只在一处**：`GetAdaptor` / `GetTaskAdaptor` 的 switch 是唯一渠道分发点。
- **校验先于计费**：用户可控乘数（`n`、`seconds`、`max_tokens`）必须在 `helper` 校验里以 400 拒绝，不能等到 quota 计算再爆。
- **配额换算不在这里做裸强转**：token / quota 转换走 `common/quota_math.go`；饱和事件写入 `relayInfo.QuotaClamp`。
- **JSON 走 `common.*`**；协议 DTO 在 `relaykit/dto`，不要引 root `dto` 里的协议结构（那里只剩任务 DTO）。

## 反模式

- ❌ 在 handler 里直接按供应商名写协议转换——应走 adaptor 或 bamboo。
- ❌ 新增请求格式却不在 `helper/valid_request.go` 限制 max-tokens / count。
- ❌ 把计费乘数写进 `PriceData.OtherRatios` 而不走 `AddOtherRatio`。
- ❌ 在 channel 适配器里做选渠道或鉴权——那是 middleware / service 的事。

## 调试路径

1. 请求进错 handler → `router/relay-router.go` 的路径与 `relay/constant` 的 mode。
2. 选对渠道但协议不对 → 先看是否走了 bamboo；再看 `GetAdaptor` 与对应 `Convert*Request`。
3. 400 参数错误 → `helper/valid_request.go` 与图像/任务边界测试。
4. usage 为 0 → adaptor `DoResponse` 或 `bamboo/usage.go`。
5. 预扣成功但结算偏差 → `relayInfo.Billing` 是否被 handler 传到 `service.SettleBilling`。

## 引用

- [channel](./channel/AGENTS.md) — 供应商 Adaptor / TaskAdaptor
- [bamboo](./bamboo/AGENTS.md) — 协议归一化内核与 fallback
