package hosttool

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatToolResultSearchHits(t *testing.T) {
	content, isErr := FormatToolResultWithLimit(ExecResult{
		OriginalName: "WebSearch",
		Kind:         "search",
		OK:           true,
		Query:        "tokyo weather",
		Hits: []SearchHit{
			{Title: "Weather", URL: "https://example.com", Snippet: "22C"},
		},
	}, 65536)
	require.False(t, isErr)
	assert.Contains(t, content, `[WebSearch] query="tokyo weather"`)
	assert.Contains(t, content, "1. Weather")
	assert.Contains(t, content, "https://example.com")
}

func TestFormatToolResultSearchOpaque(t *testing.T) {
	content, isErr := FormatToolResultWithLimit(ExecResult{
		OriginalName: "websearch",
		Kind:         "search",
		OK:           true,
		Query:        "q",
		OpaqueText:   "raw mcp text",
	}, 65536)
	require.False(t, isErr)
	assert.Contains(t, content, "raw mcp text")
	assert.NotContains(t, content, "{")
}

func TestFormatToolResultFetchPrompt(t *testing.T) {
	content, isErr := FormatToolResultWithLimit(ExecResult{
		OriginalName: "WebFetch",
		Kind:         "fetch",
		OK:           true,
		URL:          "https://example.com",
		Prompt:       "summarize",
		Body:         "# Hello",
	}, 65536)
	require.False(t, isErr)
	assert.Contains(t, content, `url="https://example.com"`)
	assert.Contains(t, content, "requested_focus: summarize")
	assert.Contains(t, content, "# Hello")
}

func TestFormatToolResultErrorClosedSet(t *testing.T) {
	content, isErr := FormatToolResultWithLimit(ExecResult{
		OriginalName: "WebSearch",
		Kind:         "search",
		ErrorCode:    ErrBackendDisabled,
	}, 65536)
	require.True(t, isErr)
	assert.Equal(t, "[WebSearch] error=backend_disabled", content)
}

func TestFormatToolResultTruncates(t *testing.T) {
	content, isErr := FormatToolResultWithLimit(ExecResult{
		OriginalName: "webfetch",
		Kind:         "fetch",
		OK:           true,
		URL:          "https://example.com",
		Body:         strings.Repeat("x", 200),
	}, 40)
	require.False(t, isErr)
	assert.Contains(t, content, "...[truncated]")
	assert.LessOrEqual(t, len([]rune(content)), 60)
}
