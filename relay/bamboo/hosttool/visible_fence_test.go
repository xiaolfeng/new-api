package hosttool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/QuantumNous/new-api/common"
)

func TestMCPResultJSONShape(t *testing.T) {
	raw := MCPResultJSON(ExecResult{
		OriginalName: "WebSearch",
		Kind:         "search",
		OK:           true,
		Query:        "cats",
		Hits:         []SearchHit{{Title: "Cat", URL: "https://c", Snippet: "meow"}},
	})
	var parsed map[string]any
	require.NoError(t, common.Unmarshal([]byte(raw), &parsed))
	assert.Equal(t, false, parsed["isError"])
	content, ok := parsed["content"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, content)
	item, _ := content[0].(map[string]any)
	require.Equal(t, "text", item["type"])
	text, _ := item["text"].(string)
	assert.Contains(t, text, `[WebSearch] query="cats"`)
}

func TestInjectOpenAIToolOutputs(t *testing.T) {
	body := []byte(`{"choices":[{"message":{"role":"assistant","content":"hi","tool_calls":[{"id":"call_1","type":"function","function":{"name":"WebSearch","arguments":"{\"query\":\"cats\"}"}}]}}]}`)
	out := InjectOpenAIToolOutputs(body, []ExecResult{{
		CallID:       "call_1",
		OriginalName: "WebSearch",
		Kind:         "search",
		OK:           true,
		Query:        "cats",
	}})
	var parsed map[string]any
	require.NoError(t, common.Unmarshal(out, &parsed))
	choices := parsed["choices"].([]any)
	msg := choices[0].(map[string]any)["message"].(map[string]any)
	calls := msg["tool_calls"].([]any)
	fn := calls[0].(map[string]any)["function"].(map[string]any)
	output, _ := fn["output"].(string)
	assert.Contains(t, output, `"content"`)
	assert.Contains(t, output, `"isError"`)
}

func TestCallInputJSONPrefersRawInput(t *testing.T) {
	assert.Equal(t, `{"query":"raw"}`, CallInputJSON(ExecResult{
		Kind:  "search",
		Query: "ignored",
		Input: []byte(`{"query":"raw"}`),
	}))
}
