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

优先复用项目已有组件与能力；项目内无合适实现时，再评估已安装依赖及成熟开源库。仅在复用、组合或合理扩展仍无法满足需求时自行实现，并说明具体能力缺口。

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
- **优先复用已有组件**：
  | 场景 | 优先检查的项目入口 |
  | --- | --- |
  | 通用弹窗布局 | `@/components/dialog` |
  | 删除、危险操作及普通确认 | `@/components/confirm-dialog` |
  | 复制按钮与剪贴板交互 | `@/components/copy-button`、`@/hooks/use-copy-to-clipboard` |
  | 空状态、加载状态、错误状态 | `@/components/empty-state`、`@/components/loading-state`、`@/components/error-state` |
  | 表格、分页、工具栏及列表布局 | `@/components/data-table` |
  | 基础控件 | `@/components/ui/` |
- **i18n 与文案**：用户可见文案必须 `useTranslation()` + `t('English key')`。`SUCCESS_MESSAGES` / `ERROR_MESSAGES` 的值只是 i18n 键，展示时必须再包一层 `t()`。状态 label 用 `labelKey`（或统一的 `label` 键），同一 feature 内不要混用。
- **数字格式化与 Intl 本地化**：
  - 普通数字使用 `@/lib/format` 的 `formatNumber` / `formatCompactNumber`；金额与额度使用 `@/lib/currency`。
  - `zhCN` / `zhTW` 是项目的界面语言码，在进入 `Intl.*` 或日期时间格式化前必须经 `@/i18n/languages` 的 `toIntlLocale()` 转换。
- **TypeScript**：避免 `any`；类型导入用 `import type`。改 TS/TSX 后必须 `bun run typecheck` 到零错误。
- **组件**：函数组件 + Hooks；props 非必要不解构，直接 `props.xxx`。禁止两层及以上嵌套三元。单文件约 200 行考虑拆分。
- **状态**：Zustand store 放 `src/stores/`，组件用选择器订阅。持久化在 store 内读写 localStorage。
- **请求**：`useQuery` / `useMutation`，`queryKey` 用层级数组；变更后 `invalidateQueries`。服务端错误统一走 `handleServerError`。axios 实例带 `withCredentials: true`。
- **表单**：React Hook Form + Zod，schema 放 feature 的 `lib/`，`z.infer` 导出类型。
- **路由**：`createFileRoute`；search 用 Zod + `validateSearch`；鉴权与重定向放 `beforeLoad`。导航用 `useNavigate` / `Link`，不要直接改 `window.location`。
- **样式**：Tailwind + `cn()`；移动优先；主题用 CSS 变量与 `dark:`。自定义 CSS 集中在 `src/styles/`。
- **测试**：Vitest + React Testing Library。测试放模块专属 `__tests__/`，按职责命名。测用户可见行为，不测内部 state。修改交互/布局/边界必须补回归。
- **品牌受保护**：不得删除或替换 new-api / QuantumNous 相关署名与标识。

## 反模式

- ❌ 在 `web/` 使用 npm / yarn / pnpm。
- ❌ 用户文案不走 `t()`，或把 `SUCCESS_MESSAGES.xxx` 直接当最终字符串 toast。
- ❌ 在路由文件里堆业务逻辑——路由只接线，逻辑回 feature。
- ❌ 直接操作 `window.location` 做站内跳转。
- ❌ 测试与实现平铺在同一目录，或用大段 class 快照 / `sleep` 冒充测试。
- ❌ mock 被测模块自身，或为覆盖率写 smoke 测试。
- ❌ 改完 TS 不跑 typecheck，或留下 lint error。
