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

	source, interactionType := ExtractLogDetailSummaries(string(recordBytes))
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

	_, interactionType := ExtractLogDetailSummaries(string(recordBytes))
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

	_, interactionType := ExtractLogDetailSummaries(string(recordBytes))
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
