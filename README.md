<div align="center">

![new-api](/web/public/logo.png)

# New API

**XiaoLFeng Edition —— 带协议归一化中继内核、真实交付计时与深度会话观测的 AI 网关**

<p align="center">
  <a href="./LICENSE">
    <img src="https://img.shields.io/badge/license-AGPLv3-brightgreen" alt="license">
  </a><!--
  --><a href="https://github.com/xiaolfeng/new-api/releases/latest">
    <img src="https://img.shields.io/github/v/release/xiaolfeng/new-api?color=brightgreen&include_prereleases" alt="release">
  </a><!--
  --><a href="https://github.com/xiaolfeng/new-api/commits/newapi-xlf-v2">
    <img src="https://img.shields.io/badge/branch-newapi--xlf--v2-blue" alt="branch">
  </a>
</p>

<p align="center">
  <a href="#版本核心特性">版本核心特性</a> •
  <a href="#基础能力">基础能力</a> •
  <a href="#快速开始">快速开始</a> •
  <a href="#部署">部署</a> •
  <a href="#开发">开发</a> •
  <a href="#文档">文档</a> •
  <a href="#上游与许可">上游与许可</a>
</p>

</div>

---

## 项目简介

这是由 **XiaoLFeng** 深度定制并独立维护的 New API 版本（分支 `newapi-xlf-v2`）：一个自托管的 AI 网关，面向应用、Agent 与团队。它在完整保留基础平台能力（多供应商路由、配额计费、访问控制、Web 控制台）的同时，围绕**协议归一化、真实请求计时与 Agent 级会话观测**构建了一套自研子系统。

接入上游模型服务，对客户端暴露统一 API，在一个控制台里管理路由、访问、用量与成本。支持的上游包括 OpenAI、Anthropic、Google Gemini、Azure OpenAI、AWS Bedrock、Vertex AI、DeepSeek、Qwen、Grok 等兼容服务。

> [!NOTE]
> 本版本的全部变更历史（278+ 次提交）、上游合并防冲突矩阵与同步 SOP，见 [XLF-CHANGELOG.md](./XLF-CHANGELOG.md)。

> [!IMPORTANT]
> - 本项目仅用于合法授权的 AI API 网关、组织级鉴权、多模型管理、用量分析与成本核算等私有部署场景。
> - 使用者须合法获取上游 API Key、账号与接口权限，并遵守上游服务条款及所在司法辖区的法律法规。
> - 对公众提供生成式 AI 服务时，应完成所在辖区要求的备案、许可、内容安全、实名核验、日志留存与纳税等义务。

---

## 版本核心特性

在基础平台之上，本版本构建了六个自研子系统，均有测试与文档覆盖：

### 1. Bamboo 协议归一化中继内核

`relay/bamboo/` 是本版本的第二个中继内核：将四大文本协议（OpenAI Chat、OpenAI Responses、Anthropic Messages、Gemini）先归一化为一条结构化事件流，再按客户端入口格式重新序列化。上游经由 `bamboo-messages` SDK 接入，提供：

- 渠道级参数覆盖（SDK 拦截器实现）；
- 结构化流式事件（thinking / text / tool_use），不再从字符串反解析；
- **宿主侧工具**：服务端执行的 Web Search / Web Fetch / 识图，以工具调用进度流式回传给客户端（设计见 `docs/rfc/RFC-0001~0003`）；
- 空响应检测与自动重试，包括上游返回 200 的静默空流；
- 流式中断降级策略（DegradedReason）与渠道级 `StripThinkTags` 内联思考标签剥离；
- Grok 客户端工具与搜索结果信封透传、Gemini 原生 WebSearch 适配为宿主工具。

当供应商不被 Bamboo 支持时，中继自动回退到经典适配器链路，渠道配置无需任何改动。

### 2. 服务端真实交付计时

告别整秒粒度的 `use_time`，每个请求都在真实写出边界计时：

| 字段 | 含义 |
| --- | --- |
| `delivery_timing.ttft_ms` | 首个**含有效内容**的字节实际刷出给客户端的耗时（毫秒） |
| `delivery_timing.total_ms` | 服务端完整交付耗时（毫秒） |
| `delivery_timing.status` | `completed` / `cancelled` / `write_error` / `upstream_error` |
| `bamboo_timing_hops[]` | 每次尝试的上游计时（总耗时、首字、thinking/输出/工具分段、分段 token 速率） |

计时模型严格区分「响应头已提交」与「内容已交付」，跨重试与多跳中继保持同一请求级时钟；并内置零值时钟防护——无首字的失败流不会再以公元元年偏移污染 TTFT 与吞吐指标（`FirstResponseDurationMs()` 对异常时间安全返回 0）。

### 3. Agent 与会话感知

`common/client_profile.go` 对客户端做特征指纹并从请求体提取会话结构，使用日志可以直接回答「哪个 Agent、哪个会话、哪一跳」：

- Claude Code（Agent + Session + 主线程/SubAgent 映射）、Codex、ZCode、OpenCode、Pi；
- 会话亲和与子 Agent 识别以「来源 / 会话」列直接展示在控制台；
- 内部宿主工具请求被打标，不计入 TPS 与缓存命中率统计。

### 4. 面向运营的可观测性

- **Token 热力图**（model-log）：周一对齐的 7×24 小时 TPS 网格，tooltip Portal 挂载杜绝漂移，支持分段计时（thinking / 输出 / 工具）；
- **结构化日志详情**：Claude / OpenAI / Responses 的请求与响应解析为可读块，工具调用与正文分离，深层内容截断加横向滚动；
- **分阶段 TPS 趋势**：父级一次聚合、多图共享，不再重复计算；
- 日志列表响应剥离 `record` / `full_log` 大字段，详情弹窗按需经 `GET /api/log/detail`（以 `request_id` 定位）懒加载。

### 5. 真实数据库下的可靠性加固

所有修复均在 SQLite、MySQL ≥ 5.7.8、PostgreSQL ≥ 9.6 三库验证，ClickHouse 可作为独立日志库：

- 计数查询以子查询封顶（`LIMIT 10001`），管理端日志搜索不会对大表全表扫描；
- 批量删除使用 `id IN (SELECT id … LIMIT n)`——裸 `DELETE … LIMIT` 在 SQLite/PG 上会被静默忽略；
- 失败计数采用 CAS 乐观锁重试循环，并发失败永不互相覆盖；
- 看板缓存条目在落库失败时保留并在下次以累计差值补写；
- ClickHouse 建表补齐 `record` / `full_log` / `tps` 列与 `token_record` 表。

### 6. 前端体验打磨

周一对齐热力图、tooltip Portal 到 `document.body`（免疫动画 `filter` 建立的包含块）、基于 `request_id` 的稳定行 ID、详情弹窗打开期间自动暂停刷新、统计 query key 不随翻页重复请求、移动端日志卡片与桌面端一致的计时指标。

---

## 基础能力

| 领域 | 能做什么 |
| --- | --- |
| 模型接入 | OpenAI Chat Completions / Responses、Anthropic Messages、Gemini API；流式、工具调用、推理、多模态按上游能力开放 |
| 路由 | 模型映射、渠道优先级与权重、重试、渠道亲和、多上游 Key |
| 用量与成本 | 配额、订阅、用量日志、缓存计费、表达式阶梯计费 |
| 访问控制 | 用户 / 分组 / 细粒度权限 / API Key 限制；OAuth/OIDC、Passkey、两步验证、会话管理 |
| 异步任务 | 以 JavaScript 插件扩展图像、视频等任务 API，含任务状态与产物获取 |
| Web 控制台 | 渠道与模型配置、用量与审计日志、Playground；支持英文、简中、繁中、法、日、俄、越 |

### 协议与端点

| 接口 | 常用端点 |
| --- | --- |
| OpenAI Chat / Responses | `POST /v1/chat/completions`、`POST /v1/responses` |
| Anthropic Messages | `POST /v1/messages` |
| Gemini | `POST /v1beta/models/{model}:generateContent`、`POST /v1beta/models/{model}:streamGenerateContent` |
| Realtime / Responses WebSocket | `GET /v1/realtime`、`GET /v1/responses`（WebSocket 升级） |
| 图像 / 音频 | `/v1/images/generations`、`/v1/images/edits`、`/v1/audio/speech`、`/v1/audio/transcriptions`、`/v1/audio/translations` |
| Embeddings / 重排 | `POST /v1/embeddings`、`POST /v1/rerank` |
| 任务插件 | `POST /v1/tasks/{pluginKey}`、`GET /v1/tasks/{taskId}` 及各插件声明的路由 |

[RelayKit](./relaykit/README.md) 负责四大文本协议间的请求 / 响应 / 流式转换，Bamboo 内核构建于其上。实际可用能力取决于渠道、上游模型与转换路径；协议特有字段与工具可能无法精确映射。WebSocket 需上游与渠道配置同时支持。

---

## 快速开始

本版本通过本仓库的 tag（`v2.1.7`、`v2.3.x`、`v2.4.x` …）经 [alpha 镜像工作流](./.github/workflows/docker-image-alpha.yml) 发布（镜像名 `<dockerhub-namespace>/newapi-fix`）。要运行本版本代码，请从源码构建——上游官方镜像**不包含**本版改动：

```bash
git clone -b newapi-xlf-v2 https://github.com/xiaolfeng/new-api.git
cd new-api

# 构建前端（会嵌入 Go 二进制）
cd web && bun install --frozen-lockfile && bun run build && cd ..

# 以 SQLite 启动
go run .
```

打开 [http://localhost:3000](http://localhost:3000)，按引导完成管理员账号初始化。

### 发起第一个请求

1. 添加渠道：填入上游 API Key、可用模型与分组，跑一次渠道测试；
2. 配置模型价格，确保用户有配额或有效订阅；
3. 在控制台创建可访问同分组与模型的 API Key；
4. OpenAI 兼容客户端把 Base URL 设为 `http://localhost:3000/v1`，使用**本网关签发的 Key**。

```bash
export NEW_API_KEY=sk-...
curl --fail-with-body http://localhost:3000/v1/models \
  -H "Authorization: Bearer ${NEW_API_KEY}"

curl --fail-with-body http://localhost:3000/v1/responses \
  -H "Authorization: Bearer ${NEW_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"model":"your-enabled-model","input":"Hello!"}'
```

---

## 部署

仓库自带的 [Compose 配置](./docker-compose.yml) 默认启动 **New API + PostgreSQL + Redis**，并含 MySQL 与独立 ClickHouse 日志库的示例。

启动前编辑 `docker-compose.yml`：替换数据库与 Redis 的示例密码（服务与连接串两处都要改），并设置持久随机的 `SESSION_SECRET`（可用 `openssl rand -hex 32` 生成）。控制台走 HTTPS 时，配置 `SESSION_COOKIE_SECURE=true` 与精确公网 HTTPS 源的 `SESSION_COOKIE_TRUSTED_URL`。

```bash
docker compose up -d
docker compose logs -f new-api
```

### 存储与配置

| 组件 | 选项 |
| --- | --- |
| 主数据库 | SQLite、MySQL ≥ 5.7.8、PostgreSQL ≥ 9.6 |
| 独立日志库 | `LOG_SQL_DSN` 配置；支持 ClickHouse |
| 缓存 | 可选 Redis + 内存缓存；多节点共享限流时使用共享 Redis |
| 容器平台 | Linux amd64 / arm64 |

| 变量 | 用途 |
| --- | --- |
| `SQL_DSN` | 主库连接串；不设则使用 SQLite |
| `LOG_SQL_DSN` | 独立日志库连接串 |
| `REDIS_CONN_STRING` | Redis 连接串 |
| `SESSION_SECRET` | 持久化认证密钥；所有节点必须一致 |
| `CRYPTO_SECRET` | 默认取 `SESSION_SECRET`；共享 Redis 的节点必须一致 |
| `SESSION_COOKIE_SECURE` | HTTPS 控制台设为 `true`；启用 Secure 刷新 Cookie 与严格的刷新/登出源校验 |
| `SESSION_COOKIE_TRUSTED_URL` | Secure 模式必填：精确 HTTPS 源逗号分隔，不带路径与通配符；本地 HTTP 留空 |
| `TRUSTED_PROXIES` | 受信反代 IP/CIDR 或 `none`；按网络环境显式配置 |

完整配置见[环境变量示例](./.env.example)、上游[环境变量参考](https://docs.newapi.ai/en/docs/installation/config-maintenance/environment-variables)与[认证与会话指南](./docs/authentication.md)。

生产环境请将控制台置于 HTTPS 之后，并为流式与 WebSocket 升级正确配置反代。多节点部署必须共享主数据库与认证密钥。升级前务必备份数据库与挂载数据，并评估迁移与兼容性。

### 同步上游主线

本版本会定期合并上游主线。自行同步前，务必先读 [XLF-CHANGELOG.md](./XLF-CHANGELOG.md#2-主线同步标准作业程序-upstream-sync-sop) 中的**防冲突矩阵与合并 SOP**——其中列出了六个必须穿越合并存活下来的子系统不变量，以及合并后必须全数通过的构建 / 测试关卡。

---

## 开发

后端为 Go + Gin；控制台为 React 19、TypeScript、Rsbuild、TanStack、Tailwind CSS 4。前端依赖与脚本一律使用 Bun；Go 语言基线见 [go.mod](./go.mod)，容器构建工具链见 [Dockerfile](./Dockerfile)。

```bash
# 后端（仓库根目录）
go build ./...
go test ./...
make test

# RelayKit 必须独立于 workspace 构建
cd relaykit && GOWORK=off go build ./...

# 前端
cd web
bun install
bun run dev -- --port 5173   # API 代理到 :3000 后端
bun run typecheck && bun run lint && bun run test && bun run build
```

| 位置 | 职责 |
| --- | --- |
| `router/`、`middleware/`、`controller/` | HTTP 路由、访问校验、API 处理器 |
| `relay/` | 上游适配器与请求路由 |
| `relay/bamboo/` | 本版归一化中继内核与宿主工具（见 `relay/bamboo/AGENTS.md`） |
| `service/`、`model/` | 业务逻辑与持久化 |
| [relaykit/](./relaykit/README.md) | 可独立构建的协议 DTO 与转换 Go 模块 |
| [plugins/tasks/](./plugins/tasks/) | JavaScript 任务插件；见[任务插件 API v1](./docs/plugin-api/v1.md) |
| `web/` | Web 控制台；见[前端约定](./web/AGENTS.md) |
| [electron/](./electron/README.md) | 桌面封装与打包 |
| [XLF-CHANGELOG.md](./XLF-CHANGELOG.md) | 版本变更台账、防冲突矩阵与合并 SOP |

贡献前先读 [AGENTS.md](./AGENTS.md)——它是项目知识库索引，并载有项目治理策略。

---

## 文档

| 资源 | 链接 |
| --- | --- |
| 本版变更与合并 SOP | [XLF-CHANGELOG.md](./XLF-CHANGELOG.md) |
| 项目知识库 | [AGENTS.md](./AGENTS.md) 及各目录 `AGENTS.md` |
| 本版设计记录 | `docs/rfc/`（宿主工具、客户端画像搜索回传、Gemini WebSearch）、`docs/superpowers/`（Bamboo 中继桥设计） |
| 基础平台文档 | [指南](https://docs.newapi.ai/en/docs) · [安装](https://docs.newapi.ai/en/docs/installation) · [API 参考](https://docs.newapi.ai/en/docs/api) |
| 缺陷与功能反馈（本 fork） | [GitHub Issues](https://github.com/xiaolfeng/new-api/issues) |
| 安全报告 | 按上游[安全策略](./.github/SECURITY.md)私下报告 |

---

## 上游与许可

本版本是 **[QuantumNous/new-api](https://github.com/QuantumNous/new-api)** 的深度 fork；上游又基于 [One API](https://github.com/songquanpeng/one-api)（MIT License）开发。基础平台的网关核心、Web 控制台与供应商适配器来自上游项目及其贡献者，本仓库的版本层由 XiaoLFeng 维护。本版特有的问题请[在本仓库提 Issue](https://github.com/xiaolfeng/new-api/issues)，勿向上游提交。

本项目采用 [GNU Affero General Public License v3.0 (AGPLv3)](./LICENSE) 授权。

Additional terms under AGPLv3 Section 7 apply. Modified versions must preserve
the author attribution notice `Frontend design and development by New API
contributors.` in the appropriate legal notices and in any prominent about,
legal, footer, or attribution location presented by the user interface.

Modified versions that present a user interface must also preserve a visible
link to the original project: <https://github.com/QuantumNous/new-api>.

署名与依赖许可另见 [NOTICE](./NOTICE) 与 [THIRD-PARTY-LICENSES.md](./THIRD-PARTY-LICENSES.md)。

---

<div align="center">

### 感谢使用 New API

**[版本变更台账](./XLF-CHANGELOG.md)** • **[问题反馈](https://github.com/xiaolfeng/new-api/issues)** • **[最新发布](https://github.com/xiaolfeng/new-api/releases)**

<sub>版本层由 XiaoLFeng 维护 · 基础平台来自 QuantumNous 与 New API contributors</sub>

</div>
