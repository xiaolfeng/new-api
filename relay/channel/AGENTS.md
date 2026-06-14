# relay/channel 知识库

## 概述

AI 上游供应商适配器集合。每个子目录（如 `openai/`、`claude/`、`gemini/`）封装一个具体供应商的协议转换、请求构造与响应解析逻辑，统一实现本目录定义的 `Adaptor` / `TaskAdaptor` 接口，供 `relay/relay_adaptor.go` 按渠道类型分发调用。

## 目录结构

```text
relay/channel/
├── adapter.go              # Adaptor / TaskAdaptor 接口定义（所有供应商实现的基础契约）
├── api_request.go          # 通用 HTTP 请求构造辅助
├── submodel/               # 各供应商的子模型清单与映射
├── task/                   # 异步任务（Midjourney/Suno 等）适配器基础
├── openai/                 # OpenAI 兼容（含 Responses API、Realtime、图像编辑）
├── claude/                 # Anthropic Claude（Messages API）
├── gemini/                 # Google Gemini（原生 + OpenAI 兼容）
├── aws/                    # AWS Bedrock
├── vertex/                 # Google Vertex AI
├── ali/                    # 阿里通义千问
├── baidu/, baidu_v2/       # 百度文心一言
├── deepseek/               # DeepSeek
├── cohere/                 # Cohere
├── minimax/                # MiniMax
├── moonshot/               # 月之暗面 Kimi
├── xunfei/                 # 讯飞星火
├── zhipu/, zhipu_4v/       # 智谱 GLM
├── tencent/                # 腾讯混元
├── volcengine/             # 火山引擎豆包
├── ollama/                 # 本地 Ollama
├── coze/                   # 字节 Coze
├── dify/                   # Dify
├── xai/                    # xAI Grok
├── perplexity/             # Perplexity
├── mistral/                # Mistral
├── cloudflare/             # Cloudflare Workers AI
├── openrouter/             # OpenRouter
├── siliconflow/            # 硅基流动
├── replicate/              # Replicate
├── palm/                   # Google PaLM（旧版）
├── jina/, jimeng/          # Jina / 即梦（图像）
├── lingyiwanwu/            # 零一万物
├── codex/                  # OpenAI Codex
├── mokaai/                 # MokaAI
├── ai360/, xinference/     # 360 / Xinference
└── ...                     # 其余供应商见目录列表
```

## 导航指南

| 任务 | 位置 | 说明 |
|------|------|------|
| 查看适配器接口契约 | `adapter.go` | `Adaptor` / `TaskAdaptor` 全部方法签名 |
| 新增一个供应商 | 新建 `<provider>/` 子目录 | 实现 `Adaptor`，再在 `relay/relay_adaptor.go` 的 `GetAdaptor` switch 注册 |
| 新增渠道类型常量 | `constant/channel.go` | `ChannelTypeXxx = N`，并在名称映射表中登记 |
| 修改 OpenAI 兼容路径 | `openai/` | Responses API、Realtime、图像编辑等都在此 |
| 异步任务（画图/视频/音频） | `task/` + 对应供应商目录 | 实现 `TaskAdaptor` |
| 子模型清单 / 映射 | `submodel/` | 各供应商默认模型列表 |
| 调试某供应商请求构造 | `<provider>/adaptor.go` 的 `ConvertOpenAIRequest` 等方法 | 入参统一为 `dto.GeneralOpenAIRequest` 等标准 DTO |

## 约定

- **统一入口 DTO**：所有 `Convert*Request` 方法的入参必须是 `dto/` 下的标准请求结构（`GeneralOpenAIRequest`、`ClaudeRequest`、`GeminiChatRequest` 等），出参为供应商原生格式。不要在适配器里直接解析 `c *gin.Context` 的 body。
- **未实现的方法必须显式报错**：对于供应商不支持的请求类型（如 Claude 不支持图像），对应 `Convert*Request` 方法应 `return nil, errors.New("not implemented")`，而不是静默返回空值——分发层依赖此错误跳过。
- **渠道类型集中管理**：所有 `ChannelTypeXxx` 常量定义在 `constant/channel.go`，适配器目录自身不定义类型常量，只引用。
- **文件命名**：`adaptor.go`（接口实现）、`constants.go` 或 `constant.go`（模型列表/端点常量）、`dto.go`（供应商私有 DTO）、`relay-<provider>.go`（核心转换逻辑）、`*_test.go`（测试，与实现同目录）。
- **依赖方向**：适配器可依赖 `dto/`、`relay/common/`、`setting/`、`model/`、`types/`，但供应商目录之间**禁止互相依赖**。

## 反模式

- ❌ 在适配器目录里直接读写数据库（`model.*`）或执行鉴权（`middleware.*`）——适配器只做协议转换，业务逻辑放在 `service/`。
- ❌ 用 `encoding/json` 直接 marshal/unmarshal——必须走 `common.Marshal` / `common.Unmarshal`（见根 AGENTS.md Rule 1）。
- ❌ 在 `GetAdaptor` 之外用 `if/else` 链分发渠道——必须用 switch 集中在 `relay_adaptor.go`。
- ❌ 给可选请求字段用非指针标量 + `omitempty`——会吞掉显式零值（见根 AGENTS.md Rule 6）。
- ❌ 新增渠道后忘记在 `streamSupportedChannels` 登记 StreamOptions 支持（见根 AGENTS.md Rule 4）。

## 调试路径

1. 请求报错 "channel not found" / 走错供应商 → 检查 `relay/relay_adaptor.go` 的 `GetAdaptor(apiType)` switch 是否覆盖该 `ChannelType`，以及 `constant/api_type.go` 的 apiType 映射。
2. 请求参数丢失或被错误改写 → 在对应 `<provider>/adaptor.go` 的 `ConvertOpenAIRequest`（或 `ConvertClaudeRequest` 等）打断点，确认入参 DTO 字段是否被指针类型正确传递（Rule 6）。
3. 响应解析失败 / usage 统计为 0 → 看 `<provider>/relay-<provider>.go` 的 `DoResponse` 与 usage 解析；流式响应检查 `StreamHandler`。
4. 新增渠道后计费异常 → 确认 `GetModelList()` 返回的模型名与 `setting/ratio_setting` 中的定价键一致。
5. 跨数据库报错（如 `@>` / `JSONB`）→ 适配器本身一般不碰 DB，排查是否误在适配器引入了 DB 调用。

## 引用

无子级 `AGENTS.md`。本目录的上级导航见根 [`AGENTS.md`](../../AGENTS.md)。
