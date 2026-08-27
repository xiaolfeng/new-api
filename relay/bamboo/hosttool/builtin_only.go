package hosttool

import (
	"context"
	"net/url"
	"strings"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

// IsResponsesBuiltinOnly reports Responses helper requests that declare only
// built-in search/fetch tools. Those must short-circuit to a synthesized
// web_search_call instead of A-thin hop2.
func IsResponsesBuiltinOnly(entryFormat types.RelayFormat, entryBytes []byte, plan *relaycommon.HostToolPlan, req *bamboocodec.RelayRequest) bool {
	if entryFormat != types.RelayFormatOpenAIResponses {
		return false
	}
	if plan == nil || !plan.Enabled || len(plan.Decls) == 0 {
		return false
	}
	if req != nil && req.Config != nil {
		for _, tool := range req.Config.Tools {
			if !IsHostToolName(tool.Name) {
				return false
			}
		}
	}
	return entryToolsAreHostOnly(entryBytes)
}

func entryToolsAreHostOnly(entryBytes []byte) bool {
	if len(entryBytes) == 0 {
		return false
	}
	var root map[string]any
	if err := common.Unmarshal(entryBytes, &root); err != nil {
		return false
	}
	tools := asMapSlice(root["tools"])
	if len(tools) == 0 {
		return false
	}
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		typ, _ := tool["type"].(string)
		name := firstNonEmpty(asString(tool["name"]))
		if fn, ok := tool["function"].(map[string]any); ok {
			if n := asString(fn["name"]); n != "" {
				name = n
			}
		}
		if IsHostToolName(name) || IsHostToolType(typ) {
			continue
		}
		return false
	}
	return true
}

type builtinInput struct {
	Kind  string // search | fetch
	Query string
	URL   string
	Name  string
}

func ExecuteBuiltinRequest(ctx context.Context, info *relaycommon.RelayInfo, st *model_setting.BambooSettings, req *bamboocodec.RelayRequest) ExecResult {
	in := classifyBuiltinInput(planOrEmpty(info), req)
	use := builtinUseFromInput(in)
	results := ExecuteCalls(ctx, info, st, []toolUseCall{use})
	if len(results) == 0 {
		return ExecResult{Kind: in.Kind, OriginalName: in.Name, ErrorCode: ErrInvalidInput}
	}
	return results[0]
}

func planOrEmpty(info *relaycommon.RelayInfo) *relaycommon.HostToolPlan {
	if info == nil {
		return nil
	}
	return info.HostToolPlan
}

func classifyBuiltinInput(plan *relaycommon.HostToolPlan, req *bamboocodec.RelayRequest) builtinInput {
	text := stripServerHelperSearchPrefix(lastUserText(req))
	searchName := builtinToolName(plan, CanonicalWebSearch)
	fetchName := builtinToolName(plan, CanonicalWebFetch)
	onlyFetch := fetchName != "" && searchName == ""

	if isBareHTTPURL(text) || onlyFetch {
		if fetchName == "" {
			fetchName = "open_page"
		}
		url := text
		if !isBareHTTPURL(url) {
			url = extractURLFromText(text)
		}
		return builtinInput{Kind: "fetch", URL: url, Name: fetchName}
	}
	if searchName == "" {
		searchName = "web_search"
	}
	return builtinInput{Kind: "search", Query: text, Name: searchName}
}

// serverHelperSearchPrefix 是客户端搜索 helper 在 user 文本里附加的固定指令前缀，
// 提取真实查询时需要剥掉。
const serverHelperSearchPrefix = "perform a web search for the query:"

func stripServerHelperSearchPrefix(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	if strings.HasPrefix(strings.ToLower(s), serverHelperSearchPrefix) {
		return strings.TrimSpace(s[len(serverHelperSearchPrefix):])
	}
	return s
}

func extractURLFromText(s string) string {
	s = strings.TrimSpace(s)
	if isBareHTTPURL(s) {
		return s
	}
	for _, prefix := range []string{"https://", "http://"} {
		i := strings.Index(s, prefix)
		if i < 0 {
			continue
		}
		rest := s[i:]
		end := len(rest)
		for j, r := range rest {
			if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '"' || r == '\'' || r == '>' || r == ')' {
				end = j
				break
			}
		}
		cand := strings.TrimRight(rest[:end], ".,;]")
		if isBareHTTPURL(cand) {
			return cand
		}
	}
	return ""
}

func builtinToolName(plan *relaycommon.HostToolPlan, canonical string) string {
	if plan == nil {
		return ""
	}
	if d := plan.DeclByCanonical(canonical); d != nil {
		return d.OriginalName
	}
	return ""
}

func lastUserText(req *bamboocodec.RelayRequest) string {
	if req == nil {
		return ""
	}
	for i := len(req.Messages) - 1; i >= 0; i-- {
		msg := req.Messages[i]
		if msg.Role != bamboosdk.RoleUser {
			continue
		}
		var parts []string
		for _, block := range msg.Content {
			text, ok := block.(*bamboosdk.TextBlock)
			if !ok || text == nil {
				continue
			}
			if s := strings.TrimSpace(text.Text); s != "" {
				parts = append(parts, s)
			}
		}
		if len(parts) > 0 {
			return strings.TrimSpace(strings.Join(parts, " "))
		}
	}
	return ""
}

func isBareHTTPURL(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, " \t\n\r") {
		return false
	}
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return false
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" || u.Scheme == "" {
		return false
	}
	return true
}

func builtinUseFromInput(in builtinInput) toolUseCall {
	if in.Kind == "fetch" {
		raw, _ := common.Marshal(map[string]any{"url": in.URL})
		return toolUseCall{ID: "host_builtin_fetch", Name: in.Name, Input: raw}
	}
	raw, _ := common.Marshal(map[string]any{"query": in.Query})
	return toolUseCall{ID: "host_builtin_search", Name: in.Name, Input: raw}
}
