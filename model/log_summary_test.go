package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestExtractLogDetailSummaries(t *testing.T) {
	recordBytes, err := common.Marshal(LogDetailRecord{
		Headers: map[string]string{
			"User-Agent": "claude-cli/1.0.0",
		},
		ClaudeRequestBlocks: []ClaudeRequestBlock{
			{
				Type: "text",
				Text: "你好",
			},
		},
	})
	require.NoError(t, err)

	source, interactionType, _, _, _ := ExtractLogDetailSummaries(string(recordBytes))
	require.Equal(t, "Claude Code", source)
	require.Equal(t, "输入", interactionType)
}

func TestFormatUserLogsHidesDetailForCommonUser(t *testing.T) {
	recordBytes, err := common.Marshal(LogDetailRecord{
		Headers: map[string]string{
			"User-Agent": "claude-cli/1.0.0",
		},
		ClaudeRequestBlocks: []ClaudeRequestBlock{
			{
				Type: "text",
				Text: "测试输入",
			},
		},
	})
	require.NoError(t, err)

	logs := []*Log{
		{
			Record:  string(recordBytes),
			FullLog: `{"request":{}}`,
		},
	}

	formatUserLogs(logs, 0, &User{Role: common.RoleCommonUser})

	require.Empty(t, logs[0].Record)
	require.Empty(t, logs[0].FullLog)

	otherMap, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.Equal(t, "Claude Code", otherMap[LogOtherClientSourceKey])
	require.Equal(t, "输入", otherMap[LogOtherInteractionTypeKey])
}

func TestFormatUserLogsKeepsDeveloperToolLogsForCodeUser(t *testing.T) {
	recordBytes, err := common.Marshal(LogDetailRecord{
		Headers: map[string]string{
			"User-Agent": "opencode/0.1.0",
		},
		ResponsesRequestBlocks: []ResponsesRequestBlock{
			{
				Type: "input_text",
				Role: "user",
				Text: "执行一个测试",
			},
		},
	})
	require.NoError(t, err)

	logs := []*Log{
		{
			Record:  string(recordBytes),
			FullLog: `{"request":{}}`,
			Other:   `{}`,
		},
	}

	formatUserLogs(logs, 0, &User{Role: common.RoleCodeUser})

	require.NotEmpty(t, logs[0].Record)
	require.NotEmpty(t, logs[0].FullLog)

	otherMap, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.Equal(t, "OpenCode", otherMap[LogOtherClientSourceKey])
	require.Equal(t, "输入", otherMap[LogOtherInteractionTypeKey])
}

func TestExtractLogDetailSummariesOpenAIOutput(t *testing.T) {
	recordBytes, err := common.Marshal(LogDetailRecord{
		OpenAIResponseBlocks: []OpenAIResponseBlock{
			{
				Type:    "content",
				Content: "完成了",
			},
		},
	})
	require.NoError(t, err)

	_, interactionType, _, _, _ := ExtractLogDetailSummaries(string(recordBytes))
	require.Equal(t, "输出", interactionType)
}

func TestExtractLogDetailSummariesOpenAIToolCallIsCallback(t *testing.T) {
	recordBytes, err := common.Marshal(LogDetailRecord{
		OpenAIRequestBlocks: []OpenAIRequestBlock{
			{
				Type: "text",
				Role: "user",
				Text: "请执行命令",
			},
		},
		OpenAIResponseBlocks: []OpenAIResponseBlock{
			{
				Type:    "content",
				Content: "我先处理。",
			},
			{
				Type: "tool_call",
				ID:   "call_1",
				Name: "exec_command",
			},
		},
	})
	require.NoError(t, err)

	_, interactionType, _, _, _ := ExtractLogDetailSummaries(string(recordBytes))
	require.Equal(t, "输入", interactionType)
}

func TestAppendAdminLogSummaries(t *testing.T) {
	recordBytes, err := common.Marshal(LogDetailRecord{
		Headers: map[string]string{
			"User-Agent": "codex_cli_rs/0.1.0",
		},
		OpenAIToolResponses: []OpenAIToolResponseBlock{
			{
				ToolCallID: "call_1",
				Type:       "tool",
				Role:       "tool",
			},
		},
	})
	require.NoError(t, err)

	logs := []*Log{{Record: string(recordBytes), Other: `{}`}}
	appendAdminLogSummaries(logs)

	otherMap, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.Equal(t, "Codex", otherMap[LogOtherClientSourceKey])
	require.Equal(t, "回调", otherMap[LogOtherInteractionTypeKey])
}

func TestExtractLogDetailSummariesWithSessionAffinity(t *testing.T) {
	recordBytes, err := common.Marshal(LogDetailRecord{
		Headers: map[string]string{
			"User-Agent":         "opencode/1.15.10",
			"X-Session-Affinity": "ses_1aa5b42cbffeQC26Uu3hvGrmkc",
		},
		OpenAIRequestBlocks: []OpenAIRequestBlock{
			{Type: "text", Role: "user", Text: "测试"},
		},
	})
	require.NoError(t, err)

	_, _, _, sessionId, parentSessionId := ExtractLogDetailSummaries(string(recordBytes))
	require.Equal(t, "ses_1aa5b42cbffeQC26Uu3hvGrmkc", sessionId)
	require.Empty(t, parentSessionId)
}

func TestExtractLogDetailSummariesWithParentSession(t *testing.T) {
	recordBytes, err := common.Marshal(LogDetailRecord{
		Headers: map[string]string{
			"User-Agent":          "opencode/1.15.10",
			"X-Session-Affinity":  "ses_1aa5b42cbffeQC26Uu3hvGrmkc",
			"X-Parent-Session-Id": "ses_1aa5b4864ffeU0ei6wLtOgldkE",
		},
		OpenAIRequestBlocks: []OpenAIRequestBlock{
			{Type: "text", Role: "user", Text: "子 Agent 请求"},
		},
	})
	require.NoError(t, err)

	_, _, _, sessionId, parentSessionId := ExtractLogDetailSummaries(string(recordBytes))
	require.Equal(t, "ses_1aa5b42cbffeQC26Uu3hvGrmkc", sessionId)
	require.Equal(t, "ses_1aa5b4864ffeU0ei6wLtOgldkE", parentSessionId)
}

func TestExtractLogDetailSummariesSessionAffinityPriority(t *testing.T) {
	recordBytes, err := common.Marshal(LogDetailRecord{
		Headers: map[string]string{
			"User-Agent":               "opencode/1.15.10",
			"X-Session-Affinity":       "ses_opencode_session",
			"X-Claude-Code-Session-Id": "ses_claudecode_session",
			"X-Claude-Code-Agent-Id":   "agent_claudecode",
		},
		OpenAIRequestBlocks: []OpenAIRequestBlock{
			{Type: "text", Role: "user", Text: "优先级测试"},
		},
	})
	require.NoError(t, err)

	_, _, agentId, sessionId, _ := ExtractLogDetailSummaries(string(recordBytes))
	require.Equal(t, "ses_opencode_session", sessionId)
	require.Equal(t, "agent_claudecode", agentId)
}

func TestExtractLogDetailSummariesWithGenericSessionId(t *testing.T) {
	recordBytes, err := common.Marshal(LogDetailRecord{
		Headers: map[string]string{
			"User-Agent":   "curl/8.0.0",
			"X-Session-Id": "76784d45-5425-4221-aa69-8c2faadf44ec",
		},
		OpenAIRequestBlocks: []OpenAIRequestBlock{
			{Type: "text", Role: "user", Text: "来自通用客户端的请求"},
		},
	})
	require.NoError(t, err)

	_, _, _, sessionId, parentSessionId := ExtractLogDetailSummaries(string(recordBytes))
	require.Equal(t, "76784d45-5425-4221-aa69-8c2faadf44ec", sessionId)
	require.Empty(t, parentSessionId)
}

func TestExtractLogDetailSummariesWithGenericSessionPriority(t *testing.T) {
	recordBytes, err := common.Marshal(LogDetailRecord{
		Headers: map[string]string{
			"User-Agent":               "custom-agent/1.0",
			"X-Session-Id":             "generic-session",
			"X-Claude-Code-Session-Id": "claude-session",
			"X-Session-Affinity":       "opencode-session",
			"X-Claude-Code-Agent-Id":   "agent-123",
			"X-Parent-Session-Id":      "parent-session",
		},
		OpenAIRequestBlocks: []OpenAIRequestBlock{
			{Type: "text", Role: "user", Text: "优先级测试"},
		},
	})
	require.NoError(t, err)

	_, _, agentId, sessionId, parentSessionId := ExtractLogDetailSummaries(string(recordBytes))
	// X-Session-Affinity 最高优先级，应覆盖 X-Claude-Code-Session-Id 和 X-Session-Id
	require.Equal(t, "opencode-session", sessionId)
	require.Equal(t, "agent-123", agentId)
	require.Equal(t, "parent-session", parentSessionId)
}

func TestParseClientSourceFromHeadersNamedAgents(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{
			name:    "claude-cli official",
			headers: map[string]string{"User-Agent": "claude-cli/2.1.88 (user, cli)"},
			want:    "Claude Code",
		},
		{
			name:    "claude-code slash",
			headers: map[string]string{"User-Agent": "claude-code/2.1.88"},
			want:    "Claude Code",
		},
		{
			name:    "claudecode",
			headers: map[string]string{"user-agent": "claudecode/1.0.0"},
			want:    "Claude Code",
		},
		{
			name:    "codex",
			headers: map[string]string{"User-Agent": "codex_cli_rs/0.20.0"},
			want:    "Codex",
		},
		{
			name:    "official codex cli ua",
			headers: map[string]string{"User-Agent": common.OfficialCodexCLIUserAgent},
			want:    "Codex",
		},
		{
			name:    "codex-cli product",
			headers: map[string]string{"User-Agent": "codex-cli/0.50.0"},
			want:    "Codex",
		},
		{
			name:    "codex originator",
			headers: map[string]string{"User-Agent": "Mozilla/5.0 Chrome/120.0.0.0", "Originator": "codex_cli_rs"},
			want:    "Codex",
		},
		{
			name:    "opencode",
			headers: map[string]string{"User-Agent": "opencode/0.1.0"},
			want:    "OpenCode",
		},
		{
			name:    "zcode",
			headers: map[string]string{"User-Agent": "ZCode/3.4.2 ai-sdk/provider-utils/4.0.39 runtime/node.js/24"},
			want:    "ZCode",
		},
		{
			name:    "grok-cli",
			headers: map[string]string{"User-Agent": "grok-cli/1.0.0"},
			want:    "Grok Build",
		},
		{
			name:    "Q1 production grok build ua",
			headers: map[string]string{"User-Agent": common.ProductionGrokBuildUserAgent},
			want:    "Grok Build",
		},
		{
			name:    "grok identifier header",
			headers: map[string]string{"User-Agent": "Mozilla/5.0 Chrome/120.0.0.0", "X-Grok-Client-Identifier": "grok-build"},
			want:    "Grok Build",
		},
		{
			name:    "chrome is not grok",
			headers: map[string]string{"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"},
			want:    "Chrome",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, parseClientSourceFromHeaders(tt.headers))
		})
	}
}

func TestParseClientSourceZCode(t *testing.T) {
	source := parseClientSourceFromHeaders(map[string]string{
		"User-Agent": "ZCode/3.4.2 ai-sdk/provider-utils/4.0.39 runtime/node.js/24",
	})
	require.Equal(t, "ZCode", source)
}

func TestIsDeveloperToolLogSourceZCode(t *testing.T) {
	require.True(t, IsDeveloperToolLogSource("ZCode"))
}

func TestIsDeveloperToolLogSourceGrokBuild(t *testing.T) {
	require.True(t, IsDeveloperToolLogSource("Grok Build"))
	require.False(t, IsDeveloperToolLogSource("Chrome"))
}

func TestFormatUserLogsKeepsGrokBuildForCodeUser(t *testing.T) {
	recordBytes, err := common.Marshal(LogDetailRecord{
		Headers: map[string]string{
			"User-Agent": "grok-cli/1.0.0",
		},
		ResponsesRequestBlocks: []ResponsesRequestBlock{
			{
				Type: "input_text",
				Role: "user",
				Text: "search something",
			},
		},
	})
	require.NoError(t, err)

	logs := []*Log{
		{
			Record:  string(recordBytes),
			FullLog: `{"request":{}}`,
		},
	}
	formatUserLogs(logs, 0, &User{Role: common.RoleCodeUser})
	require.NotEmpty(t, logs[0].Record)
	require.NotEmpty(t, logs[0].FullLog)
	otherMap, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.Equal(t, "Grok Build", otherMap[LogOtherClientSourceKey])
}

func TestFormatUserLogsHidesGrokBuildForCommonUser(t *testing.T) {
	recordBytes, err := common.Marshal(LogDetailRecord{
		Headers: map[string]string{
			"User-Agent": "grok-cli/1.0.0",
		},
		ResponsesRequestBlocks: []ResponsesRequestBlock{
			{
				Type: "input_text",
				Role: "user",
				Text: "search something",
			},
		},
	})
	require.NoError(t, err)

	logs := []*Log{
		{
			Record:  string(recordBytes),
			FullLog: `{"request":{}}`,
		},
	}
	formatUserLogs(logs, 0, &User{Role: common.RoleCommonUser})
	require.Empty(t, logs[0].Record)
	require.Empty(t, logs[0].FullLog)
	otherMap, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.Equal(t, "Grok Build", otherMap[LogOtherClientSourceKey])
}

func TestExtractLogDetailSummariesWithZCodeSubAgentSession(t *testing.T) {
	recordBytes, err := common.Marshal(LogDetailRecord{
		Headers: map[string]string{
			"User-Agent":       "ZCode/3.4.2 ai-sdk/provider-utils/4.0.39 runtime/node.js/24",
			"X-Zcode-Trace-Id": "trace_abc123",
			"X-Session-Id":     "sess_def456",
		},
		OpenAIRequestBlocks: []OpenAIRequestBlock{
			{Type: "text", Role: "user", Text: "ZCode SubAgent 请求"},
		},
	})
	require.NoError(t, err)

	source, _, agentId, sessionId, parentSessionId := ExtractLogDetailSummaries(string(recordBytes))
	require.Equal(t, "ZCode", source)
	require.Empty(t, agentId)
	require.Equal(t, "sess_def456", sessionId)
	require.Equal(t, "trace_abc123", parentSessionId)
}

func TestExtractLogDetailSummariesWithZCodeMainThreadSession(t *testing.T) {
	recordBytes, err := common.Marshal(LogDetailRecord{
		Headers: map[string]string{
			"User-Agent":       "ZCode/3.4.2 ai-sdk/provider-utils/4.0.39 runtime/node.js/24",
			"X-Zcode-Trace-Id": "trace_main_thread",
			"X-Session-Id":     "sess_main_thread",
		},
		OpenAIRequestBlocks: []OpenAIRequestBlock{
			{Type: "text", Role: "user", Text: "ZCode 主线程请求"},
		},
	})
	require.NoError(t, err)

	source, _, _, sessionId, parentSessionId := ExtractLogDetailSummaries(string(recordBytes))
	require.Equal(t, "ZCode", source)
	require.Equal(t, "sess_main_thread", sessionId)
	require.Equal(t, "trace_main_thread", parentSessionId)
}

func TestExtractLogDetailSummariesWithCodexWindowSession(t *testing.T) {
	recordBytes, err := common.Marshal(LogDetailRecord{
		Headers: map[string]string{
			"User-Agent":        "codex_cli_rs/0.20.0",
			"X-Codex-Window-Id": "win_abc123",
		},
		OpenAIRequestBlocks: []OpenAIRequestBlock{
			{Type: "text", Role: "user", Text: "Codex 窗口会话请求"},
		},
	})
	require.NoError(t, err)

	source, _, agentId, sessionId, parentSessionId := ExtractLogDetailSummaries(string(recordBytes))
	require.Equal(t, "Codex", source)
	require.Empty(t, agentId)
	require.Equal(t, "win_abc123", sessionId)
	require.Empty(t, parentSessionId)
}

func TestExtractLogDetailSummariesCodexWindowIdTakesPrecedence(t *testing.T) {
	recordBytes, err := common.Marshal(LogDetailRecord{
		Headers: map[string]string{
			"User-Agent":         "codex_cli_rs/0.20.0",
			"X-Codex-Window-Id":  "win_codex_session",
			"X-Session-Id":       "generic_session_should_be_ignored",
			"X-Session-Affinity": "affinity_should_be_ignored",
		},
		OpenAIRequestBlocks: []OpenAIRequestBlock{
			{Type: "text", Role: "user", Text: "Codex 优先级测试"},
		},
	})
	require.NoError(t, err)

	_, _, _, sessionId, parentSessionId := ExtractLogDetailSummaries(string(recordBytes))
	// Codex 分支提前返回，X-Codex-Window-Id 独占会话标识，不走通用回退链
	require.Equal(t, "win_codex_session", sessionId)
	require.Empty(t, parentSessionId)
}

func TestExtractLogDetailSummariesWithGrokBuildAgentSession(t *testing.T) {
	recordBytes, err := common.Marshal(LogDetailRecord{
		Headers: map[string]string{
			"User-Agent":        "grok-build/0.2",
			"X-Grok-Agent-Id":   "agent_grok_abc",
			"X-Grok-Session-Id": "sess_grok_456",
		},
		OpenAIRequestBlocks: []OpenAIRequestBlock{
			{Type: "text", Role: "user", Text: "Grok Build 子对话请求"},
		},
	})
	require.NoError(t, err)

	source, _, agentId, sessionId, parentSessionId := ExtractLogDetailSummaries(string(recordBytes))
	require.Equal(t, "Grok Build", source)
	require.Empty(t, agentId)
	require.Equal(t, "sess_grok_456", sessionId)
	require.Equal(t, "agent_grok_abc", parentSessionId)
}

func TestExtractLogDetailSummariesWithGrokBuildIgnoresGenericHeaders(t *testing.T) {
	// Grok Build 分支提前返回，X-Grok-* 独占会话标识，不走通用回退链。
	recordBytes, err := common.Marshal(LogDetailRecord{
		Headers: map[string]string{
			"User-Agent":            "grok-build/0.2",
			"X-Grok-Agent-Id":       "agent_main",
			"X-Grok-Session-Id":     "sess_child",
			"X-Session-Id":          "generic_ignored",
			"X-Claude-Code-Agent-Id": "claude_ignored",
		},
		OpenAIRequestBlocks: []OpenAIRequestBlock{
			{Type: "text", Role: "user", Text: "Grok 优先级测试"},
		},
	})
	require.NoError(t, err)

	source, _, agentId, sessionId, parentSessionId := ExtractLogDetailSummaries(string(recordBytes))
	require.Equal(t, "Grok Build", source)
	require.Empty(t, agentId)
	require.Equal(t, "sess_child", sessionId)
	require.Equal(t, "agent_main", parentSessionId)
}

func TestExtractLogDetailSummariesWithCodexTurnMetadataFallback(t *testing.T) {
	recordBytes, err := common.Marshal(LogDetailRecord{
		Headers: map[string]string{
			"User-Agent":            "codex_cli_rs/0.20.0",
			"X-Codex-Turn-Metadata": `{"session_id":"sess_abc","thread_id":"thread_abc","window_id":"019fbd87-1caa-7fd1-b7db-53756e002c02:0"}`,
		},
		OpenAIRequestBlocks: []OpenAIRequestBlock{
			{Type: "text", Role: "user", Text: "Codex turn metadata 兜底请求"},
		},
	})
	require.NoError(t, err)

	source, _, _, sessionId, parentSessionId := ExtractLogDetailSummaries(string(recordBytes))
	require.Equal(t, "Codex", source)
	require.Equal(t, "019fbd87-1caa-7fd1-b7db-53756e002c02:0", sessionId)
	require.Empty(t, parentSessionId)
}

func TestExtractLogDetailSummariesWithCodexDesktopClient(t *testing.T) {
	recordBytes, err := common.Marshal(LogDetailRecord{
		Headers: map[string]string{
			"User-Agent":        "codex_vscode/0.146.0-alpha.9.2",
			"X-Codex-Window-Id": "win_desktop",
		},
		OpenAIRequestBlocks: []OpenAIRequestBlock{
			{Type: "text", Role: "user", Text: "Codex Desktop 请求"},
		},
	})
	require.NoError(t, err)

	source, _, _, sessionId, parentSessionId := ExtractLogDetailSummaries(string(recordBytes))
	require.Equal(t, "Codex", source)
	require.Equal(t, "win_desktop", sessionId)
	require.Empty(t, parentSessionId)
}

func TestExtractLogDetailSummariesWithCodexParentThread(t *testing.T) {
	recordBytes, err := common.Marshal(LogDetailRecord{
		Headers: map[string]string{
			"User-Agent":               "codex_cli_rs/0.20.0",
			"X-Codex-Window-Id":        "win_sub",
			"X-Codex-Parent-Thread-Id": "thread_parent",
		},
		OpenAIRequestBlocks: []OpenAIRequestBlock{
			{Type: "text", Role: "user", Text: "Codex 子线程请求"},
		},
	})
	require.NoError(t, err)

	source, _, _, sessionId, parentSessionId := ExtractLogDetailSummaries(string(recordBytes))
	require.Equal(t, "Codex", source)
	require.Equal(t, "win_sub", sessionId)
	require.Equal(t, "thread_parent", parentSessionId)
}

func TestInferOpenAIStructuredInteractionType(t *testing.T) {
	tests := []struct {
		name           string
		requestBlocks  []OpenAIRequestBlock
		toolResponses  []OpenAIToolResponseBlock
		responseBlocks []OpenAIResponseBlock
		expected       string
	}{
		{
			name: "有用户输入且无tool response → 输入",
			requestBlocks: []OpenAIRequestBlock{
				{Type: "text", Role: "user", Text: "hello"},
			},
			toolResponses:  nil,
			responseBlocks: nil,
			expected:       "输入",
		},
		{
			name:          "有tool response但无requestBlocks → 回调",
			requestBlocks: nil,
			toolResponses: []OpenAIToolResponseBlock{
				{ToolCallID: "call_1", Name: "exec", Type: "tool", Role: "tool"},
			},
			responseBlocks: nil,
			expected:       "回调",
		},
		{
			name:          "无输入无tool response有文本输出 → 输出",
			requestBlocks: nil,
			toolResponses: nil,
			responseBlocks: []OpenAIResponseBlock{
				{Type: "content", Content: "完成了"},
			},
			expected: "输出",
		},
		{
			name: "有用户输入且有tool use → 输入",
			requestBlocks: []OpenAIRequestBlock{
				{Type: "text", Role: "user", Text: "执行命令"},
			},
			toolResponses: nil,
			responseBlocks: []OpenAIResponseBlock{
				{Type: "content", Content: "我来处理"},
				{Type: "tool_call", ID: "call_1", Name: "exec_command"},
			},
			expected: "输入",
		},
		{
			name:          "有tool use和tool response但无requestBlocks → 回调",
			requestBlocks: nil,
			toolResponses: []OpenAIToolResponseBlock{
				{ToolCallID: "call_1", Name: "exec", Type: "tool", Role: "tool"},
			},
			responseBlocks: []OpenAIResponseBlock{
				{Type: "tool_call", ID: "call_2", Name: "next_command"},
			},
			expected: "回调",
		},
		{
			name:          "空responseBlock无内容 → 回调",
			requestBlocks: nil,
			toolResponses: nil,
			responseBlocks: []OpenAIResponseBlock{
				{Type: "content", Content: ""},
			},
			expected: "回调",
		},
		{
			name:          "有tool response且无tool use有文本输出 → 输出",
			requestBlocks: nil,
			toolResponses: []OpenAIToolResponseBlock{
				{ToolCallID: "call_1", Name: "exec", Type: "tool", Role: "tool"},
			},
			responseBlocks: []OpenAIResponseBlock{
				{Type: "content", Content: "任务完成了"},
			},
			expected: "输出",
		},
		{
			name: "requestBlock有空text → 不算有输入",
			requestBlocks: []OpenAIRequestBlock{
				{Type: "text", Role: "user", Text: "  "},
			},
			toolResponses: nil,
			responseBlocks: []OpenAIResponseBlock{
				{Type: "content", Content: "响应"},
			},
			expected: "输出",
		},
		{
			name: "有用户输入且纯文本输出、无工具 → 单轮",
			requestBlocks: []OpenAIRequestBlock{
				{Type: "text", Role: "user", Text: "你好"},
			},
			toolResponses: nil,
			responseBlocks: []OpenAIResponseBlock{
				{Type: "content", Content: "你好，有什么可以帮你？"},
			},
			expected: "单轮",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferOpenAIStructuredInteractionType(tt.requestBlocks, tt.toolResponses, tt.responseBlocks)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestParseInteractionType(t *testing.T) {
	tests := []struct {
		name           string
		requestBlocks  []BambooRequestBlock
		toolResponses  []BambooToolResponseBlock
		responseBlocks []BambooResponseBlock
		expected       string
	}{
		{
			name: "bamboo input",
			requestBlocks: []BambooRequestBlock{
				{Type: "text", Text: "你好"},
			},
			toolResponses:  nil,
			responseBlocks: nil,
			expected:       "输入",
		},
		{
			name:          "bamboo output",
			requestBlocks: nil,
			toolResponses: nil,
			responseBlocks: []BambooResponseBlock{
				{Type: "text", Text: "完成了"},
			},
			expected: "输出",
		},
		{
			name:          "bamboo callback (bare tool response, no text, no tool_use)",
			requestBlocks: nil,
			toolResponses: []BambooToolResponseBlock{
				{ToolUseID: "tool_1", Name: "exec", Type: "tool", Content: "ok", Role: "tool"},
			},
			responseBlocks: nil,
			expected:       "回调",
		},
		{
			name:          "bamboo output (tool response + final text, no new tool_use)",
			requestBlocks: nil,
			toolResponses: []BambooToolResponseBlock{
				{ToolUseID: "tool_1", Name: "exec", Type: "tool", Content: "ok", Role: "tool"},
			},
			responseBlocks: []BambooResponseBlock{
				{Type: "text", Text: "最终回复"},
			},
			expected: "输出",
		},
		{
			name:          "bamboo callback (tool response + tool_use in response)",
			requestBlocks: nil,
			toolResponses: []BambooToolResponseBlock{
				{ToolUseID: "tool_1", Name: "exec", Type: "tool", Content: "ok", Role: "tool"},
			},
			responseBlocks: []BambooResponseBlock{
				{Type: "text", Text: "我再查一下"},
				{Type: "tool_use", ToolName: "exec_command"},
			},
			expected: "回调",
		},
		{
			name:          "bamboo callback (tool use)",
			requestBlocks: nil,
			toolResponses: nil,
			responseBlocks: []BambooResponseBlock{
				{Type: "tool_use", ToolName: "exec_command"},
			},
			expected: "回调",
		},
		{
			name: "bamboo 单轮（用户输入 + 纯文本输出，无工具）",
			requestBlocks: []BambooRequestBlock{
				{Type: "text", Text: "介绍一下你自己"},
			},
			toolResponses: nil,
			responseBlocks: []BambooResponseBlock{
				{Type: "text", Text: "我是一个 AI 助手"},
			},
			expected: "单轮",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferBambooStructuredInteractionType(tt.requestBlocks, tt.toolResponses, tt.responseBlocks)
			require.Equal(t, tt.expected, result)
		})
	}

	t.Run("claude leftover request text + tool result + final text → 输出", func(t *testing.T) {
		result := inferClaudeStructuredInteractionType(
			[]ClaudeRequestBlock{{Type: "text", Text: "original user prompt"}},
			[]ClaudeToolResponseBlock{{ToolUseID: "1", Name: "Read", Type: "tool_result"}},
			[]ClaudeResponseBlock{{Type: "text", Content: "Here is the file."}},
		)
		require.Equal(t, "输出", result)
	})

	t.Run("claude leftover request text + tool result + tool_use → 回调", func(t *testing.T) {
		result := inferClaudeStructuredInteractionType(
			[]ClaudeRequestBlock{{Type: "text", Text: "original user prompt"}},
			[]ClaudeToolResponseBlock{{ToolUseID: "1", Name: "Read", Type: "tool_result"}},
			[]ClaudeResponseBlock{{Type: "tool_use", ID: "2", Name: "Edit"}},
		)
		require.Equal(t, "回调", result)
	})

	t.Run("claude user turn with tool_use and no tool result → 输入", func(t *testing.T) {
		result := inferClaudeStructuredInteractionType(
			[]ClaudeRequestBlock{{Type: "text", Text: "fix the bug"}},
			nil,
			[]ClaudeResponseBlock{{Type: "tool_use", ID: "1", Name: "Read"}},
		)
		require.Equal(t, "输入", result)
	})

	t.Run("claude 用户输入 + 纯文本输出、无工具 → 单轮", func(t *testing.T) {
		result := inferClaudeStructuredInteractionType(
			[]ClaudeRequestBlock{{Type: "text", Text: "写一首诗"}},
			nil,
			[]ClaudeResponseBlock{{Type: "text", Content: "春风拂柳岸"}},
		)
		require.Equal(t, "单轮", result)
	})

	t.Run("claude 用户输入 + 文本输出但带 tool_use → 仍为输入", func(t *testing.T) {
		result := inferClaudeStructuredInteractionType(
			[]ClaudeRequestBlock{{Type: "text", Text: "写一首诗"}},
			nil,
			[]ClaudeResponseBlock{
				{Type: "text", Content: "我先查一下资料"},
				{Type: "tool_use", ID: "1", Name: "Search"},
			},
		)
		require.Equal(t, "输入", result)
	})

	t.Run("bamboo 空字段兜底", func(t *testing.T) {
		recordBytes, err := common.Marshal(LogDetailRecord{
			Prompt: map[string]interface{}{
				"lastUserMessage": map[string]interface{}{
					"content": "fallback 输入",
				},
			},
		})
		require.NoError(t, err)

		_, interactionType, _, _, _ := ExtractLogDetailSummaries(string(recordBytes))
		require.Equal(t, "输入", interactionType)
	})

	t.Run("兜底：prompt 字符串 + completion → 单轮", func(t *testing.T) {
		recordBytes, err := common.Marshal(LogDetailRecord{
			Prompt: map[string]interface{}{
				"lastUserMessage": map[string]interface{}{
					"content": "什么是量子纠缠？",
				},
			},
			Completion: "量子纠缠是……",
		})
		require.NoError(t, err)

		_, interactionType, _, _, _ := ExtractLogDetailSummaries(string(recordBytes))
		require.Equal(t, "单轮", interactionType)
	})

	t.Run("兜底：用户输入 + completion + 仅工具调用记录 → 输入", func(t *testing.T) {
		recordBytes, err := common.Marshal(LogDetailRecord{
			Prompt: map[string]interface{}{
				"lastUserMessage": map[string]interface{}{
					"content": "帮我查天气",
				},
			},
			Completion:  "好的",
			ToolInvokes: []LogToolInvokeRecord{{ID: "call_1", Name: "get_weather"}},
		})
		require.NoError(t, err)

		_, interactionType, _, _, _ := ExtractLogDetailSummaries(string(recordBytes))
		require.Equal(t, "输入", interactionType)
	})

	t.Run("兜底：携带工具结果且无文本输出 → 回调", func(t *testing.T) {
		recordBytes, err := common.Marshal(LogDetailRecord{
			Prompt: map[string]interface{}{
				"lastUserMessage": map[string]interface{}{
					"content": "继续",
				},
			},
			OpenAIToolResponses: []OpenAIToolResponseBlock{
				{ToolCallID: "call_1", Type: "tool", Role: "tool"},
			},
		})
		require.NoError(t, err)

		_, interactionType, _, _, _ := ExtractLogDetailSummaries(string(recordBytes))
		require.Equal(t, "回调", interactionType)
	})

	t.Run("扁平化 Responses prompt items + completion 纯文本 → 单轮", func(t *testing.T) {
		recordBytes, err := common.Marshal(LogDetailRecord{
			Prompt: map[string]interface{}{
				"input": []interface{}{
					map[string]interface{}{
						"type": "message",
						"role": "user",
						"content": []interface{}{
							map[string]interface{}{"type": "input_text", "text": "讲个笑话"},
						},
					},
				},
			},
			Completion: "为什么程序员分不清万圣节和圣诞节……",
		})
		require.NoError(t, err)

		_, interactionType, _, _, _ := ExtractLogDetailSummaries(string(recordBytes))
		require.Equal(t, "单轮", interactionType)
	})

	t.Run("扁平化 Responses prompt items 无 completion → 输入", func(t *testing.T) {
		recordBytes, err := common.Marshal(LogDetailRecord{
			Prompt: map[string]interface{}{
				"input": []interface{}{
					map[string]interface{}{
						"type": "message",
						"role": "user",
						"content": []interface{}{
							map[string]interface{}{"type": "input_text", "text": "讲个笑话"},
						},
					},
				},
			},
		})
		require.NoError(t, err)

		_, interactionType, _, _, _ := ExtractLogDetailSummaries(string(recordBytes))
		require.Equal(t, "输入", interactionType)
	})
}
