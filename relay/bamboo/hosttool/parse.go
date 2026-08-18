package hosttool

import (
	"bytes"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// SeedToolInput drops empty / placeholder tool-use input so stream
// content_block_start does not prepend "{}" before input_json_delta.
func SeedToolInput(input []byte) []byte {
	s := bytes.TrimSpace(input)
	if len(s) == 0 || bytes.Equal(s, []byte("null")) || bytes.Equal(s, []byte("{}")) {
		return nil
	}
	return s
}

func parseToolInput(input []byte) (raw map[string]any, scalar string) {
	input = bytes.TrimSpace(input)
	if len(input) == 0 || bytes.Equal(input, []byte("null")) {
		return map[string]any{}, ""
	}

	var obj map[string]any
	if err := common.Unmarshal(input, &obj); err == nil && obj != nil {
		return obj, ""
	}

	var s string
	if err := common.Unmarshal(input, &s); err == nil {
		return map[string]any{}, strings.TrimSpace(s)
	}

	if last := lastJSONObject(input); len(last) > 0 {
		if err := common.Unmarshal(last, &obj); err == nil && obj != nil {
			return obj, ""
		}
	}
	return map[string]any{}, ""
}

func lastJSONObject(input []byte) []byte {
	idx := bytes.LastIndexByte(input, '{')
	if idx < 0 {
		return nil
	}
	return bytes.TrimSpace(input[idx:])
}

func extractSearchQuery(raw map[string]any, scalar string) string {
	if q := firstNonEmpty(
		anyToQueryString(raw["query"]),
		anyToQueryString(raw["objective"]),
		anyToQueryString(raw["q"]),
		anyToQueryString(raw["search_query"]),
		anyToQueryString(raw["text"]),
		anyToQueryString(raw["search"]),
		anyToQueryString(raw["keyword"]),
		anyToQueryString(raw["prompt"]),
		anyToQueryString(raw["input"]),
		anyToQueryString(raw["queries"]),
		anyToQueryString(raw["keywords"]),
	); q != "" {
		return q
	}
	return scalar
}

func extractFetchURL(raw map[string]any, scalar string) string {
	if u := firstNonEmpty(anyToQueryString(raw["url"]), anyToQueryString(raw["uri"])); u != "" {
		return u
	}
	if strings.HasPrefix(scalar, "http://") || strings.HasPrefix(scalar, "https://") {
		return scalar
	}
	return ""
}

func anyToQueryString(v any) string {
	switch n := v.(type) {
	case string:
		return strings.TrimSpace(n)
	case []any:
		for _, item := range n {
			if s := anyToQueryString(item); s != "" {
				return s
			}
		}
	case []string:
		for _, item := range n {
			if s := strings.TrimSpace(item); s != "" {
				return s
			}
		}
	}
	return ""
}
