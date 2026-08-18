package hosttool

import (
	"context"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

type toolUseCall struct {
	ID    string
	Name  string
	Input []byte
}

func CollectToolUses(blocks []bamboosdk.ContentBlock) []toolUseCall {
	var out []toolUseCall
	for _, block := range blocks {
		if tu, ok := block.(*bamboosdk.ToolUseBlock); ok && tu != nil {
			out = append(out, toolUseCall{ID: tu.ID, Name: tu.Name, Input: tu.Input})
		}
	}
	return out
}

func AllHostToolUses(plan *relaycommon.HostToolPlan, uses []toolUseCall) bool {
	if len(uses) == 0 {
		return false
	}
	for _, u := range uses {
		if CanonicalFromName(u.Name) == "" {
			return false
		}
		if plan != nil && plan.DeclByCanonical(CanonicalFromName(u.Name)) == nil && plan.DeclByOriginal(u.Name) == nil {
			// 名称命中别名即可，即使 plan 里只有另一 original name。
			continue
		}
	}
	return true
}

func ExecuteCalls(ctx context.Context, info *relaycommon.RelayInfo, st *model_setting.BambooSettings, uses []toolUseCall) []ExecResult {
	results := make([]ExecResult, len(uses))
	searchCap := 3
	fetchCap := 5
	if st != nil {
		var searchMax, fetchMax int
		if info != nil && info.HostToolPlan != nil {
			for _, d := range info.HostToolPlan.Decls {
				if d.Canonical == CanonicalWebSearch {
					if d.MaxUses > 0 {
						searchMax += d.MaxUses
					} else {
						searchMax += 3
					}
				}
				if d.Canonical == CanonicalWebFetch {
					if d.MaxUses > 0 {
						fetchMax += d.MaxUses
					} else {
						fetchMax += 5
					}
				}
			}
		}
		if searchMax > 0 {
			searchCap = min(searchMax, 3)
		}
		if fetchMax > 0 {
			fetchCap = min(fetchMax, 5)
		}
	}

	type slot struct {
		idx int
		res ExecResult
	}
	ch := make(chan slot, len(uses))
	var wg sync.WaitGroup
	var searchN, fetchN int
	var mu sync.Mutex

	for i, call := range uses {
		can := CanonicalFromName(call.Name)
		mu.Lock()
		over := false
		if can == CanonicalWebSearch {
			searchN++
			over = searchN > searchCap
		} else if can == CanonicalWebFetch {
			fetchN++
			over = fetchN > fetchCap
		}
		mu.Unlock()
		if over {
			results[i] = ExecResult{OriginalName: call.Name, CallID: call.ID, Input: call.Input, Kind: kindOf(can), OK: false, ErrorCode: ErrTooManyCalls}
			continue
		}
		wg.Add(1)
		go func(idx int, call toolUseCall, can string) {
			defer wg.Done()
			start := time.Now()
			res := executeOne(ctx, info, st, call, can)
			res.DurationMs = time.Since(start).Milliseconds()
			res.OriginalName = call.Name
			res.CallID = call.ID
			res.Input = call.Input
			ch <- slot{idx: idx, res: res}
		}(i, call, can)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
	for s := range ch {
		results[s.idx] = s.res
	}

	if info != nil {
		info.HostToolExecuted = true
		if info.HostToolPlan == nil {
			info.HostToolPlan = &relaycommon.HostToolPlan{}
		}
		for i, r := range results {
			if results[i].OriginalName == "" && i < len(uses) {
				results[i].OriginalName = uses[i].Name
			}
			info.HostToolPlan.Execs = append(info.HostToolPlan.Execs, relaycommon.HostToolExecRecord{
				OriginalName: results[i].OriginalName,
				Canonical:    CanonicalFromName(results[i].OriginalName),
				Backend:      r.Backend,
				DurationMs:   r.DurationMs,
				ErrorCode:    r.ErrorCode,
				Truncated:    r.Truncated,
			})
			if shouldBillHostResult(results[i]) {
				incrementHostToolBilling(info, uses[i].Name)
			}
		}
		recordHostToolLogs(info, results)
	}
	return results
}

func recordHostToolLogs(info *relaycommon.RelayInfo, results []ExecResult) {
	if info == nil || len(results) == 0 {
		return
	}
	username := ""
	if info.UserId > 0 {
		username, _ = model.GetUsernameById(info.UserId, false)
	}
	tokenName := ""
	if info.TokenId > 0 {
		if token, err := model.GetTokenById(info.TokenId); err == nil && token != nil {
			tokenName = token.Name
		}
	}
	mode := ""
	if info.HostToolPlan != nil {
		mode = info.HostToolPlan.Mode
	}
	logs := make([]*model.ToolLog, 0, len(results))
	for _, result := range results {
		content, _ := FormatToolResult(result)
		truncated := result.Truncated || strings.Contains(content, "...[truncated]")
		logs = append(logs, &model.ToolLog{
			UserId:       info.UserId,
			Username:     username,
			TokenId:      info.TokenId,
			TokenName:    tokenName,
			ChannelId:    info.ChannelId,
			Group:        info.UsingGroup,
			ModelName:    info.OriginModelName,
			RequestId:    info.RequestId,
			Ip:           clientIPFromHeaders(info.RequestHeaders),
			OriginalName: result.OriginalName,
			Canonical:    CanonicalFromName(result.OriginalName),
			Kind:         result.Kind,
			Mode:         mode,
			Backend:      result.Backend,
			Query:        result.Query,
			URL:          result.URL,
			ErrorCode:    result.ErrorCode,
			DurationMs:   result.DurationMs,
			Truncated:    truncated,
			Result:       content,
		})
	}
	model.RecordToolLogs(logs)
}

func clientIPFromHeaders(headers map[string]string) string {
	if headers == nil {
		return ""
	}
	for _, key := range []string{"X-Real-Ip", "X-Real-IP", "X-Forwarded-For"} {
		value := strings.TrimSpace(headers[key])
		if value == "" {
			continue
		}
		if i := strings.IndexByte(value, ','); i >= 0 {
			return strings.TrimSpace(value[:i])
		}
		return value
	}
	return ""
}

func executeOne(ctx context.Context, info *relaycommon.RelayInfo, st *model_setting.BambooSettings, call toolUseCall, can string) ExecResult {
	if ctx.Err() != nil {
		return ExecResult{Kind: kindOf(can), ErrorCode: ErrCanceled}
	}
	raw, scalar := parseToolInput(call.Input)
	if can == CanonicalWebFetch {
		urlStr := extractFetchURL(raw, scalar)
		if urlStr == "" {
			return ExecResult{Kind: "fetch", ErrorCode: ErrInvalidInput}
		}
		timeout := time.Duration(st.ClampTimeout()) * time.Millisecond
		if v, ok := asFloat(raw["timeout"]); ok && v > 0 {
			sec := v
			maxSec := float64(st.ClampTimeout()) / 1000
			if sec > 120 {
				sec = 120
			}
			if sec > maxSec {
				sec = maxSec
			}
			timeout = time.Duration(sec * float64(time.Second))
		}
		return runFetch(ctx, st, fetchRequest{
			URL:          urlStr,
			Format:       asString(raw["format"]),
			Timeout:      timeout,
			Prompt:       firstNonEmpty(asString(raw["prompt"]), asString(raw["requested_focus"])),
			Pattern:      firstNonEmpty(asString(raw["pattern"]), asString(raw["find"])),
			StartLine:    firstPositiveInt(raw["start_line"], raw["startLine"]),
			MaxMatches:   firstPositiveInt(raw["max_matches"], raw["maxMatches"]),
			ContextLines: firstPositiveInt(raw["context_lines"], raw["contextLines"]),
		})
	}

	query := extractSearchQuery(raw, scalar)
	if query == "" {
		return ExecResult{Kind: "search", ErrorCode: ErrInvalidInput}
	}
	if utf8.RuneCountInString(query) > 512 {
		query = string([]rune(query)[:512])
	}
	num := st.ClampMaxSearchResults()
	if v, ok := asFloat(raw["numResults"]); ok {
		num = clampInt(int(v), 1, st.ClampMaxSearchResults())
	} else if v, ok := asFloat(raw["num_results"]); ok {
		num = clampInt(int(v), 1, st.ClampMaxSearchResults())
	}
	ctxChars := 0
	if v, ok := asFloat(raw["contextMaxCharacters"]); ok {
		ctxChars = int(v)
	}
	allowed, blocked := resolveSearchDomains(info, call.Name, raw)
	res, _ := runSearch(ctx, info, st, searchRequest{
		Query:                query,
		NumResults:           num,
		Livecrawl:            asString(raw["livecrawl"]),
		Type:                 asString(raw["type"]),
		ContextMaxCharacters: ctxChars,
		AllowedDomains:       allowed,
		BlockedDomains:       blocked,
	})
	return res
}

func resolveSearchDomains(info *relaycommon.RelayInfo, originalName string, raw map[string]any) (allowed, blocked []string) {
	if info != nil && info.HostToolPlan != nil {
		d := info.HostToolPlan.DeclByOriginal(originalName)
		if d == nil {
			d = info.HostToolPlan.DeclByCanonical(CanonicalFromName(originalName))
		}
		if d != nil && (len(d.AllowedDomains) > 0 || len(d.BlockedDomains) > 0) {
			return d.AllowedDomains, d.BlockedDomains
		}
	}
	return asStringSlice(raw["allowed_domains"]), asStringSlice(raw["blocked_domains"])
}

func shouldBillHostResult(r ExecResult) bool {
	return r.OK && r.Kind == "search"
}

func incrementHostToolBilling(info *relaycommon.RelayInfo, originalName string) {
	name := billingWebSearch
	if info.HostToolPlan != nil {
		if d := info.HostToolPlan.DeclByOriginal(originalName); d != nil && d.BillingName != "" {
			name = d.BillingName
		} else if d := info.HostToolPlan.DeclByCanonical(CanonicalFromName(originalName)); d != nil && d.BillingName != "" {
			name = d.BillingName
		} else if CanonicalFromName(originalName) == CanonicalWebFetch {
			name = billingWebFetch
		}
	} else if CanonicalFromName(originalName) == CanonicalWebFetch {
		name = billingWebFetch
	}
	if info.ResponsesUsageInfo == nil {
		info.ResponsesUsageInfo = &relaycommon.ResponsesUsageInfo{
			BuiltInTools: map[string]*relaycommon.BuildInToolInfo{},
		}
	}
	if info.ResponsesUsageInfo.BuiltInTools == nil {
		info.ResponsesUsageInfo.BuiltInTools = map[string]*relaycommon.BuildInToolInfo{}
	}
	slot := info.ResponsesUsageInfo.BuiltInTools[name]
	if slot == nil {
		slot = &relaycommon.BuildInToolInfo{ToolName: name}
		info.ResponsesUsageInfo.BuiltInTools[name] = slot
	}
	slot.CallCount++
}

func kindOf(can string) string {
	if can == CanonicalWebFetch {
		return "fetch"
	}
	return "search"
}

func firstPositiveInt(vals ...any) int {
	for _, v := range vals {
		if n := asPositiveInt(v); n > 0 {
			return n
		}
	}
	return 0
}

func asPositiveInt(v any) int {
	n, ok := asFloat(v)
	if !ok || n <= 0 || n > 1e9 {
		return 0
	}
	return int(n)
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

func asStringSlice(v any) []string {
	switch items := v.(type) {
	case []any:
		out := make([]string, 0, len(items))
		for _, item := range items {
			if s := asString(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return items
	default:
		return nil
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
