package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchClientProfileNamedAgents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ua      string
		headers map[string]string
		profile ClientProfile
		source  string
		hit     string
	}{
		{
			name:    "grok-cli",
			ua:      "grok-cli/1.0.0",
			profile: ClientProfileGrokBuild,
			source:  ClientSourceGrokBuild,
			hit:     "grok-cli",
		},
		{
			name:    "grok-build",
			ua:      "grok-build/0.2",
			profile: ClientProfileGrokBuild,
			source:  ClientSourceGrokBuild,
			hit:     "grok-build",
		},
		{
			name:    "grok-pager",
			ua:      "grok-pager/dev",
			profile: ClientProfileGrokBuild,
			source:  ClientSourceGrokBuild,
			hit:     "grok-pager",
		},
		{
			name:    "Q1 production grok build ua",
			ua:      ProductionGrokBuildUserAgent,
			profile: ClientProfileGrokBuild,
			source:  ClientSourceGrokBuild,
			hit:     "grok-pager",
		},
		{
			name:    "grok-shell",
			ua:      "grok-shell/1.0.5 (macos; aarch64)",
			profile: ClientProfileGrokBuild,
			source:  ClientSourceGrokBuild,
			hit:     "grok-shell",
		},
		{
			name:    "grok-tui",
			ua:      "grok-tui/1",
			profile: ClientProfileGrokBuild,
			source:  ClientSourceGrokBuild,
			hit:     "grok-tui",
		},
		{
			name:    "grok slash product",
			ua:      "Grok/1.2.3 darwin",
			profile: ClientProfileGrokBuild,
			source:  ClientSourceGrokBuild,
			hit:     "grok/",
		},
		{
			name:    "x-grok-client-identifier wins over chrome ua",
			ua:      "Mozilla/5.0 Chrome/120.0.0.0",
			headers: map[string]string{"X-Grok-Client-Identifier": "grok-build"},
			profile: ClientProfileGrokBuild,
			source:  ClientSourceGrokBuild,
			hit:     grokClientIdentifierHeader,
		},
		{
			name:    "claude-cli official",
			ua:      "claude-cli/2.1.88 (user, cli)",
			profile: ClientProfileClaudeCode,
			source:  ClientSourceClaudeCode,
			hit:     "claude-cli",
		},
		{
			name:    "claudecode",
			ua:      "claudecode/1.0.0",
			profile: ClientProfileClaudeCode,
			source:  ClientSourceClaudeCode,
			hit:     "claudecode",
		},
		{
			name:    "claude-code slash",
			ua:      "claude-code/2.1.88",
			profile: ClientProfileClaudeCode,
			source:  ClientSourceClaudeCode,
			hit:     "claude-code/",
		},
		{
			name:    "official codex cli ua",
			ua:      OfficialCodexCLIUserAgent,
			profile: ClientProfileCodex,
			source:  ClientSourceCodex,
			hit:     "codex_cli_rs",
		},
		{
			name:    "codex-cli slash product",
			ua:      "codex-cli/0.50.0",
			profile: ClientProfileCodex,
			source:  ClientSourceCodex,
			hit:     "codex-cli",
		},
		{
			name:    "codex originator wins over chrome ua",
			ua:      "Mozilla/5.0 Chrome/120.0.0.0",
			headers: map[string]string{"Originator": "codex_cli_rs"},
			profile: ClientProfileCodex,
			source:  ClientSourceCodex,
			hit:     "codex_cli_rs",
		},
		{
			name:    "codex_exec originator",
			headers: map[string]string{"Originator": "codex_exec"},
			profile: ClientProfileCodex,
			source:  ClientSourceCodex,
			hit:     "codex_exec",
		},
		{
			name:    "codex_cli_rs",
			ua:      "codex_cli_rs/0.20.0",
			profile: ClientProfileCodex,
			source:  ClientSourceCodex,
			hit:     "codex_cli_rs",
		},
		{
			name:    "opencode",
			ua:      "opencode/0.1.0",
			profile: ClientProfileOpenCode,
			source:  ClientSourceOpenCode,
			hit:     "opencode/",
		},
		{
			name:    "zcode",
			ua:      "ZCode/3.4.2 ai-sdk/provider-utils/4.0.39 runtime/node.js/24",
			profile: ClientProfileZCode,
			source:  ClientSourceZCode,
			hit:     "zcode/",
		},
		{
			name:    "chrome is not grok",
			ua:      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			profile: ClientProfileGeneric,
		},
		{
			name:    "bare grok substring is not grok_build",
			ua:      "my-grok-helper/1.0",
			profile: ClientProfileGeneric,
		},
		{
			name:    "claude session header alone is not claude_code",
			headers: map[string]string{"X-Claude-Code-Session-Id": "sess_1"},
			profile: ClientProfileGeneric,
		},
		{
			name:    "x-app cli alone is not claude_code",
			headers: map[string]string{"x-app": "cli"},
			profile: ClientProfileGeneric,
		},
		{
			name:    "empty",
			profile: ClientProfileGeneric,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := MatchClientProfile(tt.ua, tt.headers)
			require.Equal(t, tt.profile, got.Profile)
			assert.Equal(t, tt.source, got.Source)
			assert.Equal(t, tt.hit, got.Hit)
		})
	}
}

func TestMatchClientProfileOriginatorFallback(t *testing.T) {
	t.Parallel()
	got := MatchClientProfile("", map[string]string{
		"Originator": "claude-cli/2.1.88 (user, cli)",
	})
	require.Equal(t, ClientProfileClaudeCode, got.Profile)
	assert.Equal(t, ClientSourceClaudeCode, got.Source)
}
