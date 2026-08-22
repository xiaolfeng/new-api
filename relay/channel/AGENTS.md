# relay/channel 知识库

## 概述

AI 上游供应商适配器集合。每个子目录封装一个供应商的协议转换、请求构造与响应解析，统一实现本目录的 `Adaptor` / `TaskAdaptor`，由 `relay/relay_adaptor.go` 按渠道类型分发。

## 目录结构

```text
relay/channel/
├── adapter.go              # Adaptor / TaskAdaptor 接口
├── api_request.go          # 通用 HTTP 请求构造
├── openai/                 # OpenAI 兼容（Responses、Realtime、图像编辑）
├── claude/                 # Anthropic Claude Messages
├── gemini/                 # Google Gemini（原生 + OpenAI 兼容）
├── aws/                    # AWS Bedrock
├── vertex/                 # Google Vertex AI
├── ali/                    # 阿里通义
├── baidu/、baidu_v2/       # 百度文心
├── volcengine/             # 火山引擎豆包
├── zhipu/、zhipu_4v/       # 智谱 GLM
├── minimax/、moonshot/、deepseek/、xunfei/、tencent/
├── ollama/、coze/、dify/、xai/、perplexity/、mistral/
├── cloudflare/、openrouter/、siliconflow/、replicate/、palm/
├── jina/、jimeng/、lingyiwanwu/、codex/、mokaai/
├── ai360/、xinference/、newapi/、sub2api/、submodel/
├── advancedcustom/         # 高级自定义渠道
└── task/                   # 异步任务适配器（ali/doubao/gemini/hailuo/jimeng/kling/sora/suno/vertex/vidu）
```

## 导航指南

| 任务 | 位置 | 说明 |
|------|------|------|
| 查看适配器契约 | `adapter.go` | `Adaptor` / `TaskAdaptor` 全部方法 |
| 新增供应商 | 新建 `<provider>/` | 实现 `Adaptor`，在 `relay/relay_adaptor.go` 的 `GetAdaptor` switch 注册 |
| 新增渠道类型常量 | `constant/channel.go` | `ChannelTypeXxx = N`，并登记名称映射 |
| 改 OpenAI 兼容路径 | `openai/` | Responses、Realtime、图像编辑 |
| 异步任务（画图/视频/音频） | `task/` | 实现 `TaskAdaptor`，含预估/提交/完成三段计费钩子 |
| 调试请求构造 | `<provider>/adaptor.go` | `ConvertOpenAIRequest` 等，入参为 `relaykit/dto` 标准结构 |

## 约定

- **统一入口 DTO**：`Convert*Request` 的入参必须是 `relaykit/dto` 的标准请求（`GeneralOpenAIRequest`、`ClaudeRequest`、`GeminiChatRequest` 等），出参为供应商原生格式。不要在适配器里直接解析 `*gin.Context` body。
- **未实现必须显式报错**：供应商不支持的类型应 `return nil, errors.New("not implemented")`，不要静默返回空值——分发层依赖此错误跳过。
- **渠道类型集中管理**：`ChannelTypeXxx` 只定义在 `constant/channel.go`，适配器目录只引用。
- **StreamOptions**：新渠道若支持 `StreamOptions`，必须加入 `streamSupportedChannels`。
- **可选标量用指针 + `omitempty`**：缺省为 `nil` 省略，显式 `0` / `false` 必须发出。禁止非指针标量 + `omitempty`。
- **任务计费钩子**：`TaskAdaptor` 的 `EstimateBilling` / `AdjustBillingOnSubmit` / `AdjustBillingOnComplete` 是预扣与差额结算的唯一入口。从 `Extra["parameters"]`、task `metadata`、multipart 读取的 `n` / `seconds` / 分辨率必须套用与 DTO 校验相同的上限。
- **文件命名**：`adaptor.go`、`constants.go` / `constant.go`、`dto.go`、`relay-<provider>.go`、同目录 `*_test.go`。
- **依赖方向**：可依赖 `relaykit/dto`、`relay/common`、`setting`、`types`；供应商目录之间禁止互相依赖。不要在适配器里写鉴权。读库仅限任务完成回调等契约要求的 `model.Task` 参数，不要主动查用户/渠道表。

## 反模式

- ❌ 在适配器里做用户鉴权或改配额——适配器只做协议转换。
- ❌ 用 `encoding/json` 直接 marshal/unmarshal——走 `common.Marshal` / `common.Unmarshal`。
- ❌ 在 `GetAdaptor` 之外用 `if/else` 链分发渠道——必须集中在 `relay_adaptor.go` 的 switch。
- ❌ 给可选请求字段用非指针标量 + `omitempty`——会吞掉显式零值。
- ❌ 新增渠道后忘记登记 `streamSupportedChannels`。
- ❌ 任务乘数不设上限——会绕过 `dto.MaxImageN` / `MaxTaskDurationSeconds` 等计费安全边界。

## 调试路径

1. "channel not found" 或走错供应商 → `relay/relay_adaptor.go` 的 `GetAdaptor(apiType)` 是否覆盖该 `ChannelType`，以及 `constant/api_type.go` 的映射。
2. 请求参数丢失或被改写 → `<provider>/adaptor.go` 的 `Convert*Request`，确认指针字段是否保留显式零值。
3. 响应解析失败 / usage 为 0 → `<provider>` 的 `DoResponse`；流式看 `StreamHandler`。
4. 新增渠道后计费异常 → `GetModelList()` 的模型名是否与 `setting/ratio_setting` 定价键一致。
5. 异步任务预扣/补差不对 → `task/<provider>` 的三个 Billing 钩子，以及 handler 是否读取了返回的 OtherRatios。
6. 本应走 bamboo 却进了 adaptor → 见 [bamboo 知识库](../bamboo/AGENTS.md) 的 `ErrUnsupportedProvider` fallback。

## 引用

- [bamboo](../bamboo/AGENTS.md) — 协议归一化内核；不支持的供应商回退到本目录 adaptor
