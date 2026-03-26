package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
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

func TestFormatUserLogsHidesDetailWhenFeatureDisabled(t *testing.T) {
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

	setting := &dto.UserSetting{DeveloperToolLogEnabled: false}
	formatUserLogs(logs, 0, setting)

	require.Empty(t, logs[0].Record)
	require.Empty(t, logs[0].FullLog)

	otherMap, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.Equal(t, "Claude Code", otherMap[LogOtherClientSourceKey])
	require.Equal(t, "输入", otherMap[LogOtherInteractionTypeKey])
}

func TestFormatUserLogsKeepsDeveloperToolRecordWhenFeatureEnabled(t *testing.T) {
	recordBytes, err := common.Marshal(LogDetailRecord{
		Headers: map[string]string{
			"User-Agent": "codex-cli-rs/0.1.0",
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

	setting := &dto.UserSetting{DeveloperToolLogEnabled: true}
	formatUserLogs(logs, 0, setting)

	require.NotEmpty(t, logs[0].Record)
	require.Empty(t, logs[0].FullLog)

	otherMap, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.Equal(t, "Codex", otherMap[LogOtherClientSourceKey])
	require.Equal(t, "输入", otherMap[LogOtherInteractionTypeKey])
}
