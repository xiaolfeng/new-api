package hosttool

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCanonicalFromName(t *testing.T) {
	assert.Equal(t, CanonicalWebSearch, CanonicalFromName("WebSearch"))
	assert.Equal(t, CanonicalWebSearch, CanonicalFromName("websearch"))
	assert.Equal(t, CanonicalWebSearch, CanonicalFromName("web_search_preview"))
	assert.Equal(t, CanonicalWebSearch, CanonicalFromName("googleSearch"))
	assert.Equal(t, CanonicalWebSearch, CanonicalFromName("google_search"))
	assert.Equal(t, CanonicalWebSearch, CanonicalFromName("google-search"))
	assert.Equal(t, CanonicalWebSearch, CanonicalFromName("googleSearchRetrieval"))
	assert.Equal(t, CanonicalWebSearch, CanonicalFromName("google_search_retrieval"))
	assert.Equal(t, CanonicalWebSearch, CanonicalFromName("google-search-retrieval"))
	assert.Equal(t, CanonicalWebFetch, CanonicalFromName("WebFetch"))
	assert.Equal(t, CanonicalWebFetch, CanonicalFromName("webfetch"))
	assert.Equal(t, CanonicalWebFetch, CanonicalFromName("open_page"))
	assert.Equal(t, CanonicalWebFetch, CanonicalFromName("open_page_with_find"))
	assert.Equal(t, "", CanonicalFromName("bash"))
	assert.Equal(t, "", CanonicalFromName("apply_patch"))
	assert.Equal(t, "", CanonicalFromName("urlContext"))
	assert.Equal(t, "", CanonicalFromName("codeExecution"))
}

func TestCanonicalFromType(t *testing.T) {
	assert.Equal(t, CanonicalWebSearch, CanonicalFromType("web_search_20250305"))
	assert.Equal(t, CanonicalWebSearch, CanonicalFromType("web_search_preview"))
	assert.Equal(t, CanonicalWebSearch, CanonicalFromType("google_search"))
	assert.Equal(t, CanonicalWebSearch, CanonicalFromType("googleSearch"))
	assert.Equal(t, CanonicalWebSearch, CanonicalFromType("googlesearchretrieval"))
	assert.Equal(t, CanonicalWebFetch, CanonicalFromType("web_fetch_20250910"))
}
