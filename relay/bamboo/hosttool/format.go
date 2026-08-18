package hosttool

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
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
	CallID       string
	Input        []byte
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
		if hits := resolvedHits(r); len(hits) > 0 {
			for i, hit := range hits {
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

func visibleResultLimit(r ExecResult) int {
	if r.Kind == "fetch" {
		return relaycommon.HostToolVisibleFetchRunes
	}
	return relaycommon.HostToolVisibleSearchRunes
}

func CallInputJSON(r ExecResult) string {
	if len(r.Input) > 0 {
		return string(r.Input)
	}
	switch r.Kind {
	case "fetch":
		payload, err := common.Marshal(map[string]string{"url": r.URL})
		if err != nil {
			return "{}"
		}
		return string(payload)
	default:
		payload, err := common.Marshal(map[string]string{"query": r.Query})
		if err != nil {
			return "{}"
		}
		return string(payload)
	}
}

func InjectOpenAIToolOutputs(body []byte, results []ExecResult) []byte {
	if len(body) == 0 || len(results) == 0 {
		return body
	}
	var payload map[string]any
	if err := common.Unmarshal(body, &payload); err != nil {
		return body
	}
	choices, _ := payload["choices"].([]any)
	if len(choices) == 0 {
		return body
	}
	choice, _ := choices[0].(map[string]any)
	if choice == nil {
		return body
	}
	msg, _ := choice["message"].(map[string]any)
	if msg == nil {
		return body
	}
	calls, _ := msg["tool_calls"].([]any)
	if len(calls) == 0 {
		return body
	}
	byID := make(map[string]ExecResult, len(results))
	for _, r := range results {
		if r.CallID != "" {
			byID[r.CallID] = r
		}
	}
	for i, raw := range calls {
		call, _ := raw.(map[string]any)
		if call == nil {
			continue
		}
		id, _ := call["id"].(string)
		r, ok := byID[id]
		if !ok && i < len(results) {
			r = results[i]
		} else if !ok {
			continue
		}
		fn, _ := call["function"].(map[string]any)
		if fn == nil {
			fn = map[string]any{}
			call["function"] = fn
		}
		fn["output"] = MCPResultJSON(r)
		calls[i] = call
	}
	msg["tool_calls"] = calls
	choice["message"] = msg
	choices[0] = choice
	payload["choices"] = choices
	out, err := common.Marshal(payload)
	if err != nil {
		return body
	}
	return out
}

func MCPResultJSON(r ExecResult) string {
	text, isErr := FormatToolResultWithLimit(r, visibleResultLimit(r))
	payload, err := common.Marshal(map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": text},
		},
		"isError": isErr,
	})
	if err != nil {
		return `{"content":[{"type":"text","text":""}],"isError":true}`
	}
	return string(payload)
}

func JoinFormattedResults(results []ExecResult) string {
	parts := make([]string, 0, len(results))
	for _, r := range results {
		content, _ := FormatToolResult(r)
		parts = append(parts, content)
	}
	return strings.Join(parts, "\n\n")
}
