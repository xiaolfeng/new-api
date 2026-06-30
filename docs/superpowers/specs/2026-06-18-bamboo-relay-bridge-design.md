# bamboo-messages 协议归一化中继桥（Bridge）设计

> **文档性质**：架构设计 spec（brainstorming 产出，待批准后转 writing-plans）
> **设计日期**：2026-06-18
> **分支**：`feature/new-relay-for-bamboo`
> **前置文档**：`docs/new-api-integration.md`（落地方案草案）、`docs/new-api-feasibility.md`（可行性分析草案）
> **本 spec 状态**：已根据源码级核对修正前置文档的所有阻断性/编译级错误，并纳入用户新决策（移除 base-go）

---

## 一、执行摘要

用 bamboo-messages 的协议无关中间表示，替换 new-api 四个对话类 Helper 内部的「Convert → DoRequest → DoResponse」三段式协议转换内核，将入口协议 × 上游协议的 **N×M 转换矩阵降为 N+M**。

本次方案与前置文档草案的关键差异（基于源码核对）：

| 差异 | 草案假设 | 本 spec 修正 |
|------|---------|-------------|
| **包可见性**（草案完全遗漏的阻断点） | 直接 `import ".../bamboo-messages/bamboo"` 与 `internal/provider/*` | bamboo SDK 包 import 了 `internal/provider`，外部模块不可引用 → 必须先把 `internal/provider` 提升为公开 `provider/` 包 |
| **gin 冲突处理** | 升级 new-api gin v1.9.1 → v1.11.0（241 文件回归） | 改为移除 bamboo 对 `bamboo-base-go` 的依赖 → gin 传递链彻底消失，new-api **零 gin 改动** |
| **错误符号**（5 处编译级错误） | 引用了不存在的 `types.NewAPIError()` 构造函数、`ErrorCodeConvertResponseFailed`、`CodecError.ErrorTypeParse` 等 | 全部替换为真实存在的符号（见附录 A） |

**改造分两仓库**：

- **bamboo-messages**（用户为协作者，有改动权）：2 个独立小 PR（移除 base-go + 提升 provider 公开包）
- **new-api**（本仓库）：新增 `relay/bamboo/` 桥包 + 4 个 Helper 内核委托 + fallback + 灰度开关

---

## 二、已确认的架构决策

| # | 决策点 | 选择 | 理由 |
|---|--------|------|------|
| D1 | 集成策略 | 改 bamboo 包结构（而非 vendor/fork/独立服务） | bamboo codec 层成熟，改包结构后可直接 go import 复用，最干净 |
| D2 | bamboo 改造方式 | 提升 `internal/provider` → 公开 `provider/` 包 | 改动最小（重命名目录 + 批量改 import 路径），保留现有所有结构 |
| D3 | gin 冲突处理 | **移除 bamboo 的 `bamboo-base-go` 依赖** | 核查证实依赖极浅（仅 4 个符号），移除后 gin 冲突彻底消失，new-api 零回归（比"升级 new-api gin"更优，已取代该决策） |
| D4 | 集成深度 | 方案 A：Bridge 内核替换（4 Helper 委托 + fallback） | 唯一真正达成 N×M→N+M 的路线，fallback 保证渐进零中断 |

---

## 三、改造范围

### 3.1 在范围内（本次改造）

new-api 四个对话类 Helper 的协议转换内核：

| Helper | 文件 | 入口协议 | 入口 DTO |
|--------|------|---------|---------|
| `TextHelper` | `relay/compatible_handler.go:25` | OpenAI Chat Completions | `*dto.GeneralOpenAIRequest` |
| `ClaudeHelper` | `relay/claude_handler.go:24` | Anthropic Messages | `*dto.ClaudeRequest` |
| `GeminiHelper` | `relay/gemini_handler.go:54` | Google Gemini | `*dto.GeminiChatRequest` |
| `ResponsesHelper` | `relay/responses_handler.go:25` | OpenAI Responses | `*dto.OpenAIResponsesRequest` |

### 3.2 不在范围内（保留原生链路，零改动）

- Embedding / Rerank / Audio / Image / Task / Realtime 的所有 Helper
- 未被 bamboo 覆盖的上游渠道：AWS（SigV4）、讯飞（WebSocket）、腾讯（TC3-HMAC）、智谱 v3（JWT）、Coze / Dify、百度 v1（OAuth）、阿里 DashScope、VertexAI（service-account）

这些通过 `ErrUnsupportedProvider` fallback 回 new-api 原生三段式，代码保留不删。

---

## 四、bamboo-messages 侧改造（2 个独立 PR）

### 4.1 PR-B1：移除 bamboo-base-go 依赖（前置阻断解除）

**动机**：消除 gin 传递依赖，让 bamboo-messages 成为纯净的可复用 Go 库。

**核查事实**：bamboo 核心代码（`bamboo/` + `internal/`）对 `bamboo-base-go` 的依赖极浅——12 个文件，全部只引用同一个子包 `common/error`（别名 `xError`），且只用了 4 个符号：`Error` 类型、`NewError` 函数、`ErrorCode`+`OperationFailed` 哨兵、`ErrMessage` 字符串类型。所有 11 个 provider 调用点都是同一个模板：

```go
xError.NewError(ctx, xError.OperationFailed, "...", false, err)
```

bamboo 侧只读取返回值的 `.Error()` 当字符串，从不读 `ErrorCode` 字段、从不触发 `throw=true` 日志分支。gin 的唯一来源链是 `common/error` → `common/log/handler.go:16`（`ctx.(*gin.Context)` 断言）。

**改造步骤**：

1. 在 bamboo-messages 内新增本地错误包，例如 `internal/xerr/error.go`（约 20 行）：

   ```go
   package xerr

   import (
       "context"
       "errors"
   )

   // Error 替代 bamboo-base-go/common/error.Error 的最小实现。
   // bamboo 侧仅使用 .Error() 读取消息文本。
   type Error struct {
       err     error
       Message string
   }

   func (e *Error) Error() string {
       if e == nil || e.err == nil {
           return ""
       }
       return e.err.Error()
   }

   // NewError 兼容原 xError.NewError 签名，忽略 ErrorCode 与 throw 参数。
   func NewError(_ context.Context, _ any, msg string, _ bool, cause ...error) *Error {
       e := errors.New(msg)
       if len(cause) > 0 && cause[0] != nil {
           e = cause[0]
       }
       return &Error{err: e, Message: msg}
   }
   ```

   > 注：签名用 `_ any` 占位 `*ErrorCode`，`_ bool` 占位 `throw`，以最小化调用点 diff（也可顺手去掉这两个无用参数，diff 略大但更干净——由实现者权衡）。

2. 全局替换 12 个文件的 import：
   - `xError "github.com/bamboo-services/bamboo-base-go/common/error"` → `xError "github.com/bamboo-services/bamboo-messages/internal/xerr"`
   - 调用点 `xError.NewError(ctx, xError.OperationFailed, "...", false, err)` → 简化为 `xError.NewError(ctx, nil, "...", false, err)`（或按上一步取舍去掉多余参数）

3. `go.mod` 删除：
   - `github.com/bamboo-services/bamboo-base-go/common`（direct，第 8 行）
   - `github.com/bamboo-services/bamboo-base-go/defined`（indirect，第 16 行）

4. `go mod tidy` → gin、go-playground/validator/v10、quic-go 等整条链从依赖图消失。

**验收**：
- `go build ./...` exit 0
- `go test ./bamboo/... ./internal/...` 全绿
- `grep -r "gin-gonic" go.mod go.sum` 无命中
- `go list -m all | grep bamboo-base-go` 无输出

**工作量**：< 0.5 人天。风险极低。

### 4.2 PR-B2：提升 `internal/provider` 为公开 `provider/` 包

**动机**：解除 B-1 阻断。bamboo SDK 包（`bamboo/`、`bamboo/codec/`）的全部 4+1 个文件 import 了 `internal/provider`，导致外部模块（new-api）无法 import bamboo。

**核查事实**：导出符号的签名直接出现 internal 类型——`bamboo.NewClient(p provider.Provider)`、`bamboo.WithProvider(p provider.Provider)`、`bamboo/config.go:8` 的 `type ThinkingConfig = provider.ThinkingConfig`——类型解析阶段即触发 `use of internal package not allowed`。

**改造步骤**：

1. 目录重命名：`internal/provider/` → `provider/`
2. 全局批量替换 import 路径：
   - `github.com/bamboo-services/bamboo-messages/internal/provider` → `github.com/bamboo-services/bamboo-messages/provider`
   - 涉及 `bamboo/`（4 文件）、`bamboo/codec/`（间接经 `bamboo`）、`internal/` 内部互相引用（如有）、`example/`（1 文件）
3. `go build ./...` + `go test ./...` 验证

**验收**：
- bamboo 仓库自身 `go build ./...` exit 0、测试全绿
- **外部验证**：在 new-api 的 `go.mod` 加 `require bamboo-messages <new-tag>`，写一个最小 `import ".../bamboo-messages/bamboo"` 的测试文件，`go build` 通过（不再报 internal 错误）

**工作量**：0.5 人天（机械重命名）。风险低。

### 4.3 PR-B1 / PR-B2 的关系与顺序

- 两者**互相独立**，可并行开发，但建议 PR-B1 先合并（移除 base-go 后依赖图更干净，便于 PR-B2 的 import 重构）。
- 合并后打一个新 tag（如 `v0.x.0`），new-api 侧依赖这个 tag。

---

## 五、new-api 侧改造：relay/bamboo/ 桥包

### 5.1 新增包结构

```
relay/bamboo/
├── bridge.go            # 核心入口：ChatRelay(c, info, entryFormat, body) (*dto.Usage, *types.NewAPIError)
├── codec_map.go         # types.RelayFormat ↔ bamboo codec.FormatType 映射（包内私有，ChatRelay 内部用）
├── provider_factory.go  # RelayInfo → bamboo provider.Provider 实例化（按 ApiType 选上游协议）
├── usage.go             # bamboo Usage → dto.Usage 计费映射（含 reasoning token 累计）
└── errors.go            # 错误翻译：bamboo CodecError / BambooError → *types.NewAPIError
```

### 5.2 核心入口 `ChatRelay`

`relay/bamboo/bridge.go` —— 替代 4 个 Helper 内部的三段式内核：

```go
package bamboo

import (
    "errors"
    "github.com/gin-gonic/gin"

    bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
    bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"

    "github.com/QuantumNous/new-api/dto"
    relaycommon "github.com/QuantumNous/new-api/relay/common"
    "github.com/QuantumNous/new-api/types"
)

// ErrUnsupportedProvider 表示该上游 ApiType 未被 bamboo 覆盖，
// 调用方应 fallback 到 new-api 原生三段式。
var ErrUnsupportedProvider = errors.New("bamboo: unsupported provider for this api type")

// ChatRelay 对话中继统一内核。
//
// 替代 TextHelper/ClaudeHelper/GeminiHelper/ResponsesHelper 内部的
// Convert→DoRequest→DoResponse 三段式，用 bamboo 中间表示做协议归一化。
//
// 调用方只需传 new-api 侧的 types.RelayFormat，格式映射在 bridge 内部完成。
// info.ApiType（经 ChannelMeta 嵌入）决定上游用哪个 bamboo provider。
//
// 返回 (usage, nil) 成功；(nil, err) 失败。
// 当 err 可 errors.Is(err, ErrUnsupportedProvider) 时，调用方应 fallback 原生链路。
func ChatRelay(c *gin.Context, info *relaycommon.RelayInfo,
    entryFormat types.RelayFormat, requestBody []byte) (*dto.Usage, *types.NewAPIError) {

    // ① 入口格式映射：RelayFormat → codec FormatType（包内私有映射）
    codecFmt, ok := relayFormatToCodec(entryFormat)
    if !ok {
        // 非对话格式（Audio/Image/Task/Realtime 等）不应进入 bridge
        return nil, types.NewError(ErrUnsupportedProvider, types.ErrorCodeInvalidApiType)
    }

    entryCodec, gerr := bamboocodec.Get(codecFmt)
    if gerr != nil || entryCodec == nil {
        return nil, types.NewError(gerr, types.ErrorCodeInvalidRequest)
    }
    relayReq, perr := entryCodec.ParseRequest(requestBody)
    if perr != nil {
        return nil, translateCodecError(perr) // 内部做 *CodecError 断言，见 errors.go
    }

    // ② 上游侧：根据 ApiType 构造 bamboo provider
    p, perr := newProvider(info)
    if perr != nil {
        return nil, perr // 含 ErrUnsupportedProvider，调用方判 errors.Is 做 fallback
    }
    client := bamboosdk.NewClient(p)

    // ③ 出口侧：按入口 codec 序列化响应
    if relayReq.IsStream {
        return doStreamRelay(c, client, entryCodec, relayReq)
    }
    return doCompleteRelay(c, client, entryCodec, relayReq)
}
```

> 注：
> - 错误构造统一用 `types.NewError(...)`（**非** `types.NewAPIError`，后者是结构体类型，见附录 A）。
> - `ChatRelay` 接收 new-api 侧的 `types.RelayFormat`，格式映射收敛到 bridge 内部（`relayFormatToCodec` 包内私有），调用方无需 import codec 包。

### 5.3 流式与非流式分支

```go
// doStreamRelay 消费 bamboo StreamEvent，按入口 codec 序列化为出口 SSE。
func doStreamRelay(c *gin.Context, client bamboosdk.BambooClient,
    entryCodec bamboocodec.Codec, req *bamboocodec.RelayRequest) (*dto.Usage, *types.NewAPIError) {

    eventCh, err := client.Chat(c.Request.Context(), req.Messages, req.System, req.Config)
    if err != nil {
        return nil, types.NewError(err, types.ErrorCodeDoRequestFailed)
    }

    c.Writer.Header().Set("Content-Type", "text/event-stream")
    c.Writer.Header().Set("Cache-Control", "no-cache")
    c.Writer.Flush()

    serializer := entryCodec.NewSerializer()
    var usage dto.Usage

    for event := range eventCh {
        if event.Type == bamboosdk.EventError {
            return nil, types.NewError(event.Error, types.ErrorCodeBadResponseBody)
        }
        data, serr := serializer.Serialize(event)
        if serr != nil {
            return nil, translateCodecError(serr)
        }
        if _, werr := c.Writer.Write(data); werr != nil {
            break // 客户端断开
        }
        c.Writer.Flush()

        // 从 message_delta 提取 usage（注意字段名 CacheCreationInputTokens/CacheReadInputTokens）
        if event.Type == bamboosdk.EventMessageDelta && event.Usage != nil {
            usage.PromptTokens = int(event.Usage.InputTokens)
            usage.CompletionTokens = int(event.Usage.OutputTokens)
            // reasoning token 累计见 usage.go
        }
    }

    tail, _ := serializer.Flush()
    if len(tail) > 0 {
        c.Writer.Write(tail)
        c.Writer.Flush()
    }

    usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
    return &usage, nil
}

// doCompleteRelay 非流式中继。
func doCompleteRelay(c *gin.Context, client bamboosdk.BambooClient,
    entryCodec bamboocodec.Codec, req *bamboocodec.RelayRequest) (*dto.Usage, *types.NewAPIError) {

    resp, err := client.Complete(c.Request.Context(), req.Messages, req.System, req.Config)
    if err != nil {
        return nil, types.NewError(err, types.ErrorCodeDoRequestFailed)
    }
    body, serr := entryCodec.SerializeResponse(resp)
    if serr != nil {
        return nil, translateCodecError(serr)
    }
    c.Writer.Header().Set("Content-Type", "application/json")
    c.Writer.Write(body)

    return &dto.Usage{
        PromptTokens:     int(resp.Usage.InputTokens),
        CompletionTokens: int(resp.Usage.OutputTokens),
        TotalTokens:      int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
    }, nil
}
```

### 5.4 Provider 工厂

`relay/bamboo/provider_factory.go` —— 根据 `info.ApiType`（经 ChannelMeta 嵌入字段访问）选 bamboo provider：

```go
package bamboo

import (
    bamboocompletions "github.com/bamboo-services/bamboo-messages/provider/openai/completions"
    bambooresponses  "github.com/bamboo-services/bamboo-messages/provider/openai/responses"
    bambooanthropic  "github.com/bamboo-services/bamboo-messages/provider/anthropic"
    bamboogemini     "github.com/bamboo-services/bamboo-messages/provider/gemini"
    "github.com/bamboo-services/bamboo-messages/provider"

    "github.com/QuantumNous/new-api/constant"
    relaycommon "github.com/QuantumNous/new-api/relay/common"
    "github.com/QuantumNous/new-api/types"
)

// newProvider 根据 RelayInfo 构造对应的 bamboo provider。
//
// PR-B2 完成后，provider/ 为公开包，可直接 import。
// 未覆盖的 ApiType 返回 ErrUnsupportedProvider，由上层 fallback。
func newProvider(info *relaycommon.RelayInfo) (provider.Provider, *types.NewAPIError) {
    apiKey := info.ApiKey          // 经 *ChannelMeta 嵌入提升
    baseURL := info.ChannelBaseUrl // 同上

    switch info.ApiType {
    case constant.APITypeAnthropic:
        return bambooanthropic.NewProviderWithOptions(
            bambooanthropic.WithAPIKey(apiKey),
            bambooanthropic.WithBaseURL(baseURL),
        ), nil

    case constant.APITypeGemini:
        return bamboogemini.NewProviderWithOptions(
            bamboogemini.WithAPIKey(apiKey),
            bamboogemini.WithBaseURL(baseURL),
        ), nil

    case constant.APITypeCodex:
        return bambooresponses.NewResponsesProviderWithOptions(
            bambooresponses.WithAPIKey(apiKey),
            bambooresponses.WithBaseURL(baseURL),
        ), nil

    case constant.APITypeOpenAI,
        constant.APITypeDeepSeek, constant.APITypeMoonshot,
        constant.APITypeSiliconFlow, constant.APITypeMistral,
        constant.APITypeXai, constant.APITypeZhipuV4,
        constant.APITypePerplexity, constant.APITypeCohere,
        constant.APITypeMiniMax, constant.APITypeBaiduV2,
        constant.APITypeOpenRouter, constant.APITypeXinference:
        // OpenAI Chat Completions 兼容渠道统一走 completions provider
        return bamboocompletions.NewCompletionsProviderWithOptions(
            bamboocompletions.WithAPIKey(apiKey),
            bamboocompletions.WithBaseURL(baseURL),
        ), nil

    default:
        // AWS/讯飞/腾讯/智谱v3/Coze/Dify 等特殊协议
        return nil, types.NewError(ErrUnsupportedProvider, types.ErrorCodeInvalidApiType)
    }
}
```

### 5.5 RelayFormat ↔ codec FormatType 映射

```go
// relay/bamboo/codec_map.go
package bamboo

import (
    bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
    "github.com/QuantumNous/new-api/types"
)

func relayFormatToCodec(f types.RelayFormat) (bamboocodec.FormatType, bool) {
    switch f {
    case types.RelayFormatOpenAI:
        return bamboocodec.FormatOpenAI, true
    case types.RelayFormatClaude:
        return bamboocodec.FormatAnthropic, true
    case types.RelayFormatOpenAIResponses:
        return bamboocodec.FormatResponses, true
    case types.RelayFormatGemini:
        return bamboocodec.FormatGemini, true
    default:
        return "", false // Audio/Image/Task/Realtime/Rerank/Embedding 不在范围
    }
}
```

### 5.6 错误翻译

```go
// relay/bamboo/errors.go
package bamboo

import (
    "errors"

    bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
    "github.com/QuantumNous/new-api/types"
)

// translateCodecError 把 bamboo CodecError 翻译为 new-api 错误。
//
// 入参为 error 接口（ParseRequest/Serialize 返回裸 error），
// 内部做 *CodecError 类型断言；非 CodecError 的 error 走默认分支。
//
// CodecError.Type 实际枚举（已核对，bamboo/codec/errors.go:9-22）：
//   ErrInvalidRequest / ErrProviderError / ErrAuthError / ErrRateLimit / ErrInternal
//
// ErrorCode 映射（new-api types/error.go 真实存在的常量）：
//   注意 new-api 无 auth/rateLimit/upstream 专用 ErrorCode，
//   复用语义最接近的现有常量（AccessDenied/BadResponse/BadResponseStatusCode）。
func translateCodecError(err error) *types.NewAPIError {
    if err == nil {
        return nil
    }
    var ce *bamboocodec.CodecError
    if !errors.As(err, &ce) {
        // 非 CodecError（如 provider 内部错误），用通用转换失败码
        return types.NewError(err, types.ErrorCodeConvertRequestFailed)
    }
    switch ce.Type {
    case bamboocodec.ErrInvalidRequest:
        return types.NewError(ce, types.ErrorCodeInvalidRequest)
    case bamboocodec.ErrAuthError:
        return types.NewError(ce, types.ErrorCodeAccessDenied) // 鉴权失败复用 AccessDenied
    case bamboocodec.ErrRateLimit:
        return types.NewError(ce, types.ErrorCodeBadResponse) // 限流复用 BadResponse（无专用码）
    case bamboocodec.ErrProviderError:
        return types.NewError(ce, types.ErrorCodeBadResponseStatusCode) // 上游错误复用 BadResponseStatusCode
    default: // ErrInternal 等
        return types.NewError(ce, types.ErrorCodeConvertRequestFailed)
    }
}
```

> 注（复审修正）：
> - `ParseRequest`/`Serialize` 返回**裸 `error`**（非 `*CodecError`），故 translateCodecError 入参必须是 `error` 接口，内部用 `errors.As` 做类型断言（避免非 CodecError 时 panic）。
> - new-api `types/error.go` 真实存在的 ErrorCode 常量共 31 个（复审已全量列出），**无** `InvalidAuth`/`RateLimit`/`UpstreamError` 专用码，故 auth→`AccessDenied`、rateLimit→`BadResponse`、provider→`BadResponseStatusCode`。若需更精确的语义，可在实现阶段向 types 包新增 3 个 ErrorCode 常量。
> - 5.2/5.3 节所有 `translateCodecError(xxx)` 调用点因此都传入 `error` 接口值，类型匹配。

### 5.7 Usage 计费映射

```go
// relay/bamboo/usage.go
package bamboo

import "github.com/QuantumNous/new-api/dto"

// 注意字段名（已核对）：
//   - bamboo.Usage: InputTokens/OutputTokens/CacheCreationInputTokens/CacheReadInputTokens
//   - dto.Usage:    PromptTokens/CompletionTokens/TotalTokens
//                  + CompletionTokenDetails.ReasoningTokens（注意是 Details 无 s）
//                  + CompletionTokenDetails.AudioTokens
//
// reasoning token：bamboo 的 StreamEvent 在 thinking delta 中携带，
// 需在 doStreamRelay 循环里累计，这里提供累计函数。
//
// 复审修正：CompletionTokenDetails 是【值类型 struct】（非指针，dto/openai_response.go:232），
// 故不能 == nil 判断，也不能取址赋值；直接访问其字段即可。
func accumulateReasoning(usage *dto.Usage, delta int) {
    usage.CompletionTokenDetails.ReasoningTokens += delta
}
```

---

## 六、4 个 Helper 的改造（new-api 侧）

### 6.1 改造原则

每个 Helper 改为**入口胶水代码**：
- **保留**全部 new-api 侧业务逻辑：模型映射（`ModelMappedHelper`）、系统提示注入、thinking 后缀/budget 适配、计费调用（`PostTextConsumeQuota`）
- **替换**三段式内核为 `bamboo.ChatRelay`
- **保留**原三段式为 `originalXxxRelay` 兜底函数，在 `ErrUnsupportedProvider` 时 fallback

### 6.2 改造后 ClaudeHelper 示例（其余 3 个对称）

```go
// relay/claude_handler.go（改造后骨架）
func ClaudeHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
    info.InitChannelMeta(c)

    claudeReq, ok := info.Request.(*dto.ClaudeRequest)
    if !ok {
        return /* 原校验逻辑保留 */
    }
    request, err := common.DeepCopy(claudeReq)
    if err != nil {
        return /* ... */
    }

    // ── new-api 侧业务逻辑全部保留 ──
    helper.ModelMappedHelper(c, info, request)
    applyClaudeThinkingAdapter(info, request)    // thinking 后缀适配
    applyChannelSystemPrompt(info, request)      // 渠道系统提示注入

    // ── 协议转换内核委托给 bamboo bridge ──
    bodyBytes, _ := common.Marshal(request)

    // ChatRelay 接收 types.RelayFormat，格式映射在 bridge 内部完成
    usage, relayErr := bamboo.ChatRelay(c, info, types.RelayFormatClaude, bodyBytes)
    if relayErr != nil {
        // 未覆盖的上游 fallback 到原生三段式
        if errors.Is(relayErr, bamboo.ErrUnsupportedProvider) {
            return originalClaudeRelay(c, info, request) // 原三段式保留不动
        }
        return relayErr
    }

    // 计费（签名：extraContent 是 []string）
    service.PostTextConsumeQuota(c, info, usage, nil)
    return nil
}
```

### 6.3 必须处理的旁路（核查发现）

`TextHelper`（`relay/compatible_handler.go`）三段式外有两个旁路，改造时需谨慎对待：

1. **pass-through 旁路**（L97-107）：当 pass-through 开启时，绕过 Convert 直接用原始 body 发上游。触发条件是 **OR**：
   - 全局开关 `setting/model_setting` 的 `GlobalSettings.PassThroughRequestEnabled`（`global.go:36`）
   - 渠道级 `info.ChannelSetting.PassThroughBodyEnabled`
   
   bamboo bridge 接管后，须保留 `if passThroughGlobal || info.ChannelSetting.PassThroughBodyEnabled` 的完整判断：开启时仍走原 pass-through 路径（不进 bridge），避免改变行为。**漏掉全局开关会导致 `PassThroughRequestEnabled=true` 时请求误进 bridge。**

2. **chatCompletionsViaResponses 旁路**（L74-93）：当满足完整前置 AND 条件时，OpenAI 入口会跳过 `ConvertOpenAIRequest` 改走 Responses 路径：
   - `info.RelayMode == relayconstant.RelayModeChatCompletions`
   - **且** `!passThroughGlobal && !info.ChannelSetting.PassThroughBodyEnabled`
   - **且** `service.ShouldChatCompletionsUseResponsesGlobal(info.ChannelId, info.ChannelType, info.OriginModelName)`（`service/openai_chat_responses_mode.go:12`）
   
   改造时需保留此旁路的完整合取判断（不可简化），或确认 bamboo 的 completions provider 能等价覆盖。

> 实现阶段必须对这两个旁路写对齐测试（同请求双路径比对）。

### 6.4 关键调用顺序（核查后修正）

`controller.Relay()` 主链路的真实顺序（前置文档漏列了 `EstimateRequestToken`/`ModelPriceHelper` 两步，且 `PostTextConsumeQuota` 在 Helper 内部而非主链路）：

```
controller.Relay(c, RelayFormat)
  ├─ helper.GetAndValidateRequest(c, format)            ← 解析 HTTP body → dto.Request
  ├─ relaycommon.GenRelayInfo(c, format, request, ws)   ← 构造 RelayInfo（含 ChannelMeta）
  ├─ service.CheckSensitiveText(...)                    ← 敏感词
  ├─ service.EstimateRequestToken(...)                  ← 估算 token（前置文档漏列）
  ├─ relayInfo.SetEstimatePromptTokens(...)
  ├─ helper.ModelPriceHelper(...)                       ← 价格（前置文档漏列）
  ├─ service.PreConsumeBilling(...)                     ← 预扣费（免费模型跳过）
  └─ 重试循环 → 按 RelayFormat 分派到 XxxHelper
       └─ XxxHelper 内部：
            ├─ 业务逻辑（模型映射/系统提示/...）
            ├─ bamboo.ChatRelay(...)        ← ★ 新内核
            └─ service.PostTextConsumeQuota(c, info, usage, nil)  ← 结算（在 Helper 内）
```

bamboo bridge 只替换 Helper 内部三段式，**不动**主链路的前后步骤。

---

## 七、渐进式灰度与 Fallback

### 7.1 灰度开关

新增 `setting/model_setting/bamboo_setting.go`（复用现有注册模式，见 `setting/model_setting/global.go`）：

```go
package model_setting

import "github.com/QuantumNous/new-api/setting/config"

type BambooSettings struct {
    EnableBambooRelay bool  `json:"enable_bamboo_relay"`    // 全局开关，默认 false
    // EnabledApiTypes 由白名单控制；未实现复杂白名单时，先靠 provider_factory 的
    // switch + ErrUnsupportedProvider 天然限制覆盖范围。
}

var defaultBambooSettings = BambooSettings{EnableBambooRelay: false}
var bambooSettings = defaultBambooSettings

func init() {
    config.GlobalConfig.Register("bamboo", &bambooSettings)
}

func GetBambooSettings() *BambooSettings { return &bambooSettings }
```

Helper 改造后入口判断：

```go
if !model_setting.GetBambooSettings().EnableBambooRelay {
    return originalXxxRelay(c, info, request) // 开关关闭 → 原生路径
}
```

### 7.2 Fallback 双路径

```
XxxHelper(c, info)
  ├─ 灰度开关关闭 → originalXxxRelay (原生三段式)
  ├─ 开关开启 → bamboo.ChatRelay(...)
  │    ├─ newProvider 成功 → 走 bamboo 中继
  │    └─ 返回 ErrUnsupportedProvider → fallback originalXxxRelay
  └─ bamboo 内部错误 → 返回错误（不 fallback，因为已进入 bamboo 路径）
```

**回滚成本**：关闭 `EnableBambooRelay` 开关，全量回退原生链路，**< 1 分钟**（配置热更新）。

### 7.3 灰度阶段

| 阶段 | 开启范围 | 覆盖渠道 | 验收 |
|:---:|---------|---------|------|
| 1 | bamboo 侧 PR-B1/B2 合并，new-api 侧 bridge 包 + 单元测试 | — | bridge 单测通过，`go build ./...` 通过 |
| 2 | 仅 `APITypeOpenAI` + `RelayFormatOpenAI`（OpenAI 入口 → OpenAI 上游） | OpenAI 官方 | 端到端调通，计费准确 |
| 3 | + `APITypeDeepSeek` + OpenAI 兼容渠道 | DeepSeek/Kimi/硅基流动等 | 跨厂商验证 |
| 4 | + `APITypeAnthropic`/`RelayFormatClaude`、`APITypeGemini`/`RelayFormatGemini` | Claude/Gemini | 跨协议验证（Claude 入口 → DeepSeek 上游） |
| 5 | + `APITypeCodex`/`RelayFormatOpenAIResponses` | Codex/Responses | Responses 入口验证 |
| 6 | 全量开启 + 稳定观察 1-2 周 | ~70% 对话渠道 | 观察后可选择删除 fallback（可选） |

---

## 八、覆盖率

### 8.1 走 bamboo 中继的 ApiType（约 24 个，~70% 对话渠道）

OpenAI 兼容（completions provider）：`APITypeOpenAI` / `APITypeDeepSeek` / `APITypeMoonshot` / `APITypeSiliconFlow` / `APITypeMistral` / `APITypeXai` / `APITypeZhipuV4` / `APITypePerplexity` / `APITypeCohere` / `APITypeMiniMax` / `APITypeBaiduV2` / `APITypeOpenRouter` / `APITypeXinference`

原生协议：`APITypeAnthropic`（anthropic provider）/ `APITypeGemini`（gemini provider）/ `APITypeCodex`（responses provider）

### 8.2 保留原生链路的 ApiType（fallback）

`APITypeAws`（SigV4）/ `APITypeXunfei`（WebSocket）/ `APITypeTencent`（TC3-HMAC）/ `APITypeZhipu`（v3 JWT）/ `APITypeCoze` / `APITypeDify` / `APITypeBaidu`（v1 OAuth）/ `APITypeAli`（DashScope）/ `APITypeVertexAi`（service-account）

---

## 九、风险登记册（实证更新）

| 编号 | 风险 | 概率 | 影响 | 等级 | 缓解 |
|:---:|------|:---:|:---:|:---:|------|
| R1 | bamboo SDK 未来 breaking change | 低 | 中 | 低 | go.mod 锁定 bamboo 版本 tag |
| R2 | 流式 bridge goroutine 泄漏 | 中 | 高 | 中 | `Chat()` channel 配合 `ctx.Done()`；pprof 泄漏测试 |
| R3 | codec 字段覆盖不全（logit_bias/n/stream_options 等未建模） | 中 | 中 | 中 | `ProviderExtra` 透传兜底；逐字段对齐测试 |
| R4 | usage 缺 reasoning token 计费偏差 | 中 | 中 | 中 | Phase 6 从 thinking delta 累计补齐 |
| R5 | fallback 与 bamboo 路径行为不一致（错误码/响应头） | 中 | 中 | 中 | 同请求双路径比对测试 |
| R6 | TextHelper 的 pass-through / chatCompletionsViaResponses 旁路被误改 | 中 | 高 | 中 | 实现阶段保留旁路判断 + 对齐测试 |
| R7 | bamboo provider 的 UserAgent/Header 与 new-api 预期不符 | 低 | 低 | 低 | `WithHeader` 覆盖 |
| R8 | Realtime/Task 等 RelayMode 误入 bamboo 路径 | 低 | 高 | 低 | `relayFormatToCodec` 对非对话格式返回 false |

**与前置文档的差异**：
- 原最大风险"gin 版本冲突"已通过 D3（移除 base-go）**彻底消除**，从风险册移除
- 新增 R6（pass-through 旁路），源于核对发现的 TextHelper 隐藏分支
- 原始 8 项风险中 0 项高、5 中、3 低，均可控

---

## 十、实施路线图

### Phase 0：bamboo 侧准备（2 个 PR，并行可做，~1 天）

1. **PR-B1**：移除 bamboo-base-go 依赖（4.1 节）
2. **PR-B2**：提升 `internal/provider` → `provider/`（4.2 节）
3. 合并后打新 tag，new-api `go.mod` 引用该 tag

### Phase 1：bridge 基础设施（~1 周）

1. new-api `go.mod` 引入 bamboo-messages 新 tag，`go build ./...` 通过
2. `relay/bamboo/` 包骨架（5 文件，5.1-5.7 节）
3. `provider_factory.go` 实现，覆盖 `APITypeOpenAI` + `APITypeDeepSeek`
4. `bridge.go` 的 `ChatRelay` + `doStreamRelay` + `doCompleteRelay`
5. `codec_map.go` / `errors.go` / `usage.go`
6. `setting/model_setting/bamboo_setting.go` 灰度开关
7. 单元测试：`relay/bamboo/*_test.go`

**验收**：`ChatRelay` 以 OpenAI 入口格式调通 DeepSeek（流式 + 非流式），单测全绿。

### Phase 2：接入 TextHelper（~1 周）

1. `compatible_handler.go` 三段式替换为 `bamboo.ChatRelay`，原三段式保留为 `originalTextRelay`
2. 处理 pass-through 旁路（L97-107）与 chatCompletionsViaResponses 旁路（L74-93）
3. `ErrUnsupportedProvider` fallback
4. 灰度白名单：先 `APITypeOpenAI` + `APITypeDeepSeek`
5. 集成测试：OpenAI 入口 → DeepSeek/Moonshot/SiliconFlow

**验收**：OpenAI 格式入口端到端调通，计费准确，未覆盖渠道正确 fallback。

### Phase 3：接入其余 3 个 Helper（~1 周）

1. `ClaudeHelper`（保留 thinking 后缀适配）
2. `GeminiHelper`（保留 thinking budget 适配）
3. `ResponsesHelper`（保留 Responses→Chat fallback 逻辑）
4. 扩大灰度到 Anthropic/Gemini/Codex

**验收**：跨协议场景调通（Claude 入口 → DeepSeek 上游、Gemini 入口 → OpenAI 上游）。

### Phase 4：稳定性与边缘补齐（持续）

1. reasoning token 计费补齐（usage.go）
2. `ParamOverride`/`HeadersOverride` 透传到 provider
3. panic recovery + goroutine 泄漏检查（pprof）
4. 压测（并发流式连接）
5. 全 ApiType 灰度开启，观察 1-2 周

---

## 十一、验收检查清单

- [ ] bamboo PR-B1：`go build ./...` 通过、`grep gin go.mod` 无命中、测试全绿
- [ ] bamboo PR-B2：外部模块能 `import ".../bamboo-messages/bamboo"` 不报 internal 错误
- [ ] new-api `go build ./...` 无编译错误，gin 版本保持 v1.9.1 不变
- [ ] `go test ./relay/bamboo/...` 单元测试通过
- [ ] OpenAI 入口 → DeepSeek 上游：流式 + 非流式 + 工具调用调通
- [ ] Claude 入口 → DeepSeek 上游（跨协议）调通
- [ ] Gemini 入口 → OpenAI 上游（跨协议）调通
- [ ] 未覆盖渠道（AWS/讯飞/腾讯）正确 fallback 到原生链路
- [ ] `usage.PromptTokens`/`CompletionTokens`/`TotalTokens` 统计正确
- [ ] 客户端断开后无 goroutine 泄漏（pprof 验证）
- [ ] 灰度开关生效：关闭时走 fallback，< 1 分钟回滚
- [ ] 计费金额与改造前一致（同请求对比）
- [ ] TextHelper 的 pass-through / chatCompletionsViaResponses 旁路行为不变

---

## 十二、附录

### 附录 A：前置文档的编译级错误修正对照表

| 错误引用（文档草案） | 正确符号 | 位置 |
|---------------------|---------|------|
| `types.NewAPIError(err, code)` 当构造函数 | `NewAPIError` 是结构体类型；构造用 `types.NewError` / `types.NewErrorWithStatusCode` / `types.NewOpenAIError` | `types/error.go:90,244,299,266` |
| `ErrorCodeConvertResponseFailed` | 不存在；用 `ErrorCodeConvertRequestFailed`(error.go:65) 或 `ErrorCodeBadResponse`(74) | `types/error.go` |
| `bamboocodec.ErrorTypeParse` / `ErrorTypeSerialize` | 不存在；实际枚举 `ErrInvalidRequest`/`ErrProviderError`/`ErrAuthError`/`ErrRateLimit`/`ErrInternal` | `bamboo/codec/errors.go:9-22` |
| `Usage.CacheCreation` / `CacheRead` | `CacheCreationInputTokens` / `CacheReadInputTokens` | `bamboo/response.go:68-69` |
| `dto.Usage.CompletionTokensDetails` | `CompletionTokenDetails`（无 s） | `dto/openai_response.go:232` |

### 附录 B：new-api 侧关键事实核对结果

- `Adaptor` 接口实有 **15 个方法**（`relay/channel/adapter.go:15`），不止 4 个 Convert。但本方案走 bridge 内核替换，**不实现 Adaptor 接口**，故不受此影响。
- `RelayInfo.ApiType`/`ChannelBaseUrl`/`ApiKey` 不在 `RelayInfo` 直接字段，而在嵌入的 `*ChannelMeta`（经字段提升访问）。`ApiType` 由 `common.ChannelType2APIType(channelType)`（`common/api_type.go:5`）推导。
- `PostTextConsumeQuota` 签名：`(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.Usage, extraContent []string)`（`service/text_quota.go:322`），`extraContent` 是 `[]string`。
- `DoResponse` 返回 `usage any`，调用方做 `.(*dto.Usage)` 断言（`compatible_handler.go:214/220`）。
- `setting/model_setting/` 注册模式：`config.GlobalConfig.Register(name, ptr)`，见 `setting/model_setting/global.go:59-62`。

### 附录 C：bamboo 侧关键事实核对结果

- `bamboo/codec/codec.go:26` `Codec` 接口实有 **5 个方法**（多了 `Format() FormatType`）。
- `bamboo/stream.go` `StreamEvent` 实有 **7 字段**，`Type` 枚举 **8 个值**（`EventMessageStart`/`EventContentBlockStart`/`EventContentBlockDelta`/`EventContentBlockStop`/`EventMessageDelta`/`EventMessageStop`/`EventPing`/`EventError`）。
- 4 个 codec 的实现规模表（request 字段数 / stream.go 行数 / SerializeResponse / SerializeError / 单测）**全部精确命中**。
- 4 个 provider 构造函数（行号、函数名、WithAPIKey/WithBaseURL/WithHeader）**全部精确命中**。
- `ProviderExtra` 透传机制（`bamboo/config.go:40` + `provider/type.go:200`）真实可用。

### 附录 D：高精度复审记录（2026-06-18）

复审方式：3 个 Explore agent（new-api 侧 11 验证点 + bamboo 侧 4 致命点）+ 主 agent 跨库补查，以编译级/行为级挑刺心态独立验证 spec 全部代码骨架。

**总判定：OK-GO（with minor fixes）** —— 无致命缺陷，5 处编译级小问题已全部修正。

**关键担忧点经深挖全部通过**：

| 担忧点 | 验证结果 |
|--------|---------|
| `errors.Is(relayErr, ErrUnsupportedProvider)` 能否触发 fallback | ✅ `*types.NewAPIError` **已实现 `Unwrap() error`**（`types/error.go:101-107`），fallback 链路通畅 |
| `client.Chat` 参数类型 vs `RelayRequest` 字段类型 | ✅ 完全匹配（`[]BambooMessage`/`string`/`*RequestConfig`），无 `[]provider.Message` 误用 |
| xerr 最小替代是否丢字段 | ✅ bamboo 转换链只读 `.Error()`（convert.go:388 唯一访问点），不丢数据 |
| `BambooError` 是否实现 error 接口 | ✅ `bamboo/errors.go:46` 有 `Error() string` |
| 移除 base-go 后 gin 是否彻底消失 | ✅ anthropic/openai/genai 三个 AI SDK 均不含 gin，gin 唯一来源是 base-go |

**已修正的 5 处编译级问题**：

| # | 位置 | 问题 | 修正 |
|---|------|------|------|
| 1 | 5.2 bridge.go import | `"io"` 未使用 + `relay/common` 双别名重复 import | 删除两者 |
| 2 | 5.4 provider_factory.go import | 用 `types.NewError` 未 import types | 补 `"…/new-api/types"` |
| 3 | 5.6 errors.go switch | `ErrorCodeInvalidAuth`/`RateLimit`/`UpstreamError` 不存在（new-api 无此常量） | 复用 `AccessDenied`/`BadResponse`/`BadResponseStatusCode`；并改 translateCodecError 入参为 `error` 接口 + 内部 `errors.As` 断言 |
| 4 | 5.7 usage.go | `CompletionTokenDetails` 是值类型 struct，`== nil`/`=&OutputTokenDetails{}` 均编译错误 | 改为直接 `usage.CompletionTokenDetails.ReasoningTokens += delta` |
| 5 | 5.2/5.3 errors 传递 | `ParseRequest` 返回裸 `error`，传给 `*CodecError` 参数编译失败 | translateCodecError 入参改 `error` 接口，内部 `errors.As` 断言 |

**实现前必须先验证的点**：

1. bamboo 侧 PR-B1（移除 base-go）/ PR-B2（提升 `internal/provider` → `provider/`）必须先合并并打 tag，否则 new-api 的所有 bamboo import 无法解析。
2. 4 个 Helper 改造时需补 `"errors"` 与 `bamboo` 包 import（改造自然产物）。
3. pass-through 旁路须保留完整 OR 条件（全局 + 渠道级）；chatCompletionsViaResponses 须保留完整 AND 合取。
4. bamboo codec 的 `ParseRequest` 是否正确填充 `relayReq.IsStream`（决定流式/非流式分支）。
5. 若需精确的 auth/rateLimit/upstream 错误语义，实现阶段可向 types 包新增 3 个 ErrorCode 常量（当前复用现有常量）。

---

*spec 结束。核心价值：用 bamboo 协议无关中间表示，将 new-api 对话中继的 O(N×M) 协议转换矩阵降为 O(N+M)，并通过移除 base-go 一箭双雕地消除 gin 冲突。下一步：用户审查本 spec → 批准后转入 writing-plans 制定详细实现计划。*
