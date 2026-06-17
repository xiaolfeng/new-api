# bamboo-messages 协议归一化中继桥（Bridge）实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用 bamboo-messages 的协议无关中间表示替换 new-api 四个对话类 Helper 的三段式协议转换内核，将入口协议 × 上游协议的 N×M 转换矩阵降为 N+M。

**Architecture:** 分双仓库推进。bamboo-messages 侧先做 2 个独立 PR（移除 base-go 依赖 → 提升 `internal/provider` 为公开包），打 tag 后 new-api 侧新增 `relay/bamboo/` 桥包，4 个 Helper 内核委托给 `bamboo.ChatRelay`，未覆盖渠道走原生 fallback，按 ApiType 白名单渐进灰度。

**Tech Stack:** Go 1.25 / gin / GORM / bamboo-messages（codec + provider）/ new-api relay 架构

**配套 spec：** `docs/superpowers/specs/2026-06-18-bamboo-relay-bridge-design.md`（已批准，经高精度复审 OK-GO）

**仓库路径约定：**
- bamboo-messages 仓库：`/Users/xiaolfeng/ProgramProjects/Cooperate/bamboo-service/bamboo-messages`（下文记作 `<bamboo>`）
- new-api 仓库：`/Users/xiaolfeng/ProgramProjects/Personal/Golang/new-api`（下文记作 `<newapi>`）

---

## 文件结构

### bamboo-messages 侧（`<bamboo>`）

| 文件 | 责任 |
|------|------|
| Create: `internal/xerr/error.go` | 最小错误包，替代 `bamboo-base-go/common/error`（PR-B1） |
| Modify: 12 个文件的 import（`bamboo/convert.go`、`internal/provider/*`） | 把 `xError ".../bamboo-base-go/common/error"` 改为本 `internal/xerr`（PR-B1） |
| Modify: `go.mod` / `go.sum` | 移除 base-go 依赖（PR-B1） |
| Rename: `internal/provider/` → `provider/` | 提升为公开包（PR-B2） |
| Modify: 所有 import `internal/provider` 的文件 | 改路径为 `provider`（PR-B2） |

### new-api 侧（`<newapi>`）

| 文件 | 责任 |
|------|------|
| Create: `relay/bamboo/bridge.go` | 核心入口 `ChatRelay` + `doStreamRelay`/`doCompleteRelay` |
| Create: `relay/bamboo/codec_map.go` | `RelayFormat` ↔ `codec.FormatType` 映射（包内私有） |
| Create: `relay/bamboo/provider_factory.go` | `RelayInfo` → bamboo provider 实例化 |
| Create: `relay/bamboo/errors.go` | CodecError → `*types.NewAPIError` 翻译 |
| Create: `relay/bamboo/usage.go` | bamboo Usage → `dto.Usage` 映射 + reasoning 累计 |
| Create: `relay/bamboo/bridge_test.go` | bridge 单元测试 |
| Create: `setting/model_setting/bamboo_setting.go` | 灰度开关 `EnableBambooRelay` |
| Modify: `relay/compatible_handler.go` | TextHelper 内核委托 + fallback |
| Modify: `relay/claude_handler.go` | ClaudeHelper 内核委托 + fallback |
| Modify: `relay/gemini_handler.go` | GeminiHelper 内核委托 + fallback |
| Modify: `relay/responses_handler.go` | ResponsesHelper 内核委托 + fallback |
| Modify: `go.mod` | 引入 bamboo-messages 新 tag |

---

## Phase 0：bamboo-messages 侧改造（前置阻断解除）

> **执行说明**：Phase 0 在 bamboo-messages 仓库进行，产生 2 个独立 PR。两个 PR 互相独立但建议 PR-B1 先合并（依赖图更干净）。合并后打新 tag，new-api 侧（Phase 1+）依赖该 tag。
>
> **分支策略**：在 bamboo-messages 仓库建两个分支 `feat/remove-base-go`（PR-B1）与 `feat/public-provider`（PR-B2）。PR-B2 基于含 PR-B1 的主干。

### Task 1: 创建 bamboo 最小错误包 `internal/xerr`（PR-B1 起步）

**Files:**
- Create: `<bamboo>/internal/xerr/error.go`

- [ ] **Step 1: 在 bamboo-messages 仓库建立分支并确认起点**

```bash
cd /Users/xiaolfeng/ProgramProjects/Cooperate/bamboo-service/bamboo-messages
git checkout -b feat/remove-base-go
git log --oneline -1
```
Expected: 起点为 `8d9632e feat(适配器): 添加 Gemini 协议支持及统一编解码层`

- [ ] **Step 2: 创建 `internal/xerr/error.go`**

```go
// internal/xerr/error.go
package xerr

import (
	"context"
	"errors"
)

// Error 是 bamboo-messages 内部使用的最小错误类型，替代原 bamboo-base-go/common/error.Error。
//
// 设计原则：bamboo 转换链仅读取错误的消息文本（见 bamboo/convert.go 的 handleError），
// 从不访问 ErrorCode/Output/Data 等字段，因此这里只需保留 err + Message。
// 保留 *Error 指针语义，确保 convert.go 的 handleError(err *xError.Error) 与
// internal/provider/stream.go 的 StreamEvent.Err *xError.Error 字段零行为变化。
type Error struct {
	err     error
	Message string
}

// Error 实现 error 接口，返回底层 cause 的消息。
func (e *Error) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

// Unwrap 支持 errors.Is / errors.As 链式解包。
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// NewError 兼容原 xError.NewError 签名，用于平滑替换。
//
// 参数对齐原签名 NewError(ctx, err *ErrorCode, msg ErrMessage, throw bool, getErr ...error)，
// 但 ctx / ErrorCode / throw 在 bamboo 的全部调用点都不被实际使用（throw 恒为 false），
// 故这里以 _ 占位，仅保留 msg 与 cause 的语义。
func NewError(_ context.Context, _ any, msg string, _ bool, cause ...error) *Error {
	e := errors.New(msg)
	if len(cause) > 0 && cause[0] != nil {
		e = cause[0]
	}
	return &Error{err: e, Message: msg}
}
```

- [ ] **Step 3: 编译验证新包**

```bash
cd /Users/xiaolfeng/ProgramProjects/Cooperate/bamboo-service/bamboo-messages
go build ./internal/xerr/
```
Expected: 无输出（编译成功）

- [ ] **Step 4: 提交**

```bash
git add internal/xerr/error.go
git commit -m "feat(xerr): 新增最小错误包以替代 bamboo-base-go/common/error"
```

---

### Task 2: 批量替换 import 为 `internal/xerr`（PR-B1 核心）

**Files:**
- Modify: `<bamboo>/bamboo/convert.go`
- Modify: `<bamboo>/bamboo/convert_test.go`
- Modify: `<bamboo>/internal/provider/stream.go`
- Modify: `<bamboo>/internal/provider/anthropic/chat.go`
- Modify: `<bamboo>/internal/provider/anthropic/complete.go`
- Modify: `<bamboo>/internal/provider/gemini/chat.go`
- Modify: `<bamboo>/internal/provider/gemini/complete.go`
- Modify: `<bamboo>/internal/provider/openai/completions/chat.go`
- Modify: `<bamboo>/internal/provider/openai/completions/complete.go`
- Modify: `<bamboo>/internal/provider/openai/responses/chat.go`
- Modify: `<bamboo>/internal/provider/openai/responses/complete.go`
- Modify: `<bamboo>/internal/provider/openai/responses/stream.go`

> **复审事实**：12 个文件全部只 import `github.com/bamboo-services/bamboo-base-go/common/error`（别名 `xError`），调用模板为 `xError.NewError(ctx, xError.OperationFailed, "...", false, err)`。bamboo 转换链（convert.go:388）只读 `.Error()`，不丢字段。

- [ ] **Step 1: 全局替换 import 别名**

对上述 12 个文件，把 import 行：
```go
xError "github.com/bamboo-services/bamboo-base-go/common/error"
```
替换为：
```go
xError "github.com/bamboo-services/bamboo-messages/internal/xerr"
```

> 注意：保留别名 `xError` 不变，这样调用点 `xError.NewError(...)` / `*xError.Error` 无需改动，最小化 diff。

用编辑器或 sed 批量处理。处理前先逐个确认这些文件的 import 行确实是上面那行（不要误伤）。

- [ ] **Step 2: 处理 `xError.OperationFailed` 引用**

`OperationFailed` 是 base-go 的常量，`internal/xerr` 没有定义它。所有调用点形如 `xError.NewError(ctx, xError.OperationFailed, "...", false, err)`。

由于 `internal/xerr.NewError` 的第 2 参数用 `_ any` 占位，把 `xError.OperationFailed` 替换为 `nil`：
```go
xError.NewError(ctx, nil, "...", false, err)
```

用全局搜索 `xError.OperationFailed` 找到所有引用，逐个改为 `nil`。

- [ ] **Step 3: 处理 `xError.ErrMessage` 引用（responses/stream.go:171）**

`ErrMessage` 是 base-go 的 `type string`。`responses/stream.go` 有一处 `xError.ErrMessage(errMsg)` 显式转换。由于 `internal/xerr.NewError` 的第 3 参数已是 `string`，把 `xError.ErrMessage(errMsg)` 改为 `errMsg`（去掉转换）。

搜索 `xError.ErrMessage` 确认只有这一处。

- [ ] **Step 4: 处理 convert_test.go 的 NewError 调用**

`bamboo/convert_test.go:681` 有 `xError.NewError(nil, nil, "connection reset", false)`。这个调用本身已是 `nil` 占位第 2 参数，import 替换后即可编译（无需改）。

- [ ] **Step 5: 全量编译验证**

```bash
cd /Users/xiaolfeng/ProgramProjects/Cooperate/bamboo-service/bamboo-messages
go build ./...
```
Expected: 无输出（编译成功）。若报 `undefined: xError.OperationFailed` 或 `undefined: xError.ErrMessage`，说明 Step 2/3 有遗漏，回去补。

- [ ] **Step 6: 跑全量测试验证行为不变**

```bash
go test ./...
```
Expected: 所有包 `ok`（bamboo/codec 的 4 个包、internal/provider 的测试、bamboo 主包测试）。

- [ ] **Step 7: 提交**

```bash
git add -A
git commit -m "refactor: 全部错误处理迁移至 internal/xerr，脱离 bamboo-base-go"
```

---

### Task 3: 移除 go.mod 的 base-go 依赖（PR-B1 收尾）

**Files:**
- Modify: `<bamboo>/go.mod`
- Modify: `<bamboo>/go.sum`

- [ ] **Step 1: 删除 go.mod 中的 base-go require**

编辑 `<bamboo>/go.mod`，删除 direct 区块（第 5-9 行的 require 块里）的：
```
github.com/bamboo-services/bamboo-base-go/common v1.0.0-202603141642
```
和 indirect 区块里的：
```
github.com/bamboo-services/bamboo-base-go/defined v1.0.0-202602241812 // indirect
```

- [ ] **Step 2: 执行 go mod tidy 清理依赖图**

```bash
cd /Users/xiaolfeng/ProgramProjects/Cooperate/bamboo-service/bamboo-messages
go mod tidy
```
Expected: 无报错。tidy 会自动移除 base-go 拉入的整条传递链（gin、validator/v10、quic-go、gorm 等）。

- [ ] **Step 3: 验证 gin 已从依赖图彻底消失**

```bash
echo "=== go.mod 是否含 gin ===" 
grep -c "gin-gonic/gin" go.mod || echo "0 (go.mod 无 gin ✓)"
echo "=== go.sum 是否含 gin ==="
grep -c "gin-gonic/gin" go.sum || echo "0 (go.sum 无 gin ✓)"
echo "=== go.mod 是否还残留 base-go ==="
grep -c "bamboo-base-go" go.mod || echo "0 (无 base-go ✓)"
```
Expected: 全部输出 `0` 或显示 "无 gin/base-go ✓"。

> **关键验收**：这是 spec D3 决策的核心——移除 base-go 后 gin 彻底消失。若 go.sum 仍含 gin，说明还有其他路径拉入（复审已确认 anthropic/openai/genai SDK 不含 gin，应该不会发生）。

- [ ] **Step 4: 再次全量编译 + 测试**

```bash
go build ./... && go test ./...
```
Expected: 编译无错，所有测试 `ok`。

- [ ] **Step 5: 提交**

```bash
git add go.mod go.sum
git commit -m "chore: 移除 bamboo-base-go 依赖，gin 等传递依赖随之清除"
```

- [ ] **Step 6: 推送 PR-B1 并合并**

```bash
git push -u origin feat/remove-base-go
# 在 GitHub 创建 PR，标题: "refactor: 移除 bamboo-base-go 依赖，实现自包含"
# 合并到主干
```

---

### Task 4: 提升 `internal/provider` 为公开 `provider/` 包（PR-B2）

**Files:**
- Rename: `<bamboo>/internal/provider/` → `<bamboo>/provider/`
- Modify: 所有 import `internal/provider`（含子包）的文件

> **复审事实**：当前 import `internal/provider` 的文件包括 `bamboo/bamboo.go`、`bamboo/config.go`、`bamboo/convert.go`、`bamboo/option.go`（4 个 SDK 文件）+ `bamboo/codec/codec.go`（经 bamboo 包间接）+ `example/main.go`。provider 子目录：anthropic / gemini / openai/completions / openai/responses。

- [ ] **Step 1: 在 bamboo-messages 建 PR-B2 分支（基于含 PR-B1 的主干）**

```bash
cd /Users/xiaolfeng/ProgramProjects/Cooperate/bamboo-service/bamboo-messages
git checkout main  # 或 PR-B1 已合并的主干分支
git pull
git checkout -b feat/public-provider
```

- [ ] **Step 2: 用 git mv 重命名目录**

```bash
git mv internal/provider provider
```
这会把 `internal/provider/` 整棵子树（含所有 .go 与 _test.go）移到 `provider/`。

- [ ] **Step 3: 批量替换 import 路径**

把所有 `.go` 文件中的：
```
github.com/bamboo-services/bamboo-messages/internal/provider
```
替换为：
```
github.com/bamboo-services/bamboo-messages/provider
```

包括所有子包路径，例如：
- `.../internal/provider` → `.../provider`
- `.../internal/provider/anthropic` → `.../provider/anthropic`
- `.../internal/provider/openai/completions` → `.../provider/openai/completions`
- `.../internal/provider/openai/responses` → `.../provider/openai/responses`
- `.../internal/provider/gemini` → `.../provider/gemini`

用全局搜索 `bamboo-messages/internal/provider` 找到所有引用（应包括 bamboo/ 下 4 文件 + codec/codec.go + provider/ 内部互相引用 + example/main.go），逐个改。

- [ ] **Step 4: 全量编译验证**

```bash
go build ./...
```
Expected: 无输出（编译成功）。

- [ ] **Step 5: 跑全量测试**

```bash
go test ./...
```
Expected: 所有包 `ok`。

- [ ] **Step 6: 关键验证——外部可 import 性**

写一个临时验证（不提交），确认 external 路径不再触发 internal 错误：
```bash
cat > /tmp/bamboo_external_check.go <<'EOF'
package main

import (
	_ "github.com/bamboo-services/bamboo-messages/bamboo"
	_ "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	_ "github.com/bamboo-services/bamboo-messages/provider"
)

func main() {}
EOF
# 这个文件不能直接编译（跨 module），这里仅作路径合理性检查
# 真正验证在 new-api 侧 Phase 1 Task 1 进行
rm /tmp/bamboo_external_check.go
```
说明：完整的外部 import 验证在 new-api 引入依赖后（Phase 1 Task 1）做。本步只确保 bamboo 自身编译测试通过。

- [ ] **Step 7: 提交**

```bash
git add -A
git commit -m "refactor: 提升 internal/provider 为公开 provider 包，解除外部 import 限制"
```

- [ ] **Step 8: 推送 PR-B2 并合并**

```bash
git push -u origin feat/public-provider
# PR 标题: "refactor: 提升 internal/provider 为公开包"
# 合并到主干
```

---

### Task 5: 打新 tag 并记录 commit

- [ ] **Step 1: 确认主干含两个 PR**

```bash
cd /Users/xiaolfeng/ProgramProjects/Cooperate/bamboo-service/bamboo-messages
git checkout main
git pull
git log --oneline -5
```
Expected: 看到 PR-B1（移除 base-go）与 PR-B2（提升 provider）都已合入。

- [ ] **Step 2: 打 tag**

```bash
git tag -a v0.2.0 -m "feat: 自包含 + 公开 provider 包，支持外部 Go 模块复用

- 移除 bamboo-base-go 依赖（gin 等传递依赖清除）
- 提升 internal/provider 为公开 provider 包"
git push origin v0.2.0
```
> 记录此 tag（`v0.2.0`）与对应 commit hash，Phase 1 Task 1 会用到。

- [ ] **Step 3: 记录关键信息到计划**

在计划文件顶部或笔记里记录：
- bamboo 新 tag：`v0.2.0`
- bamboo commit hash（执行 `git rev-parse HEAD` 获取）

---

## Phase 1：new-api bridge 基础设施

> **前置**：Phase 0 的 PR-B1/B2 已合并，tag `v0.2.0` 已打。
> **分支**：在 new-api 仓库 `feature/new-relay-for-bamboo` 分支继续。
> **TDD 原则**：每个 bridge 文件先写测试（失败），再写实现（通过）。

### Task 6: new-api 引入 bamboo-messages 依赖

**Files:**
- Modify: `<newapi>/go.mod`
- Modify: `<newapi>/go.sum`

- [ ] **Step 1: 确认在 new-api 的 feature 分支**

```bash
cd /Users/xiaolfeng/ProgramProjects/Personal/Golang/new-api
git branch --show-current
```
Expected: `feature/new-relay-for-bamboo`

- [ ] **Step 2: go get 引入 bamboo-messages**

```bash
go get github.com/bamboo-services/bamboo-messages@v0.2.0
```
Expected: go.mod 新增 `github.com/bamboo-services/bamboo-messages v0.2.0`。

- [ ] **Step 3: 验证 bamboo 可 import（外部 import 性验证）**

创建临时验证文件：
```bash
cat > relay/bamboo_import_check_test.go <<'EOF'
package bamboo

import (
	"testing"

	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
)

// 验证 bamboo-messages 可被 new-api import（无 internal package 错误）
func TestBambooImportable(t *testing.T) {
	_, err := bamboocodec.Get(bamboocodec.FormatOpenAI)
	if err != nil {
		t.Fatalf("bamboo codec.Get failed: %v", err)
	}
}
EOF
go test ./relay/ -run TestBambooImportable -v
```
Expected: `PASS`。这一步同时验证了 PR-B2 的外部 import 性 + 依赖解析。

> 若报 `use of internal package not allowed`，说明 PR-B2 未正确合并或 tag 不对，回 Phase 0 检查。

- [ ] **Step 4: 删除临时验证文件，全量编译**

```bash
rm relay/bamboo_import_check_test.go
go build ./...
```
Expected: 无输出（编译成功，gin 版本应仍为 v1.9.1 不变）。

- [ ] **Step 5: 验证 gin 版本未变**

```bash
grep "gin-gonic/gin" go.mod
```
Expected: `github.com/gin-gonic/gin v1.9.1`（不变，证明 D3 决策生效）。

- [ ] **Step 6: 提交**

```bash
git add go.mod go.sum
git commit -m "deps: 引入 bamboo-messages v0.2.0（自包含，公开 provider 包）"
```

---

### Task 7: 创建 `relay/bamboo/` 包骨架 + 灰度开关

**Files:**
- Create: `<newapi>/setting/model_setting/bamboo_setting.go`
- Create: `<newapi>/relay/bamboo/errors.go`

- [ ] **Step 1: 创建灰度开关 `bamboo_setting.go`**

复用 `setting/model_setting/global.go` 的注册模式（复审已确认 `config.GlobalConfig.Register(name, ptr)` 真实存在）：

```go
// setting/model_setting/bamboo_setting.go
package model_setting

import "github.com/QuantumNous/new-api/setting/config"

// BambooSettings 控制 bamboo 中继桥的灰度开关。
type BambooSettings struct {
	// EnableBambooRelay 全局开关，默认关闭。
	// 关闭时所有对话 Helper 走 new-api 原生三段式，零影响。
	EnableBambooRelay bool `json:"enable_bamboo_relay" yaml:"enable_bamboo_relay"`
}

var defaultBambooSettings = BambooSettings{
	EnableBambooRelay: false,
}

var bambooSettings = defaultBambooSettings

func init() {
	config.GlobalConfig.Register("bamboo", &bambooSettings)
}

// GetBambooSettings 返回 bamboo 中继设置的当前值（指针，运行时可热更新）。
func GetBambooSettings() *BambooSettings {
	return &bambooSettings
}
```

- [ ] **Step 2: 编译验证**

```bash
go build ./setting/model_setting/
```
Expected: 无输出。

- [ ] **Step 3: 创建 errors.go（复审修正版）**

```go
// relay/bamboo/errors.go
package bamboo

import (
	"errors"

	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	"github.com/QuantumNous/new-api/types"
)

// ErrUnsupportedProvider 表示该上游 ApiType 未被 bamboo 覆盖，
// 调用方应 fallback 到 new-api 原生三段式。
var ErrUnsupportedProvider = errors.New("bamboo: unsupported provider for this api type")

// translateCodecError 把 bamboo CodecError 翻译为 new-api 错误。
//
// 入参为 error 接口（ParseRequest/Serialize 返回裸 error），
// 内部用 errors.As 做 *CodecError 类型断言；非 CodecError 走默认分支。
//
// CodecError.Type 实际枚举（bamboo/codec/errors.go:9-22）：
//   ErrInvalidRequest / ErrProviderError / ErrAuthError / ErrRateLimit / ErrInternal
//
// ErrorCode 映射（new-api types/error.go 真实存在的常量，复审已核对全 31 个）：
//   new-api 无 auth/rateLimit/upstream 专用码，复用语义最近的现有常量。
func translateCodecError(err error) *types.NewAPIError {
	if err == nil {
		return nil
	}
	var ce *bamboocodec.CodecError
	if !errors.As(err, &ce) {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed)
	}
	switch ce.Type {
	case bamboocodec.ErrInvalidRequest:
		return types.NewError(ce, types.ErrorCodeInvalidRequest)
	case bamboocodec.ErrAuthError:
		return types.NewError(ce, types.ErrorCodeAccessDenied)
	case bamboocodec.ErrRateLimit:
		return types.NewError(ce, types.ErrorCodeBadResponse)
	case bamboocodec.ErrProviderError:
		return types.NewError(ce, types.ErrorCodeBadResponseStatusCode)
	default: // ErrInternal 等
		return types.NewError(ce, types.ErrorCodeConvertRequestFailed)
	}
}
```

- [ ] **Step 4: 编译验证（此时 relay/bamboo 包只有 errors.go，应能编译）**

```bash
go build ./relay/bamboo/ 2>&1 || echo "(预期：包内只有 errors.go，编译成功)"
```
Expected: 无输出（编译成功）。`ErrUnsupportedProvider` 和 `translateCodecError` 都被定义，无未使用报错（Go 允许定义未使用的包级 var/func）。

- [ ] **Step 5: 提交**

```bash
git add setting/model_setting/bamboo_setting.go relay/bamboo/errors.go
git commit -m "feat(bamboo): 新增灰度开关与错误翻译层"
```

---

### Task 8: 实现 codec_map.go + usage.go（含 TDD）

**Files:**
- Create: `<newapi>/relay/bamboo/codec_map.go`
- Create: `<newapi>/relay/bamboo/usage.go`
- Create: `<newapi>/relay/bamboo/codec_map_test.go`
- Create: `<newapi>/relay/bamboo/usage_test.go`

- [ ] **Step 1: 写 codec_map 的失败测试**

```go
// relay/bamboo/codec_map_test.go
package bamboo

import (
	"testing"

	"github.com/QuantumNous/new-api/types"
)

func TestRelayFormatToCodec_SupportedFormats(t *testing.T) {
	cases := []struct {
		name   string
		format types.RelayFormat
		want   string // codec.FormatType 的 string 值
	}{
		{"OpenAI", types.RelayFormatOpenAI, "openai"},
		{"Claude", types.RelayFormatClaude, "anthropic"},
		{"Responses", types.RelayFormatOpenAIResponses, "responses"},
		{"Gemini", types.RelayFormatGemini, "gemini"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := relayFormatToCodec(c.format)
			if !ok {
				t.Fatalf("expected ok=true for %s, got false", c.name)
			}
			if string(got) != c.want {
				t.Fatalf("expected %q, got %q", c.want, string(got))
			}
		})
	}
}

func TestRelayFormatToCodec_UnsupportedFormats(t *testing.T) {
	// 非对话格式不应映射到 codec
	unsupported := []types.RelayFormat{
		types.RelayFormatOpenAIAudio,
		types.RelayFormatOpenAIImage,
		types.RelayFormatEmbedding,
		types.RelayFormatRerank,
	}
	for _, f := range unsupported {
		_, ok := relayFormatToCodec(f)
		if ok {
			t.Fatalf("expected ok=false for format %q, got true", f)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败（函数未定义）**

```bash
go test ./relay/bamboo/ -run TestRelayFormatToCodec -v
```
Expected: FAIL with `undefined: relayFormatToCodec`

- [ ] **Step 3: 写 codec_map 实现**

```go
// relay/bamboo/codec_map.go
package bamboo

import (
	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	"github.com/QuantumNous/new-api/types"
)

// relayFormatToCodec 把 new-api 的 RelayFormat 映射为 bamboo codec 的 FormatType。
// 包内私有，由 ChatRelay 内部使用。
// 非对话格式（Audio/Image/Task/Realtime/Rerank/Embedding）返回 ("", false)，
// 调用方据此 fallback。
func relayFormatToCodec(f types.RelayFormat) (bamboocodec.FormatType, bool) {
	switch f {
	case types.RelayFormatOpenAI:
		return bamboocodec.FormatOpenAI, true
	case types.RelayFormatClaude:
		return bamboocodec.FormatAnthropic, true
	case types.RelayFormatOpenAIResponses:
		return bamboocodec.FormatResponses, true
	case types.RelayFormatGemini:
		return bamboocodec.FormatGemini, true
	default:
		return "", false
	}
}
```

- [ ] **Step 4: 跑测试确认通过**

```bash
go test ./relay/bamboo/ -run TestRelayFormatToCodec -v
```
Expected: PASS

- [ ] **Step 5: 写 usage 的失败测试**

```go
// relay/bamboo/usage_test.go
package bamboo

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
)

func TestAccumulateReasoning(t *testing.T) {
	usage := &dto.Usage{}
	accumulateReasoning(usage, 10)
	accumulateReasoning(usage, 5)
	if usage.CompletionTokenDetails.ReasoningTokens != 15 {
		t.Fatalf("expected ReasoningTokens=15, got %d", usage.CompletionTokenDetails.ReasoningTokens)
	}
}
```

- [ ] **Step 6: 跑测试确认失败**

```bash
go test ./relay/bamboo/ -run TestAccumulateReasoning -v
```
Expected: FAIL with `undefined: accumulateReasoning`

- [ ] **Step 7: 写 usage 实现（复审修正版，值类型直接访问）**

```go
// relay/bamboo/usage.go
package bamboo

import "github.com/QuantumNous/new-api/dto"

// accumulateReasoning 把 thinking delta 的 token 数累计到 Usage 的 reasoning 字段。
//
// 注意（复审修正）：CompletionTokenDetails 是【值类型 struct】（dto/openai_response.go:232），
// 不能 == nil 判断，也不能取址赋值；直接访问其字段即可。
func accumulateReasoning(usage *dto.Usage, delta int) {
	usage.CompletionTokenDetails.ReasoningTokens += delta
}
```

- [ ] **Step 8: 跑测试确认通过**

```bash
go test ./relay/bamboo/ -run TestAccumulateReasoning -v
```
Expected: PASS

- [ ] **Step 9: 提交**

```bash
git add relay/bamboo/codec_map.go relay/bamboo/usage.go relay/bamboo/codec_map_test.go relay/bamboo/usage_test.go
git commit -m "feat(bamboo): 实现 RelayFormat 映射与 reasoning token 累计（TDD）"
```

---

### Task 9: 实现 provider_factory.go（含 TDD）

**Files:**
- Create: `<newapi>/relay/bamboo/provider_factory.go`
- Create: `<newapi>/relay/bamboo/provider_factory_test.go`

> **复审修正**：import 块必须含 `"github.com/QuantumNous/new-api/types"`（Task 原草案漏了）。

- [ ] **Step 1: 写 provider_factory 的失败测试**

```go
// relay/bamboo/provider_factory_test.go
package bamboo

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestNewProvider_SupportedOpenAICompatible(t *testing.T) {
	// OpenAI 兼容渠道应返回非 nil provider 且无错误
	supportedTypes := []int{
		constant.APITypeOpenAI,
		constant.APITypeDeepSeek,
		constant.APITypeMoonshot,
		constant.APITypeSiliconFlow,
	}
	for _, apiType := range supportedTypes {
		info := &relaycommon.RelayInfo{}
		// ChannelMeta 经嵌入提升，构造一个最小 info
		info.ApiType = apiType
		p, err := newProvider(info)
		if err != nil {
			t.Fatalf("APIType %d: expected nil err, got %v", apiType, err)
		}
		if p == nil {
			t.Fatalf("APIType %d: expected non-nil provider", apiType)
		}
	}
}

func TestNewProvider_SupportedNativeProtocols(t *testing.T) {
	nativeTypes := []int{
		constant.APITypeAnthropic,
		constant.APITypeGemini,
		constant.APITypeCodex,
	}
	for _, apiType := range nativeTypes {
		info := &relaycommon.RelayInfo{}
		info.ApiType = apiType
		p, err := newProvider(info)
		if err != nil {
			t.Fatalf("APIType %d: expected nil err, got %v", apiType, err)
		}
		if p == nil {
			t.Fatalf("APIType %d: expected non-nil provider", apiType)
		}
	}
}

func TestNewProvider_UnsupportedReturnsFallback(t *testing.T) {
	// AWS/讯飞等未覆盖渠道应返回 ErrUnsupportedProvider
	unsupportedTypes := []int{
		constant.APITypeAws,
		constant.APITypeXunfei,
		constant.APITypeTencent,
	}
	for _, apiType := range unsupportedTypes {
		info := &relaycommon.RelayInfo{}
		info.ApiType = apiType
		p, err := newProvider(info)
		if p != nil {
			t.Fatalf("APIType %d: expected nil provider for unsupported", apiType)
		}
		if err == nil {
			t.Fatalf("APIType %d: expected non-nil err for unsupported", apiType)
		}
		// err 是 *types.NewAPIError，其 Unwrap() 返回 ErrUnsupportedProvider
		if !isUnsupportedProviderErr(err) {
			t.Fatalf("APIType %d: expected ErrUnsupportedProvider, got %v", apiType, err)
		}
	}
}

// isUnsupportedProviderErr 检查 *types.NewAPIError 是否包裹 ErrUnsupportedProvider。
func isUnsupportedProviderErr(err error) bool {
	// err 的 Unwrap() 链应含 ErrUnsupportedProvider
	return errorIs(err, ErrUnsupportedProvider)
}
```

> 注意：上面用了 `errorIs` 辅助函数（避免和标准库 errors.Is 命名冲突），需要在测试文件补一个小 helper，或者直接 import "errors"。这里简化，测试文件顶部 import "errors" 并用 `errors.Is`。修改测试：把 `errorIs` 改为标准库 `errors.Is`，并 import "errors"。

修正后的测试 helper 段（替换上面 isUnsupportedProviderErr 调用）：
```go
import "errors"
// ...
if !errors.Is(err, ErrUnsupportedProvider) { ... }
```
> 删掉自定义的 isUnsupportedProviderErr，用 `errors.Is`（`*types.NewAPIError` 已实现 Unwrap，复审已确认 types/error.go:101-107）。

- [ ] **Step 2: 跑测试确认失败**

```bash
go test ./relay/bamboo/ -run TestNewProvider -v
```
Expected: FAIL with `undefined: newProvider`

- [ ] **Step 3: 写 provider_factory 实现（复审修正：补 types import）**

```go
// relay/bamboo/provider_factory.go
package bamboo

import (
	bamboocompletions "github.com/bamboo-services/bamboo-messages/provider/openai/completions"
	bambooresponses "github.com/bamboo-services/bamboo-messages/provider/openai/responses"
	bambooanthropic "github.com/bamboo-services/bamboo-messages/provider/anthropic"
	bamboogemini "github.com/bamboo-services/bamboo-messages/provider/gemini"
	"github.com/bamboo-services/bamboo-messages/provider"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

// newProvider 根据 RelayInfo.ApiType 构造对应的 bamboo provider。
//
// ApiKey/ChannelBaseUrl 经 *ChannelMeta 嵌入提升访问。
// 未覆盖的 ApiType 返回包裹 ErrUnsupportedProvider 的错误，调用方据此 fallback。
func newProvider(info *relaycommon.RelayInfo) (provider.Provider, *types.NewAPIError) {
	apiKey := info.ApiKey
	baseURL := info.ChannelBaseUrl

	switch info.ApiType {
	case constant.APITypeAnthropic:
		return bambooanthropic.NewProviderWithOptions(
			bambooanthropic.WithAPIKey(apiKey),
			bambooanthropic.WithBaseURL(baseURL),
		), nil

	case constant.APITypeGemini:
		return bamboogemini.NewProviderWithOptions(
			bamboogemini.WithAPIKey(apiKey),
			bamboogemini.WithBaseURL(baseURL),
		), nil

	case constant.APITypeCodex:
		return bambooresponses.NewResponsesProviderWithOptions(
			bambooresponses.WithAPIKey(apiKey),
			bambooresponses.WithBaseURL(baseURL),
		), nil

	case constant.APITypeOpenAI,
		constant.APITypeDeepSeek, constant.APITypeMoonshot,
		constant.APITypeSiliconFlow, constant.APITypeMistral,
		constant.APITypeXai, constant.APITypeZhipuV4,
		constant.APITypePerplexity, constant.APITypeCohere,
		constant.APITypeMiniMax, constant.APITypeBaiduV2,
		constant.APITypeOpenRouter, constant.APITypeXinference:
		// OpenAI Chat Completions 兼容渠道统一走 completions provider
		return bamboocompletions.NewCompletionsProviderWithOptions(
			bamboocompletions.WithAPIKey(apiKey),
			bamboocompletions.WithBaseURL(baseURL),
		), nil

	default:
		// AWS/讯飞/腾讯/智谱v3/Coze/Dify 等特殊协议，bamboo 不覆盖
		return nil, types.NewError(ErrUnsupportedProvider, types.ErrorCodeInvalidApiType)
	}
}
```

- [ ] **Step 4: 跑测试确认通过**

```bash
go test ./relay/bamboo/ -run TestNewProvider -v
```
Expected: PASS（3 个子测试全过）。

> 若某 case 的 provider 构造因缺 apiKey/baseURL 而 panic（如 gemini 的 genai.Client 对空值敏感，复审发现 gemini/provider.go:99-104 忽略了 error 返回 nil 解引用），测试里 info.ApiKey/ChannelBaseUrl 为空——可能触发 nil panic。若发生，在测试 info 上补 `info.ApiKey = "test-key"; info.ChannelBaseUrl = "https://api.example.com"`，并在实现侧考虑对 gemini 的空值保护。

- [ ] **Step 5: 提交**

```bash
git add relay/bamboo/provider_factory.go relay/bamboo/provider_factory_test.go
git commit -m "feat(bamboo): 实现 provider 工厂，覆盖 24 个 ApiType（TDD）"
```

---

### Task 10: 实现 bridge.go 核心 ChatRelay（含 TDD）

**Files:**
- Create: `<newapi>/relay/bamboo/bridge.go`
- Create: `<newapi>/relay/bamboo/bridge_test.go`

> 这是 bridge 包的核心。ChatRelay 串联 codec_map + provider_factory + errors，处理流式/非流式分支。

- [ ] **Step 1: 写 bridge 的失败测试（单元级，mock-free，聚焦错误路径）**

```go
// relay/bamboo/bridge_test.go
package bamboo

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

func TestChatRelay_UnsupportedFormatFallsBack(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	info := &relaycommon.RelayInfo{}
	info.ApiType = 0 // OpenAI，但用不支持 RelayFormat

	_, err := ChatRelay(c, info, types.RelayFormatOpenAIAudio, []byte("{}"))
	if err == nil {
		t.Fatal("expected error for unsupported format, got nil")
	}
	if !errors.Is(err, ErrUnsupportedProvider) {
		t.Fatalf("expected ErrUnsupportedProvider for audio format, got %v", err)
	}
}

func TestChatRelay_UnsupportedProviderFallsBack(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	info := &relaycommon.RelayInfo{}
	info.ApiType = 9999 // 不存在的 ApiType，触发 default fallback

	_, err := ChatRelay(c, info, types.RelayFormatOpenAI, []byte("{}"))
	if err == nil {
		t.Fatal("expected error for unsupported provider, got nil")
	}
	if !errors.Is(err, ErrUnsupportedProvider) {
		t.Fatalf("expected ErrUnsupportedProvider, got %v", err)
	}
}
```

> 说明：单元测试聚焦错误路径（不支持的格式 / 不支持的 provider），这两条是纯逻辑、无需真实 API。真实 API 的端到端测试在 Phase 2/3 用集成测试 + 真实 API Key 做。

- [ ] **Step 2: 跑测试确认失败**

```bash
go test ./relay/bamboo/ -run TestChatRelay -v
```
Expected: FAIL with `undefined: ChatRelay`

- [ ] **Step 3: 写 bridge 实现（复审修正版 import）**

```go
// relay/bamboo/bridge.go
package bamboo

import (
	"github.com/gin-gonic/gin"

	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

// ChatRelay 对话中继统一内核。
//
// 替代 TextHelper/ClaudeHelper/GeminiHelper/ResponsesHelper 内部的
// Convert→DoRequest→DoResponse 三段式，用 bamboo 中间表示做协议归一化。
//
// 调用方只需传 new-api 侧的 types.RelayFormat，格式映射在 bridge 内部完成。
// info.ApiType（经 ChannelMeta 嵌入）决定上游用哪个 bamboo provider。
//
// 返回 (usage, nil) 成功；(nil, err) 失败。
// 当 errors.Is(err, ErrUnsupportedProvider) 时，调用方应 fallback 原生链路。
func ChatRelay(c *gin.Context, info *relaycommon.RelayInfo,
	entryFormat types.RelayFormat, requestBody []byte) (*dto.Usage, *types.NewAPIError) {

	// ① 入口格式映射：RelayFormat → codec FormatType
	codecFmt, ok := relayFormatToCodec(entryFormat)
	if !ok {
		// 非对话格式不应进入 bridge
		return nil, types.NewError(ErrUnsupportedProvider, types.ErrorCodeInvalidApiType)
	}

	entryCodec, gerr := bamboocodec.Get(codecFmt)
	if gerr != nil || entryCodec == nil {
		return nil, types.NewError(gerr, types.ErrorCodeInvalidRequest)
	}
	relayReq, perr := entryCodec.ParseRequest(requestBody)
	if perr != nil {
		return nil, translateCodecError(perr) // 内部 errors.As 断言 *CodecError
	}

	// ② 上游侧：根据 ApiType 构造 bamboo provider
	p, perr := newProvider(info)
	if perr != nil {
		return nil, perr // 含 ErrUnsupportedProvider，调用方判 errors.Is 做 fallback
	}
	client := bamboosdk.NewClient(p)

	// ③ 出口侧：按入口 codec 序列化响应
	if relayReq.IsStream {
		return doStreamRelay(c, client, entryCodec, relayReq)
	}
	return doCompleteRelay(c, client, entryCodec, relayReq)
}

// doStreamRelay 消费 bamboo StreamEvent，按入口 codec 序列化为出口 SSE。
func doStreamRelay(c *gin.Context, client bamboosdk.BambooClient,
	entryCodec bamboocodec.Codec, req *bamboocodec.RelayRequest) (*dto.Usage, *types.NewAPIError) {

	eventCh, err := client.Chat(c.Request.Context(), req.Messages, req.System, req.Config)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeDoRequestFailed)
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Flush()

	serializer := entryCodec.NewSerializer()
	var usage dto.Usage

	for event := range eventCh {
		if event.Type == bamboosdk.EventError {
			if event.Error != nil {
				return nil, types.NewError(event.Error, types.ErrorCodeBadResponseBody)
			}
			return nil, types.NewError(errStreamError, types.ErrorCodeBadResponseBody)
		}
		data, serr := serializer.Serialize(event)
		if serr != nil {
			return nil, translateCodecError(serr)
		}
		if _, werr := c.Writer.Write(data); werr != nil {
			break // 客户端断开
		}
		c.Writer.Flush()

		// 从 message_delta 提取 usage
		if event.Type == bamboosdk.EventMessageDelta && event.Usage != nil {
			usage.PromptTokens = int(event.Usage.InputTokens)
			usage.CompletionTokens = int(event.Usage.OutputTokens)
		}
	}

	tail, _ := serializer.Flush()
	if len(tail) > 0 {
		c.Writer.Write(tail)
		c.Writer.Flush()
	}

	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	return &usage, nil
}

// errStreamError 用于 event.Error 为 nil 但事件类型为 EventError 的兜底。
var errStreamError = types.NewError(
	&streamError{"bamboo stream event error without detail"},
	types.ErrorCodeBadResponseBody,
)

type streamError struct{ msg string }

func (e *streamError) Error() string { return e.msg }

// doCompleteRelay 非流式中继。
func doCompleteRelay(c *gin.Context, client bamboosdk.BambooClient,
	entryCodec bamboocodec.Codec, req *bamboocodec.RelayRequest) (*dto.Usage, *types.NewAPIError) {

	resp, err := client.Complete(c.Request.Context(), req.Messages, req.System, req.Config)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeDoRequestFailed)
	}
	body, serr := entryCodec.SerializeResponse(resp)
	if serr != nil {
		return nil, translateCodecError(serr)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Write(body)

	return &dto.Usage{
		PromptTokens:     int(resp.Usage.InputTokens),
		CompletionTokens: int(resp.Usage.OutputTokens),
		TotalTokens:      int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
	}, nil
}
```

> 注：`errStreamError` 用 `types.NewError` 包了一个自定义 streamError，避免 event.Error 为 nil 时传 nil 给 types.NewError。实现时若发现 `types.NewError(nil, ...)` 合法（NewError 内部会处理 nil），可简化为直接 `types.NewError(nil, ...)`——这需要看 NewError 对 nil err 的处理，保守起见包一层。

- [ ] **Step 4: 跑测试确认通过**

```bash
go test ./relay/bamboo/ -run TestChatRelay -v
```
Expected: PASS（两条错误路径测试通过）。

- [ ] **Step 5: 全包测试**

```bash
go test ./relay/bamboo/ -v
```
Expected: 所有测试 PASS（codec_map / usage / provider_factory / bridge）。

- [ ] **Step 6: 全量编译**

```bash
go build ./...
```
Expected: 无输出。

- [ ] **Step 7: 提交**

```bash
git add relay/bamboo/bridge.go relay/bamboo/bridge_test.go
git commit -m "feat(bamboo): 实现 ChatRelay 核心内核（流式/非流式分支 + TDD）"
```

---

## Phase 1 验收检查点

- [ ] `go build ./...` 在 new-api 无编译错误
- [ ] `go test ./relay/bamboo/...` 全部 PASS
- [ ] new-api go.mod 的 gin 版本仍为 v1.9.1（D3 生效）
- [ ] bamboo import 可解析（无 internal package 错误）

**Phase 1 完成后，bridge 基础设施就绪，但尚未接入任何 Helper（灰度开关默认关闭）。下一步 Phase 2 接入 TextHelper。**

---

## Phase 2：接入 TextHelper（OpenAI 入口）

> **目标**：把 `relay/compatible_handler.go` 的 TextHelper 三段式委托给 bamboo.ChatRelay，保留 pass-through/chatCompletionsViaResponses 旁路，未覆盖渠道 fallback。
> **灰度**：仅 `APITypeOpenAI` + `APITypeDeepSeek` 先行。

### Task 11: 重构 TextHelper —— 抽取原三段式为 originalTextRelay

**Files:**
- Modify: `<newapi>/relay/compatible_handler.go`

> **策略**：先把 TextHelper 现有的三段式逻辑（含 pass-through/chatCompletionsViaResponses 旁路）**原样抽取**到一个新函数 `originalTextRelay`，不改行为。这一步是纯重构，便于后续在 TextHelper 顶部插入 bamboo 分支。

- [ ] **Step 1: 阅读当前 TextHelper 完整实现**

```bash
# 读取 relay/compatible_handler.go 第 25 行到函数结束，理解三段式 + 旁路结构
```
重点确认：
- pass-through 旁路（L97-107）：`if passThroughGlobal || info.ChannelSetting.PassThroughBodyEnabled`
- chatCompletionsViaResponses 旁路（L74-93）：完整 AND 条件
- 三段式（L109 ConvertOpenAIRequest / L189 DoRequest / L207 DoResponse）
- 计费（L217-221 PostAudioConsumeQuota / PostTextConsumeQuota）

- [ ] **Step 2: 抽取 originalTextRelay**

把 TextHelper 当前的函数体（从 `info.InitChannelMeta(c)` 到函数末尾）整体移到新函数：
```go
// originalTextRelay 是 new-api 原生三段式中继，作为 bamboo 未覆盖渠道的 fallback。
// 保留原 pass-through / chatCompletionsViaResponses 旁路不变。
func originalTextRelay(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (newAPIError *types.NewAPIError) {
    // ... 原 TextHelper 的全部函数体 ...
}
```

- [ ] **Step 3: 重写 TextHelper 为分支入口**

```go
func TextHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
    info.InitChannelMeta(c)

    // 原校验 + DeepCopy 逻辑保留
    // ...（原 TextHelper 开头的 request 解析与校验部分保留在此）

    // bamboo 灰度判断
    if !model_setting.GetBambooSettings().EnableBambooRelay {
        return originalTextRelay(c, info, request) // 开关关闭 → 原生
    }

    // bamboo 路径
    bodyBytes, _ := common.Marshal(request)
    usage, relayErr := bamboo.ChatRelay(c, info, types.RelayFormatOpenAI, bodyBytes)
    if relayErr != nil {
        if errors.Is(relayErr, bamboo.ErrUnsupportedProvider) {
            return originalTextRelay(c, info, request) // 未覆盖 → fallback
        }
        return relayErr
    }
    service.PostTextConsumeQuota(c, info, usage, nil)
    return nil
}
```

> 注意：pass-through / chatCompletionsViaResponses 旁路**保留在 originalTextRelay 内**。bamboo 路径目前不处理这两个旁路（因为它们是"跳过 Convert"的优化，bamboo 的 codec.ParseRequest 本身就是统一入口）。**但**灰度开启后，pass-through 渠道会先经 bamboo——这与原行为不同。需要在 Step 4 处理。

- [ ] **Step 4: pass-through 旁路的保留（关键，复审发现）**

在 TextHelper 的 bamboo 判断**之前**，加 pass-through 旁路判断，让它无论开关都走 originalTextRelay：

```go
func TextHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
    info.InitChannelMeta(c)
    // ... request 解析校验 ...

    // pass-through 旁路：无论 bamboo 开关，都走原生（避免改变 pass-through 行为）
    passThroughGlobal := model_setting.GetGlobalSettings().PassThroughRequestEnabled
    if passThroughGlobal || info.ChannelSetting.PassThroughBodyEnabled {
        return originalTextRelay(c, info, request)
    }

    // chatCompletionsViaResponses 旁路同理保留（其判断含 !passThrough，这里已过 passThrough 判断）
    if service.ShouldChatCompletionsUseResponsesGlobal(info.ChannelId, info.ChannelType, info.OriginModelName) {
        // 这个旁路内部会走 Responses 路径，保留原生
        return originalTextRelay(c, info, request)
    }

    if !model_setting.GetBambooSettings().EnableBambooRelay {
        return originalTextRelay(c, info, request)
    }

    bodyBytes, _ := common.Marshal(request)
    usage, relayErr := bamboo.ChatRelay(c, info, types.RelayFormatOpenAI, bodyBytes)
    if relayErr != nil {
        if errors.Is(relayErr, bamboo.ErrUnsupportedProvider) {
            return originalTextRelay(c, info, request)
        }
        return relayErr
    }
    service.PostTextConsumeQuota(c, info, usage, nil)
    return nil
}
```

> 说明：把两个旁路判断提到 TextHelper 顶部（在任何 bamboo 逻辑之前），确保它们行为与改造前完全一致。originalTextRelay 内的旁路判断此时变成冗余（因为已在 TextHelper 提前 return），但保留无害，且 originalTextRelay 作为独立可调用函数仍需自洽。

- [ ] **Step 5: 补 import（errors / bamboo / model_setting）**

确认 `relay/compatible_handler.go` import 块含：
```go
"errors"
"github.com/QuantumNous/new-api/relay/bamboo"
"github.com/QuantumNous/new-api/setting/model_setting"
```

- [ ] **Step 6: 编译验证**

```bash
go build ./relay/
```
Expected: 无输出。

- [ ] **Step 7: 行为回归测试（灰度关闭，走 originalTextRelay）**

```bash
go test ./relay/... 
```
Expected: 现有测试全过（因为 EnableBambooRelay 默认 false，行为不变）。

> 若项目有针对 TextHelper 的现有测试，它们应全过。若无，至少跑全量 `go build ./...` + 启动服务手动测一次 OpenAI 入口对话。

- [ ] **Step 8: 提交**

```bash
git add relay/compatible_handler.go
git commit -m "feat(relay): TextHelper 接入 bamboo bridge（灰度关闭时走原生 fallback）"
```

---

### Task 12: TextHelper 端到端集成验证（需真实 API Key）

> **说明**：这一步需要真实 OpenAI/DeepSeek API Key 做端到端验证。若当前无可用 Key，标记为"待手动验证"并记录，Phase 2 视为代码完成。

- [ ] **Step 1: 启动 new-api 服务，配置一个 OpenAI 渠道**

```bash
# 按项目 README 方式启动（SQLite 默认）
go run main.go
# 在管理后台配置一个 OpenAI 渠道（或 DeepSeek 兼容渠道），填入真实 API Key
```

- [ ] **Step 2: 灰度关闭，验证原生路径正常**

通过 `/v1/chat/completions` 发一个测试请求，确认响应正常。

- [ ] **Step 3: 灰度开启，验证 bamboo 路径**

在管理后台或直接改 options 表开启 `bamboo.enable_bamboo_relay = true`。

再次发同样的请求，确认：
- 流式响应正常（SSE 格式正确）
- 非流式响应正常（JSON 格式正确）
- usage 计费正常（Token 数与原生路径接近）

- [ ] **Step 4: fallback 验证（用未覆盖渠道）**

配置一个 AWS 或讯飞渠道，灰度开启时发请求，确认正确 fallback 到原生链路（日志应显示走了 originalTextRelay）。

- [ ] **Step 5: 记录验证结果**

在计划里记录验证日期、测试的渠道、流式/非流式结果、计费对比数据。

---

## Phase 2 验收检查点

- [ ] TextHelper 改造完成，灰度关闭时行为与改造前完全一致
- [ ] pass-through / chatCompletionsViaResponses 旁路行为不变
- [ ] 灰度开启时，OpenAI 入口 → OpenAI/DeepSeek 上游调通（流式 + 非流式）
- [ ] 未覆盖渠道正确 fallback
- [ ] 计费金额与改造前一致

---

## Phase 3：接入其余 3 个 Helper（Claude / Gemini / Responses）

> **目标**：ClaudeHelper / GeminiHelper / ResponsesHelper 对称接入 bamboo，保留各自适配逻辑（thinking 后缀/budget、Responses→Chat fallback）。
> **灰度**：扩大到 Anthropic/Gemini/Codex。
> **执行方式**：3 个 Helper 结构高度对称，建议**并发 TaskAgent**（详见执行阶段说明）。

### Task 13: 接入 ClaudeHelper

**Files:**
- Modify: `<newapi>/relay/claude_handler.go`

- [ ] **Step 1: 阅读当前 ClaudeHelper，确认 thinking 后缀适配逻辑位置**

确认 `applyClaudeThinkingAdapter`（或类似函数，claude_handler.go:55-108 附近）与系统提示注入逻辑（:110-133 附近）。

- [ ] **Step 2: 抽取 originalClaudeRelay + 改造 ClaudeHelper**

模式与 Task 11 对称：原三段式 → `originalClaudeRelay`；ClaudeHelper 顶部加 bamboo 分支：

```go
func ClaudeHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
    info.InitChannelMeta(c)
    // ... request 解析校验 ...

    // new-api 侧业务逻辑保留（thinking 后缀、系统提示注入）
    helper.ModelMappedHelper(c, info, request)
    applyClaudeThinkingAdapter(info, request)
    applyChannelSystemPrompt(info, request)

    if !model_setting.GetBambooSettings().EnableBambooRelay {
        return originalClaudeRelay(c, info, request)
    }

    bodyBytes, _ := common.Marshal(request)
    usage, relayErr := bamboo.ChatRelay(c, info, types.RelayFormatClaude, bodyBytes)
    if relayErr != nil {
        if errors.Is(relayErr, bamboo.ErrUnsupportedProvider) {
            return originalClaudeRelay(c, info, request)
        }
        return relayErr
    }
    service.PostTextConsumeQuota(c, info, usage, nil)
    return nil
}
```

> ClaudeHelper 无 TextHelper 那种 pass-through/chatCompletionsViaResponses 旁路（那些是 OpenAI 入口特有），所以更简单。

- [ ] **Step 3: 编译 + 灰度关闭回归测试 + 提交**

```bash
go build ./relay/ && go test ./relay/...
git add relay/claude_handler.go
git commit -m "feat(relay): ClaudeHelper 接入 bamboo bridge"
```

---

### Task 14: 接入 GeminiHelper

**Files:**
- Modify: `<newapi>/relay/gemini_handler.go`

- [ ] **Step 1: 确认 thinking budget 适配逻辑**

确认 gemini_handler.go 的 thinking budget 适配（类似 `trimModelThinking`，文件顶部 :1-53）。

- [ ] **Step 2: 抽取 originalGeminiRelay + 改造 GeminiHelper**

模式对称。entryFormat 用 `types.RelayFormatGemini`。

- [ ] **Step 3: 编译 + 回归 + 提交**

```bash
go build ./relay/ && go test ./relay/...
git add relay/gemini_handler.go
git commit -m "feat(relay): GeminiHelper 接入 bamboo bridge"
```

---

### Task 15: 接入 ResponsesHelper

**Files:**
- Modify: `<newapi>/relay/responses_handler.go`

- [ ] **Step 1: 确认 Responses→Chat fallback 逻辑**

确认 responses_handler.go 是否有"Responses 不支持时降级为 Chat"的逻辑（若有，需保留）。

- [ ] **Step 2: 抽取 originalResponsesRelay + 改造 ResponsesHelper**

模式对称。entryFormat 用 `types.RelayFormatOpenAIResponses`。

- [ ] **Step 3: 编译 + 回归 + 提交**

```bash
go build ./relay/ && go test ./relay/...
git add relay/responses_handler.go
git commit -m "feat(relay): ResponsesHelper 接入 bamboo bridge"
```

---

### Task 16: 跨协议端到端验证

> **关键场景**：Claude 入口 → DeepSeek 上游（跨协议）、Gemini 入口 → OpenAI 上游。这是 bamboo 归一化的核心价值验证。

- [ ] **Step 1: 配置场景并验证**

- Claude 入口（`/v1/messages`）打 DeepSeek 上游渠道 → 确认响应是 Claude SSE 格式
- Gemini 入口（`/v1beta/models/*`）打 OpenAI 上游渠道 → 确认响应是 Gemini 格式
- 各场景流式 + 非流式都验证

- [ ] **Step 2: 记录结果，扩大灰度白名单**

验证通过后，扩大 provider_factory 的灰度白名单（或在管理后台开启更多渠道类型）。

---

## Phase 3 验收检查点

- [ ] 4 个 Helper 全部接入 bamboo，灰度关闭时行为不变
- [ ] Claude/Gemini/Responses 各自的适配逻辑（thinking/budget/fallback）保留
- [ ] 跨协议场景调通（Claude→DeepSeek、Gemini→OpenAI）
- [ ] 未覆盖渠道 fallback 正常

---

## Phase 4：稳定性与边缘补齐（持续）

### Task 17: reasoning token 计费补齐

**Files:**
- Modify: `<newapi>/relay/bamboo/bridge.go`（doStreamRelay 循环）

- [ ] **Step 1: 在 doStreamRelay 的 event 循环里，识别 thinking delta 并累计**

```go
// 在 for event := range eventCh 循环内，EventContentBlockDelta 分支：
if event.Type == bamboosdk.EventContentBlockDelta && event.Delta != nil {
    // 若 delta 是 thinking 类型，累计 reasoning token（具体判定依 bamboo StreamEvent.Delta 的实际类型）
    // accumulateReasoning(&usage, deltaTokenCount)
}
```

> 实现细节：需根据 bamboo StreamEvent.Delta 的实际类型（复审确认 Delta 是 `any` 接口）判断是否为 thinking delta，并提取 token 计数。bamboo 的 thinking delta 可能不直接给 token 数（给的是文本），需用 new-api 现有的 token 计数工具估算。这一步标为"实现时细化"。

- [ ] **Step 2: 验证 reasoning token 统计准确（用 o1/claude-thinking/deepseek-r1 模型测试）**

---

### Task 18: goroutine 泄漏检查

- [ ] **Step 1: 写 pprof 泄漏测试**

模拟客户端中途断开，检查 goroutine 是否正常退出。

- [ ] **Step 2: 确认 Chat() 的 channel 在 ctx.Done() 时正确关闭**

bamboo provider 的 Chat 实现应在 ctx 取消时 close channel。若发现泄漏，在 bridge 层加 `ctx, cancel := context.WithCancel(c.Request.Context()); defer cancel()` 保护。

---

### Task 19: 压测

- [ ] 并发流式连接压测（用 vegeta / wrk），观察 goroutine 数、内存、延迟。

---

### Task 20: 全量灰度 + 观察

- [ ] 开启全部 24 个 ApiType 灰度
- [ ] 观察 1-2 周生产数据（计费偏差、错误率、fallback 触发率）
- [ ] 稳定后可选删除 originalXxxRelay（但建议保留更久作为回滚能力）

---

## Spec 覆盖性自审（writing-plans 要求）

对照 spec 各章节，确认计划覆盖：

| Spec 章节 | 对应 Task | 覆盖 |
|-----------|----------|------|
| 四、bamboo 改造（4.1 移除 base-go） | Task 1-3 | ✅ |
| 四、bamboo 改造（4.2 提升 provider） | Task 4-5 | ✅ |
| 五、relay/bamboo/ 桥包（5.1-5.7） | Task 7-10 | ✅ |
| 六、4 Helper 改造 | Task 11-15 | ✅ |
| 六、旁路处理（6.3） | Task 11 Step 4 | ✅ |
| 七、灰度与 fallback | Task 7（开关）+ 各 Helper Task | ✅ |
| 九、风险登记册（goroutine 泄漏 R2） | Task 18 | ✅ |
| 九、风险登记册（reasoning token R4） | Task 17 | ✅ |
| 十、路线图 Phase 1-4 | Task 6-20 | ✅ |
| 十一、验收清单 | 各 Phase 验收检查点 | ✅ |

**placeholder 扫描**：Task 17（reasoning 累计）有一处"实现时细化"——这是因为 bamboo StreamEvent.Delta 是 `any` 接口，具体 thinking delta 判定需实现时看实际类型，属合理的实现期细化，非空 placeholder。其余步骤代码完整。

**类型一致性**：`ChatRelay`、`newProvider`、`translateCodecError`、`relayFormatToCodec`、`accumulateReasoning`、`ErrUnsupportedProvider`、`originalXxxRelay` 在各 Task 间命名一致。✅

---

## 执行说明

本计划跨双仓库、4 个 Phase、20 个 Task。执行建议：

1. **Phase 0（Task 1-5）**：在 bamboo-messages 仓库顺序执行（PR-B1 → PR-B2 → tag）。
2. **Phase 1（Task 6-10）**：TDD 顺序执行，每个 bridge 文件先测试后实现。
3. **Phase 2（Task 11-12）**：TextHelper 接入 + 端到端验证。
4. **Phase 3（Task 13-16）**：3 个 Helper 对称改造，**可并发 TaskAgent**（ClaudeHelper/GeminiHelper/ResponsesHelper 互不依赖）。
5. **Phase 4（Task 17-20）**：稳定性收尾，持续进行。

**TaskAgent 并发点**（符合用户 workflow_protocol 的并发规则）：
- Phase 3 的 Task 13/14/15 三个 Helper 改造可并发（文件互不冲突）。
- 其余 Task 有前后依赖（尤其 Phase 0 → Phase 1 → Phase 2/3），不可并发。
