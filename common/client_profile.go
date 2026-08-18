package common

import "strings"

// ClientProfile 是请求级客户端身份，只改变响应形状，不当鉴权。
type ClientProfile string

const (
	ClientProfileGrokBuild  ClientProfile = "grok_build"
	ClientProfileClaudeCode ClientProfile = "claude_code"
	ClientProfileCodex      ClientProfile = "codex"
	ClientProfileOpenCode   ClientProfile = "opencode"
	ClientProfileZCode      ClientProfile = "zcode"
	ClientProfileGeneric    ClientProfile = "generic"
)

// 日志展示名必须与现网 other.client_source 逐字相同。
const (
	ClientSourceGrokBuild  = "Grok Build"
	ClientSourceClaudeCode = "Claude Code"
	ClientSourceCodex      = "Codex"
	ClientSourceOpenCode   = "OpenCode"
	ClientSourceZCode      = "ZCode"
)

// ClientIdentity 是一次 UA / 头匹配的结果。
type ClientIdentity struct {
	Profile ClientProfile
	Source  string // 日志展示名；generic 时为空，由遗留浏览器表填写
	Hit     string // 命中的 token 或头名，仅 admin
}

type namedClientToken struct {
	token   string
	profile ClientProfile
	source  string
}

// 具名白名单按此顺序匹配，且必须排在 chrome/ 等遗留浏览器分支之前。
// 禁止用裸 "grok" 子串：Mozilla Chrome 不得变成 grok_build。
var namedClientTokens = []namedClientToken{
	{token: "grok-cli", profile: ClientProfileGrokBuild, source: ClientSourceGrokBuild},
	{token: "grok-build", profile: ClientProfileGrokBuild, source: ClientSourceGrokBuild},
	{token: "grok-pager", profile: ClientProfileGrokBuild, source: ClientSourceGrokBuild},
	{token: "grok-shell", profile: ClientProfileGrokBuild, source: ClientSourceGrokBuild},
	{token: "grok-tui", profile: ClientProfileGrokBuild, source: ClientSourceGrokBuild},
	{token: "grok/", profile: ClientProfileGrokBuild, source: ClientSourceGrokBuild},
	{token: "claude-cli", profile: ClientProfileClaudeCode, source: ClientSourceClaudeCode},
	{token: "claudecode", profile: ClientProfileClaudeCode, source: ClientSourceClaudeCode},
	{token: "claude-code/", profile: ClientProfileClaudeCode, source: ClientSourceClaudeCode},
	{token: "codex_cli_rs", profile: ClientProfileCodex, source: ClientSourceCodex},
	{token: "codex-cli-rs", profile: ClientProfileCodex, source: ClientSourceCodex},
	{token: "codex_vscode", profile: ClientProfileCodex, source: ClientSourceCodex},
	{token: "codex-tui", profile: ClientProfileCodex, source: ClientSourceCodex},
	{token: "codex-desktop", profile: ClientProfileCodex, source: ClientSourceCodex},
	{token: "codex desktop", profile: ClientProfileCodex, source: ClientSourceCodex},
	{token: "codex_exec", profile: ClientProfileCodex, source: ClientSourceCodex},
	{token: "codex-cli", profile: ClientProfileCodex, source: ClientSourceCodex},
	{token: "opencode/", profile: ClientProfileOpenCode, source: ClientSourceOpenCode},
	{token: "crush/", profile: ClientProfileOpenCode, source: ClientSourceOpenCode},
	{token: "zcode/", profile: ClientProfileZCode, source: ClientSourceZCode},
}

const grokClientIdentifierHeader = "x-grok-client-identifier"

// ProductionGrokBuildUserAgent 是 2026-08-18 从生产抓到的 Grok Build UA（RFC-0002 Q1）。
const ProductionGrokBuildUserAgent = "grok-pager/1.0.5 grok-shell/1.0.5 (macos; aarch64)"

// OfficialCodexCLIUserAgent 是 openai/codex 官方 CLI UA 形状（codex-rs default_client）。
const OfficialCodexCLIUserAgent = "codex_cli_rs/0.50.0 (macos 15.0.0; aarch64)"

// MatchClientProfile 从 User-Agent 与已知头解析一次客户端身份。
// 解析顺序：x-grok-client-identifier → UA（空则 Originator）→ 具名白名单
// → 仍 generic 时再用 Originator 跑一遍白名单（Codex 稳定身份在 originator）→ generic。
// X-Claude-Code-Session-Id / x-app=cli 只作辅证，不得单独当命中 claude_code。
func MatchClientProfile(userAgent string, headers map[string]string) ClientIdentity {
	if id := matchGrokClientIdentifier(headers); id.Profile != "" {
		return id
	}

	ua := strings.TrimSpace(userAgent)
	if ua == "" {
		ua = headerIgnoreCase(headers, "user-agent")
	}
	originator := headerIgnoreCase(headers, "originator")
	if ua == "" {
		ua = originator
	}
	if id := matchNamedClientToken(ua); id.Profile != ClientProfileGeneric {
		return id
	}
	if originator != "" && !strings.EqualFold(strings.TrimSpace(originator), strings.TrimSpace(ua)) {
		if id := matchNamedClientToken(originator); id.Profile != ClientProfileGeneric {
			return id
		}
	}
	return ClientIdentity{Profile: ClientProfileGeneric}
}

func matchNamedClientToken(raw string) ClientIdentity {
	folded := strings.ToLower(strings.TrimSpace(raw))
	if folded == "" {
		return ClientIdentity{Profile: ClientProfileGeneric}
	}
	for _, item := range namedClientTokens {
		if strings.Contains(folded, item.token) {
			return ClientIdentity{
				Profile: item.profile,
				Source:  item.source,
				Hit:     item.token,
			}
		}
	}
	return ClientIdentity{Profile: ClientProfileGeneric}
}

func matchGrokClientIdentifier(headers map[string]string) ClientIdentity {
	if headerIgnoreCase(headers, grokClientIdentifierHeader) == "" {
		return ClientIdentity{}
	}
	return ClientIdentity{
		Profile: ClientProfileGrokBuild,
		Source:  ClientSourceGrokBuild,
		Hit:     grokClientIdentifierHeader,
	}
}

func headerIgnoreCase(headers map[string]string, name string) string {
	if len(headers) == 0 {
		return ""
	}
	want := strings.ToLower(strings.TrimSpace(name))
	for key, value := range headers {
		if strings.ToLower(strings.TrimSpace(key)) == want {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
