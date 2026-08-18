package hosttool

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVisibleFenceSearchAndFetch(t *testing.T) {
	search := VisibleFence(ExecResult{
		OriginalName: "WebSearch",
		Kind:         "search",
		OK:           true,
		Query:        "cats",
		Hits:         []SearchHit{{Title: "Cat", URL: "https://c", Snippet: "meow"}},
	})
	require.Contains(t, search, "<<<host_web_search>>>")
	require.Contains(t, search, "<<<end_host_web_search>>>")
	assert.Contains(t, search, `[WebSearch] query="cats"`)

	fetch := VisibleFence(ExecResult{
		OriginalName: "WebFetch",
		Kind:         "fetch",
		OK:           true,
		URL:          "https://example.com",
		Body:         strings.Repeat("x", 5000),
	})
	require.Contains(t, fetch, "<<<host_web_fetch>>>")
	assert.Less(t, len([]rune(fetch)), 5000)
}
