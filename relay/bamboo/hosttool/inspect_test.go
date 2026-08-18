package hosttool

import (
	"testing"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func enabledSettings() *model_setting.BambooSettings {
	st := *model_setting.GetBambooSettings()
	st.EnableHostTools = true
	st.HostToolMode = "loop"
	return &st
}

func TestInspectAndRewriteDisabled(t *testing.T) {
	st := *model_setting.GetBambooSettings()
	st.EnableHostTools = false
	req := &bamboocodec.RelayRequest{Config: &bamboosdk.RequestConfig{}}
	plan, err := InspectAndRewrite(types.RelayFormatClaude, []byte(`{"tools":[]}`), req, &st)
	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.False(t, plan.Enabled)
	assert.Empty(t, req.Config.Tools)
}

func TestInspectClaudeServerToolRemash(t *testing.T) {
	reqDTO := dto.ClaudeRequest{
		Model: "claude-sonnet-4-20250514",
		Tools: []any{map[string]any{
			"type":     "web_search_20250305",
			"name":     "web_search",
			"max_uses": 2.0,
		}},
	}
	raw, err := common.Marshal(reqDTO)
	require.NoError(t, err)
	relayReq := &bamboocodec.RelayRequest{Config: &bamboosdk.RequestConfig{}}
	plan, err := InspectAndRewrite(types.RelayFormatClaude, raw, relayReq, enabledSettings())
	require.NoError(t, err)
	require.True(t, plan.Enabled)
	require.Len(t, plan.Decls, 1)
	assert.Equal(t, CanonicalWebSearch, plan.Decls[0].Canonical)
	assert.Equal(t, "web_search", plan.Decls[0].OriginalName)
	assert.Equal(t, 2, plan.Decls[0].MaxUses)
	require.Len(t, relayReq.Config.Tools, 1)
	assert.Equal(t, "web_search", relayReq.Config.Tools[0].Name)
	assert.NotEmpty(t, relayReq.Config.Tools[0].Description)
	assert.Contains(t, string(relayReq.Config.Tools[0].InputSchema), `"query"`)
}

func TestInspectOpenAIWebSearchOptions(t *testing.T) {
	reqDTO := dto.GeneralOpenAIRequest{
		Model:            "gpt-4o",
		WebSearchOptions: &dto.WebSearchOptions{SearchContextSize: "low"},
	}
	raw, err := common.Marshal(reqDTO)
	require.NoError(t, err)
	relayReq := &bamboocodec.RelayRequest{Config: &bamboosdk.RequestConfig{}}
	plan, err := InspectAndRewrite(types.RelayFormatOpenAI, raw, relayReq, enabledSettings())
	require.NoError(t, err)
	require.True(t, plan.Enabled)
	require.Len(t, plan.Decls, 1)
	assert.Equal(t, "web_search_options", plan.Decls[0].Source)
	assert.Equal(t, "web_search", plan.Decls[0].OriginalName)
	require.Len(t, relayReq.Config.Tools, 1)
}

func TestInspectResponsesPreview(t *testing.T) {
	tools, err := common.Marshal([]map[string]any{{"type": "web_search_preview"}})
	require.NoError(t, err)
	reqDTO := dto.OpenAIResponsesRequest{
		Model: "gpt-4o",
		Tools: tools,
	}
	raw, err := common.Marshal(reqDTO)
	require.NoError(t, err)
	relayReq := &bamboocodec.RelayRequest{Config: &bamboosdk.RequestConfig{}}
	plan, err := InspectAndRewrite(types.RelayFormatOpenAIResponses, raw, relayReq, enabledSettings())
	require.NoError(t, err)
	require.True(t, plan.Enabled)
	require.Len(t, plan.Decls, 1)
	assert.Equal(t, "web_search_preview", plan.Decls[0].BillingName)
	assert.Equal(t, "web_search_preview", plan.Decls[0].OriginalName)
}

func TestInspectResponsesFiltersAllowWins(t *testing.T) {
	tools, err := common.Marshal([]map[string]any{{
		"type": "web_search",
		"filters": map[string]any{
			"allowed_domains":  []string{"docs.x.ai"},
			"excluded_domains": []string{"reddit.com"},
		},
	}})
	require.NoError(t, err)
	reqDTO := dto.OpenAIResponsesRequest{
		Model: "grok-4.6",
		Tools: tools,
	}
	raw, err := common.Marshal(reqDTO)
	require.NoError(t, err)
	relayReq := &bamboocodec.RelayRequest{Config: &bamboosdk.RequestConfig{}}
	plan, err := InspectAndRewrite(types.RelayFormatOpenAIResponses, raw, relayReq, enabledSettings())
	require.NoError(t, err)
	require.True(t, plan.Enabled)
	require.Len(t, plan.Decls, 1)
	assert.Equal(t, []string{"docs.x.ai"}, plan.Decls[0].AllowedDomains)
	assert.Empty(t, plan.Decls[0].BlockedDomains)
}

func TestInspectResponsesFiltersExcluded(t *testing.T) {
	tools, err := common.Marshal([]map[string]any{{
		"type": "web_search_2025_08_26",
		"filters": map[string]any{
			"excluded_domains": []string{"pinterest.com"},
		},
	}})
	require.NoError(t, err)
	reqDTO := dto.OpenAIResponsesRequest{Model: "grok-4.6", Tools: tools}
	raw, err := common.Marshal(reqDTO)
	require.NoError(t, err)
	relayReq := &bamboocodec.RelayRequest{Config: &bamboosdk.RequestConfig{}}
	plan, err := InspectAndRewrite(types.RelayFormatOpenAIResponses, raw, relayReq, enabledSettings())
	require.NoError(t, err)
	require.Len(t, plan.Decls, 1)
	assert.Empty(t, plan.Decls[0].AllowedDomains)
	assert.Equal(t, []string{"pinterest.com"}, plan.Decls[0].BlockedDomains)
}

func TestInspectDedupCanonical(t *testing.T) {
	reqDTO := dto.ClaudeRequest{
		Model: "claude",
		Tools: []any{
			map[string]any{"name": "WebSearch", "input_schema": map[string]any{"type": "object"}},
			map[string]any{"type": "web_search_20250305", "name": "web_search"},
		},
	}
	raw, err := common.Marshal(reqDTO)
	require.NoError(t, err)
	relayReq := &bamboocodec.RelayRequest{Config: &bamboosdk.RequestConfig{
		Tools: []bamboosdk.Tool{
			{Name: "WebSearch", InputSchema: []byte(`{"type":"object"}`)},
			{Name: "web_search"},
		},
	}}
	plan, err := InspectAndRewrite(types.RelayFormatClaude, raw, relayReq, enabledSettings())
	require.NoError(t, err)
	require.Len(t, plan.Decls, 1)
	assert.Contains(t, plan.Stripped, "web_search")
	require.Len(t, relayReq.Config.Tools, 1)
	assert.Equal(t, "WebSearch", relayReq.Config.Tools[0].Name)
}
