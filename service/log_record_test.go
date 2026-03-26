package service

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func stringPtr(value string) *string {
	return &value
}

func enableRecordConsumeLogDetailForTest(t *testing.T) {
	t.Helper()

	retrySetting := operation_setting.GetRetrySetting()
	previous := retrySetting.RecordConsumeLogDetailEnabled
	retrySetting.RecordConsumeLogDetailEnabled = true
	t.Cleanup(func() {
		retrySetting.RecordConsumeLogDetailEnabled = previous
	})
}

func TestBuildClaudeResponseBlocksFromSSE(t *testing.T) {
	responseBody := strings.Join([]string{
		`{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"glm-5","content":[]}}`,
		`{"type":"ping"}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":"sig"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"用户"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"想让我试试。"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"好嘞~"}}`,
		`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":" 让我试试几种不同的 MCP 工具！"}}`,
		`{"type":"content_block_stop","index":1}`,
		`{"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"call_wait","name":"mcp__wait__wait","input":{}}}`,
		`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"seconds\":0.5}"}}`,
		`{"type":"content_block_stop","index":2}`,
		`{"type":"content_block_start","index":3,"content_block":{"type":"tool_use","id":"call_fetch","name":"mcp__fetch__fetch","input":{}}}`,
		`{"type":"content_block_delta","index":3,"delta":{"type":"input_json_delta","partial_json":"{\"max_length\":500,\"url\":\"https://httpbin.org/get\"}"}}`,
		`{"type":"content_block_stop","index":3}`,
		`{"type":"message_delta","delta":{"stop_reason":"tool_use"}}`,
		`{"type":"message_stop"}`,
	}, "\n")

	blocks := buildClaudeResponseBlocksFromSSE(responseBody)
	require.Len(t, blocks, 4)

	require.Equal(t, model.ClaudeResponseBlock{
		Type:    "thinking",
		Content: "用户想让我试试。",
	}, blocks[0])
	require.Equal(t, model.ClaudeResponseBlock{
		Type:    "text",
		Content: "好嘞~ 让我试试几种不同的 MCP 工具！",
	}, blocks[1])

	require.Equal(t, "tool_use", blocks[2].Type)
	require.Equal(t, "call_wait", blocks[2].ID)
	require.Equal(t, "mcp__wait__wait", blocks[2].Name)
	require.Equal(t, map[string]any{"seconds": 0.5}, blocks[2].Input)

	require.Equal(t, "tool_use", blocks[3].Type)
	require.Equal(t, "call_fetch", blocks[3].ID)
	require.Equal(t, "mcp__fetch__fetch", blocks[3].Name)
	require.Equal(t, map[string]any{
		"max_length": float64(500),
		"url":        "https://httpbin.org/get",
	}, blocks[3].Input)
}

func TestBuildLogRecordClaudeStreamUsesStructuredBlocks(t *testing.T) {
	enableRecordConsumeLogDetailForTest(t)

	relayInfo := &relaycommon.RelayInfo{
		Request: &dto.ClaudeRequest{
			Messages: []dto.ClaudeMessage{
				{
					Role: "assistant",
					Content: []dto.ClaudeMediaMessage{
						{
							Type: "tool_use",
							Id:   "call_1",
							Name: "mcp__fetch__fetch",
						},
					},
				},
				{
					Role: "user",
					Content: []dto.ClaudeMediaMessage{
						{
							Type: "text",
							Text: stringPtr("第一段输入"),
						},
						{
							Type: "text",
							Text: stringPtr("第二段输入"),
						},
						{
							Type:      "tool_result",
							ToolUseId: "call_1",
							Content:   "tool output should not be stored",
						},
					},
				},
			},
		},
		IsStream: true,
		ResponseBody: strings.Join([]string{
			`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"思考中"}}`,
			`{"type":"content_block_stop","index":0}`,
			`{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"call_1","name":"mcp__fetch__fetch","input":{}}}`,
			`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"url\":\"https://example.com\"}"}}`,
			`{"type":"content_block_stop","index":1}`,
		}, "\n"),
		CompletionText:          "思考中",
		FinalRequestRelayFormat: types.RelayFormatClaude,
	}

	recordJSON := BuildLogRecord(relayInfo)
	require.NotEmpty(t, recordJSON)

	var record model.LogDetailRecord
	require.NoError(t, common.UnmarshalJsonStr(recordJSON, &record))
	require.Equal(t, map[string]interface{}{
		"role":    "user",
		"content": "第一段输入\n第二段输入",
		"contentList": []interface{}{
			map[string]interface{}{"type": "text", "text": "第一段输入"},
			map[string]interface{}{"type": "text", "text": "第二段输入"},
		},
	}, record.Prompt["lastUserMessage"])
	require.Equal(t, []model.ClaudeRequestBlock{
		{Type: "text", Text: "第一段输入"},
		{Type: "text", Text: "第二段输入"},
	}, record.ClaudeRequestBlocks)
	require.Equal(t, []model.ClaudeToolResponseBlock{
		{
			ToolUseID: "call_1",
			Name:      "mcp__fetch__fetch",
			Type:      "tool_result",
			Role:      "user",
		},
	}, record.ClaudeToolResponses)
	require.Len(t, record.ClaudeResponseBlocks, 2)
	require.Len(t, record.ToolInvokes, 1)
	require.Equal(t, "mcp__fetch__fetch", record.ToolInvokes[0].Name)
	require.Equal(t, map[string]any{"url": "https://example.com"}, record.ToolInvokes[0].Input)
	require.Nil(t, record.ToolInvokes[0].Result)
	require.Empty(t, record.ToolInvokes[0].ResultText)
}

func TestBuildLogRecordNonClaudeStreamSkipsStructuredBlocks(t *testing.T) {
	enableRecordConsumeLogDetailForTest(t)

	relayInfo := &relaycommon.RelayInfo{
		IsStream:                true,
		ResponseBody:            `{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`,
		CompletionText:          "plain completion",
		FinalRequestRelayFormat: types.RelayFormatOpenAI,
	}

	recordJSON := BuildLogRecord(relayInfo)
	require.NotEmpty(t, recordJSON)

	var record model.LogDetailRecord
	require.NoError(t, common.UnmarshalJsonStr(recordJSON, &record))
	require.Empty(t, record.ClaudeResponseBlocks)
	require.Empty(t, record.ClaudeRequestBlocks)
	require.Empty(t, record.ClaudeToolResponses)
	require.Equal(t, "plain completion", record.Completion)
}

func TestBuildLogRecordClaudeToolOnlyRequestDoesNotStorePromptContent(t *testing.T) {
	enableRecordConsumeLogDetailForTest(t)

	relayInfo := &relaycommon.RelayInfo{
		Request: &dto.ClaudeRequest{
			Messages: []dto.ClaudeMessage{
				{
					Role: "assistant",
					Content: []dto.ClaudeMediaMessage{
						{
							Type: "tool_use",
							Id:   "call_wait",
							Name: "mcp__wait__wait",
						},
					},
				},
				{
					Role: "user",
					Content: []dto.ClaudeMediaMessage{
						{
							Type:      "tool_result",
							ToolUseId: "call_wait",
							Content: []map[string]any{
								{
									"type": "text",
									"text": "Waited for 1 seconds successfully.",
								},
							},
						},
					},
				},
			},
		},
		FinalRequestRelayFormat: types.RelayFormatClaude,
	}

	recordJSON := BuildLogRecord(relayInfo)
	require.NotEmpty(t, recordJSON)

	var record model.LogDetailRecord
	require.NoError(t, common.UnmarshalJsonStr(recordJSON, &record))
	require.Nil(t, record.Prompt)
	require.Empty(t, record.ClaudeRequestBlocks)
	require.Equal(t, []model.ClaudeToolResponseBlock{
		{
			ToolUseID: "call_wait",
			Name:      "mcp__wait__wait",
			Type:      "tool_result",
			Role:      "user",
		},
	}, record.ClaudeToolResponses)
}

func TestBuildResponsesResponseBlocksFromSSE(t *testing.T) {
	responseBody := strings.Join([]string{
		`{"type":"response.content_part.added","content_index":0,"item_id":"msg_1","output_index":1,"part":{"type":"output_text","text":""}}`,
		`{"type":"response.output_text.delta","content_index":0,"item_id":"msg_1","output_index":1,"delta":"我"}`,
		`{"type":"response.output_text.delta","content_index":0,"item_id":"msg_1","output_index":1,"delta":"先试一下"}`,
		`{"type":"response.output_text.done","content_index":0,"item_id":"msg_1","output_index":1,"text":"我先试一下"}`,
		`{"type":"response.output_item.added","item":{"id":"fc_1","type":"function_call","status":"in_progress","arguments":"","call_id":"call_1","name":"exec_command"},"output_index":2}`,
		`{"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":2,"delta":"{\"cmd\":\"pwd\"}"}`,
		`{"type":"response.function_call_arguments.done","item_id":"fc_1","output_index":2,"arguments":"{\"cmd\":\"pwd\"}"}`,
		`{"type":"response.output_item.done","item":{"id":"fc_1","type":"function_call","status":"completed","arguments":"{\"cmd\":\"pwd\"}","call_id":"call_1","name":"exec_command"},"output_index":2}`,
	}, "\n")

	blocks := buildResponsesResponseBlocksFromSSE(responseBody)
	require.Len(t, blocks, 2)

	require.Equal(t, model.ResponsesResponseBlock{
		ID:      "msg_1",
		Type:    "output_text",
		Content: "我先试一下",
	}, blocks[0])
	require.Equal(t, "fc_1", blocks[1].ID)
	require.Equal(t, "function_call", blocks[1].Type)
	require.Equal(t, "call_1", blocks[1].CallID)
	require.Equal(t, "exec_command", blocks[1].Name)
	require.Equal(t, map[string]any{"cmd": "pwd"}, blocks[1].Arguments)
}

func TestBuildLogRecordResponsesStructuredBlocks(t *testing.T) {
	enableRecordConsumeLogDetailForTest(t)

	relayInfo := &relaycommon.RelayInfo{
		Request: &dto.OpenAIResponsesRequest{
			Input: json.RawMessage(`[
				{
					"type":"message",
					"role":"developer",
					"content":[
						{"type":"input_text","text":"<system-reminder>\n请遵循约束\n</system-reminder>"}
					]
				},
				{
					"type":"message",
					"role":"user",
					"content":[
						{"type":"input_text","text":"第一段输入"},
						{"type":"input_text","text":"第二段输入"}
					]
				},
				{
					"type":"function_call",
					"call_id":"call_1",
					"name":"exec_command",
					"arguments":"{\"cmd\":\"pwd\"}"
				},
				{
					"type":"function_call_output",
					"call_id":"call_1",
					"output":"should not store"
				},
				{
					"type":"message",
					"role":"assistant",
					"content":[
						{"type":"output_text","text":"上一轮输出"}
					]
				}
			]`),
		},
		IsStream: true,
		ResponseBody: strings.Join([]string{
			`{"type":"response.content_part.added","content_index":0,"item_id":"msg_1","output_index":1,"part":{"type":"output_text","text":""}}`,
			`{"type":"response.output_text.delta","content_index":0,"item_id":"msg_1","output_index":1,"delta":"开始处理"}`,
			`{"type":"response.output_item.added","item":{"id":"fc_1","type":"function_call","status":"in_progress","arguments":"","call_id":"call_1","name":"exec_command"},"output_index":2}`,
			`{"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":2,"delta":"{\"cmd\":\"pwd\"}"}`,
			`{"type":"response.output_item.done","item":{"id":"fc_1","type":"function_call","status":"completed","arguments":"{\"cmd\":\"pwd\"}","call_id":"call_1","name":"exec_command"},"output_index":2}`,
		}, "\n"),
		CompletionText:          "开始处理",
		FinalRequestRelayFormat: types.RelayFormatOpenAIResponses,
	}

	recordJSON := BuildLogRecord(relayInfo)
	require.NotEmpty(t, recordJSON)
	require.NotContains(t, recordJSON, "should not store")

	var record model.LogDetailRecord
	require.NoError(t, common.UnmarshalJsonStr(recordJSON, &record))
	require.NotContains(t, record.Prompt, "lastUserMessage")
	require.Equal(t, []interface{}{
		map[string]interface{}{
			"type":    "function_call_output",
			"call_id": "call_1",
			"name":    "exec_command",
		},
		map[string]interface{}{
			"type": "output_text",
			"text": "上一轮输出",
			"role": "assistant",
		},
	}, record.Prompt["input"])
	require.Empty(t, record.ResponsesRequestBlocks)
	require.Equal(t, []model.ResponsesToolResponseBlock{
		{
			CallID: "call_1",
			Name:   "exec_command",
			Type:   "function_call_output",
		},
	}, record.ResponsesToolResponses)
	require.Len(t, record.ResponsesResponseBlocks, 2)
	require.Equal(t, model.ResponsesResponseBlock{
		ID:      "msg_1",
		Type:    "output_text",
		Content: "开始处理",
	}, record.ResponsesResponseBlocks[0])
	require.Equal(t, "function_call", record.ResponsesResponseBlocks[1].Type)
	require.Len(t, record.ToolInvokes, 1)
	require.Equal(t, "call_1", record.ToolInvokes[0].ID)
	require.Equal(t, "exec_command", record.ToolInvokes[0].Name)
	require.Equal(t, map[string]any{"cmd": "pwd"}, record.ToolInvokes[0].Input)
}

func TestParseResponsesInputForRecordUsesOnlyLatestCallbackSegment(t *testing.T) {
	recordData := parseResponsesInputForRecord(json.RawMessage(`[
		{
			"type":"message",
			"role":"user",
			"content":[
				{"type":"input_text","text":"第一轮输入"}
			]
		},
		{
			"type":"function_call",
			"call_id":"call_1",
			"name":"exec_command",
			"arguments":"{\"cmd\":\"pwd\"}"
		},
		{
			"type":"function_call_output",
			"call_id":"call_1",
			"name":"exec_command",
			"output":"tool output"
		}
	]`))

	require.Equal(t, []map[string]interface{}{
		{
			"type":    "function_call_output",
			"call_id": "call_1",
			"name":    "exec_command",
		},
	}, recordData.PromptInput)
	require.Empty(t, recordData.RequestBlocks)
	require.Equal(t, []model.ResponsesToolResponseBlock{
		{
			CallID: "call_1",
			Name:   "exec_command",
			Type:   "function_call_output",
		},
	}, recordData.ToolResponses)
	require.Empty(t, recordData.LastUserText)
}

func TestParseResponsesInputForRecordUsesOnlyLatestOutputSegment(t *testing.T) {
	recordData := parseResponsesInputForRecord(json.RawMessage(`[
		{
			"type":"message",
			"role":"user",
			"content":[
				{"type":"input_text","text":"第一轮输入"}
			]
		},
		{
			"type":"function_call",
			"call_id":"call_1",
			"name":"exec_command",
			"arguments":"{\"cmd\":\"pwd\"}"
		},
		{
			"type":"function_call_output",
			"call_id":"call_1",
			"name":"exec_command",
			"output":"tool output"
		},
		{
			"type":"message",
			"role":"assistant",
			"content":[
				{"type":"output_text","text":"最终输出"}
			]
		}
	]`))

	require.Equal(t, []map[string]interface{}{
		{
			"type":    "function_call_output",
			"call_id": "call_1",
			"name":    "exec_command",
		},
		{
			"type": "output_text",
			"text": "最终输出",
			"role": "assistant",
		},
	}, recordData.PromptInput)
	require.Empty(t, recordData.RequestBlocks)
	require.Equal(t, []model.ResponsesToolResponseBlock{
		{
			CallID: "call_1",
			Name:   "exec_command",
			Type:   "function_call_output",
		},
	}, recordData.ToolResponses)
	require.Empty(t, recordData.LastUserText)
}

func TestParseResponsesInputForRecordUsesLatestInputSegment(t *testing.T) {
	recordData := parseResponsesInputForRecord(json.RawMessage(`[
		{
			"type":"message",
			"role":"user",
			"content":[
				{"type":"input_text","text":"第一轮输入"}
			]
		},
		{
			"type":"function_call",
			"call_id":"call_1",
			"name":"exec_command",
			"arguments":"{\"cmd\":\"pwd\"}"
		},
		{
			"type":"function_call_output",
			"call_id":"call_1",
			"name":"exec_command",
			"output":"tool output"
		},
		{
			"type":"message",
			"role":"assistant",
			"content":[
				{"type":"output_text","text":"上一轮输出"}
			]
		},
		{
			"type":"message",
			"role":"user",
			"content":[
				{"type":"input_text","text":"最后输入"}
			]
		}
	]`))

	require.Equal(t, []map[string]interface{}{
		{
			"type": "input_text",
			"text": "最后输入",
			"role": "user",
		},
	}, recordData.PromptInput)
	require.Equal(t, []model.ResponsesRequestBlock{
		{
			Type: "input_text",
			Role: "user",
			Text: "最后输入",
		},
	}, recordData.RequestBlocks)
	require.Empty(t, recordData.ToolResponses)
	require.Equal(t, "最后输入", recordData.LastUserText)
}

func TestBuildLogRecordResponsesLatestInputShowsRequestContent(t *testing.T) {
	enableRecordConsumeLogDetailForTest(t)

	relayInfo := &relaycommon.RelayInfo{
		Request: &dto.OpenAIResponsesRequest{
			Input: json.RawMessage(`[
				{
					"type":"message",
					"role":"user",
					"content":[
						{"type":"input_text","text":"第一轮输入"}
					]
				},
				{
					"type":"function_call",
					"call_id":"call_1",
					"name":"exec_command",
					"arguments":"{\"cmd\":\"pwd\"}"
				},
				{
					"type":"function_call_output",
					"call_id":"call_1",
					"name":"exec_command",
					"output":"tool output"
				},
				{
					"type":"message",
					"role":"assistant",
					"content":[
						{"type":"output_text","text":"上一轮输出"}
					]
				},
				{
					"type":"message",
					"role":"user",
					"content":[
						{"type":"input_text","text":"最后输入"}
					]
				}
			]`),
		},
		FinalRequestRelayFormat: types.RelayFormatOpenAIResponses,
	}

	recordJSON := BuildLogRecord(relayInfo)
	require.NotEmpty(t, recordJSON)

	var record model.LogDetailRecord
	require.NoError(t, common.UnmarshalJsonStr(recordJSON, &record))
	require.Equal(t, map[string]interface{}{
		"role":    "user",
		"content": "最后输入",
	}, record.Prompt["lastUserMessage"])
	require.Equal(t, []interface{}{
		map[string]interface{}{
			"type": "input_text",
			"text": "最后输入",
			"role": "user",
		},
	}, record.Prompt["input"])
	require.Equal(t, []model.ResponsesRequestBlock{
		{
			Type: "input_text",
			Role: "user",
			Text: "最后输入",
		},
	}, record.ResponsesRequestBlocks)
	require.Empty(t, record.ResponsesToolResponses)
}

func TestBuildLogRecordNonResponsesSkipsStructuredBlocks(t *testing.T) {
	enableRecordConsumeLogDetailForTest(t)

	relayInfo := &relaycommon.RelayInfo{
		Request: &dto.OpenAIResponsesRequest{
			Input: json.RawMessage(`[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}]`),
		},
		IsStream:                true,
		ResponseBody:            `{"type":"response.output_text.delta","content_index":0,"item_id":"msg_1","output_index":1,"delta":"hi"}`,
		CompletionText:          "hi",
		FinalRequestRelayFormat: types.RelayFormatOpenAI,
	}

	recordJSON := BuildLogRecord(relayInfo)
	require.NotEmpty(t, recordJSON)

	var record model.LogDetailRecord
	require.NoError(t, common.UnmarshalJsonStr(recordJSON, &record))
	require.Empty(t, record.ResponsesRequestBlocks)
	require.Empty(t, record.ResponsesToolResponses)
	require.Empty(t, record.ResponsesResponseBlocks)
	require.Equal(t, "hi", record.Completion)
}

func TestSanitizeToolLogValueTruncatesLongNestedStringValues(t *testing.T) {
	longValue := strings.Repeat("你好", 120)

	sanitized := sanitizeToolLogValue(map[string]any{
		"short": "ok",
		"nested": map[string]any{
			"value": longValue,
		},
		"items": []any{
			map[string]any{
				"text": longValue,
			},
		},
	})

	sanitizedMap, ok := sanitized.(map[string]any)
	require.True(t, ok)

	nestedMap, ok := sanitizedMap["nested"].(map[string]any)
	require.True(t, ok)
	nestedValue, ok := nestedMap["value"].(string)
	require.True(t, ok)
	require.Contains(t, nestedValue, "......")
	require.True(t, utf8.RuneCountInString(nestedValue) <= maxLoggedJSONValueLength)
	require.True(t, strings.HasPrefix(nestedValue, string([]rune(longValue)[:10])))
	require.True(t, strings.HasSuffix(nestedValue, string([]rune(longValue)[len([]rune(longValue))-10:])))

	items, ok := sanitizedMap["items"].([]any)
	require.True(t, ok)
	itemMap, ok := items[0].(map[string]any)
	require.True(t, ok)
	itemValue, ok := itemMap["text"].(string)
	require.True(t, ok)
	require.Contains(t, itemValue, "......")
	require.True(t, utf8.RuneCountInString(itemValue) <= maxLoggedJSONValueLength)
}
