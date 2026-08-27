package hosttool

import (
	"strings"
)

const (
	CanonicalWebSearch = "host.web_search"
	CanonicalWebFetch  = "host.web_fetch"

	ModeLoop   = "loop"
	ModeReturn = "return"

	sourceFunction         = "function"
	sourceServerType       = "server_type"
	sourceWebSearchOptions = "web_search_options"

	billingWebSearch        = "web_search"
	billingWebSearchPreview = "web_search_preview"
	billingWebFetch         = "web_fetch"
)

func normalizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)
	return name
}

func CanonicalFromName(name string) string {
	switch normalizeName(name) {
	case "websearch", "web_search", "web-search", "web_search_preview", "websearchpreview",
		"googlesearch", "google_search", "google-search",
		"googlesearchretrieval", "google_search_retrieval", "google-search-retrieval":
		return CanonicalWebSearch
	case "webfetch", "web_fetch", "web-fetch", "open_page", "openpage", "open-page",
		"open_page_with_find", "openpagewithfind", "open-page-with-find":
		return CanonicalWebFetch
	default:
		return ""
	}
}

func CanonicalFromType(typ string) string {
	t := normalizeName(typ)
	if t == "" {
		return ""
	}
	if strings.HasPrefix(t, "web_search") || strings.HasPrefix(t, "google_search") || strings.HasPrefix(t, "googlesearch") {
		return CanonicalWebSearch
	}
	if strings.HasPrefix(t, "web_fetch") {
		return CanonicalWebFetch
	}
	return CanonicalFromName(t)
}

func IsHostToolName(name string) bool {
	return CanonicalFromName(name) != ""
}

func IsHostToolType(typ string) bool {
	return CanonicalFromType(typ) != ""
}

func billingNameFor(original, typ, canonical string) string {
	n := normalizeName(original)
	t := normalizeName(typ)
	if n == "web_search_preview" || t == "web_search_preview" {
		return billingWebSearchPreview
	}
	if canonical == CanonicalWebFetch {
		return billingWebFetch
	}
	return billingWebSearch
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func parseMaxUses(v any) int {
	switch n := v.(type) {
	case float64:
		if n <= 0 {
			return 0
		}
		return int(n)
	case int:
		if n <= 0 {
			return 0
		}
		return n
	case int64:
		if n <= 0 {
			return 0
		}
		return int(n)
	case jsonNumber:
		i, err := n.Int64()
		if err != nil || i <= 0 {
			return 0
		}
		return int(i)
	default:
		return 0
	}
}

// jsonNumber matches encoding/json.Number without importing encoding/json for calls.
type jsonNumber interface {
	Int64() (int64, error)
}

func clampInt(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

func truncateRunes(s string, max int) (string, bool) {
	if max <= 0 || s == "" {
		return s, false
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s, false
	}
	cut := runes[:max]
	// 回退到最后一个换行，避免切断一行中间。
	for i := len(cut) - 1; i >= 0; i-- {
		if cut[i] == '\n' {
			cut = cut[:i]
			break
		}
	}
	return string(cut) + "\n...[truncated]", true
}
