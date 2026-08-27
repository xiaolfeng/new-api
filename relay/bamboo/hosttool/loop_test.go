package hosttool

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/stretchr/testify/assert"
)

func TestDecideAction(t *testing.T) {
	plan := &relaycommon.HostToolPlan{
		Enabled: true,
		Mode:    ModeLoop,
		Decls:   []relaycommon.HostToolDecl{{OriginalName: "WebSearch", Canonical: CanonicalWebSearch}},
	}
	assert.Equal(t, ActionPassthrough, DecideAction(plan, nil))
	assert.Equal(t, ActionHop2, DecideAction(plan, []toolUseCall{{Name: "WebSearch"}}))
	assert.Equal(t, ActionPassthrough, DecideAction(plan, []toolUseCall{{Name: "WebSearch"}, {Name: "bash"}}))

	plan.Mode = ModeReturn
	assert.Equal(t, ActionFoldB, DecideAction(plan, []toolUseCall{{Name: "webfetch"}}))
}

// 只有具名客户端（如 Claude Code）在所有权表中透传首个 tool_call；Grok Build 与 generic 走网关 A-thin。
func TestDecideActionPassthroughClientOwnedTools(t *testing.T) {
	plan := &relaycommon.HostToolPlan{
		Enabled: true,
		Mode:    ModeLoop,
		Decls:   []relaycommon.HostToolDecl{{OriginalName: "web_search", Canonical: CanonicalWebSearch}},
	}
	grok := &relaycommon.RelayInfo{ClientProfile: common.ClientProfileGrokBuild}
	claude := &relaycommon.RelayInfo{ClientProfile: common.ClientProfileClaudeCode}

	// Claude Code 自带 helper 回写，命中所有权表做透传
	assert.Equal(t, ActionPassthrough, DecideActionForClient(plan, []toolUseCall{{Name: "web_search"}}, claude))
	assert.Equal(t, ActionPassthrough, DecideActionForClient(plan, []toolUseCall{{Name: "WebSearch"}}, claude))
	assert.Equal(t, ActionHop2, DecideActionForClient(plan, []toolUseCall{{Name: "web_fetch"}}, claude))

	// Grok Build 由网关 A-thin 代跑，不走客户端本地透传
	assert.Equal(t, ActionHop2, DecideActionForClient(plan, []toolUseCall{{Name: "web_search"}}, grok))
	assert.Equal(t, ActionHop2, DecideActionForClient(plan, []toolUseCall{{Name: "WebSearch"}}, grok))
	assert.Equal(t, ActionHop2, DecideActionForClient(plan, []toolUseCall{{Name: "web_search"}}, nil))

	// 混入非 host 工具时整组透传，网关不代跑任何调用。
	assert.Equal(t, ActionPassthrough, DecideActionForClient(plan, []toolUseCall{{Name: "WebSearch"}, {Name: "bash"}}, claude))

	// 总开关关闭时回到纯工具绑定路径：全部由网关 A-thin 代跑。
	st := model_setting.GetBambooSettings()
	prev := st.EnableClientStrictEgress
	st.EnableClientStrictEgress = boolPtr(false)
	t.Cleanup(func() { st.EnableClientStrictEgress = prev })
	assert.Equal(t, ActionHop2, DecideActionForClient(plan, []toolUseCall{{Name: "WebSearch"}}, claude))
}

func TestSuppressHostToolEcho(t *testing.T) {
	grok := &relaycommon.RelayInfo{ClientProfile: common.ClientProfileGrokBuild}
	codex := &relaycommon.RelayInfo{ClientProfile: common.ClientProfileCodex}
	claude := &relaycommon.RelayInfo{ClientProfile: common.ClientProfileClaudeCode}
	generic := &relaycommon.RelayInfo{ClientProfile: common.ClientProfileGeneric}

	// ② 表内 Agent 客户端：网关代跑后抑制回抛，防本地主循环直连官方代理。
	assert.True(t, SuppressHostToolEcho(grok, ExecResult{Kind: "search", OriginalName: "web_search"}))
	assert.True(t, SuppressHostToolEcho(codex, ExecResult{Kind: "search", OriginalName: "web_search"}))
	assert.False(t, SuppressHostToolEcho(grok, ExecResult{Kind: "fetch", OriginalName: "open_page"}))
	assert.False(t, SuppressHostToolEcho(generic, ExecResult{Kind: "search", OriginalName: "web_search"}))
	// ① 所有权表客户端（Claude Code）：helper 自带回写，同样不回抛。
	assert.True(t, SuppressHostToolEcho(claude, ExecResult{Kind: "search", OriginalName: "web_search"}))

	st := model_setting.GetBambooSettings()
	prev := st.EnableClientStrictEgress
	st.EnableClientStrictEgress = boolPtr(false)
	t.Cleanup(func() { st.EnableClientStrictEgress = prev })
	assert.False(t, SuppressHostToolEcho(grok, ExecResult{Kind: "search", OriginalName: "web_search"}))
}

func boolPtr(v bool) *bool { return &v }
