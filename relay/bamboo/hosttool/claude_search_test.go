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

func TestIsClaudeWebFetchHelper(t *testing.T) {
	helperBody, err := common.Marshal(map[string]any{
		"model": "claude-opus-4-6",
		"tools": []any{
			map[string]any{"type": "web_fetch_20250910", "name": "web_fetch", "max_uses": 5},
		},
	})
	require.NoError(t, err)
	assert.True(t, IsClaudeWebFetchHelper(types.RelayFormatClaude, helperBody))
	assert.True(t, IsClaudeServerToolHelper(types.RelayFormatClaude, helperBody))
	assert.False(t, IsClaudeWebSearchHelper(types.RelayFormatClaude, helperBody))
	assert.False(t, IsClaudeWebFetchHelper(types.RelayFormatOpenAIResponses, helperBody))

	newer, err := common.Marshal(map[string]any{
		"tools": []any{
			map[string]any{"type": "web_fetch_20260209", "name": "web_fetch"},
		},
	})
	require.NoError(t, err)
	assert.True(t, IsClaudeWebFetchHelper(types.RelayFormatClaude, newer))
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

func TestClaudeServerSearchStreamFramesSplitsHits(t *testing.T) {
	frames, err := ClaudeServerSearchStreamFrames("claude-opus-4-6", "req1", ExecResult{
		Kind:  "search",
		OK:    true,
		Query: "筱锋",
		Hits: []SearchHit{
			{Title: "GitHub", URL: "https://github.com/XiaoLFeng"},
			{Title: "Blog", URL: "https://blog.x-lf.com"},
		},
	})
	require.NoError(t, err)
	joined := strings.Builder{}
	for _, frame := range frames {
		joined.Write(frame)
	}
	text := joined.String()
	assert.Equal(t, 1, strings.Count(text, `"type":"server_tool_use"`))
	assert.Equal(t, 2, strings.Count(text, `"type":"web_search_tool_result"`))
	assert.Contains(t, text, `"index":1`)
	assert.Contains(t, text, `"index":2`)
	assert.Equal(t, 1, strings.Count(text, `"type":"input_json_delta"`))
}

func TestMarshalClaudeServerSearchMultipleHitsFromOpaque(t *testing.T) {
	body, err := MarshalClaudeServerSearch("claude-opus-4-6", "req1", ExecResult{
		Kind:  "search",
		OK:    true,
		Query: "筱锋",
		OpaqueText: `{"results":[
			{"url":"https://github.com/XiaoLFeng","title":"GitHub"},
			{"url":"https://blog.x-lf.com","title":"Blog"},
			{"url":"https://x.com/lfeng_xiao","title":"X"}
		]}`,
	})
	require.NoError(t, err)
	var root map[string]any
	require.NoError(t, common.Unmarshal(body, &root))
	content := root["content"].([]any)
	require.Len(t, content, 4)
	assert.Equal(t, "server_tool_use", content[0].(map[string]any)["type"])
	wantURLs := []string{
		"https://github.com/XiaoLFeng",
		"https://blog.x-lf.com",
		"https://x.com/lfeng_xiao",
	}
	for i, want := range wantURLs {
		block := content[i+1].(map[string]any)
		assert.Equal(t, "web_search_tool_result", block["type"])
		hits, ok := block["content"].([]any)
		require.True(t, ok)
		require.Len(t, hits, 1)
		assert.Equal(t, want, hits[0].(map[string]any)["url"])
	}
	assert.Equal(t, 3, strings.Count(string(body), `"type":"web_search_tool_result"`))
}

func TestMarshalClaudeServerFetch(t *testing.T) {
	body, err := MarshalClaudeServerFetch("claude-opus-4-6", "req1", ExecResult{
		Kind: "fetch",
		OK:   true,
		URL:  "https://blog.x-lf.com",
		Body: "# hello",
	})
	require.NoError(t, err)
	text := string(body)
	assert.Contains(t, text, `"type":"server_tool_use"`)
	assert.Contains(t, text, `"name":"web_fetch"`)
	assert.Contains(t, text, `"type":"web_fetch_tool_result"`)
	assert.Contains(t, text, `"type":"web_fetch_result"`)
	assert.Contains(t, text, `"url":"https://blog.x-lf.com"`)
	assert.Contains(t, text, `# hello`)
	assert.NotContains(t, text, `"type":"web_search_tool_result"`)
	assert.NotContains(t, text, `[WebFetch] url=`)

	var root map[string]any
	require.NoError(t, common.Unmarshal(body, &root))
	content := root["content"].([]any)
	require.GreaterOrEqual(t, len(content), 2)
	first := content[0].(map[string]any)
	input := first["input"].(map[string]any)
	assert.Equal(t, "https://blog.x-lf.com", input["url"])
}

func TestClaudeServerFetchStreamFrames(t *testing.T) {
	frames, err := ClaudeServerFetchStreamFrames("claude-opus-4-6", "req1", ExecResult{
		Kind: "fetch",
		OK:   true,
		URL:  "https://example.com",
		Body: "page",
	})
	require.NoError(t, err)
	joined := strings.Builder{}
	for _, frame := range frames {
		joined.Write(frame)
	}
	text := joined.String()
	assert.Contains(t, text, `"type":"server_tool_use"`)
	assert.Contains(t, text, `"name":"web_fetch"`)
	assert.Contains(t, text, `"input":""`)
	assert.Contains(t, text, `"type":"input_json_delta"`)
	assert.Contains(t, text, `\"url\":\"https://example.com\"`)
	assert.Contains(t, text, `"type":"web_fetch_tool_result"`)
	assert.Equal(t, 1, strings.Count(text, `"type":"web_fetch_tool_result"`))
	assert.Equal(t, 1, strings.Count(text, `"type":"input_json_delta"`))
}
