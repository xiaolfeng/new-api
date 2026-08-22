# web 知识库

## 概述

管理控制台与用户门户。单仓前端（不再分 `default` / `classic`），React 19 + TypeScript + Rsbuild + Base UI + Tailwind。按 `src/features/<feature>/` 划分功能域，路由用 TanStack Router 文件路由。

## 目录结构

```text
web/
├── package.json / rsbuild.config.ts / bun.lock
├── scripts/                 # i18n:sync、copyright、format 包装
├── public/
└── src/
    ├── main.tsx             # 入口
    ├── routes/              # 文件路由（_authenticated / (auth) / (errors)）
    ├── features/            # 功能域（channels、keys、users、wallet、playground…）
    ├── components/          # 通用组件
    │   ├── ui/              # 基础 UI
    │   ├── layout/          # 壳层布局
    │   ├── data-table/      # 表格
    │   └── ai-elements/     # 对话 / 推理 / 工具卡片
    ├── stores/              # Zustand
    ├── hooks/ / lib/ / context/ / config/
    ├── i18n/locales/        # en / zh / zh-TW / fr / ru / ja / vi
    └── styles/
```

## 导航指南

| 任务 | 位置 | 说明 |
|------|------|------|
| 加一个页面 | `src/features/<feature>/` + `src/routes/` | feature 放 UI / api / hooks；route 只做接线 |
| 改登录 / OAuth / Passkey | `src/features/auth/` | 含 otp、secure-verification |
| 改渠道管理 | `src/features/channels/` | 表格、抽屉、模型映射 |
| 改系统设置 | `src/features/system-settings/` | 按 auth / billing / models / integrations 分子目录 |
| 改 Playground 对话 | `src/features/playground/` + `components/ai-elements/` | 流式在 `use-stream-request.ts` |
| 改通用表格 | `src/components/data-table/` | 不要在 feature 里再造一套 |
| 改 i18n | `src/i18n/locales/*.json` | 键为英文源文案；改完 `bun run i18n:sync` |
| 调 API | 各 feature 的 `api.ts` | 统一 axios 实例，React Query 缓存 |

## 约定

- **包管理只用 bun**：`bun install` / `bun add` / `bun run <script>`。不要用 npm / yarn / pnpm。
- **i18n**：用户可见文案必须 `useTranslation()` + `t('English key')`。非 React 环境可用 `import { t } from 'i18next'`，但不会随语言切换重渲染。子组件即使父级已调用 hook 也要自己取 `t`。`SUCCESS_MESSAGES` / `ERROR_MESSAGES` 的值只是 i18n 键，展示时必须再包一层 `t()`。状态 label 用 `labelKey`（或统一的 `label` 键），同一 feature 内不要混用。
- **TypeScript**：避免 `any`；类型导入用 `import type`。改 TS/TSX 后必须 `bun run typecheck`（`tsgo -b`）到零错误。提交前对改动文件跑 lint，修掉全部 error。
- **组件**：函数组件 + Hooks；props 非必要不解构，直接 `props.xxx`。禁止两层及以上嵌套三元。单文件约 200 行考虑拆分。
- **状态**：Zustand store 放 `src/stores/`，组件用选择器订阅。持久化在 store 内读写 localStorage。
- **请求**：`useQuery` / `useMutation`，`queryKey` 用层级数组；变更后 `invalidateQueries`。服务端错误走 `handleServerError`。axios 实例带 `withCredentials: true`。
- **表单**：React Hook Form + Zod，schema 放 feature 的 `lib/`，`z.infer` 导出类型。
- **路由**：`createFileRoute`；search 用 Zod + `validateSearch`；鉴权与重定向放 `beforeLoad`。导航用 `useNavigate` / `Link`，不要直接改 `window.location`。
- **样式**：Tailwind + `cn()`；移动优先；主题用 CSS 变量与 `dark:`。自定义 CSS 集中在 `src/styles/`。
- **文件组织**：feature 内含 `components/`、`lib/`、`hooks/`，以及按需的 `api.ts`、`types.ts`、`constants.ts`。通用组件在 `src/components/`，工具在 `src/lib/`。组件文件 PascalCase，工具 kebab-case。
- **测试**：Vitest + React Testing Library。测试放模块专属 `__tests__/`，按职责命名（`layout.test.ts`）。测用户可见行为，不测内部 state。Bug 先写失败用例再修。布局 / 焦点 / 键盘 / 空态 / 错误态变更必须补回归。提交前至少跑受影响测试 + typecheck + 相关 lint。
- **可访问性**：语义化 HTML、键盘可达、对比度 WCAG 2.1 AA。装饰图标 `aria-hidden="true"`。
- **安全**：不在前端存密钥；慎用 `dangerouslySetInnerHTML`；前后端都要校验。
- **品牌**：不得删除或替换 new-api / QuantumNous 相关署名与标识（copyright 脚本会检查）。

## 反模式

- ❌ 在 `web/` 使用 npm / yarn / pnpm。
- ❌ 用户文案不走 `t()`，或把 `SUCCESS_MESSAGES.xxx` 直接当最终字符串 toast。
- ❌ 在路由文件里堆业务逻辑——路由只接线，逻辑回 feature。
- ❌ 直接操作 `window.location` 做站内跳转。
- ❌ 测试与实现平铺在同一目录，或用大段 class 快照 / `sleep` 冒充测试。
- ❌ mock 被测模块自身，或为覆盖率写 smoke 测试。
- ❌ 改完 TS 不跑 typecheck，或留下 lint error。

## 调试路径

1. 页面空白 / 路由 404 → `src/routes/` 文件名与 `routeTree.gen.ts` 是否同步（dev 会生成）。
2. 登录后仍跳转登录页 → `routes/_authenticated` 的 `beforeLoad` 与 `stores` 里的 auth。
3. 接口 401 / CSRF → axios `withCredentials`、cookie、后端 CORS。
4. 文案未翻译 → 对应 locale 是否缺键；`bun run i18n:sync` 报告。
5. 表格行为异常 → 先看 `components/data-table/`，再看 feature 列定义。
6. Playground 流式中断 → `features/playground/hooks/use-stream-request.ts`。
7. 类型失败 → `bun run typecheck`；lint → `bun run lint`。

## 引用

无子级 `AGENTS.md`。后端约定见根 [`AGENTS.md`](../AGENTS.md)。
