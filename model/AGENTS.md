# model 知识库

## 概述

数据访问层（GORM v2）。定义表结构、索引、迁移与查询，是 SQLite / MySQL / PostgreSQL 三库兼容的落地处。分层最底层：Router → Controller → Service → **Model**。

## 目录结构

```text
model/
├── main.go                # DB 初始化、连接、AutoMigrate、跨库列名/布尔变量
├── locking.go             # lockForUpdate：跨库 SELECT ... FOR UPDATE
├── setup.go / option.go / utils.go / db_time.go / errors.go
├── user.go + user_cache.go / user_session.go / user_oauth_binding.go
├── token.go + token_cache.go
├── channel.go + channel_cache.go / channel_satisfy.go
├── ability.go             # group × model × channel 启用关系
├── log.go / log_record.go / log_summary.go / token_record.go / tool_log.go
├── pricing.go + pricing_default.go / pricing_refresh.go
├── model_meta.go / model_extra.go / vendor_meta.go
├── subscription.go / quota_reserve.go / topup.go / redemption.go / checkin.go
├── task.go / midjourney.go
├── auth_flow.go / authz_role.go / casbin_rule.go
├── passkey.go / twofa.go / external_identity_claim.go
├── system_instance.go / system_task.go
├── usedata.go / usedata_flow.go / usedata_rankings.go
├── custom_oauth_provider.go / prefill_group.go / missing_models.go
├── frontend_option_migration.go / gorm_logger.go
└── *_test.go
```

## 导航指南

| 任务 | 位置 | 说明 |
|------|------|------|
| 加新表 / 字段 | 新建 `xxx.go` + `main.go` 的 `AutoMigrate` | 见下方迁移约定 |
| 行锁 | `locking.go` `lockForUpdate(tx)` | 禁止 GORM v1 的 `Set("gorm:query_option", "FOR UPDATE")` |
| 跨库列引号 | `main.go` 的 `commonGroupCol` / `commonKeyCol` / `logGroupCol` / `logKeyCol` | `Where(commonGroupCol+" = ?", v)` |
| 跨库布尔 | `commonTrueVal` / `commonFalseVal` | PG=`true`/`false`，其他=`1`/`0` |
| 判断数据库 | `common.UsingMainDatabase` / `common.UsingLogDatabase` | 清空表、方言分支 |
| 系统配置 | `option.go` 的 `OptionMap` | 改完调 `model.UpdateOption` 同步缓存 |
| 渠道能力匹配 | `channel_satisfy.go` + `ability.go` | group × model → 启用渠道 |
| 授权数据 | `authz_role.go` / `casbin_rule.go` | 供 `service/authz` 使用 |
| 用户 / 令牌缓存 | `user_cache.go` / `token_cache.go` / `channel_cache.go` | 写路径必须更新缓存 |

## 约定

### 跨库兼容

- **保留字列用变量拼接**：`group`、`key` 用 `commonGroupCol` / `commonKeyCol`（主库）或 `logGroupCol` / `logKeyCol`（日志库）。这些变量在 `main.go` 按方言赋值（PostgreSQL `"group"`，MySQL/SQLite `` `group` ``）。
- **布尔值用变量**：裸 SQL 用 `commonTrueVal` / `commonFalseVal`。
- **行锁用 `lockForUpdate(tx)`**：GORM v2 会忽略 v1 的 `query_option`。该辅助对 MySQL/PG 发 `FOR UPDATE`，SQLite 跳过。不要在调用点再写一遍 `clause.Locking`。语义不同的方言锁（例如 MySQL next-key）只能放在显式数据库分支里，且三库都要有合法回退。
- **清空表要分支**：SQLite 不支持 `TRUNCATE`，`UsingSQLite` 时用 `DELETE`。
- **优先 GORM 方法**：能 `Where` / `Find` / `Create` / `Updates` 就不要裸 SQL。
- **不要用 `gorm:"default:true"`** 表达已由代码保证的业务默认值——MySQL 与 PG 对布尔默认的规范化不同，会导致每次启动 `AutoMigrate` 反复 `ALTER TABLE`。

### 迁移

- **新表**：struct + GORM tag → 加入 `main.go` 的 `DB.AutoMigrate(...)`。主键交给 GORM，不要手写 `AUTO_INCREMENT` / `SERIAL`。
- **新字段**：加字段即可；`AutoMigrate` 会 `ADD COLUMN`。禁止 `ALTER COLUMN`（SQLite 不支持）。
- **JSON 用 TEXT**：不要用 `JSONB`。订阅等复杂结构在 SQLite 有单独处理（见 `ensureSubscriptionPlanTableSQLite`）。
- **三库都要跑通**。

### 通用

- **双 DB**：业务用 `DB`，日志用 `LOG_DB`（`Log` / `TokenRecord` / `ToolLog`）。
- **缓存**：user / token / channel 有内存缓存，写操作必须同步。
- **时间**：用 `db_time.go` 处理跨库时间格式。
- **配额列是 int32**：换算饱和边界与 `common/quota_math.go` 一致。
- **JSON 走 `common.*`**。

## 反模式

- ❌ `Where("group = ?", v)`——必须 `commonGroupCol+" = ?"`。
- ❌ 裸写 `true` / `1` 当布尔——用 `commonTrueVal` / `commonFalseVal`。
- ❌ `tx.Set("gorm:query_option", "FOR UPDATE")`——GORM v2 不锁行。
- ❌ 无 SQLite 分支的 `TRUNCATE`。
- ❌ `ALTER COLUMN`、`JSONB`、无兜底的 `GROUP_CONCAT` / `@>`。
- ❌ 写 DB 却不更新 `*_cache.go`。
- ❌ 查日志用 `DB` 而不是 `LOG_DB`。
- ❌ 用 `encoding/json`。

## 调试路径

1. `group` / `key` 语法错误 → 是否漏用跨库列变量。
2. PostgreSQL `column "true" doesn't exist` → 布尔字面量未走 `commonTrueVal`。
3. 并发下额度被超扣 → 是否用了 `lockForUpdate`，而不是失效的 v1 query_option。
4. 启动反复 ALTER → 布尔 `default:true` 类 tag。
5. 缓存与 DB 不一致 → `*_cache.go` 是否覆盖全部写路径。
6. 日志查不到 → 是否误用 `DB`，时间过滤是否用 `created_at` 时间戳。
