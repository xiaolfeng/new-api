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
	assert.NotContains(t, text, `[web_search] query=`)
	var root map[string]any
	require.NoError(t, common.Unmarshal(body, &root))
	usage, ok := root["usage"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(0), usage["output_tokens"])
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
	require.Len(t, output, 1)
	item, ok := output[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "failed", item["status"])
}

func TestBuiltinStreamFramesOrder(t *testing.T) {
	frames, err := BuiltinStreamFrames("grok-4.6", "req1", 1, ExecResult{
		Kind:  "search",
		OK:    true,
		Query: "q",
		Hits:  []SearchHit{{URL: "https://example.com", Title: "T"}},
	})
	require.NoError(t, err)
	require.Len(t, frames, 5)
	joined := string(frames[0]) + string(frames[1]) + string(frames[2]) + string(frames[3]) + string(frames[4])
	assert.Contains(t, string(frames[0]), "event: response.created")
	assert.Contains(t, string(frames[1]), "event: response.in_progress")
	assert.Contains(t, string(frames[2]), "event: response.output_item.added")
	assert.Contains(t, string(frames[3]), "event: response.output_item.done")
	assert.Contains(t, string(frames[4]), "event: response.completed")
	assert.Contains(t, joined, `"type":"web_search_call"`)
	assert.NotContains(t, joined, "tool_use")
	assert.False(t, strings.Contains(joined, `[web_search] query=`))
}
