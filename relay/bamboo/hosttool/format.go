package hosttool

import (
	"fmt"
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

const (
	ErrSSRFBlocked     = "ssrf_blocked"
	ErrTimeout         = "timeout"
	ErrBackendDisabled = "backend_disabled"
	ErrUpstream4xx     = "upstream_4xx"
	ErrTooManyCalls    = "too_many_calls"
	ErrInvalidInput    = "invalid_input"
	ErrCanceled        = "canceled"
)

type SearchHit struct {
	Title   string
	URL     string
	Snippet string
}

type ExecResult struct {
	OriginalName string
	Kind         string // search | fetch
	OK           bool
	ErrorCode    string
	Query        string
	URL          string
	Hits         []SearchHit
	OpaqueText   string
	Body         string
	Prompt       string
	Pattern      string
	Backend      string
	DurationMs   int64
	Truncated    bool
}

func FormatToolResult(r ExecResult) (content string, isError bool) {
	return FormatToolResultWithLimit(r, model_setting.GetBambooSettings().ClampMaxResultRunes())
}

func FormatToolResultWithLimit(r ExecResult, maxRunes int) (content string, isError bool) {
	name := r.OriginalName
	if name == "" {
		name = r.Kind
	}
	if !r.OK {
		code := r.ErrorCode
		if code == "" {
			code = ErrInvalidInput
		}
		content = fmt.Sprintf("[%s] error=%s", name, code)
		content, r.Truncated = truncateRunes(content, maxRunes)
		return content, true
	}
	switch r.Kind {
	case "fetch":
		var b strings.Builder
		fmt.Fprintf(&b, "[%s] url=%q\n", name, r.URL)
		if strings.TrimSpace(r.Prompt) != "" {
			focus := strings.ReplaceAll(strings.TrimSpace(r.Prompt), "\n", " ")
			fmt.Fprintf(&b, "requested_focus: %s\n", focus)
		}
		b.WriteString(r.Body)
		content, r.Truncated = truncateRunes(b.String(), maxRunes)
		return content, false
	default:
		var b strings.Builder
		fmt.Fprintf(&b, "[%s] query=%q\n", name, r.Query)
		if len(r.Hits) > 0 {
			for i, hit := range r.Hits {
				title := hit.Title
				if title == "" {
					title = "-"
				}
				fmt.Fprintf(&b, "%d. %s\n   %s\n   %s\n", i+1, title, hit.URL, hit.Snippet)
			}
		} else if strings.TrimSpace(r.OpaqueText) != "" {
			b.WriteString(r.OpaqueText)
			if !strings.HasSuffix(r.OpaqueText, "\n") {
				b.WriteByte('\n')
			}
		} else {
			b.WriteString("0 results\n")
		}
		content, r.Truncated = truncateRunes(b.String(), maxRunes)
		return content, false
	}
}

func VisibleFence(r ExecResult) string {
	limit := relaycommon.HostToolVisibleSearchRunes
	start, end := relaycommon.HostWebSearchFenceStart, relaycommon.HostWebSearchFenceEnd
	if r.Kind == "fetch" {
		limit = relaycommon.HostToolVisibleFetchRunes
		start, end = relaycommon.HostWebFetchFenceStart, relaycommon.HostWebFetchFenceEnd
	}
	body, _ := FormatToolResultWithLimit(r, limit)
	if strings.TrimSpace(body) == "" {
		return ""
	}
	return start + "\n" + body + "\n" + end + "\n"
}

func JoinFormattedResults(results []ExecResult) string {
	parts := make([]string, 0, len(results))
	for _, r := range results {
		content, _ := FormatToolResult(r)
		parts = append(parts, content)
	}
	return strings.Join(parts, "\n\n")
}
