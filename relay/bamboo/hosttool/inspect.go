package hosttool

import (
	"strings"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

const (
	searchDescription = "Search the public web. Use for current events, docs, or facts beyond the knowledge cutoff. Input: query (required)."
	fetchDescription  = "Fetch a single http(s) URL and return extracted text/markdown. JavaScript SPA shells include title, meta, HTML comments, and resource URLs from the HTTP HTML (no browser). Optional: prompt, format, timeout, pattern, start_line."
)

var searchSchemaJSON = []byte(`{"type":"object","properties":{"query":{"type":"string","description":"Search query"},"allowed_domains":{"type":"array","items":{"type":"string"}},"blocked_domains":{"type":"array","items":{"type":"string"}}},"required":["query"]}`)

var fetchSchemaJSON = []byte(`{"type":"object","properties":{"url":{"type":"string","description":"http(s) URL to fetch"},"prompt":{"type":"string","description":"Optional focus question for the reader model"},"format":{"type":"string","enum":["text","markdown","html"]},"timeout":{"type":"number","description":"Timeout in seconds, max 120"},"pattern":{"type":"string","description":"Optional case-insensitive regex; return matching lines with context"},"start_line":{"type":"number","description":"1-indexed line to start from (ignored when pattern is set)"},"max_matches":{"type":"number","description":"Max regex matches, default 50"},"context_lines":{"type":"number","description":"Context lines around each match, default 10"}},"required":["url"]}`)

// InspectAndRewrite 扫描 Helper remash DTO + Config.Tools，按 canonical 去重并注入 function。
func InspectAndRewrite(entryFormat types.RelayFormat, entryBytes []byte, req *bamboocodec.RelayRequest, st *model_setting.BambooSettings) (*relaycommon.HostToolPlan, error) {
	plan := &relaycommon.HostToolPlan{
		Enabled: false,
		Mode:    st.ResolvedHostToolMode(),
	}
	if st == nil || !st.EnableHostTools {
		return plan, nil
	}

	found := make([]relaycommon.HostToolDecl, 0, 4)
	found = append(found, scanEntryBytes(entryFormat, entryBytes)...)
	if req != nil && req.Config != nil {
		found = append(found, scanConfigTools(req.Config.Tools)...)
	}

	// 按 Canonical 去重，保留第一条。
	seen := make(map[string]int, 2)
	var decls []relaycommon.HostToolDecl
	var stripped []string
	for _, d := range found {
		if d.Canonical == "" {
			continue
		}
		if idx, ok := seen[d.Canonical]; ok {
			if d.OriginalName != decls[idx].OriginalName {
				stripped = append(stripped, d.OriginalName)
			}
			if d.HadSchema && !decls[idx].HadSchema {
				decls[idx].HadSchema = true
			}
			if d.MaxUses > 0 && decls[idx].MaxUses == 0 {
				decls[idx].MaxUses = d.MaxUses
			}
			mergeDeclFilters(&decls[idx], d)
			continue
		}
		seen[d.Canonical] = len(decls)
		decls = append(decls, d)
	}

	if len(decls) == 0 {
		plan.Stripped = stripped
		return plan, nil
	}

	if req == nil {
		plan.Enabled = true
		plan.Decls = decls
		plan.Stripped = stripped
		return plan, nil
	}
	if req.Config == nil {
		req.Config = &bamboosdk.RequestConfig{}
	}

	rewritten, injected := rewriteTools(req.Config.Tools, decls)
	req.Config.Tools = rewritten

	plan.Enabled = true
	plan.Decls = decls
	plan.Injected = injected
	plan.Stripped = stripped
	return plan, nil
}

func scanEntryBytes(entryFormat types.RelayFormat, entryBytes []byte) []relaycommon.HostToolDecl {
	if len(entryBytes) == 0 {
		return nil
	}
	var root map[string]any
	if err := common.Unmarshal(entryBytes, &root); err != nil {
		return nil
	}
	var out []relaycommon.HostToolDecl
	switch entryFormat {
	case types.RelayFormatClaude:
		out = append(out, scanToolObjects(asMapSlice(root["tools"]), true)...)
	case types.RelayFormatOpenAI:
		out = append(out, scanToolObjects(asMapSlice(root["tools"]), false)...)
		if _, ok := root["web_search_options"]; ok && root["web_search_options"] != nil {
			out = append(out, relaycommon.HostToolDecl{
				OriginalName: "web_search",
				Canonical:    CanonicalWebSearch,
				Source:       sourceWebSearchOptions,
				BillingName:  billingWebSearch,
			})
		}
	case types.RelayFormatOpenAIResponses:
		out = append(out, scanToolObjects(asMapSlice(root["tools"]), true)...)
	case types.RelayFormatGemini:
		for _, tool := range asMapSlice(root["tools"]) {
			out = append(out, scanToolObjects(asMapSlice(tool["functionDeclarations"]), false)...)
		}
	}
	return out
}

func scanConfigTools(tools []bamboosdk.Tool) []relaycommon.HostToolDecl {
	var out []relaycommon.HostToolDecl
	for _, t := range tools {
		if can := CanonicalFromName(t.Name); can != "" {
			out = append(out, relaycommon.HostToolDecl{
				OriginalName: t.Name,
				Canonical:    can,
				Source:       sourceFunction,
				HadSchema:    len(t.InputSchema) > 0 && string(t.InputSchema) != "null",
				BillingName:  billingNameFor(t.Name, "", can),
			})
		}
	}
	return out
}

func scanToolObjects(objs []map[string]any, allowType bool) []relaycommon.HostToolDecl {
	var out []relaycommon.HostToolDecl
	for _, obj := range objs {
		if obj == nil {
			continue
		}
		typ, _ := obj["type"].(string)
		name := firstNonEmpty(asString(obj["name"]))
		if fn, ok := obj["function"].(map[string]any); ok {
			if n := asString(fn["name"]); n != "" {
				name = n
			}
		}
		can := CanonicalFromName(name)
		src := sourceFunction
		if can == "" && allowType {
			can = CanonicalFromType(typ)
			if can != "" {
				src = sourceServerType
				if name == "" {
					if can == CanonicalWebFetch {
						name = "web_fetch"
					} else if normalizeName(typ) == "web_search_preview" {
						name = "web_search_preview"
					} else {
						name = "web_search"
					}
				}
			}
		}
		if can == "" && CanonicalFromType(typ) != "" {
			can = CanonicalFromType(typ)
			src = sourceServerType
			if name == "" {
				name = "web_search"
				if can == CanonicalWebFetch {
					name = "web_fetch"
				}
			}
		}
		if can == "" {
			continue
		}
		hadSchema := false
		if schema, ok := obj["input_schema"]; ok && schema != nil {
			hadSchema = true
		}
		if fn, ok := obj["function"].(map[string]any); ok {
			if schema, ok := fn["parameters"]; ok && schema != nil {
				hadSchema = true
			}
		}
		allowed, blocked := parseDomainFilters(obj)
		out = append(out, relaycommon.HostToolDecl{
			OriginalName:   name,
			Canonical:      can,
			Source:         src,
			HadSchema:      hadSchema,
			BillingName:    billingNameFor(name, typ, can),
			MaxUses:        parseMaxUses(obj["max_uses"]),
			AllowedDomains: allowed,
			BlockedDomains: blocked,
		})
	}
	return out
}

func rewriteTools(existing []bamboosdk.Tool, decls []relaycommon.HostToolDecl) ([]bamboosdk.Tool, []string) {
	byName := make(map[string]bamboosdk.Tool, len(existing))
	for _, t := range existing {
		byName[t.Name] = t
	}
	out := make([]bamboosdk.Tool, 0, len(decls)+len(existing))
	used := make(map[string]bool)
	var injected []string

	for _, d := range decls {
		if t, ok := byName[d.OriginalName]; ok && len(t.InputSchema) > 0 && string(t.InputSchema) != "null" {
			if strings.TrimSpace(t.Description) == "" {
				t.Description = defaultDescription(d.Canonical)
			}
			out = append(out, t)
			used[t.Name] = true
			continue
		}
		tool := injectedTool(d)
		out = append(out, tool)
		used[tool.Name] = true
		injected = append(injected, tool.Name)
	}

	// 保留非 host 工具。
	for _, t := range existing {
		if used[t.Name] {
			continue
		}
		if CanonicalFromName(t.Name) != "" {
			continue
		}
		out = append(out, t)
	}
	return out, injected
}

func injectedTool(d relaycommon.HostToolDecl) bamboosdk.Tool {
	schema := searchSchemaJSON
	desc := searchDescription
	if d.Canonical == CanonicalWebFetch {
		schema = fetchSchemaJSON
		desc = fetchDescription
	}
	name := d.OriginalName
	if name == "" {
		if d.Canonical == CanonicalWebFetch {
			name = "web_fetch"
		} else {
			name = "web_search"
		}
	}
	return bamboosdk.Tool{
		Name:        name,
		Description: desc,
		InputSchema: append([]byte(nil), schema...),
	}
}

func defaultDescription(canonical string) string {
	if canonical == CanonicalWebFetch {
		return fetchDescription
	}
	return searchDescription
}

func asMapSlice(v any) []map[string]any {
	switch items := v.(type) {
	case []any:
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	case []map[string]any:
		return items
	default:
		return nil
	}
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func parseDomainFilters(obj map[string]any) (allowed, blocked []string) {
	if obj == nil {
		return nil, nil
	}
	filters, _ := obj["filters"].(map[string]any)
	allowed = firstStringSlice(
		asStringSlice(obj["allowed_domains"]),
		asStringSlice(nestedSlice(filters, "allowed_domains")),
	)
	blocked = firstStringSlice(
		asStringSlice(obj["blocked_domains"]),
		asStringSlice(obj["excluded_domains"]),
		asStringSlice(nestedSlice(filters, "excluded_domains")),
		asStringSlice(nestedSlice(filters, "blocked_domains")),
	)
	if len(allowed) > 0 {
		return allowed, nil
	}
	return nil, blocked
}

func nestedSlice(m map[string]any, key string) any {
	if m == nil {
		return nil
	}
	return m[key]
}

func firstStringSlice(sets ...[]string) []string {
	for _, set := range sets {
		if len(set) > 0 {
			return set
		}
	}
	return nil
}

func mergeDeclFilters(dst *relaycommon.HostToolDecl, src relaycommon.HostToolDecl) {
	if dst == nil {
		return
	}
	if len(dst.AllowedDomains) == 0 && len(src.AllowedDomains) > 0 {
		dst.AllowedDomains = src.AllowedDomains
		dst.BlockedDomains = nil
		return
	}
	if len(dst.AllowedDomains) == 0 && len(dst.BlockedDomains) == 0 && len(src.BlockedDomains) > 0 {
		dst.BlockedDomains = src.BlockedDomains
	}
}
