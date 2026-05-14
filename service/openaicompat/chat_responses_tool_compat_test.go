package openaicompat

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestResponsesResponseToChatCompletionsKeepsTextAndToolCalls(t *testing.T) {
	resp := &dto.OpenAIResponsesResponse{
		ID:        "resp_test",
		CreatedAt: 1700000000,
		Model:     "gpt-4o",
		Output: []dto.ResponsesOutput{
			{
				Type: "message",
				Role: "assistant",
				Content: []dto.ResponsesOutputContent{
					{Type: "output_text", Text: "我先查一下。"},
				},
			},
			{
				Type:      "function_call",
				ID:        "fc_1",
				CallId:    "call_1",
				Name:      "exec_command",
				Arguments: json.RawMessage(`{"cmd":"pwd"}`),
			},
		},
	}

	out, _, err := ResponsesResponseToChatCompletionsResponse(resp, "chatcmpl_test")
	require.NoError(t, err)
	require.Len(t, out.Choices, 1)
	require.Equal(t, "tool_calls", out.Choices[0].FinishReason)
	require.Equal(t, "我先查一下。", out.Choices[0].Message.Content)

	toolCalls := out.Choices[0].Message.ParseToolCalls()
	require.Len(t, toolCalls, 1)
	require.Equal(t, "call_1", toolCalls[0].ID)
	require.Equal(t, "function", toolCalls[0].Type)
	require.Equal(t, "exec_command", toolCalls[0].Function.Name)
	require.Equal(t, `{"cmd":"pwd"}`, toolCalls[0].Function.Arguments)
}

func TestChatCompletionsRequestToResponsesMapsLegacyFunctions(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-4o",
		Messages: []dto.Message{
			{Role: "user", Content: "查天气"},
		},
		Functions: json.RawMessage(`[
			{
				"name":"get_weather",
				"description":"Get weather info",
				"parameters":{"type":"object","properties":{"city":{"type":"string"}}}
			}
		]`),
		FunctionCall: json.RawMessage(`{"name":"get_weather"}`),
	}

	out, err := ChatCompletionsRequestToResponsesRequest(req)
	require.NoError(t, err)
	require.NotNil(t, out)

	var tools []map[string]any
	require.NoError(t, common.Unmarshal(out.Tools, &tools))
	require.Len(t, tools, 1)
	require.Equal(t, "function", tools[0]["type"])
	require.Equal(t, "get_weather", tools[0]["name"])
	require.Equal(t, "Get weather info", tools[0]["description"])
	require.NotNil(t, tools[0]["parameters"])

	var toolChoice map[string]any
	require.NoError(t, common.Unmarshal(out.ToolChoice, &toolChoice))
	require.Equal(t, "function", toolChoice["type"])
	require.Equal(t, "get_weather", toolChoice["name"])
}

func TestChatCompletionsRequestToResponsesMapsLegacyFunctionCallAuto(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-4o",
		Messages: []dto.Message{
			{Role: "user", Content: "hello"},
		},
		Functions:    json.RawMessage(`[{"name":"noop","parameters":{"type":"object"}}]`),
		FunctionCall: json.RawMessage(`"auto"`),
	}

	out, err := ChatCompletionsRequestToResponsesRequest(req)
	require.NoError(t, err)

	var toolChoice string
	require.NoError(t, common.Unmarshal(out.ToolChoice, &toolChoice))
	require.Equal(t, "auto", toolChoice)
}
