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
		Model: "test-model",
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
	reqDTO := dto.OpenAIResponsesRequest{Model: "test-model", Tools: tools}
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

func geminiUserContents(text string) []dto.GeminiChatContent {
	return []dto.GeminiChatContent{{
		Role:  "user",
		Parts: []dto.GeminiPart{{Text: text}},
	}}
}

func TestInspectGeminiGoogleSearchRemash(t *testing.T) {
	reqDTO := dto.GeminiChatRequest{Contents: geminiUserContents("查一下最新比赛")}
	reqDTO.SetTools([]dto.GeminiChatTool{{
		GoogleSearch: map[string]string{},
	}})
	raw, err := common.Marshal(reqDTO)
	require.NoError(t, err)
	relayReq := &bamboocodec.RelayRequest{Config: &bamboosdk.RequestConfig{}}
	plan, err := InspectAndRewrite(types.RelayFormatGemini, raw, relayReq, enabledSettings())
	require.NoError(t, err)
	require.True(t, plan.Enabled)
	require.Len(t, plan.Decls, 1)
	assert.Equal(t, CanonicalWebSearch, plan.Decls[0].Canonical)
	assert.Equal(t, "googleSearch", plan.Decls[0].OriginalName)
	assert.Equal(t, sourceServerType, plan.Decls[0].Source)
	assert.Equal(t, billingWebSearch, plan.Decls[0].BillingName)
	require.Len(t, relayReq.Config.Tools, 1)
	assert.Equal(t, "googleSearch", relayReq.Config.Tools[0].Name)
	assert.NotEmpty(t, relayReq.Config.Tools[0].Description)
	assert.Contains(t, string(relayReq.Config.Tools[0].InputSchema), `"query"`)
}

func TestInspectGeminiGoogleSearchRetrieval(t *testing.T) {
	reqDTO := dto.GeminiChatRequest{Contents: geminiUserContents("latest news")}
	reqDTO.SetTools([]dto.GeminiChatTool{{
		GoogleSearchRetrieval: map[string]any{
			"dynamicRetrievalConfig": map[string]any{
				"mode":             "MODE_DYNAMIC",
				"dynamicThreshold": 0.3,
			},
		},
	}})
	raw, err := common.Marshal(reqDTO)
	require.NoError(t, err)
	relayReq := &bamboocodec.RelayRequest{Config: &bamboosdk.RequestConfig{}}
	plan, err := InspectAndRewrite(types.RelayFormatGemini, raw, relayReq, enabledSettings())
	require.NoError(t, err)
	require.True(t, plan.Enabled)
	require.Len(t, plan.Decls, 1)
	assert.Equal(t, "googleSearchRetrieval", plan.Decls[0].OriginalName)
	assert.Equal(t, CanonicalWebSearch, plan.Decls[0].Canonical)
	require.Len(t, relayReq.Config.Tools, 1)
	assert.Equal(t, "googleSearchRetrieval", relayReq.Config.Tools[0].Name)
}

func TestInspectGeminiGoogleSearchSnakeCase(t *testing.T) {
	raw := []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}],"tools":[{"google_search":{}}]}`)
	relayReq := &bamboocodec.RelayRequest{Config: &bamboosdk.RequestConfig{}}
	plan, err := InspectAndRewrite(types.RelayFormatGemini, raw, relayReq, enabledSettings())
	require.NoError(t, err)
	require.True(t, plan.Enabled)
	require.Len(t, plan.Decls, 1)
	assert.Equal(t, "google_search", plan.Decls[0].OriginalName)
	require.Len(t, relayReq.Config.Tools, 1)
	assert.Equal(t, "google_search", relayReq.Config.Tools[0].Name)
}

func TestInspectGeminiGoogleSearchRetrievalSnakeCase(t *testing.T) {
	raw := []byte(`{"tools":[{"google_search_retrieval":{"dynamicRetrievalConfig":{"mode":"MODE_DYNAMIC"}}}]}`)
	relayReq := &bamboocodec.RelayRequest{Config: &bamboosdk.RequestConfig{}}
	plan, err := InspectAndRewrite(types.RelayFormatGemini, raw, relayReq, enabledSettings())
	require.NoError(t, err)
	require.True(t, plan.Enabled)
	require.Len(t, plan.Decls, 1)
	assert.Equal(t, "google_search_retrieval", plan.Decls[0].OriginalName)
}

func TestInspectGeminiToolsAsSingleObject(t *testing.T) {
	raw := []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}],"tools":{"googleSearch":{}}}`)
	relayReq := &bamboocodec.RelayRequest{Config: &bamboosdk.RequestConfig{}}
	plan, err := InspectAndRewrite(types.RelayFormatGemini, raw, relayReq, enabledSettings())
	require.NoError(t, err)
	require.True(t, plan.Enabled)
	require.Len(t, plan.Decls, 1)
	assert.Equal(t, "googleSearch", plan.Decls[0].OriginalName)
}

func TestInspectGeminiGoogleSearchNull(t *testing.T) {
	raw := []byte(`{"tools":[{"googleSearch":null}]}`)
	relayReq := &bamboocodec.RelayRequest{Config: &bamboosdk.RequestConfig{}}
	plan, err := InspectAndRewrite(types.RelayFormatGemini, raw, relayReq, enabledSettings())
	require.NoError(t, err)
	assert.False(t, plan.Enabled)
	assert.Empty(t, relayReq.Config.Tools)
}

func TestInspectGeminiGoogleSearchExcludeDomains(t *testing.T) {
	reqDTO := dto.GeminiChatRequest{Contents: geminiUserContents("docs")}
	reqDTO.SetTools([]dto.GeminiChatTool{{
		GoogleSearch: map[string]any{
			"excludeDomains": []string{"pinterest.com"},
		},
	}})
	raw, err := common.Marshal(reqDTO)
	require.NoError(t, err)
	relayReq := &bamboocodec.RelayRequest{Config: &bamboosdk.RequestConfig{}}
	plan, err := InspectAndRewrite(types.RelayFormatGemini, raw, relayReq, enabledSettings())
	require.NoError(t, err)
	require.Len(t, plan.Decls, 1)
	assert.Equal(t, []string{"pinterest.com"}, plan.Decls[0].BlockedDomains)
	assert.Empty(t, plan.Decls[0].AllowedDomains)
}

func TestInspectGeminiDedupGoogleSearchAndWebSearchDecl(t *testing.T) {
	reqDTO := dto.GeminiChatRequest{Contents: geminiUserContents("hi")}
	reqDTO.SetTools([]dto.GeminiChatTool{{
		GoogleSearch: map[string]string{},
		FunctionDeclarations: []map[string]any{
			{"name": "web_search"},
		},
	}})
	raw, err := common.Marshal(reqDTO)
	require.NoError(t, err)
	relayReq := &bamboocodec.RelayRequest{Config: &bamboosdk.RequestConfig{}}
	plan, err := InspectAndRewrite(types.RelayFormatGemini, raw, relayReq, enabledSettings())
	require.NoError(t, err)
	require.Len(t, plan.Decls, 1)
	assert.Equal(t, "googleSearch", plan.Decls[0].OriginalName)
	assert.Contains(t, plan.Stripped, "web_search")
	require.Len(t, relayReq.Config.Tools, 1)
	assert.Equal(t, "googleSearch", relayReq.Config.Tools[0].Name)
}

func TestInspectGeminiGoogleSearchKeepsCustomFunction(t *testing.T) {
	reqDTO := dto.GeminiChatRequest{Contents: geminiUserContents("hi")}
	reqDTO.SetTools([]dto.GeminiChatTool{{
		GoogleSearch: map[string]string{},
		FunctionDeclarations: []map[string]any{
			{"name": "get_weather", "description": "weather"},
		},
	}})
	raw, err := common.Marshal(reqDTO)
	require.NoError(t, err)
	relayReq := &bamboocodec.RelayRequest{Config: &bamboosdk.RequestConfig{
		Tools: []bamboosdk.Tool{{Name: "get_weather", Description: "weather"}},
	}}
	plan, err := InspectAndRewrite(types.RelayFormatGemini, raw, relayReq, enabledSettings())
	require.NoError(t, err)
	require.True(t, plan.Enabled)
	require.Len(t, plan.Decls, 1)
	assert.Equal(t, "googleSearch", plan.Decls[0].OriginalName)
	names := make([]string, 0, len(relayReq.Config.Tools))
	for _, tool := range relayReq.Config.Tools {
		names = append(names, tool.Name)
	}
	assert.Contains(t, names, "googleSearch")
	assert.Contains(t, names, "get_weather")
}

func TestInspectGeminiGoogleSearchDisabled(t *testing.T) {
	st := *model_setting.GetBambooSettings()
	st.EnableHostTools = false
	reqDTO := dto.GeminiChatRequest{Contents: geminiUserContents("hi")}
	reqDTO.SetTools([]dto.GeminiChatTool{{GoogleSearch: map[string]string{}}})
	raw, err := common.Marshal(reqDTO)
	require.NoError(t, err)
	relayReq := &bamboocodec.RelayRequest{Config: &bamboosdk.RequestConfig{}}
	plan, err := InspectAndRewrite(types.RelayFormatGemini, raw, relayReq, &st)
	require.NoError(t, err)
	assert.False(t, plan.Enabled)
	assert.Empty(t, relayReq.Config.Tools)
}

func TestInspectOpenAIGoogleSearchFunctionName(t *testing.T) {
	reqDTO := dto.GeneralOpenAIRequest{
		Model: "gpt-4o",
		Tools: []dto.ToolCallRequest{{
			Type: "function",
			Function: dto.FunctionRequest{
				Name:        "googleSearch",
				Description: "Search the web",
			},
		}},
	}
	raw, err := common.Marshal(reqDTO)
	require.NoError(t, err)
	relayReq := &bamboocodec.RelayRequest{Config: &bamboosdk.RequestConfig{}}
	plan, err := InspectAndRewrite(types.RelayFormatOpenAI, raw, relayReq, enabledSettings())
	require.NoError(t, err)
	require.True(t, plan.Enabled)
	require.Len(t, plan.Decls, 1)
	assert.Equal(t, "googleSearch", plan.Decls[0].OriginalName)
	assert.Equal(t, CanonicalWebSearch, plan.Decls[0].Canonical)
	require.Len(t, relayReq.Config.Tools, 1)
	assert.Equal(t, "googleSearch", relayReq.Config.Tools[0].Name)
	assert.Contains(t, string(relayReq.Config.Tools[0].InputSchema), `"query"`)
}
