# middleware 知识库

## 概述

HTTP 横切关注点层。处理鉴权、限流、请求分发（distributor）、审计、CORS、日志、国际化等所有"穿过"请求链路的通用逻辑。其中 `distributor.go` 是 relay 链路的关键枢纽——它把 AI 请求路由到具体渠道并注入上下文。

## 目录结构

```text
middleware/
├── auth.go                       # 鉴权核心：TokenAuth / UserAuth / AdminAuth / RootAuth
├── distributor.go                # 渠道分发（relay 链路枢纽）：选渠道 + 注入 context
├── rate-limit.go                 # 用户级请求限流（IP/用户维度，Redis 或内存）
├── model-rate-limit.go           # 模型级请求限流（MRRL/MRRLS，按 model 维度）
├── email-verification-rate-limit.go # 邮箱验证码限流
├── secure_verification.go        # 安全验证（敏感操作二次确认）
├── turnstile-check.go            # Cloudflare Turnstile 人机校验
├── audit.go                      # 管理/root 写操作审计
├── logger.go                     # 请求日志
├── request-id.go                 # X-Request-ID 注入与传递
├── request_body_limit.go         # 请求体大小限制
├── body_cleanup.go               # 请求体回收清理
├── cors.go                       # CORS
├── gzip.go                       # Gzip 压缩
├── i18n.go                       # 后端 i18n 语言解析（Accept-Language）
├── cache.go / disable-cache.go   # 响应缓存控制
├── header_nav.go                 # 自定义 header 导航（含 jimeng/kling 适配器）
├── jimeng_adapter.go / kling_adapter.go # 供应商特定 header 适配
├── performance.go                # 性能监控埋点
├── stats.go                      # 请求统计
├── recover.go                    # panic 恢复
└── utils.go                      # 通用辅助
```

## 导航指南

| 任务 | 位置 | 说明 |
|------|------|------|
| 理解 relay 请求如何选到渠道 | `distributor.go` `Distribute()` | 指定渠道优先 → 亲和性 → 随机满足的渠道 |
| 加/改鉴权 | `auth.go` | `TokenAuth`（API key）、`UserAuth`/`AdminAuth`/`RootAuth`（session + 角色） |
| Token 鉴权流程 | `auth.go` `TokenAuth()` → `SetupContextForToken()` | 解析 key → 查 token → 校验 → 注入 context |
| 用户级限流 | `rate-limit.go` | Redis（`rateLimit:<mark><ip>`）或内存限流器 |
| 模型级限流 | `model-rate-limit.go` | `MRRL`（请求数）/ `MRRLS`（成功数），按 model 维度 |
| 管理 API 审计 | `audit.go` | AdminAuth/RootAuth 写操作的审计兜底（内聚在鉴权链路） |
| 加自定义 header 处理 | `header_nav.go` | 解析自定义 header 注入上下文 |
| 后端语言切换 | `i18n.go` | 解析 `Accept-Language` / cookie 设置语言 |

## 约定

### 鉴权（auth.go）

- **角色分级**：普通用户（`UserAuth`，role ≥ 1）→ 管理员（`AdminAuth`，role ≥ 10）→ 超管（`RootAuth`，role ≥ 100）。`authHelper(c, minRole)` 是统一入口。
- **双认证模式**：
  - **Session 模式**（Web 控制台）：`UserAuth`/`AdminAuth`/`RootAuth` 校验 session 中的用户角色。
  - **Token 模式**（API 调用）：`TokenAuth()` 解析 `Authorization: Bearer <key>`，查 `model.Token`，`SetupContextForToken` 注入用户/令牌到 context。
- **宽松只读鉴权**：`TokenAuthReadOnly` 用于只读查询接口，对 token 状态校验更宽松（见 `auth.go`）。
- **审计内聚**：管理/root 写操作的审计兜底在鉴权链路里完成（`auth.go` 约 156 行），不分散到各 controller。

### 分发（distributor.go）

- **选渠道路径**：指定渠道（`ContextKeyTokenSpecificChannelId`）→ 渠道亲和性（Session Affinity）→ `CacheGetRandomSatisfiedChannel`（按 group × model 能力匹配 + 优先级随机）。
- **上下文注入**：`SetupContextForSelectedChannel` 把选中的 channel + model 注入 context，供后续 relay handler 使用。
- **亲和性记录**：选中渠道后调 `service.RecordChannelAffinity` 记录，用于后续相同 session 的粘性路由。
- **不在这里做协议转换**：distributor 只负责选渠道 + 注入上下文，实际的请求格式转换在 `relay/channel/` 完成。

### 限流

- **两级限流**：用户级（`rate-limit.go`，IP/用户维度）+ 模型级（`model-rate-limit.go`，按 model 维度）。两者独立，都要过。
- **Redis 优先**：有限流配置时走 Redis（`rdb.LLen` + 滑动窗口）；无 Redis 时降级到内存限流器（`common.InMemoryRateLimiter`）。
- **maxCount=0 表示不限**：`model-rate-limit.go` 中 `checkRedisRateLimit` 对 `maxCount == 0` 直接放行。

### 通用

- **中间件顺序**：`recover` → `request-id` → `cors` → `gzip` → `logger` → 鉴权 → 限流 → 业务。改顺序要谨慎，鉴权必须在限流之前（否则限流拿不到用户）。
- **JSON 走 common.\***（根 Rule 1）。

## 反模式

- ❌ 在 distributor 里做协议转换或调上游——它只选渠道 + 注入上下文。
- ❌ 在 controller 里重复做鉴权——鉴权集中在 middleware，controller 信任 context 中的用户/令牌。
- ❌ 改鉴权顺序把限流放鉴权前——限流需要用户信息做维度。
- ❌ 新增限流维度时忘记 Redis 与内存两条路径都要实现。
- ❌ 在审计逻辑里分散到各 controller——审计内聚在鉴权链路（`auth.go`）。
- ❌ 用 `encoding/json`（Rule 1）。
- ❌ 写只兼容单一数据库的查询（Rule 2）——middleware 多数走 service/model，但仍需注意。

## 调试路径

1. 请求 401/403 → `auth.go` 对应鉴权函数（`TokenAuth` / `authHelper`），检查 token 状态、用户角色、session。
2. 请求报"无可用渠道" → `distributor.go` `Distribute()` → 确认 group × model 在 `model/channel_satisfy.go` 有匹配 → `ability.go` 的启用关系。
3. 渠道选错（非亲和）→ `distributor.go` 的亲和性分支 + `service/channel_affinity.go` 的 session 匹配。
4. 限流误触发 → `rate-limit.go`（用户级）或 `model-rate-limit.go`（模型级）；检查 Redis key 与 maxCount/duration 配置。
5. 管理 API 无审计记录 → `audit.go` 是否在鉴权链路被正确触发（`auth.go` 的 AdminAuth/RootAuth 分支）。
6. 自定义 header 未生效 → `header_nav.go` 的解析逻辑，或 `jimeng_adapter.go`/`kling_adapter.go` 的供应商特定适配。

## 引用

无子级 `AGENTS.md`。上级导航见根 [`AGENTS.md`](../AGENTS.md)。
