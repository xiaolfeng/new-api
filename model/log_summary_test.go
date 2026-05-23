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
	require.Equal(t, "回调", interactionType)
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
			name: "有用户输入且有tool use → 回调",
			requestBlocks: []OpenAIRequestBlock{
				{Type: "text", Role: "user", Text: "执行命令"},
			},
			toolResponses: nil,
			responseBlocks: []OpenAIResponseBlock{
				{Type: "content", Content: "我来处理"},
				{Type: "tool_call", ID: "call_1", Name: "exec_command"},
			},
			expected: "回调",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferOpenAIStructuredInteractionType(tt.requestBlocks, tt.toolResponses, tt.responseBlocks)
			require.Equal(t, tt.expected, result)
		})
	}
}
