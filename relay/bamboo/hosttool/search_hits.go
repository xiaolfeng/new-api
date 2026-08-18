package hosttool

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// resolvedHits prefers structured Hits; if a backend only returned OpaqueText
// (Exa / Parallel MCP JSON), parse that so Claude/Responses helpers still see
// every URL instead of an empty web_search_tool_result.
func resolvedHits(result ExecResult) []SearchHit {
	if len(result.Hits) > 0 {
		return result.Hits
	}
	return parseSearchHits(result.OpaqueText)
}

func limitHits(hits []SearchHit, num int) []SearchHit {
	if num > 0 && len(hits) > num {
		return hits[:num]
	}
	return hits
}

func parseSearchHits(text string) []SearchHit {
	text = unwrapSearchJSON(text)
	if text == "" {
		return nil
	}
	var root any
	if err := common.Unmarshal([]byte(text), &root); err != nil {
		return nil
	}
	return hitsFromValue(root)
}

func unwrapSearchJSON(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```JSON")
		text = strings.TrimPrefix(text, "```")
		if i := strings.LastIndex(text, "```"); i >= 0 {
			text = text[:i]
		}
		text = strings.TrimSpace(text)
	}
	if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
		return text
	}
	if i := strings.Index(text, "{"); i >= 0 {
		return strings.TrimSpace(text[i:])
	}
	if i := strings.Index(text, "["); i >= 0 {
		return strings.TrimSpace(text[i:])
	}
	return text
}

func hitsFromValue(v any) []SearchHit {
	switch typed := v.(type) {
	case []any:
		return hitsFromSlice(typed)
	case map[string]any:
		for _, key := range []string{"results", "data", "items", "hits", "organic"} {
			if sl, ok := typed[key].([]any); ok {
				return hitsFromSlice(sl)
			}
		}
		if hit, ok := hitFromMap(typed); ok {
			return []SearchHit{hit}
		}
	}
	return nil
}

func hitsFromSlice(items []any) []SearchHit {
	out := make([]SearchHit, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		hit, ok := hitFromMap(m)
		if !ok {
			continue
		}
		out = append(out, hit)
	}
	return out
}

func hitFromMap(m map[string]any) (SearchHit, bool) {
	if m == nil {
		return SearchHit{}, false
	}
	url := firstNonEmpty(
		asString(m["url"]),
		asString(m["link"]),
		asString(m["href"]),
	)
	if strings.TrimSpace(url) == "" {
		return SearchHit{}, false
	}
	return SearchHit{
		Title:   firstNonEmpty(asString(m["title"]), asString(m["name"])),
		URL:     url,
		Snippet: firstNonEmpty(asString(m["snippet"]), asString(m["content"]), asString(m["text"]), joinExcerpts(m["excerpts"])),
	}, true
}

func joinExcerpts(v any) string {
	switch typed := v.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if s := strings.TrimSpace(asString(item)); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, " ")
	case []string:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if s := strings.TrimSpace(item); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, " ")
	default:
		return ""
	}
}
