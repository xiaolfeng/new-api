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

// 所有权表驱动：不同客户端身份命中同一张表，规则不再按品牌散写。
func TestDecideActionPassthroughClientOwnedTools(t *testing.T) {
	plan := &relaycommon.HostToolPlan{
		Enabled: true,
		Mode:    ModeLoop,
		Decls:   []relaycommon.HostToolDecl{{OriginalName: "web_search", Canonical: CanonicalWebSearch}},
	}
	grok := &relaycommon.RelayInfo{ClientProfile: common.ClientProfileGrokBuild}
	claude := &relaycommon.RelayInfo{ClientProfile: common.ClientProfileClaudeCode}

	for _, info := range []*relaycommon.RelayInfo{grok, claude} {
		assert.Equal(t, ActionPassthrough, DecideActionForClient(plan, []toolUseCall{{Name: "web_search"}}, info))
		// canonical 等价别名同权，不区分原始大小写拼法。
		assert.Equal(t, ActionPassthrough, DecideActionForClient(plan, []toolUseCall{{Name: "WebSearch"}}, info))
		assert.Equal(t, ActionHop2, DecideActionForClient(plan, []toolUseCall{{Name: "web_fetch"}}, info))
		assert.Equal(t, ActionHop2, DecideActionForClient(plan, []toolUseCall{{Name: "web_search"}}, nil))
	}

	// 混入非 host 工具时整组透传，网关不代跑任何调用。
	assert.Equal(t, ActionPassthrough, DecideActionForClient(plan, []toolUseCall{{Name: "web_search"}, {Name: "read_file"}}, grok))
	assert.Equal(t, ActionPassthrough, DecideActionForClient(plan, []toolUseCall{{Name: "WebSearch"}, {Name: "bash"}}, claude))

	// 总开关关闭时回到纯工具绑定路径：全部由网关 A-thin 代跑。
	st := model_setting.GetBambooSettings()
	prev := st.EnableClientStrictEgress
	st.EnableClientStrictEgress = boolPtr(false)
	t.Cleanup(func() { st.EnableClientStrictEgress = prev })
	assert.Equal(t, ActionHop2, DecideActionForClient(plan, []toolUseCall{{Name: "web_search"}}, grok))
	assert.Equal(t, ActionHop2, DecideActionForClient(plan, []toolUseCall{{Name: "WebSearch"}}, claude))
}

func TestSuppressHostToolEcho(t *testing.T) {
	grok := &relaycommon.RelayInfo{ClientProfile: common.ClientProfileGrokBuild}
	generic := &relaycommon.RelayInfo{ClientProfile: common.ClientProfileGeneric}

	assert.True(t, SuppressHostToolEcho(grok, ExecResult{Kind: "search", OriginalName: "web_search"}))
	assert.False(t, SuppressHostToolEcho(grok, ExecResult{Kind: "fetch", OriginalName: "open_page"}))
	assert.False(t, SuppressHostToolEcho(generic, ExecResult{Kind: "search", OriginalName: "web_search"}))

	st := model_setting.GetBambooSettings()
	prev := st.EnableClientStrictEgress
	st.EnableClientStrictEgress = boolPtr(false)
	t.Cleanup(func() { st.EnableClientStrictEgress = prev })
	assert.False(t, SuppressHostToolEcho(grok, ExecResult{Kind: "search", OriginalName: "web_search"}))
}

func boolPtr(v bool) *bool { return &v }
