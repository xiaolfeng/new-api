# service 知识库

## 概述

业务逻辑层。承接 `controller/` 的请求，编排 `model/` 数据访问与外部调用，是计费、渠道选择、令牌统计、任务轮询等核心业务规则的所在地。分层架构中处于 Controller 与 Model 之间（Router → Controller → **Service** → Model）。

## 目录结构

```text
service/
├── billing.go                 # 计费入口：PreConsumeBilling / SettleBilling
├── billing_session.go         # BillingSession：封装预扣 → 结算 → 退款的会话状态
├── pre_consume_quota.go       # 预扣费实现
├── quota.go                   # 配额计算核心（TokenDetails、ratio 应用、quota 换算）
├── text_quota.go              # 文本请求的 quota 计算
├── tiered_settle.go           # 阶梯计费结算（接入 pkg/billingexpr）
├── tool_billing.go            # 工具调用计费（web_search / file_search / image_gen）
├── violation_fee.go           # 违规费用处理
├── task_billing.go            # 异步任务（画图/视频）计费
├── task_polling.go            # 异步任务轮询回调
├── channel.go                 # 渠道管理业务逻辑
├── channel_select.go          # 渠道选择（RetryParam、CacheGetRandomSatisfiedChannel）
├── channel_affinity.go        # 渠道亲和性（Session Affinity）
├── convert.go                 # 请求格式转换辅助
├── log_record.go              # 日志记录（写 Log + LogRecord 结构化数据）
├── log_info_generate.go       # 日志摘要生成
├── token_counter.go           # token 计数
├── token_estimator.go         # token 预估（请求前）
├── tokenizer.go               # 分词器
├── quota → billing 链路        # 见下方"导航指南"
├── sensitive.go               # 敏感词检测
├── group.go                   # 分组管理
├── http.go / http_client.go   # HTTP 客户端封装
├── image.go / audio.go        # 多模态处理
├── midjourney.go              # Midjourney 任务逻辑
├── rankings.go                # 排行榜数据
├── webhook.go                 # Webhook 通知
├── epay.go / waffo_pancake.go # 支付集成
├── openaicompat/              # OpenAI 兼容层（协议互转）
├── passkey/                   # Passkey/WebAuthn 业务逻辑
└── *_test.go                  # 单元测试（与实现同目录）
```

## 导航指南

| 任务 | 位置 | 说明 |
|------|------|------|
| 理解计费主链路 | `billing.go` → `billing_session.go` | `PreConsumeBilling` 创建 session 存入 `relayInfo.Billing`，`SettleBilling` 结算 |
| 预扣费实现 | `pre_consume_quota.go` | 请求前预估并扣除配额 |
| 配额/token 换算 | `quota.go` | `TokenDetails`、ratio 乘数、quota 计算 |
| 阶梯/动态计费 | `tiered_settle.go` + `pkg/billingexpr/` | 先读 `pkg/billingexpr/expr.md`（Rule 7） |
| 工具调用计费 | `tool_billing.go` | web_search / file_search / image_generation 的按次计费 |
| 渠道选择逻辑 | `channel_select.go` | `RetryParam`、重试、`CacheGetRandomSatisfiedChannel` |
| 渠道亲和性 | `channel_affinity.go` | X-Session-Id 粘性路由 |
| 写日志 | `log_record.go` | 结构化日志（record / full_log）写入 |
| token 预估（请求前） | `token_estimator.go` | 用于预扣费前的消耗预估 |
| OpenAI ↔ 其他格式互转 | `openaicompat/` | chat_completions ↔ responses 等协议转换 |
| Passkey 业务 | `passkey/` | WebAuthn 注册/认证流程 |

## 约定

- **计费会话化**：所有计费走 `BillingSession`（`billing.go` 的 `PreConsumeBilling` → `SettleBilling`），session 挂在 `relayInfo.Billing` 上。旧路径 `PostConsumeQuota` 仅作为按次计费等无 session 场景的兜底，新代码优先用 session。
- **预扣 → 结算 → 退款三段式**：请求前 `PreConsumeBilling` 预扣，请求后 `SettleBilling` 按实际消耗补扣或返还（`delta = actual - preConsumed`，正补扣、负返还）。日志里会记录差额方向。
- **阶梯计费必读 expr.md**：改动 `tiered_settle.go` 或任何 `billingexpr` 相关逻辑前，必须先读 `pkg/billingexpr/expr.md`（根 AGENTS.md Rule 7）。
- **token 归一化**：`BuildTieredTokenParams`（`tiered_settle.go`）处理 GPT 格式（prompt/completion 含子类）与 Claude 格式（text-only）的差异，按表达式是否引用子类变量决定是否扣除——不要绕过这个归一化直接用原始 token 数。
- **渠道选择经缓存**：选渠道走 `CacheGetRandomSatisfiedChannel`（带缓存），不要直接查 `model.GetRandomSatisfiedChannel`。
- **JSON 走 common.\***：marshal/unmarshal 用 `common.Marshal` / `common.Unmarshal`，禁止 `encoding/json`（根 AGENTS.md Rule 1）。
- **文件命名**：业务领域用扁平文件（`billing.go`、`channel.go`），仅复杂的子系统用子目录（`openaicompat/`、`passkey/`）。

## 反模式

- ❌ 在 service 里直接操作 `*gin.Context` 的响应写入或 `c.JSON()`——service 返回数据/错误，由 controller 写响应。
- ❌ 绕过 `BillingSession` 自己拼预扣/结算逻辑——会导致日志、退款、阶梯计费不一致。
- ❌ 改阶梯计费却不读 `pkg/billingexpr/expr.md`——会破坏 token 归一化与表达式契约（Rule 7）。
- ❌ 用 `encoding/json`（Rule 1）。
- ❌ 写只兼容单一数据库的查询（Rule 2）——尽管 service 层多数走 GORM 抽象，但凡涉及 `model.*` 的 raw 查询仍需三库兼容。

## 调试路径

1. 计费金额不对 → `billing.go` `SettleBilling` 看 `delta` 计算 → `quota.go` 看 ratio 应用 → 若阶梯计费，转 `tiered_settle.go` + `pkg/billingexpr/`。
2. 预扣费后未返还 → `billing_session.go` 的结算/退款分支，确认 `delta < 0` 路径执行了返还。
3. 渠道选错 / 不选 → `channel_select.go` 的 `CacheGetRandomSatisfiedChannel` → `model/channel_satisfy.go` 看能力匹配。
4. token 统计偏差 → `token_counter.go` / `token_estimator.go`，区分预估（请求前）与实际（响应 usage）。
5. 日志缺失结构化字段 → `log_record.go` 的 record/full_log 写入逻辑。
6. 工具调用未计费 → `tool_billing.go`，确认 `other` 里的 call_count/price 字段被读取。

## 引用

无子级 `AGENTS.md`。上级导航见根 [`AGENTS.md`](../AGENTS.md)。
