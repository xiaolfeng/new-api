package hosttool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSearchHitsExaJSON(t *testing.T) {
	text := `{
  "search_id": "search_d0825f8132c785195e0ea48f6dc2a226",
  "results": [
    {"url": "https://github.com/XiaoLFeng", "title": "XiaoLFeng - GitHub", "excerpts": ["I'm XiaoLFeng"]},
    {"url": "https://github.com/XiaoLFeng/XiaoLFeng", "title": "GitHub profile", "excerpts": ["Config files"]},
    {"url": "https://blog.x-lf.com/1003.html", "title": "301跳转", "excerpts": ["更换域名"]},
    {"url": "https://x.com/lfeng_xiao", "title": "筱锋xiao_lfeng", "publish_date": null},
    {"url": "https://space.bilibili.com/244321572", "title": "Bilibili", "excerpts": ["软件工程"]}
  ]
}`
	hits := parseSearchHits(text)
	require.Len(t, hits, 5)
	assert.Equal(t, "https://github.com/XiaoLFeng", hits[0].URL)
	assert.Equal(t, "XiaoLFeng - GitHub", hits[0].Title)
	assert.Equal(t, "I'm XiaoLFeng", hits[0].Snippet)
	assert.Equal(t, "https://space.bilibili.com/244321572", hits[4].URL)
}

func TestParseSearchHitsLinkAliasAndFence(t *testing.T) {
	text := "```json\n{\"results\":[{\"link\":\"https://example.com\",\"name\":\"Example\",\"content\":\"hi\"}]}\n```"
	hits := parseSearchHits(text)
	require.Len(t, hits, 1)
	assert.Equal(t, "https://example.com", hits[0].URL)
	assert.Equal(t, "Example", hits[0].Title)
	assert.Equal(t, "hi", hits[0].Snippet)
}

func TestResolvedHitsFallsBackToOpaqueText(t *testing.T) {
	hits := resolvedHits(ExecResult{
		OK:         true,
		OpaqueText: `{"results":[{"url":"https://a.example","title":"A"},{"url":"https://b.example","title":"B"}]}`,
	})
	require.Len(t, hits, 2)
	assert.Equal(t, "https://b.example", hits[1].URL)
}

func TestLimitHits(t *testing.T) {
	hits := []SearchHit{{URL: "https://a"}, {URL: "https://b"}, {URL: "https://c"}}
	assert.Len(t, limitHits(hits, 2), 2)
	assert.Len(t, limitHits(hits, 0), 3)
}
