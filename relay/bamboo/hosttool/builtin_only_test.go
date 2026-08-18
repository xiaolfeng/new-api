package hosttool

import (
	"testing"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsResponsesBuiltinOnlyWebSearchOnly(t *testing.T) {
	tools, err := common.Marshal([]map[string]any{{"type": "web_search"}})
	require.NoError(t, err)
	raw, err := common.Marshal(dto.OpenAIResponsesRequest{Model: "grok-4.6", Tools: tools})
	require.NoError(t, err)
	relayReq := &bamboocodec.RelayRequest{Config: &bamboosdk.RequestConfig{
		Tools: []bamboosdk.Tool{{Name: "web_search"}},
	}}
	plan := &relaycommon.HostToolPlan{
		Enabled: true,
		Decls:   []relaycommon.HostToolDecl{{OriginalName: "web_search", Canonical: CanonicalWebSearch}},
	}
	assert.True(t, IsResponsesBuiltinOnly(types.RelayFormatOpenAIResponses, raw, plan, relayReq))
}

func TestIsResponsesBuiltinOnlyRejectsClaudeMix(t *testing.T) {
	raw, err := common.Marshal(dto.ClaudeRequest{
		Model: "claude",
		Tools: []any{
			map[string]any{"name": "WebSearch"},
			map[string]any{"name": "Bash"},
		},
	})
	require.NoError(t, err)
	plan := &relaycommon.HostToolPlan{
		Enabled: true,
		Decls:   []relaycommon.HostToolDecl{{OriginalName: "WebSearch", Canonical: CanonicalWebSearch}},
	}
	assert.False(t, IsResponsesBuiltinOnly(types.RelayFormatClaude, raw, plan, nil))
}

func TestIsResponsesBuiltinOnlyRejectsXSearch(t *testing.T) {
	tools, err := common.Marshal([]map[string]any{
		{"type": "web_search"},
		{"type": "x_search"},
	})
	require.NoError(t, err)
	raw, err := common.Marshal(dto.OpenAIResponsesRequest{Model: "grok-4.6", Tools: tools})
	require.NoError(t, err)
	plan := &relaycommon.HostToolPlan{
		Enabled: true,
		Decls:   []relaycommon.HostToolDecl{{OriginalName: "web_search", Canonical: CanonicalWebSearch}},
	}
	assert.False(t, IsResponsesBuiltinOnly(types.RelayFormatOpenAIResponses, raw, plan, &bamboocodec.RelayRequest{
		Config: &bamboosdk.RequestConfig{Tools: []bamboosdk.Tool{{Name: "web_search"}}},
	}))
}

func TestClassifyBuiltinInputURL(t *testing.T) {
	req := &bamboocodec.RelayRequest{
		Messages: []bamboosdk.BambooMessage{bamboosdk.NewUserMessage("https://docs.x.ai/developers/tools/web-search")},
	}
	in := classifyBuiltinInput(&relaycommon.HostToolPlan{
		Decls: []relaycommon.HostToolDecl{{OriginalName: "open_page", Canonical: CanonicalWebFetch}},
	}, req)
	assert.Equal(t, "fetch", in.Kind)
	assert.Equal(t, "https://docs.x.ai/developers/tools/web-search", in.URL)
	assert.Equal(t, "open_page", in.Name)
}

func TestClassifyBuiltinInputQuery(t *testing.T) {
	req := &bamboocodec.RelayRequest{
		Messages: []bamboosdk.BambooMessage{bamboosdk.NewUserMessage("xAI web_search Responses API")},
	}
	in := classifyBuiltinInput(&relaycommon.HostToolPlan{
		Decls: []relaycommon.HostToolDecl{{OriginalName: "web_search", Canonical: CanonicalWebSearch}},
	}, req)
	assert.Equal(t, "search", in.Kind)
	assert.Equal(t, "xAI web_search Responses API", in.Query)
}

func TestClassifyBuiltinInputStripsClaudeSearchPrefix(t *testing.T) {
	req := &bamboocodec.RelayRequest{
		Messages: []bamboosdk.BambooMessage{
			bamboosdk.NewUserMessage("Perform a web search for the query: 筱锋 Xiao Lfeng 程序员"),
		},
	}
	in := classifyBuiltinInput(&relaycommon.HostToolPlan{
		Decls: []relaycommon.HostToolDecl{{OriginalName: "web_search", Canonical: CanonicalWebSearch}},
	}, req)
	assert.Equal(t, "search", in.Kind)
	assert.Equal(t, "筱锋 Xiao Lfeng 程序员", in.Query)
}

func TestClassifyBuiltinInputFetchOnlyExtractsURL(t *testing.T) {
	req := &bamboocodec.RelayRequest{
		Messages: []bamboosdk.BambooMessage{
			bamboosdk.NewUserMessage("Please fetch https://blog.x-lf.com/1003.html now"),
		},
	}
	in := classifyBuiltinInput(&relaycommon.HostToolPlan{
		Decls: []relaycommon.HostToolDecl{{OriginalName: "web_fetch", Canonical: CanonicalWebFetch}},
	}, req)
	assert.Equal(t, "fetch", in.Kind)
	assert.Equal(t, "https://blog.x-lf.com/1003.html", in.URL)
	assert.Equal(t, "web_fetch", in.Name)
}

func TestLastUserTextSkipsAssistant(t *testing.T) {
	req := &bamboocodec.RelayRequest{
		Messages: []bamboosdk.BambooMessage{
			bamboosdk.NewUserMessage("first"),
			bamboosdk.NewAssistantMessage("ignore me"),
			bamboosdk.NewUserMessage("second"),
		},
	}
	assert.Equal(t, "second", lastUserText(req))
}
