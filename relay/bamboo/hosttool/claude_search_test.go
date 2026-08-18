package hosttool

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsClaudeWebSearchHelper(t *testing.T) {
	helperBody, err := common.Marshal(map[string]any{
		"model": "claude-opus-4-6",
		"tools": []any{
			map[string]any{"type": "web_search_20250305", "name": "web_search", "max_uses": 8},
		},
	})
	require.NoError(t, err)
	assert.True(t, IsClaudeWebSearchHelper(types.RelayFormatClaude, helperBody))
	assert.False(t, IsClaudeWebSearchHelper(types.RelayFormatOpenAIResponses, helperBody))

	mixed, err := common.Marshal(map[string]any{
		"tools": []any{
			map[string]any{"type": "web_search_20250305", "name": "web_search"},
			map[string]any{"name": "bash", "input_schema": map[string]any{"type": "object"}},
		},
	})
	require.NoError(t, err)
	assert.False(t, IsClaudeWebSearchHelper(types.RelayFormatClaude, mixed))

	mainLoop, err := common.Marshal(map[string]any{
		"tools": []any{
			map[string]any{"name": "WebSearch", "input_schema": map[string]any{"type": "object"}},
		},
	})
	require.NoError(t, err)
	assert.False(t, IsClaudeWebSearchHelper(types.RelayFormatClaude, mainLoop))
}

func TestMarshalClaudeServerSearchHits(t *testing.T) {
	body, err := MarshalClaudeServerSearch("claude-opus-4-6", "req1", ExecResult{
		Kind:  "search",
		OK:    true,
		Query: "tokyo weather",
		Hits:  []SearchHit{{Title: "Weather", URL: "https://example.com"}},
	})
	require.NoError(t, err)
	text := string(body)
	assert.Contains(t, text, `"type":"server_tool_use"`)
	assert.Contains(t, text, `"type":"web_search_tool_result"`)
	assert.Contains(t, text, `"title":"Weather"`)
	assert.Contains(t, text, `"url":"https://example.com"`)
	assert.Contains(t, text, `"query":"tokyo weather"`)
	assert.NotContains(t, text, `"type":"web_search_call"`)
	assert.NotContains(t, text, `[WebSearch] query=`)

	var root map[string]any
	require.NoError(t, common.Unmarshal(body, &root))
	content, ok := root["content"].([]any)
	require.True(t, ok)
	require.GreaterOrEqual(t, len(content), 2)
	first, ok := content[0].(map[string]any)
	require.True(t, ok)
	input, ok := first["input"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "tokyo weather", input["query"])
}

func TestMarshalClaudeServerSearchError(t *testing.T) {
	body, err := MarshalClaudeServerSearch("claude-opus-4-6", "req1", ExecResult{
		Kind:      "search",
		OK:        false,
		Query:     "q",
		ErrorCode: ErrBackendDisabled,
	})
	require.NoError(t, err)
	var root map[string]any
	require.NoError(t, common.Unmarshal(body, &root))
	content := root["content"].([]any)
	result := content[1].(map[string]any)
	errObj, ok := result["content"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, ErrBackendDisabled, errObj["error_code"])
}

func TestClaudeServerSearchStreamFrames(t *testing.T) {
	frames, err := ClaudeServerSearchStreamFrames("claude-opus-4-6", "req1", ExecResult{
		Kind:  "search",
		OK:    true,
		Query: "cats",
		Hits:  []SearchHit{{Title: "Cats", URL: "https://example.com/cats"}},
	})
	require.NoError(t, err)
	require.NotEmpty(t, frames)
	joined := strings.Builder{}
	for _, frame := range frames {
		joined.Write(frame)
	}
	text := joined.String()
	assert.Contains(t, text, "event: message_start")
	assert.Contains(t, text, `"type":"server_tool_use"`)
	assert.Contains(t, text, `"input":""`)
	assert.Contains(t, text, `"type":"input_json_delta"`)
	assert.Contains(t, text, `\"query\":\"cats\"`)
	assert.Contains(t, text, `"type":"web_search_tool_result"`)
	assert.Contains(t, text, "event: message_stop")
	assert.NotContains(t, text, `"type":"web_search_call"`)

	assert.Equal(t, 1, strings.Count(text, `"type":"web_search_tool_result"`))
	assert.Equal(t, 1, strings.Count(text, `"type":"input_json_delta"`))
}
