package hosttool

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
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
