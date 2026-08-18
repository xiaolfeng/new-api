package hosttool

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalBuiltinCompleteSearch(t *testing.T) {
	body, err := MarshalBuiltinComplete("grok-4.6", "req1", 1, ExecResult{
		Kind:  "search",
		OK:    true,
		Query: "tokyo weather",
		Hits:  []SearchHit{{Title: "Weather", URL: "https://example.com"}},
	})
	require.NoError(t, err)
	text := string(body)
	assert.Contains(t, text, `"type":"web_search_call"`)
	assert.Contains(t, text, `"query":"tokyo weather"`)
	assert.Contains(t, text, `"url":"https://example.com"`)
	assert.Contains(t, text, `"type":"output_text"`)
	assert.Contains(t, text, `"type":"url_citation"`)
	assert.Contains(t, text, "Weather")
	assert.NotContains(t, text, `[web_search] query=`)
	var root map[string]any
	require.NoError(t, common.Unmarshal(body, &root))
	usage, ok := root["usage"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(0), usage["output_tokens"])
	output, ok := root["output"].([]any)
	require.True(t, ok)
	require.Len(t, output, 2)
	msg := output[1].(map[string]any)
	assert.Equal(t, "message", msg["type"])
	content := msg["content"].([]any)
	part := content[0].(map[string]any)
	assert.NotEmpty(t, part["text"])
	anns, ok := part["annotations"].([]any)
	require.True(t, ok)
	require.Len(t, anns, 1)
	assert.Equal(t, "https://example.com", anns[0].(map[string]any)["url"])
}

func TestMarshalBuiltinCompleteFetchNoSearchBillShape(t *testing.T) {
	body, err := MarshalBuiltinComplete("grok-4.6", "req1", 1, ExecResult{
		Kind:    "fetch",
		OK:      true,
		URL:     "https://example.com",
		Body:    "# Hello",
		Pattern: "nav",
	})
	require.NoError(t, err)
	text := string(body)
	assert.Contains(t, text, `"type":"open_page"`)
	assert.Contains(t, text, `"url":"https://example.com"`)
	assert.Contains(t, text, `"pattern":"nav"`)
	assert.NotContains(t, text, "# Hello")
}

func TestMarshalBuiltinCompleteBackendOff(t *testing.T) {
	body, err := MarshalBuiltinComplete("grok-4.6", "req1", 1, ExecResult{
		Kind:      "search",
		OK:        false,
		Query:     "q",
		ErrorCode: ErrBackendDisabled,
	})
	require.NoError(t, err)
	var root map[string]any
	require.NoError(t, common.Unmarshal(body, &root))
	assert.Equal(t, "incomplete", root["status"])
	output, ok := root["output"].([]any)
	require.True(t, ok)
	require.GreaterOrEqual(t, len(output), 2)
	item, ok := output[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "failed", item["status"])
	msg := output[1].(map[string]any)
	assert.Equal(t, "message", msg["type"])
	text := string(body)
	assert.Contains(t, text, "No search results found.")
}

func TestFormatGrokSearchOutputCitations(t *testing.T) {
	text, anns := formatGrokSearchOutput(ExecResult{
		OK: true,
		Hits: []SearchHit{
			{Title: "GitHub", URL: "https://github.com/XiaoLFeng"},
			{Title: "Blog", URL: "https://blog.x-lf.com"},
		},
	})
	assert.Contains(t, text, "[GitHub](https://github.com/XiaoLFeng)")
	assert.Contains(t, text, "[Blog](https://blog.x-lf.com)")
	require.Len(t, anns, 2)
	first := anns[0].(map[string]any)
	assert.Equal(t, "url_citation", first["type"])
	assert.Equal(t, "https://github.com/XiaoLFeng", first["url"])
	assert.Equal(t, 0, first["start_index"])
	assert.Greater(t, first["end_index"], first["start_index"])
}

func TestBuiltinStreamFramesOrder(t *testing.T) {
	frames, err := BuiltinStreamFrames("grok-4.6", "req1", 1, ExecResult{
		Kind:  "search",
		OK:    true,
		Query: "q",
		Hits:  []SearchHit{{URL: "https://example.com", Title: "T"}},
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(frames), 5)
	joined := strings.Builder{}
	for _, frame := range frames {
		joined.Write(frame)
	}
	text := joined.String()
	assert.Contains(t, string(frames[0]), "event: response.created")
	assert.Contains(t, string(frames[1]), "event: response.in_progress")
	assert.Contains(t, text, "event: response.completed")
	assert.Contains(t, text, `"type":"web_search_call"`)
	assert.Contains(t, text, `"type":"output_text"`)
	assert.Contains(t, text, `"type":"url_citation"`)
	assert.NotContains(t, text, "tool_use")
	assert.False(t, strings.Contains(text, `[web_search] query=`))
}
