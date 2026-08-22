# relaykit 知识库

## 概述

独立可构建的协议模块。承载上游协议 DTO、协议互转、错误/格式类型与推理映射，供 root module 的 relay / service / channel 引用。本模块禁止依赖 `github.com/QuantumNous/new-api`。

## 目录结构

```text
relaykit/
├── go.mod / go.sum        # 独立 module：github.com/QuantumNous/new-api/relaykit
├── dto/                   # 上游协议 DTO（OpenAI / Claude / Gemini / Responses / 图像音频等）
├── relayconvert/          # 协议转换门面与内部实现
│   ├── request_compat.go / response_compat.go
│   ├── request_registry.go / response_registry.go / text_converter_registry.go
│   ├── convmeta/          # 转换元数据（格式、选项、Meta）
│   ├── reasoning/         # 推理内容后缀处理
│   ├── kitutil/           # 无宿主依赖的 JSON / 日志 / 掩码辅助
│   └── internal/          # oai_chat / oai_responses / claude_messages / gemini_chat / shared
├── types/                 # RelayFormat、NewAPIError、RequestMeta、文件源
└── reasonmap/             # finish/reason 映射
```

## 导航指南

| 任务 | 位置 | 说明 |
|------|------|------|
| 改 OpenAI / Claude / Gemini 请求响应结构 | `dto/` | 按协议分文件；可选标量必须用指针 + `omitempty` |
| 入口协议互转 | `relayconvert/request_compat.go` | 门面函数，实现在 `internal/` |
| 响应协议互转 | `relayconvert/response_compat.go` | 含流式与非流式 |
| 转换上下文 | `relayconvert/convmeta/` | `Meta`、格式、选项；host 侧 `relay/common` 有别名 |
| 错误类型 / 中继格式 | `types/` | `NewAPIError`、`RelayFormat`、`RequestMeta` |
| 在 kit 内做 JSON | `relayconvert/kitutil/json.go` | kit 不能引用 root `common` |
| 独立编译检查 | 模块根目录 | `cd relaykit && GOWORK=off go build ./...` |

## 约定

- **模块独立**：`relaykit/` 不得 import root module，不得读取 root 配置、生成物或 workspace 才存在的文件。改动后必须 `cd relaykit && GOWORK=off go build ./...`，只过 root build 不够。
- **JSON 走 kitutil**：kit 内 marshal/unmarshal 用 `relaykit/relayconvert/kitutil`，不要直接调 `encoding/json`。host 代码继续用 `common.*`（其内部可再委托 kitutil）。
- **可选请求字段用指针**：从客户端 JSON 解析再marshal 到上游的标量必须是 `*int` / `*uint` / `*float64` / `*bool` + `omitempty`。缺省为 `nil` 省略；显式 `0` / `false` 必须保留。
- **转换实现放 internal**：`relayconvert` 包根只暴露门面；具体 `to_*` 放 `internal/oai_chat`、`internal/oai_responses`、`internal/claude_messages`、`internal/gemini_chat`。
- **类型归属**：协议结构在 `dto/`，跨协议的错误/格式/文件源在 `types/`。root `types/` 只保留计费侧的 `PriceData` 等宿主类型。
- **root `dto/` 不再放协议**：OpenAI/Claude/Gemini 等已迁到这里；root `dto/` 只保留 midjourney / suno / task / video。

## 反模式

- ❌ 在 `relaykit` 里 import `github.com/QuantumNous/new-api/...`（`relaykit` 自身除外）——会破坏独立构建。
- ❌ 在 kit 里引用 `setting/`、`model/`、`common/`——那些是宿主概念。
- ❌ 用非指针标量 + `omitempty` 表达可选请求参数——会吞掉显式零值。
- ❌ 把转换逻辑写进 channel 适配器或 service，而不是 `relayconvert`——协议互转应集中在本模块。
- ❌ 只跑 root `go build` 就宣称 kit 没问题——必须 `GOWORK=off` 在本模块验证。

## 调试路径

1. `GOWORK=off go build` 失败、报找不到 root 包 → 某文件 import 了宿主包，按 import 图拆除。
2. 协议字段丢失或零值被省略 → 对应 `dto/` 结构是否用了指针；再看 `relayconvert/internal/` 的字段拷贝。
3. 转换结果格式不对 → 从 `request_compat.go` / `response_compat.go` 门面跟进到对应 `internal/*`。
4. host 编译失败但 kit 通过 → 检查 host 对已改公开 API 的引用（`relay/common` 别名、`service` 转换调用）。
5. JSON 行为与 host 不一致 → kit 是否绕过了 `kitutil`。
