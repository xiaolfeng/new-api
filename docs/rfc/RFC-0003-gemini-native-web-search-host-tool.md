# RFC-0003：Gemini 原生 googleSearch 接入 Host-Tool


| 字段       | 值                                                                                                                                                          |
| -------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **RFC**  | RFC-0003                                                                                                                                                   |
| **标题**   | Gemini 原生 `googleSearch` / `googleSearchRetrieval` 归一为 `host.web_search`                                                                                  |
| **作者**   | 筱锋 / AI-assisted                                                                                                                                            |
| **日期**   | 2026-08-26                                                                                                                                                 |
| **状态**   | Draft                                                                                                                                                      |
| **文档类型** | 可评审 RFC（非实现）                                                                                                                                               |
| **最终路径** | [`docs/rfc/RFC-0003-gemini-native-web-search-host-tool.md`](docs/rfc/RFC-0003-gemini-native-web-search-host-tool.md)                                       |
| **前置文档** | [`docs/rfc/RFC-0001-bamboo-host-side-tools.md`](docs/rfc/RFC-0001-bamboo-host-side-tools.md)、[`docs/rfc/RFC-0002-client-profile-web-search-return.md`](docs/rfc/RFC-0002-client-profile-web-search-return.md) |
| **落地注记** | RFC-0001 把 Gemini `google_search` 标成「不归一」。bamboo-messages Gemini codec 在 host tools 关着时也会丢掉该对象，bamboo 路径上的原生 Grounding 实际已经失效。本 RFC 用 Host-Tool 把搜索能力补回来。 |
| **影响范围** | 仅 `EnableBambooRelay=true` **且** `enable_host_tools=true` 的 Gemini 入口（及所有入口里名为 `googleSearch` 的 function）。原生 `originalGeminiRelay` 零改动。                      |


> **阅读约定**
>
> - **§ Key Decisions → v1 Locked** 是实现必须遵守的契约。
> - 对 RFC-0001 的修订集中在 **§ Amendments to RFC-0001**；未列入的锁定项保持有效（尤其 D1 原生三段式 no-op、D2 A-thin、D4 混合 uses passthrough、禁止 hop3、执行仍在 `relay/bamboo/hosttool`、Plan 类型仍在 `relay/common`）。
> - **不改** `relaykit/`，**不升级** bamboo-messages。codec 继续只认 `functionDeclarations`；inspect 从 Helper remash 的 `entryBytes` 把 server-side 搜索认回来。
> - GroundingMetadata 合成、`urlContext` → `host.web_fetch`、hop1 无 `tool_use` 时网关主动搜，都不是 v1。

---

## Overview

Gemini 原生客户端（Google GenAI SDK / `/v1beta/models/{model}:generateContent`）用 server-side 工具声明搜索，而不是 OpenAI/Claude 那种 function：

```json
{
  "contents": [{"role": "user", "parts": [{"text": "查一下最新比赛"}]}],
  "tools": [{"googleSearch": {}}]
}
```

或带动态检索：

```json
{
  "tools": [{
    "googleSearchRetrieval": {
      "dynamicRetrievalConfig": {
        "mode": "MODE_DYNAMIC",
        "dynamicThreshold": 0.3
      }
    }
  }]
}
```

RFC-0001 明确把这条排除：别名表写「不归一」，Non-Goals 写「Gemini `google_search`」。当时的理由是原生路径已有 `google_search` 加价（默认 $14/1K），且 bamboo Gemini codec 只走 `functionDeclarations`。

现状比 RFC-0001 更糟：`bamboo-messages@v1.0.7` 的 `codec/gemini` `geminiTool` **只有** `functionDeclarations`，`parseTools` 丢掉 `googleSearch` / `googleSearchRetrieval` / `urlContext` / `codeExecution`。上游 `provider/gemini/buildTools` 永远序列化成 `[{"functionDeclarations":[...]}]`。

因此只要 `EnableBambooRelay=true`，**即使 `enable_host_tools=false`**，Gemini 原生搜索也不会到达 Google Grounding。本 RFC 不修复「host tools 关时的 codec 丢工具」（那是 bamboo-messages 合同），只在 host tools 开时把声明认成 `host.web_search`，注入 function，走 RFC-0001 Mode A-thin。

Gemini SDK 要的是一次 `generateContent` 拿回 Candidate 文本，不是 Claude helper 的 `server_tool_use`，也不是 Responses 的 `web_search_call`。所以 **不**走 `doHostClaudeSearch` / `doHostBuiltinResponses` 短路。

---

## Background

### 源码事实（2026-08-26 核对）

| 事实 | 位置 | 含义 |
| --- | --- | --- |
| Gemini Helper 走 bamboo | `relay/gemini_handler.go` `EnableBambooRelay` 时 `Marshal(request)` → `ChatRelay(..., RelayFormatGemini, bodyBytes)` | inspect 看到的是 DTO remash，不是 HTTP raw body |
| `Tools` 是 `json.RawMessage` | `relaykit/dto/gemini.go` `GeminiChatRequest.Tools` | remash **保留**客户端原始键（camelCase 或 snake_case） |
| DTO 已有 Google 检索字段 | `GeminiChatTool.GoogleSearch` / `GoogleSearchRetrieval` | RFC-0001 §4「无 Google 内置检索字段可救」过时 |
| inspect 只扫 functionDeclarations | `relay/bamboo/hosttool/inspect.go` `RelayFormatGemini` | `googleSearch` 不产生 `HostToolDecl` |
| codec 丢掉 server-side 工具 | `bamboo/codec/gemini/request.go` `geminiTool` / `parseTools` | `Config.Tools` 里没有 googleSearch |
| 上游只发 functionDeclarations | `provider/gemini/tools.go` `buildTools` | 注入 function 后，官方 `{"googleSearch":{}}` **不会**出现在上游 body |
| 原生加价 | `relay/channel/gemini/relay-gemini.go` `markGeminiGoogleSearchCall` → `gemini_google_search_call` → `collectToolSurchargeItem(..., google_search)` | **只**在 `originalGeminiRelay` 的 adaptor 路径。bamboo 不设这个 flag |
| 无 Gemini inspect 单测 | `inspect_test.go` | 本 RFC 必须从真实 DTO remash 补 |

### 痛点

1. bamboo 开、host tools 关：搜索被 codec 静默丢掉，客户端以为在搜。
2. bamboo 开、host tools 开：inspect 仍看不见 `googleSearch`，注入失败，行为与 1 相同。
3. 若哪天 codec 开始透传 `googleSearch`：官方 Gemini 渠道会触发 Grounding 加价；非 Gemini 渠道会因无法识别对象而 400。Host-Tool 必须在那之前把声明改写成 function。

---

## Goals & Non-Goals

### Goals

1. `enable_host_tools=true` 时，Gemini 入口的 `googleSearch` / `googleSearchRetrieval`（含 snake_case、tools 单 object）归一为 `CanonicalWebSearch`。
2. 注入带 `query` schema 的 function，走既有 hop1 → `DecideAction` → `Execute` → hop2（Mode A-thin）或 Mode B dump。
3. 上游请求不得再带 `googleSearch` / `googleSearchRetrieval` 键，避免官方 Grounding。
4. 成功检索计网关 `web_search`（$10/1K 默认）；失败不计；不得与 `google_search` 双计。
5. `tool_logs.original_name` 保留客户端声明名；`canonical = host.web_search`。
6. `originalGeminiRelay` 仍可走官方 Grounding + `gemini_google_search_call`。

### Non-Goals

- `urlContext` → `host.web_fetch`（codec 同样丢掉，另开 RFC）。
- `codeExecution` / `file_search` / `image_generation` / Vertex `enterpriseWebSearch` / `/v1/alpha/search`。
- 改 `relaykit/`、升级 bamboo-messages、让 codec 透传 googleSearch。
- 合成 `groundingMetadata.groundingChunks`（当前 DTO 只有 `webSearchQueries`）。
- 原生 adaptor 路径拦截。
- 新 ClientProfile / 新 ReturnKind。
- hop1 没有 `tool_use` 时网关主动搜一遍。
- 把 `dynamicThreshold` 做成「低于阈值就不搜」——声明了就视为启用 host 搜索，是否 `functionCall` 交给模型。

---

## Key Decisions

### v1 Locked


| # | 决策 | 锁定值 | 理由 |
| --- | --- | --- | --- |
| G1 | 作用路径 | 只做 bamboo `ChatRelay` + `enable_host_tools`；原生三段式 no-op | RFC-0001 D1。本 RFC 不顺手修 Gemini pass-through |
| G2 | 执行模型 | Mode A-thin（`host_tool_mode=loop`）或运营显式 `return`。**禁止** Gemini 专用短路合成 | Gemini SDK 要 Candidate 文本。Claude/Responses helper 帧它解析不了 |
| G3 | 注入名 | `injectedTool` 继续用 `OriginalName`（`googleSearch` / `google_search` / `googleSearchRetrieval` / `google_search_retrieval`） | 少一层映射；`tool_logs` / Mode B dump / `DeclByOriginal` 与声明一致。上游是 functionDeclarations，不是 `googleSearch:{}` 键 |
| G4 | 计费名 | `BillingName = web_search`，不是 `google_search` | 网关代跑 SearXNG/Parallel/Exa，不是 Google Grounding。`HostToolExecuted` 时忽略 `gemini_google_search_call` |
| G5 | 空 `tool_use` | hop1 无 host uses → 现有 passthrough。v1 **不**强制搜 | 官方 googleSearch 是模型决定是否 grounding。启发式「像搜索就搜」会误伤问候语 |
| G6 | 扫描形态 | camelCase + snake_case；tools 数组或单 object；同一 object 上 googleSearch 与 functionDeclarations 并存；`googleSearch: null` 视为未声明 | 对齐 `GeminiChatRequest.GetTools` 与 SDK 真实 JSON |
| G7 | 域名过滤 | `excludeDomains` / `exclude_domains` / `blocked_domains` → `BlockedDomains`。`dynamicThreshold` 忽略 | 复用 `parseDomainFilters`；阈值不是 host-tool 合同 |
| G8 | 别名范围 | `CanonicalFromName` / `CanonicalFromType` 对 **所有入口格式** 生效 | OpenAI function 名叫 `googleSearch` 时也必须拦截，否则 `to_gemini_chat_req.go` 会把它翻成官方 `GoogleSearch:{}` |
| G9 | Grounding 合成 | v1 不做 | 要扩 `relaykit` DTO。另开 RFC |


### 关闭的问题

- **Q-G1 注入名用 `web_search` 还是 OriginalName？** OriginalName（G3）。若预发上游因保留名 400，再改注入名并做映射；不在 v1 预留双名。
- **Q-G2 hop1 不 call 是否强制搜？** 否（G5）。预发看 `admin_info.host_tools.executed`；调用率低另开 RFC。
- **Q-G3 计费用 $14 google_search 还是 $10 web_search？** `web_search`（G4）。运营可用 `tool_price` 覆盖。

---

## Amendments to RFC-0001

本 RFC **修订**下列 RFC-0001 项。未列出的锁定项保持有效。


| RFC-0001 | 本 RFC 修订 | 理由 |
| --- | --- | --- |
| **Non-Goals**「Gemini `google_search`、`file_search`、…」 | **删掉** `google_search` 这一项。`file_search` / `image_generation` / `/v1/alpha/search` 仍排除 | 本 RFC 覆盖 googleSearch 声明。其它 Gemini 内置工具仍不归一 |
| **§ 源码级协议缺口** Gemini 行「`google_search` 不在本 RFC」 | inspect **必须**扫 `googleSearch` / `googleSearchRetrieval`（及 snake_case）。codec 仍只解析 functionDeclarations——这是已知损失，靠 entryBytes 补回 | DTO remash 保得住这些键；只扫 Config.Tools 会漏 |
| **§4 Gemini DTO**「无 Google 内置检索字段可救」 | **勘误**：`GeminiChatTool` 已有字段；`Tools` 是 RawMessage。inspect 读 remash JSON | 2026-08 核对 |
| **§3 别名表** Gemini `google_search` **不归一** | 改为归一 `host.web_search`。见下方别名 | 独立计费那条路只存在于原生 adaptor |
| **附录 A / ClientProfile** | **不**新增 Gemini ReturnKind。generic / 全局 `host_tool_mode` | Gemini 出口是 Candidate 文本 |

别名（trim + 大小写不敏感，`normalizeName` **不**剥连字符）：

```text
aliases[host.web_search] += googlesearch, google_search, google-search,
                            googlesearchretrieval, google_search_retrieval, google-search-retrieval
type prefix google_search / googlesearch → host.web_search
```

RFC-0001 原有 `websearch` / `web_search` / `web-search` / `web_search_preview` 与 `web_search_` 前缀 **保持**。

---

## Proposed Design

### 1. Inspect

`scanEntryBytes` 的 `RelayFormatGemini`：

1. `geminiToolObjects(root["tools"])`：`[]any` / `[]map[string]any` 走现有 `asMapSlice`；`map[string]any` 包成单元素切片。
2. 每个 tool 先 `scanGeminiServerSearch`，再扫 `functionDeclarations`；若空则扫 `function_declarations`。
3. `scanGeminiServerSearch` 按键顺序：`googleSearch`、`google_search`、`googleSearchRetrieval`、`google_search_retrieval`。值 `nil` 跳过。命中则 `Source=server_type`，`BillingName=web_search`，`OriginalName` 等于 JSON 键。对 object 值跑 `parseDomainFilters`（含 camelCase `excludeDomains` / `allowedDomains`）。
4. 同一 canonical 去重仍走 `InspectAndRewrite` 现有逻辑：第一条保留，其它 OriginalName 进 `Stripped`。

`rewriteTools` **不改**。codec 已丢掉 server tool，`byName[OriginalName]` miss，走 `injectedTool`。自定义 function 留在 rewrite 尾部。

验收：`req.Config.Tools` 只有 bamboo function；语义上对应上游 `functionDeclarations`，**没有** `googleSearch` 对象键。禁止再写一份 entryBytes strip——上游根本不发 entryBytes。

### 2. 执行与回写

不改 `loop.go` / `execute.go` / `bridge.go` 短路表。

```
GeminiHelper Marshal
  → ParseRequest（codec 丢 googleSearch）
  → InspectAndRewrite（entryBytes 认回，注入 function）
  → hop1
  → 全 host uses：Execute + hop2
  → SerializeResponse → Gemini Candidate
```

混合 uses 仍 passthrough（RFC-0001 D4）。hop1 无 uses 仍 passthrough（G5）。

### 3. 计费

`incrementHostToolBilling` 已按 `Decl.BillingName` 计。Gemini server-search 的 BillingName 是 `web_search`。

`service/text_quota.go`：

```go
if ctx.GetBool("gemini_google_search_call") && !relayInfo.HostToolExecuted {
    items = collectToolSurchargeItem(items, dto.BuildInToolGoogleSearch, 1, summary.ModelName)
}
```

与 Claude 段「HostToolExecuted 时清 `claude_web_search_requests`」同一模式，防止以后合成 grounding 时双计。

原生路径不设 `HostToolExecuted`，`gemini_google_search_call` 行为不变。

### 4. 可观测

注入名 = 声明名时，hop1 `tool_use.Name` 就是 `googleSearch`（或客户端键）。`ToolLog.OriginalName` / `HostToolExecRecord.OriginalName` 不用改 execute.go。`CanonicalFromName` 必须识别这些名字，否则 Canonical 为空、费用列漏计。

---

## Testing Requirements

从真实 DTO `common.Marshal` 出发，不用手写「假想 HTTP body」冒充 remash。snake_case / 单 object 可以用字面 JSON，因为 RawMessage 路径下这就是 Helper 会交给 inspect 的字节。

1. registry：google* 别名 → `host.web_search`；`bash` 仍空。`CanonicalFromType("google_search")` / `"googlesearchretrieval"` 命中。
2. Gemini `googleSearch` remash：`plan.Enabled`，`OriginalName=googleSearch`，`Config.Tools` 恰好一条 function，schema 含 `query`。
3. `googleSearchRetrieval` + `dynamicRetrievalConfig`：同样归一。
4. snake_case `google_search` entryBytes：能识别。
5. `tools` 为单 object `{"googleSearch":{}}`：能识别。
6. `googleSearch: null`：不启用。
7. 去重：`googleSearch` + `functionDeclarations[{name:web_search}]` → 一个 canonical。
8. 混合：`googleSearch` + `get_weather` → plan 启用且 rewrite 后仍留 `get_weather`。
9. host tools 关：不注入。
10. OpenAI Chat function 名 `googleSearch`：也被识别（G8）。
11. `excludeDomains` → `BlockedDomains`。
12. 计费：`HostToolExecuted` + `gemini_google_search_call` 不得出 `google_search` 项；有 host `web_search` CallCount 时只出 `web_search`。原生 flag、无 HostToolExecuted 时仍出 `google_search`（现有测试保持）。

禁止随机 fuzz、sleep、打真 Google。

---

## Risks


| ID | 风险 | 缓解 |
| --- | --- | --- |
| R1 | 模型把 googleSearch 当 grounding，不对注入的 function 发 `functionCall` | v1 接受（G5）。description 写清当前事件必须调用。预发看 `executed` |
| R2 | 上游 Gemini 拒绝名为 `googleSearch` 的 functionDeclaration | 单测锁 bamboo Config。预发抓上游 body。若 400，把注入名改为 `web_search` 并映射 OriginalName |
| R3 | 双计 `web_search` + `google_search` | G4 守卫 |
| R4 | 运营以为开 bamboo 就能用官方 Grounding | 本文写明：host tools 关时搜索仍被 codec 丢掉。要官方 Grounding 只能关 bamboo 或走原生 adaptor |
| R5 | tools 单 object / snake_case 漏扫 | 测试 4、5 |

---

## Follow-ups（不是本 RFC）

1. GroundingMetadata 合成（`webSearchQueries` + `groundingChunks[].web.{uri,title}`）。改 `relaykit` DTO。
2. Gemini server-search-only 且 hop1 无 uses 时，用最后一条 user text 跑 `ExecuteBuiltinRequest` 再 hop2。
3. `urlContext` → `host.web_fetch`。
4. 若 R2 发生：注入名改为 `web_search`。

---

## PR Plan

| PR | 标题 | 文件 | 依赖 |
| --- | --- | --- | --- |
| A | RFC-0003 文档 | `docs/rfc/RFC-0003-...md`；RFC-0001 文首后续链接 | 无 |
| B | 别名 + Gemini inspect + 计费守卫 | `relay/bamboo/hosttool/{registry,inspect}.go` + tests；`service/text_quota.go` + test | A |

C/D 是 Follow-ups，本 RFC 不交付。
