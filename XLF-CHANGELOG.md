# XLF-CHANGELOG —— XiaoLFeng 定制版变更与合并防冲突台账

> **分支说明**：当前分支为 `newapi-xlf-v2`，由 **XiaoLFeng** 基于上游开源项目 [`QuantumNous/new-api`](https://github.com/QuantumNous/new-api) 深度定制。
> **维护目的**：本台账系统性记录本定制分支自创设以来（2026-03 至今，共计 278 次专属提交）的所有功能增强、架构重构、协议扩展与缺陷修复。由于本分支需定期同步上游主线最新源（`git fetch upstream && git merge upstream/main`），为避免在合并 `git diff` 时因上下文漂移或语义冲突误删改定制特性，特编制本台账作为**冲突排查、意图对齐与不变量保护**的权威参考依据。

---

## 目录
1. [定制架构与主线防冲突矩阵 (Conflict Prevention Matrix)](#1-定制架构与主线防冲突矩阵)
2. [主线同步标准作业程序 (Upstream Sync SOP)](#2-主线同步标准作业程序)
3. [版本演进历程与核心改动 (Milestone Changelog)](#3-版本演进历程与核心改动)
   - [v2.4.1+ 核心稳定性审查与缺陷清零 (2026-10-05 ~ 2026-10-07)](#v241-核心稳定性审查与缺陷清零)
   - [v2.4.1 模型映射回归与合并收口 (2026-10-02)](#v241-模型映射回归与合并收口)
   - [v2.4.0 交付计时重构与主线大版本适配 (2026-09-28 ~ 2026-10-02)](#v240-交付计时重构与主线大版本适配)
   - [v2.3.x SDK v1.0 时代与高阶工具链路 (2026-08 ~ 2026-09)](#v23x-sdk-v10-时代与高阶工具链路)
   - [v2.2.x Bamboo 归一化中继内核与宿主工具体系 (2026-05 ~ 2026-07)](#v22x-bamboo-归一化中继内核与宿主工具体系)
   - [v2.1.x 结构化日志、空响应重试与热力图创制 (2026-03 ~ 2026-04)](#v21x-结构化日志空响应重试与热力图创制)
4. [全量提交溯源台账 (278 提交全量明细表)](#4-全量提交溯源台账)

---

## 1. 定制架构与主线防冲突矩阵

本分支相比上游 `QuantumNous/new-api`，在保留原有网关核心功能的同时，构建了 6 大独立/增强子系统。下表列出了在执行 `git merge upstream/main` 时最容易产生冲突的文件清单及**合并保护策略**：

| 子系统 | 高频冲突文件路径 | XiaoLFeng 改动本质与意图 | 主线合并保护策略 (Merge Strategy) |
| :--- | :--- | :--- | :--- |
| **Bamboo 中继归一化内核** | `relay/bamboo/**`<br>`relay/bamboo/hosttool/**` | 独立的中继协议归一化引擎，将 40+ 上游供应商统一为结构化流，内建 SDK 拦截器与流式重试机制 | **全量保留**。上游主线无此目录；若主线修改了 `relay/relay_adaptor.go`，须确保 `bamboo.ChatRelay` 优先分发逻辑不被删除 |
| **真实交付计时体系** | `relay/common/delivery_timing.go`<br>`relay/common/relay_info.go`<br>`relay/bamboo/delivery.go`<br>`service/log_info_generate.go`<br>`pkg/perf_metrics/metrics.go` | 突破主线粗粒度整秒 `use_time` 限制，引入纳秒级服务端首字写出 (`ttft_ms`)、总交付耗时 (`total_ms`)、多跳追踪 (`bamboo_timing_hops`)，提供 `FirstResponseDurationMs()` 彻底解决零值纪元偏移污染指标的问题 | **必须保留定制字段与守卫**。主线若更新了 `RelayInfo`，合并时保留 `DeliveryTiming`、`BambooTiming`、`FirstResponseDurationMs()`；`metrics.go` 须保留安全首字耗时判断 |
| **协议双向转换桥** | `relaykit/relayconvert/**` | 支持 OpenAI Responses 协议与标准 Chat 格式的双向无损转换，包含 output item ID 对齐、原请求字段回显与工具调用提升 | **必须保留扩展转换逻辑**。改动后必须执行 `cd relaykit && GOWORK=off go build ./...` 确保独立模块契约不被破坏 |
| **深度日志与会话感知** | `common/client_profile.go`<br>`model/log.go`<br>`controller/log.go`<br>`router/api-router.go`<br>`web/src/features/usage-logs/**` | 1. 自动识别 Claude Code / Codex / ZCode / Pi 客户端与 SubAgent 会话；<br>2. 列表响应剥离 `record`/`full_log` 大字段，改由 `/api/log/detail` 按需拉取；<br>3. 前端稳定行 ID 与详情弹层自动暂停刷新 | **禁止回退大字段列表**。若主线修改 `GetAllLogs`/`GetUserLogs`，必须保留 `stripLogDetailPayload`；保留 `/api/log/detail` 路由与控制器方法；保留前端 `getLogRowId` |
| **数据可靠性与多库兼容** | `model/log.go`<br>`model/usedata.go`<br>`model/token_record.go`<br>`model/tool_log.go`<br>`model/main.go` | 1. `countLogsCapped`：子查询上限封顶扫描 (10001)，防大表扫死；<br>2. 批量删除：`id IN (SELECT id ... LIMIT n)`，解决 SQLite/PG 忽略 LIMIT 缺陷；<br>3. CAS 乐观锁：`RecordFailedTokenRecord` 原子自增，防并发丢计数；<br>4. 缓存保护：`SaveQuotaDataCache` 落库失败保留条目，防统计永久丢失；<br>5. ClickHouse：补齐 `record`/`full_log`/`tps` 列并自动建 `token_record` 表 | **严禁覆盖为裸 SQL 或无锁覆盖**。主线的 `Delete().Limit()` 在 SQLite/PG 无效，Count() 无封顶，Update 会丢计数。合并时必须保留 XiaoLFeng 的高可靠版本 |
| **看板图表与前端体验** | `web/src/features/dashboard/**`<br>`web/src/features/model-log/**`<br>`web/src/features/tool-logs/**` | 1. 热力图日期周一对齐 (`buildMondayAlignedHeatmapDates`)；<br>2. Tooltip Portal 渲染到 `document.body`，彻底根除包含块 `filter: blur(0px)` 导致的漂移；<br>3. 图表共享单次聚合，防重复计算卡顿；<br>4. 稀疏时间点平滑补齐；<br>5. 工具日志嵌套弹窗修复 | **保留前端体验修正**。主线若更新了图表组件，需保留 Portal 挂载、周一对齐逻辑与父级图表聚合复用 |

---

## 2. 主线同步标准作业程序 (Upstream Sync SOP)

当需要从上游仓库 `QuantumNous/new-api` 同步最新代码时，请严格按照以下步骤操作：

### 步骤 1：准备工作与分支检查

```bash
# 确认本地工作区干净
git status

# 确认 remotes 配置正确（origin 指向个人 fork，upstream 指向上游官方）
git remote -v

# 拉取上游最新分支与标签
git fetch upstream
```

### 步骤 2：发起合并

```bash
# 切换到定制分支
git checkout newapi-xlf-v2

# 发起合并
git merge upstream/main
```

### 步骤 3：冲突解决要诀（对照第 1 节矩阵）

- **遇到 `relay/` 冲突**：保留 `relay/bamboo/` 目录全量代码；保留 `RelayInfo` 中的 `DeliveryTiming`、`BambooTiming`、`FirstResponseDurationMs()`；

- **遇到 `model/` 冲突**：重点保护 `countLogsCapped`、`id IN (SELECT id ... LIMIT n)` 批量删除、`RecordFailedTokenRecord` CAS 乐观锁与 `SaveQuotaDataCache` 失败保留；

- **遇到 `service/` 冲突**：保留 `CalculateTPS` 的 `frtMs > 0` 守卫与 `appendDeliveryTiming`；

- **遇到 `web/` 冲突**：保留热力图 `createPortal` 挂载、稳定行 ID 与 `/api/log/detail` 按需加载。

### 步骤 4：强制验证清单（必须全部通过才可提交）

```bash
# 1. 后端全量编译与代码检查
go build ./...
go vet ./...

# 2. relaykit 独立构建（禁止依赖主仓 workspace）
cd relaykit && GOWORK=off go build ./... && cd ..

# 3. 核心后端回归测试
go test ./model/ -run 'TokenRecord|ClickHouse' -count=1
go test ./service/ -count=1
go test ./pkg/perf_metrics/ -count=1
go test ./relay/common/ -count=1
go test ./controller/ -run 'TestGetLogDetail' -count=1

# 4. 前端类型检查
cd web && bun run typecheck && cd ..
```

### 步骤 5：登记台账

合并完成后，在此文件（`XLF-CHANGELOG.md`）的第 3 节与第 4 节追加合并记录与修复说明。


---

## 3. 版本演进历程与核心改动

### v2.4.1+ 核心稳定性审查与缺陷清零

**周期**：2026-10-05 ~ 2026-10-07 | **核心提交**：`c2004515a` → `6a57536ad` → `f480f17a2`

本阶段针对整个系统开展了深度的静态代码审查与根因缺陷清零工作，完整落地了 17 项系统级缺陷修复 (Q-01 ~ Q-17)，并在首字时间纪元偏移与监控指标保护方面进行了关键架构加固：

- **首字时间零值纪元偏移与监控中毒修复 (Q-16 / Q-17)** (`f480f17a2`)：

  - **根因**：流式请求未产生有效首字即中断时，`FirstResponseTime` 为零值，与 `StartTime` 相减产生约 `-6.4×10¹³ ms` 的负数偏移；`metrics.go` 凭 `Committed()` 即判定 `hasTtft=true`，导致 `generationMs` 膨胀至数万秒并污染监控指标桶；日志与配额结算中的 `frt` 也因负数写入脏数据。

  - **修复**：在 `RelayInfo` 新增 `FirstResponseDurationMs()`，未发生首字或时间倒流时安全返回 0；`metrics.go` 依赖正向耗时判定 `hasTtft`；`GenerateTextOtherInfo` 与配额结算统一使用安全耗时并补齐 `ChannelMeta` 空保护；`CalculateTPS` 增加流式 `frtMs > 0` 守卫，异常时安全降级为总耗时。

- **15 项系统级缺陷全量重构修复 (Q-01 ~ Q-15)** (`6a57536ad`)：

  - **Q-01 (高危数据可靠性)**：`model/usedata.go` `SaveQuotaDataCache` 落库失败按条目保留缓存，彻底解决失败时清空缓存导致额度统计永久丢失的隐患。

  - **Q-02 (高危多库兼容)**：`model/main.go` 补齐 ClickHouse `logs` 表的 `record`、`full_log`、`tps` 列，并自动初始化 `token_record` MergeTree 表。

  - **Q-03 (高危模型统计)**：`model/token_record.go` 修复 `buildTokenRecordHours` 硬编码 24 桶导致 168 小时（7天）窗口只统计最早 24 小时的严重偏差。

  - **Q-04 (中危列表性能)**：列表响应剥离 `record` 与 `full_log` 大字段，新增 `GET /api/log/detail?request_id=` 单条详情端点（带权限与用户归属校验）；前端懒挂载 `DetailsDialog` 并按需拉取。

  - **Q-05 (中危数据库压力)**：`countLogsCapped` 改用子查询封顶扫描（10001 行），解决 `Limit(10000).Count()` 在 GORM 中限制无效导致大表全表扫描的问题。

  - **Q-06 (前端网络优化)**：`buildLogStatsQueryKey` 剔除 `page`/`pageSize` 分页参数，翻页不再重复请求全局统计。

  - **Q-07 (前端视觉定位)**：两处热力图 Tooltip 统一改用 `createPortal(document.body)` 渲染，彻底消除页面过渡动画 `filter: blur(0px)` 建立 Fixed 包含块导致的 Tooltip 坐标漂移。

  - **Q-08 (前端布局稳定)**：`token-heatmap.tsx` 月份行增加显式高度 `h-[12px]`，星期列对齐到 `pt-[16px]`，解决绝对定位文字导致的高度坍塌。

  - **Q-09 (前端数据对齐)**：`buildMondayAlignedHeatmapDates` 将热力图起始日期严格对齐到星期一，消除日期与星期标签错位。

  - **Q-10 (图表平滑度)**：`fillTimePoints` 优化为保留所有已有数据点，仅向前补充历史空桶，防止稀疏点被错误丢弃。

  - **Q-11 (前端计算性能)**：将 `processChartData` 提升至父级 `dashboard/index.tsx` 一次性计算，消除两个懒加载子图表的重复耗时聚合。

  - **Q-12 (表格状态安全)**：`useDataTable` 引入基于 `request_id` 的稳定行标识 `getLogRowId`；详情弹层打开期间通过 `useOptionalUsageLogsContext` 联动暂停自动刷新，防止刷新切行。

  - **Q-13 (数据库批量删除)**：`model/log.go` 与 `model/tool_log.go` 批量清理改用 `id IN (SELECT id ... LIMIT n)` 子查询，解决 SQLite/PostgreSQL DELETE 构建器不支持 LIMIT 导致单次删除超出批次的缺陷。

  - **Q-14 (多库方言对齐)**：每日 token 统计中的时间格式化表达式改用 `tokenRecordDateExpr`，根据 `common.LogDatabaseType()` 方言正确生成 SQL，解决独立日志库报错。

  - **Q-15 (并发数据安全)**：`RecordFailedTokenRecord` 引入 CAS 乐观锁重试循环，消除并发失败请求覆盖写入导致计数丢失的问题。

- **日志来源与会话列增强** (`c2004515a`)：

  - 增加来源列与会话列直观展示，重构请求/响应记录的结构化抽屉。

### v2.4.1 模型映射回归与合并收口

**周期**：2026-10-02 | **版本标记**：`v2.4.1` (`4383c94a5`) | **核心提交**：`b06233f34` → `2e6751b29` → `d967e173a` → `4383c94a5`

- **模型映射黑名单机制** (`d967e173a` / `2e6751b29`)：补齐 `/v1/responses` 入口的模型重定向映射逻辑，断言映射名经黑名单保留后直达上游，防止误重定向。

- **合并冲突收口** (`b06233f34`)：修复合入上游主线后引起的 i18n 缺失文案、渠道 Bamboo 设置关联、日志流式列冗余展示问题。

- **版本提升** (`4383c94a5`)：正式提升版本号至 `v2.4.1`。

### v2.4.0 交付计时重构与主线大版本适配

**周期**：2026-09-28 ~ 2026-10-02 | **版本标记**：`v2.4.0` (`b06233f34` 之前) | **核心提交**：`afd97f3f8` → `4273b50bb` → `5344941c6` → `5944ff689`

- **主线大版本合入** (`afd97f3f8`)：合入上游 `origin/main`（包含大量插件系统、计费表达式、模型元数据同步等），完成全链路功能适配。

- **真实交付耗时体系重构** (`5944ff689`)：

  - 创建 `relay/common/delivery_timing.go` 与 `relay/bamboo/delivery.go`，建立请求级服务端写出监听器；

  - 严谨分离「响应头提交 (`MarkCommitted`)」与「有效内容写出 (`RecordContent`)」；

  - 彻底解决流式输出下将上游网络等待计入传输、或空帧提前触发 TTFT 导致的计时失真。

- **SDK 升级与 Pi 客户端适配** (`5344941c6`)：升级 `bamboo-messages` SDK 至 `v1.0.14`，适配 Pi 编程助手识别与专用会话格式。

### v2.3.x SDK v1.0 时代与高阶工具链路

**周期**：2026-08-01 ~ 2026-09-14 | **版本标记**：`v2.3.12` ~ `v2.3.17`

本阶段 Bamboo 协议内核进入成熟期，经历了 SDK 从 v0.9 到 v1.0 的全面演进，并围绕 Agentic 场景构建了完备的工具调用与思考链回放体系：

- **SDK 迭代演进**：持续紧跟底层规范，连续升级 messages SDK：`v1.0.0` → `v1.0.1` → `v1.0.5` → `v1.0.6` → `v1.0.7` → `v1.0.8` → `v1.0.9` → `v1.0.10` → `v1.0.11` → `v1.0.12` → `v1.0.13`。

- **宿主工具所有权表驱动重构** (`08a288a5d`)：移除特定品牌写死逻辑，全面转向表驱动的 host-tool 调度模式。

- **Grok 与 Gemini 原生搜索链路** (`715c2d751`, `332bf3099`, `7ab9cc129`, `5272c1d8c`, `6ec510afc`)：透传 Grok Web Search 并构造 helper 回包；将 Gemini 原生搜索无缝转化为标准 host-tool。

- **思考签名与推理保护** (`ce313f6d5`, `b8c8c35f4`)：对齐 thinking signature 血统，防止 Responses 协议下的明文推理被截断或泄露。

- **内部工具请求隔离** (`b79c9b28f`)：增加内部请求标记，避免系统自发生成的识图与搜索内部请求污染 TPS 与缓存率统计指标。

- **Responses <-> Chat 协议双向映射** (`407373ac4`, `716e5db2f`, `5264b391e`)：对齐 output item ID 前缀，完善原请求字段在双向格式转换中的保真回显。

- **上游多跳计时记录**：在 `LogOther` 中引入 `bamboo_timing_hops`，完整记录重试与链式调用各跳的性能明细。

- **主线定期同步** (`1fa41c472` on 2026-08-17)：合入上游 `QuantumNous/main` 并解决冲突。

### v2.2.x Bamboo 归一化中继内核与宿主工具体系

**周期**：2026-05-01 ~ 2026-07-30 | **版本标记**：`v2.2.6-alpha.37 ~ 39`

本阶段奠定了 `newapi-xlf-v2` 最具特色的架构基础——Bamboo 归一化中继体系的建立与落地：

- **分支合并与内核创立** (`35831141f` on 2026-06-30)：正式将 `feature/new-relay-for-bamboo` 核心特性合入 `newapi-xlf-v2`，开启双中继内核时代。

- **宿主工具链路建设 (RFC-0001 / RFC-0002 / RFC-0003)**：实现基于服务器端的 Web Search、Web Fetch 及多模态识图工具流式回显。

- **降级与容灾策略** (`eb1f028ee`, `041e9f752`)：引入 `DegradedReason`，并在系统设置中提供流式中断降级策略配置界面。

- **内联思考标签剥离** (`6ff66ff02`)：增加渠道级 `StripThinkTags` 开关，在代理侧自动剥除模型内联输出的 `<thought>` 标签。

- **会话与 Agent 识别体系** (`7ed4f4419`, `b58eddb8d`, `ae8789359`, `60238647e`)：

  - 建立客户端特征指纹识别库 `common/client_profile.go`；

  - 深度解析 Claude Code, Codex, ZCode 请求结构，提取 Session ID 与主线程/SubAgent 关系并在日志高亮展示。

- **主线合并** (`ddd729529` on 2026-07-24 / `b60ff1be3` on 2026-06-30)：定期并入上游 `QuantumNous/main` 代码。

### v2.1.x 结构化日志、空响应重试与热力图创制

**周期**：2026-03-20 ~ 2026-04-30 | **版本标记**：`v2.1.7` (`4e7881d59`)

本阶段为 XiaoLFeng 改版的初始创立阶段，重点攻克日志可观测性与异常重试痛点：

- **消费日志结构化存储** (`1d4efe3c9`, `201b179f7`, `19e17736f`)：在 GORM 模型与数据库中新增 `record`、`full_log`、`other`、`tps` 列，开启日志全生命周期结构化记录。

- **空响应重试机制** (`2ef655bd5`, `008949393`)：首创针对上游返回 HTTP 200 但内容为空流（如限流静默空包）的自动重试检测机制，重构流扫描器为 `StreamResult` 模式。

- **模型日志 TPS 热力图** (`b039f1be3`, `7889d73d1`, `3998427c1`)：自研卡片式模型热力图与全局统计卡片，支持小时级 TPS 渲染与主题切换。

- **日志运维工具** (`d7b307184`, `6ddaf47f6`, `cc230bb05`)：支持日志自动刷新倒计时、可配置刷新间隔及日志文件安全清理接口。

- **发布版本** (`4e7881d59`)：发布改版首个正式标签版本 `v2.1.7`。


---

## 4. 全量提交溯源台账

以下为 `newapi-xlf-v2` 分支自创设以来全部 278 次专属提交记录（按时间倒序排列），可通过提交哈希在 Git 中直接检视具体改动：

| # | 提交哈希 | 日期 | 类别 | 模块 (Scope) | 提交说明 | 版本阶段 |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| 1 | `f480f17a2` | 2026-10-07 | fix | `日志` | 修复首字时间零值导致的指标中毒与负数用时 | `v2.4.1+` |
| 2 | `6a57536ad` | 2026-10-07 | fix | `日志` | 修复翻页性能、热力图漂移与统计丢数等 15 项缺陷 | `v2.4.1+` |
| 3 | `c2004515a` | 2026-10-05 | feat | `日志` | 添加来源与会话列及记录结构化展示 | `v2.4.1+` |
| 4 | `4383c94a5` | 2026-10-02 | chore | `版本` | 版本号提升至 v2.4.1 | `v2.4.1` |
| 5 | `d967e173a` | 2026-10-02 | test | `bamboo` | 回归测试改为断言映射名经黑名单保留后直达上游 | `v2.4.1` |
| 6 | `2e6751b29` | 2026-10-02 | fix | `bamboo` | 补齐 /v1/responses 入口的模型映射并新增回归测试 | `v2.4.1` |
| 7 | `b06233f34` | 2026-10-02 | fix | `合并` | 修复主线合并引入的 i18n 丢失、渠道 Bamboo 设置与日志流列冗余 | `v2.4.1` |
| 8 | `5944ff689` | 2026-10-02 | fix | `bamboo` | 重构流式耗时与 TTFT 体系消除计时失真 | `v2.4.0` |
| 9 | `5344941c6` | 2026-10-01 | fix | `bamboo` | 升级 messages SDK 至 v1.0.14 并适配 Pi 与会话识别 | `v2.4.0` |
| 10 | `4273b50bb` | 2026-09-30 | chore | `配置` | 在 gitignore 中忽略 opencode 与 zcode 本地目录 | `v2.4.0` |
| 11 | `afd97f3f8` | 2026-09-28 | chore | `合并` | 合入 origin/main 最新变更并完成全链路功能适配 | `v2.4.0` |
| 12 | `a8513f931` | 2026-09-14 | fix | `bamboo` | 升级 messages SDK 至 v1.0.13 | `v2.3.x` |
| 13 | `65ec61577` | 2026-09-11 | fix | `bamboo` | 升级 messages SDK 至 v1.0.12 并优化 client_stream 工具调用 stop_reason 兜底 | `v2.3.x` |
| 14 | `26704afd7` | 2026-09-11 | fix | `bamboo` | 升级 messages SDK 至 v1.0.11 | `v2.3.x` |
| 15 | `4b8345bbf` | 2026-09-08 | fix | `bamboo` | 升级 messages SDK 至 v1.0.10 | `v2.3.x` |
| 16 | `7fbf14e2e` | 2026-09-03 | feat | `日志` | 添加分阶段 TPS 趋势 | `v2.3.x` |
| 17 | `4edc4d87b` | 2026-09-01 | fix | `bamboo` | 升级 messages SDK 至 v1.0.9 | `v2.3.x` |
| 18 | `596369fd2` | 2026-09-01 | fix | `bamboo` | 升级 messages SDK 至 v1.0.8 | `v2.3.x` |
| 19 | `5272c1d8c` | 2026-08-27 | fix | `bamboo` | 移除grok build搜索透传并抑制回抛防止死循环 | `v2.3.x` |
| 20 | `08a288a5d` | 2026-08-27 | refactor | `bamboo` | 移除host-tool品牌特化改为所有权表驱动 | `v2.3.x` |
| 21 | `6ec510afc` | 2026-08-27 | feat | `bamboo` | 适配Gemini原生WebSearch到host-tool | `v2.3.x` |
| 22 | `a3885460d` | 2026-08-25 | fix | `bamboo` | 升级 messages SDK 至 v1.0.7 | `v2.3.x` |
| 23 | `97722d67f` | 2026-08-25 | fix | `bamboo` | 升级 messages SDK 至 v1.0.6 | `v2.3.x` |
| 24 | `ce313f6d5` | 2026-08-25 | feat | `bamboo` | 对齐思考签名血统并升级 SDK 至 v1.0.5 | `v2.3.x` |
| 25 | `b8c8c35f4` | 2026-08-25 | fix | `bamboo` | 修复Responses明文推理回放并升级SDK | `v2.3.x` |
| 26 | `eb3c53aea` | 2026-08-25 | fix | `日志` | 修复Claude Code工具回调轮误标输入 | `v2.3.x` |
| 27 | `c64d1a6b8` | 2026-08-23 | feat | `日志` | 添加内部工具请求标识徽章与悬停说明 | `v2.3.x` |
| 28 | `45a0b47ec` | 2026-08-23 | feat | `日志` | 添加单轮交互类型区分非Agentic请求 | `v2.3.x` |
| 29 | `99159fb6c` | 2026-08-23 | fix | `bamboo` | 修复失败请求双日志记录与结算后重试漏计 | `v2.3.x` |
| 30 | `52dd2af2a` | 2026-08-22 | docs | `知识库` | 同步分层 AGENTS 以匹配当前架构 | `v2.3.x` |
| 31 | `b7401c636` | 2026-08-22 | fix | `bamboo` | 修复Responses出口用量为0并升级SDK | `v2.3.x` |
| 32 | `4ebf571d4` | 2026-08-22 | fix | `bamboo` | 升级 messages SDK 至 v1.0.1 | `v2.3.x` |
| 33 | `32a795e72` | 2026-08-21 | fix | `bamboo` | 升级 messages SDK 至 v1.0.0 | `v2.3.x` |
| 34 | `c0897e7bd` | 2026-08-21 | fix | `bamboo` | 修复host-tool用量合并与Grok会话显示并升级SDK | `v2.3.x` |
| 35 | `b79c9b28f` | 2026-08-21 | fix | `bamboo` | 隔离内部工具请求避免污染TPS与缓存率统计 | `v2.3.x` |
| 36 | `91a380c87` | 2026-08-20 | fix | `bamboo` | 识图失败支持重试与fail-open降级 | `v2.3.x` |
| 37 | `6816ec3cf` | 2026-08-20 | fix | `bamboo` | 透传Grok客户端工具以恢复自动向后迭代 | `v2.3.x` |
| 38 | `8d0141810` | 2026-08-20 | fix | `bamboo` | 解决宿主工具流式首字被缓冲阻塞 | `v2.3.x` |
| 39 | `c8dd49a3b` | 2026-08-19 | fix | `bamboo` | 升级 messages SDK 至 v0.9.9 | `v2.3.x` |
| 40 | `715c2d751` | 2026-08-19 | fix | `bamboo` | 透传Grok搜索并补齐helper回包 | `v2.3.x` |
| 41 | `332bf3099` | 2026-08-19 | fix | `relay` | 拆分搜索命中并补齐Grok必填字段 | `v2.3.x` |
| 42 | `a94d71d7a` | 2026-08-19 | fix | `relay` | 给Grok正文消息项补空content | `v2.3.x` |
| 43 | `7ab9cc129` | 2026-08-19 | fix | `bamboo` | 解析搜索命中并补齐Grok Chat信封 | `v2.3.x` |
| 44 | `c2528fefd` | 2026-08-18 | fix | `工具日志` | 修复请求ID跳转与嵌套弹窗被关闭 | `v2.3.x` |
| 45 | `9b33c1fc8` | 2026-08-18 | fix | `relay` | 保留Claude空input后的工具参数delta | `v2.3.x` |
| 46 | `4de501841` | 2026-08-18 | fix | `relay` | 给Grok流式信封事件根补created | `v2.3.x` |
| 47 | `56c826f34` | 2026-08-18 | feat | `relay` | 按UA隔离搜索回包并补齐Grok信封 | `v2.3.x` |
| 48 | `9793a1241` | 2026-08-18 | feat | `-` | 将识图纳入思考链与工具日志并收拢交互展示 | `v2.3.x` |
| 49 | `2724aa35d` | 2026-08-18 | feat | `-` | 将宿主工具标签移入交互列并重做工具日志 | `v2.3.x` |
| 50 | `9e6cbb8f6` | 2026-08-18 | feat | `bamboo` | 将检索抓取改为正式工具调用 | `v2.3.x` |
| 51 | `7f518c2c6` | 2026-08-18 | feat | `bamboo` | 向用户流式回传识图与检索过程 | `v2.3.x` |
| 52 | `4ac52c292` | 2026-08-18 | fix | `bamboo` | 避免识图前替换并补费用标签 | `v2.3.x` |
| 53 | `52beb6b22` | 2026-08-18 | feat | `-` | 接通Playground贴图并增强WebFetch空壳抽取 | `v2.3.x` |
| 54 | `63e1dea81` | 2026-08-18 | feat | `bamboo` | 完善端侧检索并约束识图只解析 | `v2.3.x` |
| 55 | `94cacbf0a` | 2026-08-18 | feat | `bamboo` | 添加无Vision模型的图片预识别 | `v2.3.x` |
| 56 | `b04b94400` | 2026-08-18 | fix | `工具日志` | 修复共用主库时未创建 tool_logs 表 | `v2.3.x` |
| 57 | `de4083607` | 2026-08-18 | feat | `-` | 添加工具日志并修复 host_tool 入参与缓存率 | `v2.3.x` |
| 58 | `9fd239cd1` | 2026-08-17 | fix | `usage-logs` | 恢复自定义流展示并修正交互类型 | `v2.3.x` |
| 59 | `3c7d18a80` | 2026-08-17 | feat | `bamboo` | 实现端侧 WebSearch/WebFetch 工具 | `v2.3.x` |
| 60 | `1fa41c472` | 2026-08-17 | merge | `-` | 合并上游 QuantumNous/main 最新变更到 newapi-xlf-v2 | `v2.3.x` |
| 61 | `407373ac4` | 2026-08-05 | fix | `relayconvert` | output item ID 对齐 OpenAI 前缀并修正测试边界 | `v2.3.x` |
| 62 | `7404c564b` | 2026-08-05 | fix | `service` | 修正 Responses 日志记录解析 | `v2.3.x` |
| 63 | `cc0109371` | 2026-08-05 | fix | `claude` | 非流式响应聚合多块 text/thinking 内容 | `v2.3.x` |
| 64 | `aa835f04e` | 2026-08-04 | test | `relayconvert` | 修正两处断言对齐实际转换语义 | `v2.3.x` |
| 65 | `6edd4f914` | 2026-08-04 | test | `relayconvert` | 修正 TestMultiModalContent 类型断言对齐项目契约 | `v2.3.x` |
| 66 | `716e5db2f` | 2026-08-04 | feat | `relayconvert` | 补齐 responses→chat 转换的字段处理 | `v2.3.x` |
| 67 | `5264b391e` | 2026-08-04 | fix | `relayconvert` | 实现 chat→responses 转换回显原请求字段 | `v2.3.x` |
| 68 | `130d0370b` | 2026-08-04 | chore | `deps` | 升级 bamboo-messages v0.9.7 → v0.9.8 | `v2.3.x` |
| 69 | `e06a545a6` | 2026-08-02 | chore | `deps` | 升级 bamboo-messages v0.9.6 → v0.9.7 | `v2.3.x` |
| 70 | `021fc7844` | 2026-08-02 | fix | `bamboo` | 识别客户端取消避免误记 500 错误日志 | `v2.3.x` |
| 71 | `8f99d52f3` | 2026-08-02 | chore | `deps` | 升级 bamboo-messages v0.9.4 → v0.9.5 | `v2.3.x` |
| 72 | `7269eda18` | 2026-08-02 | fix | `bamboo` | 归一化 reasoning_effort 防止 qwen 上游拒绝 | `v2.3.x` |
| 73 | `80a7e47ef` | 2026-08-02 | chore | `deps` | 升级 bamboo-messages v0.9.3 → v0.9.4 | `v2.3.x` |
| 74 | `240e548c7` | 2026-08-02 | fix | `bamboo` | 失败请求补写日志并携带 bamboo debug 信息 | `v2.3.x` |
| 75 | `f3372774f` | 2026-08-02 | chore | `deps` | 升级 bamboo-messages v0.9.2 → v0.9.3 | `v2.3.x` |
| 76 | `95d5d2b1e` | 2026-08-02 | chore | `deps` | 升级 bamboo-messages v0.9.1 → v0.9.2 | `v2.3.x` |
| 77 | `ae8789359` | 2026-08-02 | feat | `使用日志` | 完善 Codex 会话识别与前端兜底显示 | `v2.3.x` |
| 78 | `d95e226a6` | 2026-08-02 | chore | `deps` | 升级 bamboo-messages v0.9.0 → v0.9.1 | `v2.3.x` |
| 79 | `6ff66ff02` | 2026-08-02 | feat | `bamboo` | 添加渠道级 StripThinkTags 内联思考标签剥离开关 | `v2.3.x` |
| 80 | `b58eddb8d` | 2026-08-01 | feat | `-` | 升级 bamboo-messages v0.9.0 并添加 Codex 会话识别 | `v2.3.x` |
| 81 | `92c433a54` | 2026-07-28 | feat | `heatmap` | 优化热力图国际化与交互体验 | `v2.2.x` |
| 82 | `7ed4f4419` | 2026-07-25 | feat | `使用日志` | 添加 ZCode 客户端会话识别与主线程/SubAgent 映射 | `v2.2.x` |
| 83 | `b296868d1` | 2026-07-24 | fix | `使用日志` | 移除合并引入的重复耗时列并补齐移动端字段 | `v2.2.x` |
| 84 | `ddd729529` | 2026-07-24 | merge | `-` | 合并上游 QuantumNous/main 最新变更到 newapi-xlf-v2 | `v2.2.x` |
| 85 | `439e2c189` | 2026-07-24 | fix | `使用日志` | 恢复详细记录中请求头显示 | `v2.2.x` |
| 86 | `b87f2d83f` | 2026-07-08 | feat | `bamboo` | 添加 Legacy 缓存键开关支持 | `v2.2.x` |
| 87 | `041e9f752` | 2026-07-05 | refactor | `bamboo` | 适配 SDK v0.8.15，移除 SmoothPacer 并添加 DegradedReason 解析 | `v2.2.x` |
| 88 | `eb1f028ee` | 2026-07-05 | feat | `系统设置` | 添加 DegradedReason 流式中断降级策略配置 UI | `v2.2.x` |
| 89 | `0fdda3700` | 2026-07-05 | fix | `bamboo` | 修复缓存 token 双重计算并适配 v0.8.14 | `v2.2.x` |
| 90 | `4f7f34c3a` | 2026-07-04 | chore | `deps` | upgrade bamboo-messages to v0.8.13 | `v2.2.x` |
| 91 | `f0ccf2db9` | 2026-07-04 | chore | `deps` | upgrade bamboo-messages to v0.8.12 | `v2.2.x` |
| 92 | `993e72d3e` | 2026-07-04 | chore | `deps` | upgrade bamboo-messages to v0.8.11 | `v2.2.x` |
| 93 | `c0df05559` | 2026-07-04 | style | `前端` | 统一 import 排序与代码格式化 | `v2.2.x` |
| 94 | `c2503fa58` | 2026-07-04 | fix | `计费` | 修复 input_tokens_total 对非 Claude 语义缺失 | `v2.2.x` |
| 95 | `0e7b19512` | 2026-07-04 | fix | `bamboo` | 适配 SDK v0.8.10 流式 debug 收集并修复速率计算 | `v2.2.x` |
| 96 | `280b3019f` | 2026-07-04 | chore | `deps` | upgrade bamboo-messages to v0.8.10 | `v2.2.x` |
| 97 | `151d42ea0` | 2026-07-04 | fix | `bamboo` | 适配 SDK v0.8.9 错误类型变更 | `v2.2.x` |
| 98 | `b8905b2fc` | 2026-07-04 | chore | `deps` | upgrade bamboo-messages to v0.8.9 | `v2.2.x` |
| 99 | `dd36b36bd` | 2026-07-04 | refactor | `bamboo` | 重构 debug 信息为分块结构展示 | `v2.2.x` |
| 100 | `21ef92812` | 2026-07-04 | chore | `deps` | upgrade bamboo-messages to v0.8.8 | `v2.2.x` |
| 101 | `275da5a09` | 2026-07-02 | chore | `deps` | upgrade bamboo-messages to v0.8.7 | `v2.2.x` |
| 102 | `2970b46ba` | 2026-07-02 | fix | `bamboo` | 修复流式 usage 提取丢失与兜底缺失 | `v2.2.x` |
| 103 | `1c9890de1` | 2026-07-02 | feat | `bamboo` | 添加传统模式开关与流式用量修复 | `v2.2.x` |
| 104 | `1799f402a` | 2026-07-02 | chore | `deps` | upgrade bamboo-messages to v0.8.4 | `v2.2.x` |
| 105 | `9afbfe07c` | 2026-07-02 | revert | `-` | 回退 CI 工作流至 GitHub-hosted runner | `v2.2.x` |
| 106 | `bd4fe5e70` | 2026-07-01 | ci | `-` | 迁移全部工作流至 self-hosted runner | `v2.2.x` |
| 107 | `75c367dc2` | 2026-07-01 | chore | `deps` | upgrade bamboo-messages to v0.8.3 | `v2.2.x` |
| 108 | `1d9827281` | 2026-07-01 | chore | `deps` | upgrade bamboo-messages to v0.8.2 | `v2.2.x` |
| 109 | `c6ba8d444` | 2026-07-01 | chore | `deps` | upgrade bamboo-messages to v0.8.1 | `v2.2.x` |
| 110 | `531f8cf68` | 2026-07-01 | chore | `deps` | upgrade bamboo-messages to v0.8.0 | `v2.2.x` |
| 111 | `f9a78ad72` | 2026-06-30 | fix | `-` | 修复 Docker 构建编译错误 | `v2.2.x` |
| 112 | `0e7b1628d` | 2026-06-30 | fix | `-` | 修复 Docker 构建编译错误 | `v2.2.x` |
| 113 | `19881cc94` | 2026-06-30 | chore | `deps` | upgrade bamboo-messages to v0.7.9 | `v2.2.x` |
| 114 | `35831141f` | 2026-06-30 | merge | `-` | 合并 feature/new-relay-for-bamboo 分支到 newapi-xlf-v2 | `v2.2.x` |
| 115 | `1aa24f46a` | 2026-06-30 | fix | `-` | 修复 openaicompat→relayconvert 重命名后的残留 import 和包名 | `v2.2.x` |
| 116 | `b60ff1be3` | 2026-06-30 | merge | `-` | 合并 main 分支最新变更到 newapi-xlf-v2 | `v2.2.x` |
| 117 | `02186b9a0` | 2026-06-29 | feat | `bamboo` | 增强 timing 数据展示与修正交互类型推断优先级 | `v2.2.x` |
| 118 | `91e3b82fe` | 2026-06-29 | chore | `deps` | upgrade bamboo-messages to v0.7.7 | `v2.2.x` |
| 119 | `10fde4022` | 2026-06-29 | fix | `bamboo` | 修复 relay 错误时未计费的问题 | `v2.2.x` |
| 120 | `6a5ab5d97` | 2026-06-29 | chore | `deps` | upgrade bamboo-messages to v0.7.6 | `v2.2.x` |
| 121 | `73fc0a2ef` | 2026-06-29 | chore | `deps` | upgrade bamboo-messages to v0.7.5 | `v2.2.x` |
| 122 | `23153aeb6` | 2026-06-29 | fix | `bamboo` | doStreamRelay 新增空响应检测，对齐 doCompleteRelay 行为 | `v2.2.x` |
| 123 | `49c3f9d3e` | 2026-06-29 | refactor | `bamboo` | 回退 bridge 层 ping goroutine，改由 SDK 驱动 keep-alive | `v2.2.x` |
| 124 | `f656ae0fb` | 2026-06-29 | fix | `usage-logs` | 修复 bamboo 交互列误分类与缓存命中率超 100% | `v2.2.x` |
| 125 | `f07d8bb5d` | 2026-06-28 | chore | `deps` | upgrade bamboo-messages to v0.7.3 | `v2.2.x` |
| 126 | `47711122c` | 2026-06-28 | fix | `i18n` | translate Chinese residuals and add missing keys in usage-logs locales | `v2.2.x` |
| 127 | `8735a489f` | 2026-06-28 | test | `bamboo` | add interaction type test cases for backend and frontend | `v2.2.x` |
| 128 | `62dbcb648` | 2026-06-28 | fix | `bamboo` | correct interaction type inference for bamboo relay logs | `v2.2.x` |
| 129 | `30383ca45` | 2026-06-26 | chore | `deps` | bump bamboo-messages to v0.7.2 | `v2.2.x` |
| 130 | `4b6b55137` | 2026-06-26 | debug | `bamboo` | 添加 finish_reason SSE 帧日志用于排查工具调用断流 | `v2.2.x` |
| 131 | `3b07de67a` | 2026-06-26 | chore | `deps` | bump bamboo-messages to v0.7.1 | `v2.2.x` |
| 132 | `bbe8d5bff` | 2026-06-26 | fix | `bamboo` | EventError 路径补充 serializer.Flush 确保 [DONE] 发送 | `v2.2.x` |
| 133 | `0f535d998` | 2026-06-25 | test | `bamboo` | add param override integration tests (4 providers × scenarios) | `v2.2.x` |
| 134 | `8b9807336` | 2026-06-25 | feat | `bamboo` | adapt param override error in ChatRelay | `v2.2.x` |
| 135 | `e5f1902b9` | 2026-06-25 | feat | `bamboo` | inject param override interceptor into provider | `v2.2.x` |
| 136 | `c12fc1da5` | 2026-06-25 | chore | `deps` | bump bamboo-messages to v0.7.0 | `v2.2.x` |
| 137 | `342f7c4b7` | 2026-06-25 | chore | `relay` | 升级 bamboo-messages 至 v0.6.4 | `v2.2.x` |
| 138 | `fa2203c5b` | 2026-06-25 | chore | `relay` | 升级 bamboo-messages 至 v0.6.3 | `v2.2.x` |
| 139 | `2a5514c85` | 2026-06-25 | chore | `relay` | 升级 bamboo-messages 至 v0.6.2 | `v2.2.x` |
| 140 | `08b4a5c40` | 2026-06-25 | chore | `relay` | 升级 bamboo-messages 至 v0.6.1 | `v2.2.x` |
| 141 | `b9009fa39` | 2026-06-24 | fix | `docker` | 构建阶段安装 git 以支持新发布依赖的拉取 | `v2.2.x` |
| 142 | `58f74ecde` | 2026-06-24 | chore | `relay` | 升级 bamboo-messages 至 v0.6.0 | `v2.2.x` |
| 143 | `a92f10444` | 2026-06-24 | feat | `relay` | 完善 Bamboo 日志记录的工具响应内容与名称解析 | `v2.2.x` |
| 144 | `26944f460` | 2026-06-24 | feat | `relay` | 实现 Bamboo 日志独立数据结构与解析逻辑 | `v2.2.x` |
| 145 | `ac5dce52c` | 2026-06-24 | chore | `relay` | 升级 bamboo-messages 至 v0.5.6 | `v2.2.x` |
| 146 | `2ac401bac` | 2026-06-24 | fix | `relay/bamboo` | 从 SDK 事件流累积响应内容块到日志详细记录 | `v2.2.x` |
| 147 | `a8bf8a05f` | 2026-06-24 | chore | `relay` | 升级 bamboo-messages 至 v0.5.5 | `v2.2.x` |
| 148 | `2548c0488` | 2026-06-24 | fix | `usage-logs` | 修复日志详情面板超出高度无法滚动的问题 | `v2.2.x` |
| 149 | `d1af2e021` | 2026-06-24 | fix | `relay/bamboo` | 从 N2N 中间态构建日志详细记录 | `v2.2.x` |
| 150 | `021b93979` | 2026-06-24 | chore | `relay` | 升级 bamboo-messages 至 v0.5.4 | `v2.2.x` |
| 151 | `4ce34e240` | 2026-06-24 | fix | `bamboo-relay` | 审查修复计时机、日志详情、缓存率列与翻译补全 | `v2.2.x` |
| 152 | `b0ad5ec2e` | 2026-06-23 | chore | `relay` | 升级 bamboo-messages 至 v0.5.2 | `v2.2.x` |
| 153 | `7014fbcd7` | 2026-06-23 | refactor | `usage-logs` | 优化 bamboo 计时展示交互 | `v2.2.x` |
| 154 | `39bea86b1` | 2026-06-23 | feat | `bamboo-relay` | 添加分阶段流式计时监控 | `v2.2.x` |
| 155 | `714bd9dd1` | 2026-06-23 | chore | `relay` | 升级 bamboo-messages 至 v0.5.0 | `v2.2.x` |
| 156 | `1cd7b9c7b` | 2026-06-23 | fix | `bamboo-relay` | 修复 coding-plan BaseURL 选择与 OpenAI /v1 路径缺失 | `v2.2.x` |
| 157 | `25e526276` | 2026-06-23 | feat | `bamboo-bridge` | 添加 TTFT 记录与流式平滑缓冲设置支持 | `v2.2.x` |
| 158 | `d04dae3a1` | 2026-06-23 | chore | `relay` | 升级 bamboo-messages 至 v0.4.11 | `v2.2.x` |
| 159 | `d2f250542` | 2026-06-22 | chore | `relay` | 升级 bamboo-messages 至 v0.4.10 | `v2.2.x` |
| 160 | `3e6264da3` | 2026-06-22 | feat | `bamboo-bridge` | 集成流式平滑缓冲策略并接入全局设置 | `v2.2.x` |
| 161 | `5c426af77` | 2026-06-22 | feat | `bamboo-bridge` | 完善格式链路追踪机制 | `v2.2.x` |
| 162 | `89f2055a1` | 2026-06-22 | chore | `relay` | 升级 bamboo-messages 至 v0.4.8 | `v2.2.x` |
| 163 | `d73bc1f9f` | 2026-06-21 | chore | `relay` | 升级 bamboo-messages 至 v0.4.7 | `v2.2.x` |
| 164 | `f6092676e` | 2026-06-21 | fix | `relay` | 重试失败时清除 Channel Affinity 缓存以允许切换通道 | `v2.2.x` |
| 165 | `1ea955cbe` | 2026-06-21 | feat | `bamboo-bridge` | 增加非流式空响应检测设置 ContextKeyEmptyResponse | `v2.2.x` |
| 166 | `220c11ac5` | 2026-06-21 | chore | `relay` | 升级 bamboo-messages 至 v0.4.6 | `v2.2.x` |
| 167 | `58829a1a4` | 2026-06-21 | chore | `relay` | 升级 bamboo-messages 至 v0.4.5 | `v2.2.x` |
| 168 | `168e5df50` | 2026-06-21 | refactor | `-` | 移除 main.go 中未使用的 model_setting 导入 | `v2.2.x` |
| 169 | `aefb230d4` | 2026-06-21 | feat | `relay` | 重构 bamboo debug 信息收集为结构化日志展示 | `v2.2.x` |
| 170 | `e3dd9e330` | 2026-06-20 | chore | `relay` | 升级 bamboo-messages 至 v0.4.1 | `v2.2.x` |
| 171 | `6a488b67e` | 2026-06-20 | chore | `relay` | 升级 bamboo-messages 至 v0.4.0 | `v2.2.x` |
| 172 | `d39173f18` | 2026-06-20 | fix | `relay` | 补全 bamboo-messages v0.3.3 的 go.sum 校验和 | `v2.2.x` |
| 173 | `c46b266c1` | 2026-06-20 | chore | `relay` | 升级 bamboo-messages 至 v0.3.3 | `v2.2.x` |
| 174 | `44c08d425` | 2026-06-20 | chore | `relay` | 升级 bamboo-messages 至 v0.3.2 | `v2.2.x` |
| 175 | `e3c3191a2` | 2026-06-20 | fix | `渠道` | 将 bamboo 上游格式选择提升为独立适配器分区 | `v2.2.x` |
| 176 | `cdd85d7c2` | 2026-06-20 | feat | `系统设置` | 添加 bamboo 上游格式与调试日志配置界面 | `v2.2.x` |
| 177 | `819ae41f1` | 2026-06-20 | feat | `relay` | 添加 bamboo 上游格式覆盖与调试日志同步 | `v2.2.x` |
| 178 | `0591ff22c` | 2026-06-19 | feat | `relay` | 适配 bamboo 桥接的 coding-plan 快捷 URL 与自定义 header 透传 | `v2.2.x` |
| 179 | `8018cf884` | 2026-06-19 | feat | `relay` | 升级 bamboo-messages 并优化不同 API 类型的兼容模式 | `v2.2.x` |
| 180 | `6915e1172` | 2026-06-19 | fix | `系统设置` | 修复 bamboo 中继桥开关无法开启的问题 | `v2.2.x` |
| 181 | `118ca1a90` | 2026-06-19 | feat | `系统设置` | 添加 bamboo 中继桥灰度开关管理界面 | `v2.2.x` |
| 182 | `f8bb29c00` | 2026-06-19 | feat | `relay` | 接入 bamboo-messages 协议归一化中继桥 | `v2.2.x` |
| 183 | `e45a8ab4c` | 2026-06-18 | docs | `bamboo-relay` | 新增 bamboo-messages 协议归一化中继桥设计与实现计划 | `v2.2.x` |
| 184 | `ad0dc56da` | 2026-06-14 | docs | `知识库` | 初始化分层 AGENTS.md 项目知识库 | `v2.2.x` |
| 185 | `b5da182fe` | 2026-06-14 | fix | `使用日志` | 恢复详细/完整日志按钮并修复 TPS 列缺值回退 | `v2.2.x` |
| 186 | `072ba4fac` | 2026-06-14 | fix | `仪表板` | 修复 Token 热力图工作日标签错位与圆度过大 | `v2.2.x` |
| 187 | `1ca4dba98` | 2026-06-14 | style | `使用日志` | 优化表格样式和标签对齐 | `v2.2.x` |
| 188 | `4bb0e9f90` | 2026-06-14 | fix | `仪表板` | 修复 PostgreSQL 日期格式与 Token 统计跨数据库兼容性 | `v2.2.x` |
| 189 | `68313abc6` | 2026-06-14 | feat | `日志` | 支持通用 X-Session-Id header 并明确优先级 | `v2.2.x` |
| 190 | `c4d3c7e5f` | 2026-06-14 | fix | `openai` | 修复 CustomEvent 指针接收者导致的编译错误 | `v2.2.x` |
| 191 | `09369b826` | 2026-06-14 | fix | `国际化` | 补充热力图组件缺失的翻译键 | `v2.2.x` |
| 192 | `91f9341bf` | 2026-06-14 | fix | `仪表板` | 修复 Token 使用热力图不显示及缺失汉化 | `v2.2.x` |
| 193 | `ff3c20f35` | 2026-06-14 | merge | `-` | 合并 main 分支最新变更到 newapi-xlf-v2 | `v2.2.x` |
| 194 | `90cdab9ca` | 2026-06-07 | fix | `usage-logs` | 修复 ChatCompletions 交互类型判断优先级与 Anthropic 对齐 | `v2.2.x` |
| 195 | `43ca4f20e` | 2026-06-02 | feat | `usage-logs` | 全面修复使用日志展示与交互问题 | `v2.2.x` |
| 196 | `a40b5adf0` | 2026-06-01 | refactor | `使用日志` | 优化会话信息显示层级关系 | `v2.2.x` |
| 197 | `d7e5ac2e1` | 2026-06-01 | merge | `-` | 整合前端主题重构与后端优化变更 | `v2.2.x` |
| 198 | `f994e5b46` | 2026-06-01 | other | `-` | Update AI tool entry in gitignore | `v2.2.x` |
| 199 | `d1f7f759d` | 2026-05-25 | chore | `i18n` | 同步多语言翻译并按字母序整理 key | `v2.2.x` |
| 200 | `f5b83fd84` | 2026-05-25 | refactor | `使用日志` | 迁移 Session 标签至统一颜色系统 | `v2.2.x` |
| 201 | `e8f421c53` | 2026-05-24 | refactor | `前端` | 统一颜色系统，迁移至 colors.ts 并优化 Badge 渲染 | `v2.2.x` |
| 202 | `43631e714` | 2026-05-24 | fix | `日志` | 工具回调场景下跳过 request blocks 记录 | `v2.2.x` |
| 203 | `9cac2ab31` | 2026-05-24 | fix | `log` | align ChatCompletions log parsing with Anthropic context-aware pattern | `v2.2.x` |
| 204 | `91ccdd9d1` | 2026-05-24 | feat | `使用日志` | 扩展 Session 追踪支持 OpenCode Session Affinity 及子 Agent 检测 | `v2.2.x` |
| 205 | `60238647e` | 2026-05-24 | feat | `使用日志` | 添加 Claude Code Agent/Session 追踪与前端展示 | `v2.2.x` |
| 206 | `cbd3e9133` | 2026-05-23 | feat | `使用日志` | 新增自动刷新 UI、翻页关闭自动刷新、列可见性修复及耗时列优化 | `v2.2.x` |
| 207 | `7a9b87e0d` | 2026-05-23 | refactor | `系统设置` | 移除模型设置中误放的站点自定义字段 | `v2.2.x` |
| 208 | `3b628388c` | 2026-05-23 | feat | `模型日志` | 新增 model-log 页面，复刻经典界面 token 统计功能 | `v2.2.x` |
| 209 | `e6fa6c80a` | 2026-05-22 | chore | `-` | checkpoint workspace changes on newapi-xlf-v2 | `v2.2.x` |
| 210 | `5a2c80ae7` | 2026-05-22 | fix | `openai` | 补回 streamItems 声明与收集，修复编译 undefined 错误 | `v2.2.x` |
| 211 | `4b2444525` | 2026-05-22 | fix | `日志` | 修复 details-dialog 中多余 )} 导致构建失败 | `v2.2.x` |
| 212 | `7e8216ba1` | 2026-05-22 | chore | `-` | 添加 .sisyphus/ 到 gitignore | `v2.2.x` |
| 213 | `31d4a5e5a` | 2026-05-22 | merge | `-` | 合并 upstream main 分支最新变更 | `v2.2.x` |
| 214 | `bee9cc610` | 2026-05-14 | feat | `前端` | 新增日志记录设置（详细记录/5分钟完整日志）并整理 i18n 键排序 | `v2.2.x` |
| 215 | `a01153de7` | 2026-05-14 | feat | `前端` | 新增使用日志详细/完整日志查看、OpenAI 结构化交互推断与 i18n 补全 | `v2.2.x` |
| 216 | `159fcb932` | 2026-05-14 | feat | `relay` | 实现 Responses API 自动降级到 Chat Completions 并增强日志摘要推断 | `v2.2.x` |
| 217 | `58037f6a1` | 2026-05-14 | fix | `relay` | 提取 namespace 类型工具中嵌套的 function 工具定义 | `v2.2.x` |
| 218 | `33e47f4e1` | 2026-05-14 | fix | `relay` | 修复 Responses→Chat Completions 转换器审计发现的 11 个问题 | `v2.2.x` |
| 219 | `3570b8ec6` | 2026-05-14 | fix | `relay` | 跳过 Responses API 中非 function 类型的工具定义 | `v2.2.x` |
| 220 | `ff07425f8` | 2026-05-14 | fix | `-` | 修复 Responses→Chat Completions 转换中 developer role 未映射为 system 的问题 | `v2.2.x` |
| 221 | `d0a311976` | 2026-05-14 | feat | `-` | 新增客制化操作区 section，修复保存刷新丢失 bug | `v2.2.x` |
| 222 | `33208d261` | 2026-05-14 | feat | `relay` | wire Responses→Chat conversion, add UI toggle and unit tests | `v2.2.x` |
| 223 | `f0256ffa4` | 2026-05-14 | feat | `relay` | add Responses↔Chat Completions conversion layer | `v2.2.x` |
| 224 | `f550b77b7` | 2026-05-14 | feat | `setting` | add ResponsesToChatCompletionsEnabled setting field and i18n keys | `v2.2.x` |
| 225 | `f015a9e50` | 2026-05-14 | merge | `-` | 合并 main 分支至 newapi-xlf-v2 | `v2.2.x` |
| 226 | `5dd669fd2` | 2026-05-04 | feat | `web/default` | 同步 classic 前端功能至 default 主题 | `v2.2.x` |
| 227 | `9a9c660b4` | 2026-05-03 | feat | `playground,日志` | 实现 tool_calls 支持与使用日志结构化解析 | `v2.2.x` |
| 228 | `c1e6cf366` | 2026-05-03 | other | `-` | Merge branch 'main' into newapi-xlf-v2 | `v2.2.x` |
| 229 | `16563cead` | 2026-04-29 | fix | `log` | 修复 mergeResponsesText 参数类型不匹配导致编译失败 | `v2.1.x` |
| 230 | `302be3fdc` | 2026-04-29 | chore | `classic` | 删除已迁移至 classic 目录的旧路径文件 | `v2.1.x` |
| 231 | `439a51c95` | 2026-04-29 | fix | `classic` | 迁移 ModelLog 和 SettingsEmptyResponseRetry 到 classic 目录 | `v2.1.x` |
| 232 | `005b7888e` | 2026-04-29 | other | `-` | Merge branch 'main' into newapi-xlf-v2 | `v2.1.x` |
| 233 | `0c21ce284` | 2026-04-29 | feat | `日志` | 添加 OpenAI Chat-Completions 结构化请求块记录与前端渲染 | `v2.1.x` |
| 234 | `4754a7679` | 2026-04-25 | feat | `-` | 合并表达式计费系统和 Waffo Pancake 支付功能 | `v2.1.x` |
| 235 | `ad8b72431` | 2026-04-14 | style | `模型日志` | 优化折线图坐标轴和数据显示样式 | `v2.1.x` |
| 236 | `f093f74f9` | 2026-04-13 | merge | `-` | 合并 main 分支，保留双方新增功能与翻译 | `v2.1.x` |
| 237 | `25096e696` | 2026-04-13 | ci | `docker` | 更新镜像名和 action 版本，统一改为 tag 触发构建 | `v2.1.x` |
| 238 | `7497cc91c` | 2026-04-13 | refactor | `模型日志` | 替换热力图为折线图对比方案并添加模型筛选 | `v2.1.x` |
| 239 | `78c93f1f9` | 2026-04-13 | chore | `-` | 清理不需要的 .github 社区模板和工作流 | `v2.1.x` |
| 240 | `2ecf5654e` | 2026-04-06 | fix | `模型日志` | 纠正 total_tokens 统计口径为输出 Token 并优化热力图样式 | `v2.1.x` |
| 241 | `008949393` | 2026-04-06 | refactor | `流式处理` | 重构流扫描器使用 StreamResult 模式并优化 CustomEvent 传参 | `v2.1.x` |
| 242 | `3998427c1` | 2026-04-05 | feat | `日志` | 添加模型日志全局统计摘要卡片 | `v2.1.x` |
| 243 | `bd0241a61` | 2026-04-01 | feat | `log` | 日志添加失败率展示，以及处理自动刷新的时间范围间隔 | `v2.1.x` |
| 244 | `b039f1be3` | 2026-03-30 | feat | `日志` | 添加 OpenAI 结构化响应日志解析与 TPS 热力图可视化 | `v2.1.x` |
| 245 | `034df0604` | 2026-03-28 | feat | `日志` | 令牌记录使用上游模型名称并优化热力图暗色主题适配 | `v2.1.x` |
| 246 | `7889d73d1` | 2026-03-27 | refactor | `日志` | 重构模型日志热力图为卡片式布局并优化视觉风格 | `v2.1.x` |
| 247 | `bfb37a59c` | 2026-03-27 | feat | `日志` | 添加 Claude 非流式响应结构化日志解析并聚合多段文本与工具调用 | `v2.1.x` |
| 248 | `f2d629380` | 2026-03-27 | refactor | `日志` | 简化模型统计卡片为悬浮提示展示 | `v2.1.x` |
| 249 | `a84b891a1` | 2026-03-27 | feat | `日志` | 将开发工具日志权限改为按角色控制并优化模型热力图 | `v2.1.x` |
| 250 | `be59e1741` | 2026-03-26 | refactor | `日志` | 优化模型日志热力图响应式布局 | `v2.1.x` |
| 251 | `8eb1b0cd7` | 2026-03-26 | refactor | `日志` | 提取令牌记录自增表达式构建函数 | `v2.1.x` |
| 252 | `a85f95f15` | 2026-03-26 | feat | `日志` | 添加开发工具日志权限控制与客户端来源摘要提取 | `v2.1.x` |
| 253 | `1adf4b480` | 2026-03-26 | feat | `日志` | 添加 Responses 请求数据从 prompt.input 兜底解析逻辑 | `v2.1.x` |
| 254 | `cc230bb05` | 2026-03-26 | feat | `日志` | 添加自动刷新倒计时与固定刷新间隔配置 | `v2.1.x` |
| 255 | `8ac5cfb7c` | 2026-03-26 | refactor | `日志` | 移除 Codex 客户端交互类型隐藏逻辑 | `v2.1.x` |
| 256 | `2b6d64e9f` | 2026-03-26 | refactor | `日志` | 重构响应输入解析逻辑仅提取最新回调片段 | `v2.1.x` |
| 257 | `bde3e831a` | 2026-03-26 | feat | `日志` | 添加工具调用日志深层截断与水平滚动展示 | `v2.1.x` |
| 258 | `913015ba7` | 2026-03-26 | feat | `日志` | 重构 Claude 日志展示结构并分离工具调用与文本回答 | `v2.1.x` |
| 259 | `26bceec25` | 2026-03-26 | feat | `日志` | 添加 Claude 流式响应结构化解析与展示 | `v2.1.x` |
| 260 | `bd838acf6` | 2026-03-26 | feat | `日志` | 拆分摘要与完整日志记录为独立开关 | `v2.1.x` |
| 261 | `3bb56846b` | 2026-03-26 | feat | `日志` | 添加完整日志记录功能并支持 5 分钟限时开启 | `v2.1.x` |
| 262 | `f70ce6056` | 2026-03-25 | fix | `序列化` | 优化 JSON 序列化降级策略避免数据丢失 | `v2.1.x` |
| 263 | `8399a0425` | 2026-03-25 | fix | `日志` | 修复 other 字段解析失败导致日志信息丢失 | `v2.1.x` |
| 264 | `6ddaf47f6` | 2026-03-25 | feat | `日志` | 添加日志文件查看与清理接口 | `v2.1.x` |
| 265 | `d7b307184` | 2026-03-23 | feat | `日志` | 添加日志自动刷新功能并优化来源标签颜色 | `v2.1.x` |
| 266 | `4e7881d59` | 2026-03-22 | chore | `发布` | 升级版本到 2.1.7 | `v2.1.x` |
| 267 | `e25339e82` | 2026-03-22 | feat | `日志` | 扩展客户端来源识别并支持非流式 TPS 计算 | `v2.1.x` |
| 268 | `0907ea5da` | 2026-03-22 | feat | `日志` | 修复 responses 渠道响应文本记录缺失问题 | `v2.1.x` |
| 269 | `a96a90c24` | 2026-03-22 | fix | `日志` | 修正交互类型判断逻辑优先级 | `v2.1.x` |
| 270 | `051273608` | 2026-03-22 | feat | `日志` | 添加日志来源与交互类型列展示 | `v2.1.x` |
| 271 | `f422213e7` | 2026-03-21 | feat | `日志` | 优化日志详情展示并精简 prompt 存储 | `v2.1.x` |
| 272 | `4d21c324f` | 2026-03-20 | feat | `日志` | 完善多渠道请求与响应内容记录 | `v2.1.x` |
| 273 | `fe39b1a87` | 2026-03-20 | feat | `日志` | 引入按钮组件为日志详情做准备 | `v2.1.x` |
| 274 | `1d4efe3c9` | 2026-03-20 | feat | `日志` | 在消费日志中添加记录详情字段 | `v2.1.x` |
| 275 | `e72bac898` | 2026-03-20 | feat | `日志` | 在消费日志中添加记录详情字段 | `v2.1.x` |
| 276 | `201b179f7` | 2026-03-20 | feat | `日志` | 添加 TPS 字段独立存储与详情记录开关 | `v2.1.x` |
| 277 | `19e17736f` | 2026-03-20 | feat | `日志` | 添加消费日志详情记录与展示功能 | `v2.1.x` |
| 278 | `2ef655bd5` | 2026-03-20 | feat | `重试` | 添加空响应重试机制及 TPS 指标计算 | `v2.1.x` |

---

> *本台账由代码助手于 2026-10-07 基于本地 Git 历史深度归纳生成，合并主线时请保持更新。*