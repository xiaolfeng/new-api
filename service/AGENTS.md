# service 知识库

## 概述

业务逻辑层。承接 `controller/`，编排 `model/` 与外部调用，是计费、渠道选择、鉴权授权、令牌统计、任务轮询的所在地。分层位置：Router → Controller → **Service** → Model。

## 目录结构

```text
service/
├── billing.go / billing_session.go / billing_usage.go
├── quota.go / text_quota.go / tiered_settle.go
├── violation_fee.go / task_billing.go / task_polling.go / task.go
├── channel.go / channel_select.go / channel_affinity.go
├── log_record.go / log_info_generate.go
├── token_counter.go / token_estimator.go / tokenizer.go
├── auth_session.go / auth_token.go / auth_cleanup.go
├── system_task.go / system_instance.go
├── convert.go / request_converter.go / openai_chat_responses_compat.go
├── http.go / http_client.go / protected_fetch_client.go
├── sensitive.go / group.go / midjourney.go / webhook.go
├── epay.go / waffo_pancake.go / funding_source.go
├── authz/                     # Casbin 授权（角色 / 策略 / 渠道资源）
├── passkey/                   # WebAuthn 业务
└── *_test.go
```

## 导航指南

| 任务 | 位置 | 说明 |
|------|------|------|
| 理解计费主链路 | `billing.go` → `billing_session.go` | `PreConsumeBilling` 把 session 挂到 `relayInfo.Billing`，`SettleBilling` 结算 |
| 配额 / token 换算 | `quota.go` | TokenDetails、ratio、quota；换算必须走 `common/quota_math.go` |
| 阶梯 / 动态计费 | `tiered_settle.go` + `pkg/billingexpr/` | 先读 `pkg/billingexpr/expr.md` |
| 饱和审计 | `log_info_generate.go` `attachQuotaSaturation` | 写入 `other.admin_info.quota_saturation` |
| 渠道选择 | `channel_select.go` | `RetryParam`、`CacheGetRandomSatisfiedChannel` |
| 渠道亲和性 | `channel_affinity.go` | Session 粘性路由 |
| 权限判定 | `authz/` | Casbin SyncedEnforcer；`UserSubject` / `RoleSubject` |
| token 预估 | `token_estimator.go` | 预扣前估算 |
| 协议互转 | `relaykit/relayconvert`（不要在本目录重写） | service 只做宿主侧薄封装 |
| Passkey | `passkey/` | WebAuthn 注册 / 认证 |

## 约定

- **计费会话化**：新代码走 `BillingSession`（`PreConsumeBilling` → `SettleBilling`）。`PostConsumeQuota` 只给无 session 的按次计费兜底。
- **预扣 → 结算 → 退款**：`delta = actual - preConsumed`，正补扣、负返还。饱和的超大预扣必须以额度不足失败，禁止静默回绕。
- **配额换算集中**：用 `common.QuotaFromFloat` / `QuotaRound` / `QuotaFromDecimal` 及其 `*Checked` 变体。计费路径捕获 `QuotaClamp`，写日志前调用 `attachQuotaSaturation`。
- **乘数走 AddOtherRatio**：禁止直接写 `PriceData.OtherRatios`。
- **阶梯计费必读 expr.md**：改 `tiered_settle.go` 或 `billingexpr` 前先读设计文档。`BuildTieredTokenParams` 负责 GPT / Claude token 归一化，不要绕过。
- **渠道选择经缓存**：走 `CacheGetRandomSatisfiedChannel`，不要直接 `model.GetRandomSatisfiedChannel`。
- **授权在 authz**：角色与策略经 Casbin，不要在 controller 里散落字符串权限判断。
- **JSON 走 `common.*`**。文件命名：扁平业务文件；只有 `authz/`、`passkey/` 这种子系统才建子目录。

## 反模式

- ❌ 在 service 里 `c.JSON()` 写响应——返回数据/错误，由 controller 输出。
- ❌ 绕过 `BillingSession` 自己拼预扣/结算。
- ❌ `int(float64(quota) * ratio)` 这类裸强转。
- ❌ 改阶梯计费却不读 `pkg/billingexpr/expr.md`。
- ❌ 在本目录重做 OpenAI ↔ Claude ↔ Gemini 转换——那是 `relaykit/relayconvert` 的职责。
- ❌ 用 `encoding/json`；写只兼容单一数据库的查询。

## 调试路径

1. 计费金额不对 → `SettleBilling` 的 `delta` → `quota.go` 的 ratio → 阶梯则看 `tiered_settle.go` + `billingexpr`。
2. 预扣后未返还 → `billing_session.go` 的 `delta < 0` 分支。
3. 饱和未体现在日志 → `relayInfo.QuotaClamp` 是否被 `attachQuotaSaturation` 写入。
4. 渠道选错 / 不选 → `channel_select.go` → `model/channel_satisfy.go`。
5. 权限拒绝异常 → `authz/enforcer.go` 与 `casbin_rule` / 角色种子。
6. token 统计偏差 → 区分 `token_estimator.go`（预估）与响应 usage（实际）。

## 引用

无子级 `AGENTS.md`。协议转换见 [relaykit](../relaykit/AGENTS.md)。
