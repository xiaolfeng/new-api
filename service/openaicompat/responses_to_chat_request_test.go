package openaicompat

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mustRaw marshals v to json.RawMessage, panicking on error (test-only helper).
func mustRaw(v any) json.RawMessage {
	b, err := common.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// ---------- Test 1: Simple Conversation ----------

func TestSimpleConversation(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-4o",
		Input: mustRaw([]map[string]any{
			{"role": "system", "content": "You are helpful."},
			{"role": "user", "content": "Hello!"},
		}),
	}

	out, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.NotNil(t, out)

	assert.Equal(t, "gpt-4o", out.Model)
	require.Len(t, out.Messages, 2)
	assert.Equal(t, "system", out.Messages[0].Role)
	assert.Equal(t, "You are helpful.", out.Messages[0].Content)
	assert.Equal(t, "user", out.Messages[1].Role)
	assert.Equal(t, "Hello!", out.Messages[1].Content)
}

// ---------- Test 2: Instructions Mapping ----------

func TestInstructionsMapping(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:        "gpt-4o",
		Instructions: mustRaw("Always respond in French."),
		Input: mustRaw([]map[string]any{
			{"role": "user", "content": "Hello!"},
		}),
	}

	out, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.NotNil(t, out)

	// instructions → first system message
	require.Len(t, out.Messages, 2)
	assert.Equal(t, "system", out.Messages[0].Role)
	assert.Equal(t, "Always respond in French.", out.Messages[0].Content)
	assert.Equal(t, "user", out.Messages[1].Role)
}

// ---------- Test 3: Function Call Merging ----------

func TestFunctionCallMerging(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-4o",
		Input: mustRaw([]map[string]any{
			{"role": "user", "content": "What's the weather?"},
			{"type": "function_call", "call_id": "call_1", "name": "get_weather", "arguments": `{"city":"Paris"}`},
			{"type": "function_call", "call_id": "call_2", "name": "get_time", "arguments": `{"tz":"Europe/Paris"}`},
			{"type": "function_call_output", "call_id": "call_1", "output": "Sunny, 22°C"},
			{"type": "function_call_output", "call_id": "call_2", "output": "14:30"},
		}),
	}

	out, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.NotNil(t, out)

	// Expected messages:
	// 0: user "What's the weather?"
	// 1: assistant with tool_calls[call_1, call_2] (merged)
	// 2: tool call_1 "Sunny, 22°C"
	// 3: tool call_2 "14:30"
	require.Len(t, out.Messages, 4)

	assert.Equal(t, "user", out.Messages[0].Role)

	// Assistant message with merged tool calls
	assert.Equal(t, "assistant", out.Messages[1].Role)
	toolCalls := out.Messages[1].ParseToolCalls()
	require.Len(t, toolCalls, 2)
	assert.Equal(t, "call_1", toolCalls[0].ID)
	assert.Equal(t, "get_weather", toolCalls[0].Function.Name)
	assert.Equal(t, `{"city":"Paris"}`, toolCalls[0].Function.Arguments)
	assert.Equal(t, "call_2", toolCalls[1].ID)
	assert.Equal(t, "get_time", toolCalls[1].Function.Name)

	// Tool outputs
	assert.Equal(t, "tool", out.Messages[2].Role)
	assert.Equal(t, "call_1", out.Messages[2].ToolCallId)
	assert.Equal(t, "Sunny, 22°C", out.Messages[2].Content)

	assert.Equal(t, "tool", out.Messages[3].Role)
	assert.Equal(t, "call_2", out.Messages[3].ToolCallId)
	assert.Equal(t, "14:30", out.Messages[3].Content)
}

// ---------- Test 4: Function Call Without Assistant ----------

func TestFunctionCallWithoutAssistant(t *testing.T) {
	// function_call items at the start (no prior assistant message) → synthesize empty assistant
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-4o",
		Input: mustRaw([]map[string]any{
			{"type": "function_call", "call_id": "call_1", "name": "search", "arguments": `{"q":"test"}`},
			{"type": "function_call_output", "call_id": "call_1", "output": "result"},
		}),
	}

	out, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.NotNil(t, out)

	// Expected:
	// 0: assistant (synthesized) with tool_calls
	// 1: tool result
	require.Len(t, out.Messages, 2)

	assert.Equal(t, "assistant", out.Messages[0].Role)
	toolCalls := out.Messages[0].ParseToolCalls()
	require.Len(t, toolCalls, 1)
	assert.Equal(t, "call_1", toolCalls[0].ID)
	assert.Equal(t, "search", toolCalls[0].Function.Name)

	assert.Equal(t, "tool", out.Messages[1].Role)
	assert.Equal(t, "call_1", out.Messages[1].ToolCallId)
}

// ---------- Test 5: Function Call Output ----------

func TestFunctionCallOutput(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-4o",
		Input: mustRaw([]map[string]any{
			{"role": "assistant", "content": ""},
			{"type": "function_call", "call_id": "fc_123", "name": "calc", "arguments": "{}"},
			{"type": "function_call_output", "call_id": "fc_123", "output": "42"},
		}),
	}

	out, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.NotNil(t, out)

	// The assistant message is added first, then function_call goes to pending.
	// function_call_output flushes: always creates a new assistant with tool_calls, then adds tool result.
	require.Len(t, out.Messages, 3)

	assert.Equal(t, "assistant", out.Messages[0].Role)
	assert.Equal(t, "", out.Messages[0].Content)

	assert.Equal(t, "assistant", out.Messages[1].Role)
	toolCalls := out.Messages[1].ParseToolCalls()
	require.Len(t, toolCalls, 1)
	assert.Equal(t, "fc_123", toolCalls[0].ID)
	assert.Equal(t, "calc", toolCalls[0].Function.Name)

	assert.Equal(t, "tool", out.Messages[2].Role)
	assert.Equal(t, "fc_123", out.Messages[2].ToolCallId)
	assert.Equal(t, "42", out.Messages[2].Content)
}

// ---------- Test 6: Multi-Modal Content ----------

func TestMultiModalContent(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-4o",
		Input: mustRaw([]map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "Describe this image"},
					{"type": "input_image", "image_url": "https://example.com/img.png"},
					{"type": "input_file", "file": map[string]any{"file_data": "data:application/pdf;base64,AAA"}},
				},
			},
		}),
	}

	out, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.NotNil(t, out)

	require.Len(t, out.Messages, 1)
	msg := out.Messages[0]
	assert.Equal(t, "user", msg.Role)

	parts, ok := msg.Content.([]dto.MediaContent)
	require.True(t, ok, "content should be []dto.MediaContent")
	require.Len(t, parts, 3)

	// input_text → text
	assert.Equal(t, "text", parts[0].Type)
	assert.Equal(t, "Describe this image", parts[0].Text)

	// input_image → image_url
	assert.Equal(t, "image_url", parts[1].Type)
	assert.NotNil(t, parts[1].ImageUrl)

	// input_file → file
	assert.Equal(t, "file", parts[2].Type)
	assert.NotNil(t, parts[2].File)
}

// ---------- Test 7: Tools Format ----------

func TestToolsFormat(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-4o",
		Input: mustRaw([]map[string]any{
			{"role": "user", "content": "test"},
		}),
		Tools: mustRaw([]map[string]any{
			{
				"type":        "function",
				"name":        "get_weather",
				"description": "Get weather info",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"city": map[string]any{"type": "string"},
					},
				},
			},
		}),
	}

	out, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.NotNil(t, out)

	require.Len(t, out.Tools, 1)
	tool := out.Tools[0]
	assert.Equal(t, "function", tool.Type)
	assert.Equal(t, "get_weather", tool.Function.Name)
	assert.Equal(t, "Get weather info", tool.Function.Description)
	assert.NotNil(t, tool.Function.Parameters)
}

// ---------- Test 8: Tool Choice Format ----------

func TestToolChoiceFormat(t *testing.T) {
	t.Run("function_type_wrapping", func(t *testing.T) {
		req := &dto.OpenAIResponsesRequest{
			Model: "gpt-4o",
			Input: mustRaw([]map[string]any{
				{"role": "user", "content": "test"},
			}),
			ToolChoice: mustRaw(map[string]any{
				"type": "function",
				"name": "get_weather",
			}),
		}

		out, err := ResponsesRequestToChatCompletionsRequest(req)
		require.NoError(t, err)
		require.NotNil(t, out)

		// Should be wrapped: {"type":"function","function":{"name":"get_weather"}}
		tc, ok := out.ToolChoice.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "function", tc["type"])
		fn, ok := tc["function"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "get_weather", fn["name"])
	})

	t.Run("string_value", func(t *testing.T) {
		req := &dto.OpenAIResponsesRequest{
			Model: "gpt-4o",
			Input: mustRaw([]map[string]any{
				{"role": "user", "content": "test"},
			}),
			ToolChoice: mustRaw("auto"),
		}

		out, err := ResponsesRequestToChatCompletionsRequest(req)
		require.NoError(t, err)
		require.NotNil(t, out)

		// "auto" string passthrough
		assert.Equal(t, "auto", out.ToolChoice)
	})

	t.Run("required_value", func(t *testing.T) {
		req := &dto.OpenAIResponsesRequest{
			Model: "gpt-4o",
			Input: mustRaw([]map[string]any{
				{"role": "user", "content": "test"},
			}),
			ToolChoice: mustRaw(map[string]any{
				"type": "required",
			}),
		}

		out, err := ResponsesRequestToChatCompletionsRequest(req)
		require.NoError(t, err)
		require.NotNil(t, out)

		tc, ok := out.ToolChoice.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "required", tc["type"])
	})
}

// ---------- Test 9: Text Format To Response Format ----------

func TestTextFormatToResponseFormat(t *testing.T) {
	t.Run("json_object", func(t *testing.T) {
		req := &dto.OpenAIResponsesRequest{
			Model: "gpt-4o",
			Input: mustRaw([]map[string]any{
				{"role": "user", "content": "test"},
			}),
			Text: mustRaw(map[string]any{
				"format": map[string]any{
					"type": "json_object",
				},
			}),
		}

		out, err := ResponsesRequestToChatCompletionsRequest(req)
		require.NoError(t, err)
		require.NotNil(t, out)

		require.NotNil(t, out.ResponseFormat)
		assert.Equal(t, "json_object", out.ResponseFormat.Type)
	})

	t.Run("json_schema", func(t *testing.T) {
		req := &dto.OpenAIResponsesRequest{
			Model: "gpt-4o",
			Input: mustRaw([]map[string]any{
				{"role": "user", "content": "test"},
			}),
			Text: mustRaw(map[string]any{
				"format": map[string]any{
					"type": "json_schema",
					"name": "my_schema",
					"schema": map[string]any{
						"type": "object",
					},
				},
			}),
		}

		out, err := ResponsesRequestToChatCompletionsRequest(req)
		require.NoError(t, err)
		require.NotNil(t, out)

		require.NotNil(t, out.ResponseFormat)
		assert.Equal(t, "json_schema", out.ResponseFormat.Type)
		assert.NotNil(t, out.ResponseFormat.JsonSchema)
	})

	t.Run("text_format", func(t *testing.T) {
		req := &dto.OpenAIResponsesRequest{
			Model: "gpt-4o",
			Input: mustRaw([]map[string]any{
				{"role": "user", "content": "test"},
			}),
			Text: mustRaw(map[string]any{
				"format": map[string]any{
					"type": "text",
				},
			}),
		}

		out, err := ResponsesRequestToChatCompletionsRequest(req)
		require.NoError(t, err)
		require.NotNil(t, out)

		require.NotNil(t, out.ResponseFormat)
		assert.Equal(t, "text", out.ResponseFormat.Type)
	})
}

// ---------- Test 10: Direct Field Mapping ----------

func TestDirectFieldMapping(t *testing.T) {
	stream := true
	temp := 0.7
	topP := 0.9
	maxTokens := uint(1024)
	topLogProbs := 5

	req := &dto.OpenAIResponsesRequest{
		Model:            "gpt-4o",
		Input:            mustRaw([]map[string]any{{"role": "user", "content": "hi"}}),
		Stream:           &stream,
		Temperature:      &temp,
		TopP:             &topP,
		MaxOutputTokens:  &maxTokens,
		TopLogProbs:      &topLogProbs,
		User:             mustRaw("user-123"),
		Store:            mustRaw(true),
		Metadata:         mustRaw(map[string]any{"key": "value"}),
	}

	out, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.NotNil(t, out)

	assert.Equal(t, "gpt-4o", out.Model)
	require.NotNil(t, out.Stream)
	assert.True(t, *out.Stream)
	require.NotNil(t, out.Temperature)
	assert.InDelta(t, 0.7, *out.Temperature, 0.001)
	require.NotNil(t, out.TopP)
	assert.InDelta(t, 0.9, *out.TopP, 0.001)
	require.NotNil(t, out.MaxCompletionTokens)
	assert.Equal(t, uint(1024), *out.MaxCompletionTokens)
	require.NotNil(t, out.TopLogProbs)
	assert.Equal(t, 5, *out.TopLogProbs)
	assert.Equal(t, mustRaw("user-123"), out.User)
	assert.Equal(t, mustRaw(true), out.Store)
}

// ---------- Test 11: Previous Response ID Error ----------

func TestPreviousResponseIdError(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:               "gpt-4o",
		PreviousResponseID:  "resp_abc123",
		Input:               mustRaw("hello"),
	}

	out, err := ResponsesRequestToChatCompletionsRequest(req)
	assert.Nil(t, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "previous_response_id")
}

// ---------- Test 12: Unsupported Fields Stripped ----------

func TestUnsupportedFieldsStripped(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:            "gpt-4o",
		Input:            mustRaw([]map[string]any{{"role": "user", "content": "hi"}}),
		ContextManagement: mustRaw(map[string]any{"enabled": true}),
		Include:          mustRaw([]string{"message"}),
		Conversation:     mustRaw(map[string]any{}),
		Truncation:       mustRaw("auto"),
		MaxToolCalls:     func() *uint { v := uint(5); return &v }(),
		Preset:           mustRaw("default"),
	}

	// Should not error — unsupported fields are silently dropped
	out, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.NotNil(t, out)

	// Verify the conversion still works with basic fields
	assert.Equal(t, "gpt-4o", out.Model)
	require.Len(t, out.Messages, 1)
	assert.Equal(t, "user", out.Messages[0].Role)
}

// ---------- Test 13: Reasoning Mapping ----------

func TestReasoningMapping(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "o3-mini",
		Input: mustRaw([]map[string]any{{"role": "user", "content": "Solve this"}}),
		Reasoning: &dto.Reasoning{
			Effort: "high",
		},
	}

	out, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.NotNil(t, out)

	assert.Equal(t, "high", out.ReasoningEffort)
}

// ---------- Test 14: Stream Options Passthrough ----------

func TestStreamOptionsPassthrough(t *testing.T) {
	stream := true
	req := &dto.OpenAIResponsesRequest{
		Model:  "gpt-4o",
		Input:  mustRaw([]map[string]any{{"role": "user", "content": "hi"}}),
		Stream: &stream,
		StreamOptions: &dto.StreamOptions{
			IncludeUsage: true,
		},
	}

	out, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.NotNil(t, out)

	require.NotNil(t, out.StreamOptions)
	assert.True(t, out.StreamOptions.IncludeUsage)
}
