package common

import "strings"

const (
	HostToolCanonicalSearch = "host.web_search"
	HostToolCanonicalFetch  = "host.web_fetch"
)

// HostToolPlan 是端侧工具检测/改写/执行的本地副本。
// 放在 relay/common，避免 RelayInfo 导入 relay/bamboo/hosttool 造成循环依赖。
type HostToolPlan struct {
	Enabled          bool
	Mode             string // "loop" | "return"
	Decls            []HostToolDecl
	Injected         []string
	Stripped         []string
	Execs            []HostToolExecRecord
	StoppedReason    string
	BuiltinResponses bool
}

// HostToolDecl 一条归一后的 host 工具声明。
type HostToolDecl struct {
	OriginalName   string
	Canonical      string // host.web_search | host.web_fetch
	Source         string // function | server_type | web_search_options
	HadSchema      bool
	BillingName    string // web_search | web_search_preview | web_fetch
	MaxUses        int    // 0 = 未声明
	AllowedDomains []string
	BlockedDomains []string
}

// HostToolExecRecord 一次工具执行的可观测记录（admin_info）。
type HostToolExecRecord struct {
	OriginalName string `json:"original_name"`
	Canonical    string `json:"canonical"`
	Backend      string `json:"backend,omitempty"`
	DurationMs   int64  `json:"duration_ms"`
	ErrorCode    string `json:"error_code,omitempty"`
	Truncated    bool   `json:"truncated,omitempty"`
}

func (p *HostToolPlan) DeclByOriginal(name string) *HostToolDecl {
	if p == nil {
		return nil
	}
	for i := range p.Decls {
		if p.Decls[i].OriginalName == name {
			return &p.Decls[i]
		}
	}
	return nil
}

func (p *HostToolPlan) DeclByCanonical(canonical string) *HostToolDecl {
	if p == nil {
		return nil
	}
	for i := range p.Decls {
		if p.Decls[i].Canonical == canonical {
			return &p.Decls[i]
		}
	}
	return nil
}

// SuccessfulExecCounts 统计本轮成功执行的检索 / 抓取次数，供费用列标签使用。
func (p *HostToolPlan) SuccessfulExecCounts() (search, fetch int) {
	if p == nil {
		return 0, 0
	}
	for _, ex := range p.Execs {
		if strings.TrimSpace(ex.ErrorCode) != "" {
			continue
		}
		switch ex.Canonical {
		case HostToolCanonicalFetch:
			fetch++
		case HostToolCanonicalSearch:
			search++
		}
	}
	return search, fetch
}
