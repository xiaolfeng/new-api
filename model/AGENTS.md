# model 知识库

## 概述

数据访问层（GORM v2）。定义所有数据表结构、索引与迁移，并提供查询方法。是跨三库兼容（SQLite / MySQL / PostgreSQL）的一线阵地——根 AGENTS.md Rule 2 的所有具体模式都在这里落地。分层架构的最底层（Router → Controller → Service → **Model**）。

## 目录结构

```text
model/
├── main.go                # DB 初始化、连接、AutoMigrate、跨库变量定义（核心）
├── setup.go               # 初始化配置表
├── option.go              # 系统配置（OptionMap，内存缓存 + DB 持久化）
├── utils.go               # 通用查询辅助
├── db_time.go             # 跨库时间字段处理
├── errors.go              # 数据层错误定义
├── user.go                # 用户表 + user_cache.go（内存缓存）
├── token.go               # API 令牌 + token_cache.go
├── channel.go             # 渠道表 + channel_cache.go / channel_satisfy.go（能力匹配）
├── ability.go             # 模型能力表（group × model × channel 的启用关系）
├── log.go                 # 日志主表（LOG_DB，可独立于主库）
├── log_record.go          # 结构化日志记录（record / full_log）
├── log_summary.go         # 日志聚合统计
├── token_record.go        # Token 用量按日记录（热力图数据源）
├── pricing.go             # 定价表 + pricing_default.go / pricing_refresh.go
├── model_meta.go          # 模型元数据 + model_extra.go
├── vendor_meta.go         # 供应商元数据
├── subscription.go        # 订阅计划 / 订阅实例 / 预扣记录
├── topup.go               # 充值记录
├── redemption.go          # 兑换码
├── checkin.go             # 签到
├── task.go                # 异步任务（Midjourney/Suno 等）
├── midjourney.go          # Midjourney 任务
├── passkey.go             # WebAuthn 凭证
├── twofa.go               # 双因素认证
├── perf_metric.go         # 性能指标
├── usedata.go             # 用量数据 + usedata_rankings.go（排行榜）
├── custom_oauth_provider.go # 自定义 OAuth 提供商
├── user_oauth_binding.go  # 用户 OAuth 绑定
├── prefill_group.go       # 预填充分组
├── missing_models.go      # 缺失模型追踪
└── *_test.go              # 测试
```

## 导航指南

| 任务 | 位置 | 说明 |
|------|------|------|
| 加新表 / 字段 | 新建 `xxx.go` + 在 `main.go` `AutoMigrate` 列表注册 | 见下方"约定"的迁移规则 |
| 跨库列引号（group/key 等保留字） | `main.go` 的 `commonGroupCol` / `commonKeyCol` / `logGroupCol` / `logKeyCol` | 用字符串拼接 `Where(commonGroupCol+" = ?", v)` |
| 跨库布尔值 | `main.go` 的 `commonTrueVal` / `commonFalseVal` | PostgreSQL=`true`/`false`，其他=`1`/`0` |
| 判断当前数据库 | `common.UsingPostgreSQL` / `common.UsingSQLite` / `common.UsingMySQL` | 用于分支清空（TRUNCATE vs DELETE）等 |
| 系统配置读写 | `option.go` `OptionMap`（内存）+ DB `options` 表 | 改配置后调 `model.UpdateOption` 同步缓存 |
| 渠道能力匹配 | `channel_satisfy.go` + `ability.go` | group × model → 启用的 channel 列表 |
| 日志查询（含分页/过滤） | `log.go` | 用 `logGroupCol` 拼 group 列；时间用 `created_at` 时间戳 |
| 定价读取 | `pricing.go` + `pricing_refresh.go` | 带缓存刷新机制 |
| 用户/令牌缓存 | `user_cache.go` / `token_cache.go` / `channel_cache.go` | 内存缓存层，避免每次查库 |

## 约定

### 跨库兼容（Rule 2 的具体落地）

- **保留字列用变量拼接**：`group`、`key` 是 SQL 保留字，直接写会语法错误。用 `commonGroupCol`/`commonKeyCol`（主库）或 `logGroupCol`/`logKeyCol`（日志库）拼接，如：
  ```go
  // ability.go
  DB.Table("abilities").Where(commonGroupCol+" = ? and enabled = ?", group, true)
  ```
  这些变量在 `main.go` 初始化时按数据库类型赋值（PostgreSQL 用 `"group"`，MySQL/SQLite 用 `` `group` ``）。
- **布尔值用变量**：裸 SQL 中布尔字面量用 `commonTrueVal`/`commonFalseVal`，不要直接写 `true` 或 `1`。
- **清空表要分支**：SQLite 不支持 `TRUNCATE`，要 `if common.UsingSQLite { DELETE } else { TRUNCATE }`（见 `ability.go`）。
- **优先 GORM 方法**：能用 `Where`/`Find`/`Create`/`Updates` 就别写裸 SQL。只有保留字列名才被迫拼接。

### 迁移

- **新表**：定义 struct（带 GORM tag）→ 在 `main.go` 的 `DB.AutoMigrate(...)` 列表里加一行。
- **新字段**：在 struct 上加字段 + GORM tag → `AutoMigrate` 会自动 `ADD COLUMN`。**禁止**用 `ALTER COLUMN`（SQLite 不支持改列定义）。
- **SQLite 特殊表**：有些表（如 `SubscriptionPlan`）用 JSON 存储复杂结构，SQLite 需要单独处理（见 `main.go` 的 `ensureSubscriptionPlanTableSQLite`）——用 `TEXT` 而非 `JSONB`。
- **三库都要跑通**：改完迁移后，确认在 SQLite / MySQL / PostgreSQL 上都不报错。

### 通用

- **双 DB 实例**：主业务用 `DB`，日志用 `LOG_DB`（可配置为不同数据库）。查日志走 `LOG_DB`，查业务走 `DB`。
- **缓存层**：高频读（user/token/channel）有 `*_cache.go` 内存缓存。写操作要同步更新缓存，不要只改 DB。
- **时间字段**：用 `db_time.go` 的辅助处理跨库时间格式差异（PostgreSQL 可能返回 `"2026-03-26T00:00:00Z"`）。
- **JSON 走 common.\***（根 Rule 1）；DTO 字段零值保留用指针（根 Rule 6，虽主要影响 dto/，但 model 里存 JSON 的字段同理）。

## 反模式

- ❌ 裸写 `Where("group = ?", v)`——`group` 是保留字，必须用 `commonGroupCol+" = ?"`。
- ❌ 裸写 `true`/`1` 作为布尔值——用 `commonTrueVal`/`commonFalseVal`。
- ❌ 用 `TRUNCATE` 不加 SQLite 分支——SQLite 只支持 `DELETE`。
- ❌ 用 `ALTER COLUMN` 改列——SQLite 不支持，改用 ADD COLUMN 或重建表。
- ❌ 用 `JSONB` 列类型——用 `TEXT` 存 JSON，保证三库兼容。
- ❌ 用 MySQL 专有函数（`GROUP_CONCAT`）或 PostgreSQL 专有操作符（`@>`、`?`）而无兜底。
- ❌ 写 DB 但不同步更新 `*_cache.go` 的内存缓存——会导致脏读。
- ❌ 查日志用 `DB` 而非 `LOG_DB`——日志可能在不同数据库。
- ❌ 用 `encoding/json`（Rule 1）。

## 调试路径

1. SQL 语法错误（`group`/`key` 列）→ 检查是否漏用 `commonGroupCol`/`commonKeyCol`/`logGroupCol`/`logKeyCol`。
2. 某数据库上报（如 PostgreSQL 报 `column "true" doesn't exist`）→ 布尔字面量没用 `commonTrueVal`/`commonFalseVal`。
3. 迁移失败 → `main.go` 的 `AutoMigrate` 列表；若是 SQLite，检查是否误用了 `ALTER COLUMN` 或 `JSONB`。
4. 缓存与 DB 不一致 → `*_cache.go` 的更新点是否覆盖了所有写路径。
5. 日志查不到 → 确认用的是 `LOG_DB` 而非 `DB`，且时间过滤用的是 `created_at` 时间戳。
6. 清空表报错 → 检查 `TRUNCATE` 是否加了 `common.UsingSQLite` 的 `DELETE` 分支。

## 引用

无子级 `AGENTS.md`。上级导航见根 [`AGENTS.md`](../AGENTS.md)。
