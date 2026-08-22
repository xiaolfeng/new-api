# middleware 知识库

## 概述

HTTP 横切关注点。处理鉴权、限流、渠道分发、审计、CORS、日志与国际化。`distributor.go` 是 relay 链路枢纽：选渠道并注入上下文。

## 目录结构

```text
middleware/
├── auth.go                       # TokenAuth / UserAuth / AdminAuth / RootAuth
├── auth_origin.go                # 请求来源校验
├── distributor.go                # 选渠道 + 注入 context
├── rate-limit.go                 # 用户级限流（IP / 用户，Redis 或内存）
├── model-rate-limit.go           # 模型级限流（MRRL / MRRLS）
├── email-verification-rate-limit.go
├── secure_verification.go        # 敏感操作二次确认
├── turnstile-check.go            # Cloudflare Turnstile
├── audit.go                      # 管理 / root 写操作审计
├── trusted_proxies.go            # 可信代理
├── logger.go / request-id.go / request_body_limit.go / body_cleanup.go
├── cors.go / gzip.go / i18n.go / recover.go
├── cache.go / disable-cache.go
├── header_nav.go / jimeng_adapter.go / kling_adapter.go
├── performance.go / stats.go / utils.go
└── *_test.go
```

## 导航指南

| 任务 | 位置 | 说明 |
|------|------|------|
| 理解如何选到渠道 | `distributor.go` `Distribute()` | 指定渠道 → 亲和性 → 随机满足的渠道 |
| 加 / 改鉴权 | `auth.go` | API key 走 `TokenAuth`；控制台走 session 角色链 |
| Token 鉴权流程 | `TokenAuth()` → `SetupContextForToken()` | 解析 key → 查 token → 注入 context |
| 用户级限流 | `rate-limit.go` | Redis 滑动窗口，否则内存限流器 |
| 模型级限流 | `model-rate-limit.go` | `MRRL` 请求数 / `MRRLS` 成功数 |
| 管理 API 审计 | `audit.go` | 内聚在 Admin/Root 鉴权链路 |
| 自定义 header | `header_nav.go` | 含 jimeng / kling 适配 |
| 后端语言 | `i18n.go` | `Accept-Language` / cookie |

## 约定

### 鉴权

- **角色分级**：`UserAuth`（role ≥ 1）→ `AdminAuth`（≥ 10）→ `RootAuth`（≥ 100）。统一入口是 `authHelper(c, minRole)`。细粒度资源动作走 `service/authz`，不要在中间件再发明一套字符串权限。
- **双认证**：Web 控制台用 session；API 用 `Authorization: Bearer <key>`，由 `SetupContextForToken` 注入用户与令牌。
- **宽松只读**：`TokenAuthReadOnly` 用于只读查询。
- **审计内聚**：管理 / root 写操作的审计兜底在鉴权链路完成，不分散到 controller。

### 分发

- **选渠道路径**：指定渠道（`ContextKeyTokenSpecificChannelId`）→ 亲和性 → `CacheGetRandomSatisfiedChannel`。
- **上下文注入**：`SetupContextForSelectedChannel` 写入 channel + model，供 relay handler 使用。
- **亲和性**：选中后 `service.RecordChannelAffinity`。
- **不做协议转换**：distributor 只选渠道、注上下文。

### 限流

- **两级独立**：用户级 + 模型级都要过。
- **Redis 优先**，无 Redis 降级 `common.InMemoryRateLimiter`。
- **`maxCount == 0` 表示不限**。

### 通用

- **顺序**：`recover` → `request-id` → `cors` → `gzip` → `logger` → 鉴权 → 限流 → 业务。鉴权必须在限流之前。
- **JSON 走 `common.*`**。

## 反模式

- ❌ 在 distributor 里做协议转换或调上游。
- ❌ 在 controller 里重复鉴权——信任 context 中的用户 / 令牌。
- ❌ 把限流放到鉴权前面。
- ❌ 新增限流维度只实现 Redis 或只实现内存。
- ❌ 把审计逻辑拆到各个 controller。
- ❌ 用 `encoding/json`。

## 调试路径

1. 401 / 403 → `auth.go` 的 `TokenAuth` / `authHelper`；细粒度拒绝再看 `service/authz`。
2. 「无可用渠道」 → `Distribute()` → `model/channel_satisfy.go` → `ability.go`。
3. 渠道选错 → 亲和性分支 + `service/channel_affinity.go`。
4. 限流误触发 → `rate-limit.go` 或 `model-rate-limit.go` 的 Redis key 与 maxCount。
5. 管理 API 无审计 → `audit.go` 是否挂在 Admin/Root 链上。
6. 自定义 header 无效 → `header_nav.go` 或 jimeng / kling 适配器。
