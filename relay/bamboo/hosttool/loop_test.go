package hosttool

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestDecideActionClaudeStrictPassthroughWebSearch(t *testing.T) {
	plan := &relaycommon.HostToolPlan{
		Enabled: true,
		Mode:    ModeLoop,
		Decls:   []relaycommon.HostToolDecl{{OriginalName: "WebSearch", Canonical: CanonicalWebSearch}},
	}
	info := &relaycommon.RelayInfo{ClientProfile: common.ClientProfileClaudeCode}
	require.True(t, model_setting.GetBambooSettings().ClaudeStrictEgressEnabled())
	assert.Equal(t, ActionPassthrough, DecideActionForClient(plan, []toolUseCall{{Name: "WebSearch"}}, info))
	assert.Equal(t, ActionHop2, DecideActionForClient(plan, []toolUseCall{{Name: "WebSearch"}}, nil))
	assert.Equal(t, ActionPassthrough, DecideActionForClient(plan, []toolUseCall{{Name: "WebSearch"}, {Name: "bash"}}, info))

	st := model_setting.GetBambooSettings()
	prev := st.EnableClaudeStrictEgress
	st.EnableClaudeStrictEgress = boolPtr(false)
	t.Cleanup(func() { st.EnableClaudeStrictEgress = prev })
	assert.Equal(t, ActionHop2, DecideActionForClient(plan, []toolUseCall{{Name: "WebSearch"}}, info))
}

func boolPtr(v bool) *bool { return &v }
