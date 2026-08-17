package oaichat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatCompletionsResponseToResponsesPreservesTextToolCallsAndUsage(t *testing.T) {
	chat := &dto.OpenAITextResponse{
		Id:      "chatcmpl_1",
		Model:   "gpt-test",
		Created: 456,
		Choices: []dto.OpenAITextResponseChoice{
			{
				Message:      assistantMessageWithTool("I will call.", "call_1", "lookup", `{"q":"x"}`),
				FinishReason: "tool_calls",
			},
		},
		Usage: dto.Usage{PromptTokens: 3, CompletionTokens: 5, TotalTokens: 8},
	}

	resp, usage, err := ChatCompletionsResponseToResponsesResponse(chat, "resp_1", nil)
	require.NoError(t, err)
	require.NotNil(t, usage)

	assert.Equal(t, "resp_1", resp.ID)
	assert.Equal(t, "response", resp.Object)
	assert.Equal(t, `"completed"`, string(resp.Status))
	assert.Equal(t, 3, resp.Usage.InputTokens)
	assert.Equal(t, 5, resp.Usage.OutputTokens)
	require.Len(t, resp.Output, 2)
	assert.Equal(t, responsesOutputTypeMessage, resp.Output[0].Type)
	assert.Equal(t, "I will call.", resp.Output[0].Content[0].Text)
	assert.Equal(t, responsesOutputTypeFunctionCall, resp.Output[1].Type)
	assert.Equal(t, "call_1", resp.Output[1].CallId)
	assert.Equal(t, "lookup", resp.Output[1].Name)
	assert.Equal(t, `"{\"q\":\"x\"}"`, string(resp.Output[1].Arguments))
}

func TestChatCompletionsResponseToResponsesMapsIncompleteFinishReasons(t *testing.T) {
	tests := []struct {
		name         string
		finishReason string
		wantReason   string
	}{
		{name: "length", finishReason: "length", wantReason: responsesIncompleteReasonMaxTokens},
		{name: "content filter", finishReason: "content_filter", wantReason: responsesIncompleteReasonContentFilter},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, _, err := ChatCompletionsResponseToResponsesResponse(&dto.OpenAITextResponse{
				Id:    "chatcmpl_1",
				Model: "gpt-test",
				Choices: []dto.OpenAITextResponseChoice{
					{
						Message:      dto.Message{Role: "assistant", Content: "partial"},
						FinishReason: tt.finishReason,
					},
				},
			}, "resp_1", nil)
			require.NoError(t, err)

			assert.Equal(t, `"incomplete"`, string(resp.Status))
			require.NotNil(t, resp.IncompleteDetails)
			assert.Equal(t, tt.wantReason, resp.IncompleteDetails.Reason)
			require.Len(t, resp.Output, 1)
			assert.Equal(t, "incomplete", resp.Output[0].Status)
		})
	}
}

func TestChatCompletionsStreamToResponsesEventsAggregatesUsageAndToolArgs(t *testing.T) {
	state := NewChatToResponsesStreamState("resp_1", "gpt-test")
	state.Created = 123
	toolIndex := 0

	var events []ChatToResponsesStreamEvent
	events = append(events, mustResponsesEventsFromChatChunk(t, state, &dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl_1",
		Model:   "gpt-test",
		Created: 123,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{Index: 0, Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Role: "assistant"}},
		},
	})...)
	events = append(events, mustResponsesEventsFromChatChunk(t, state, &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{Index: 0, Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Content: lo.ToPtr("hello")}},
		},
	})...)
	events = append(events, mustResponsesEventsFromChatChunk(t, state, &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{Index: 0, Delta: dto.ChatCompletionsStreamResponseChoiceDelta{ToolCalls: []dto.ToolCallResponse{
				{Index: &toolIndex, ID: "call_1", Type: "function", Function: dto.FunctionResponse{Name: "lookup"}},
			}}},
		},
	})...)
	events = append(events, mustResponsesEventsFromChatChunk(t, state, &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{Index: 0, Delta: dto.ChatCompletionsStreamResponseChoiceDelta{ToolCalls: []dto.ToolCallResponse{
				{Index: &toolIndex, Function: dto.FunctionResponse{Arguments: `{"q":"x"}`}},
			}}},
		},
	})...)
	finishReason := "tool_calls"
	events = append(events, mustResponsesEventsFromChatChunk(t, state, &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{Index: 0, FinishReason: &finishReason},
		},
	})...)
	events = append(events, mustResponsesEventsFromChatChunk(t, state, &dto.ChatCompletionsStreamResponse{
		Usage: &dto.Usage{PromptTokens: 2, CompletionTokens: 4, TotalTokens: 6},
	})...)
	events = append(events, FinalizeChatCompletionsStreamToResponses(state)...)

	require.Len(t, events, 10)
	assert.Equal(t, responsesEventCreated, events[0].Type)
	assert.Equal(t, responsesEventOutputTextDelta, events[2].Type)
	assert.Equal(t, "hello", events[2].Payload.Delta)
	assert.Equal(t, responsesEventFunctionArgsDelta, events[4].Type)
	assert.Equal(t, `{"q":"x"}`, events[4].Payload.Delta)
	assert.Equal(t, responsesEventCompleted, events[9].Type)
	require.NotNil(t, events[9].Payload.Response)
	assert.Equal(t, 6, events[9].Payload.Response.Usage.TotalTokens)
	require.Len(t, events[9].Payload.Response.Output, 2)
	assert.Equal(t, "hello", events[9].Payload.Response.Output[0].Content[0].Text)
	assert.Equal(t, `"{\"q\":\"x\"}"`, string(events[9].Payload.Response.Output[1].Arguments))
}

func mustResponsesEventsFromChatChunk(t *testing.T, state *ChatToResponsesStreamState, chunk *dto.ChatCompletionsStreamResponse) []ChatToResponsesStreamEvent {
	t.Helper()
	events, err := ChatCompletionsStreamChunkToResponsesEvents(chunk, state)
	require.NoError(t, err)
	return events
}

func TestChatCompletionsResponseToResponsesUsesOpenAIItemIDPrefixes(t *testing.T) {
	resp, _, err := ChatCompletionsResponseToResponsesResponse(&dto.OpenAITextResponse{
		Id:    "chatcmpl-idtest",
		Model: "gpt-test",
		Choices: []dto.OpenAITextResponseChoice{
			{Message: dto.Message{Role: "assistant", Content: "test"}, FinishReason: "stop"},
		},
	}, "resp_idtest", nil)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Output)
	assert.True(t, strings.HasPrefix(resp.Output[0].ID, "msg_"), "message ID %q", resp.Output[0].ID)

	resp2, _, err := ChatCompletionsResponseToResponsesResponse(&dto.OpenAITextResponse{
		Id:    "chatcmpl-fctest",
		Model: "gpt-test",
		Choices: []dto.OpenAITextResponseChoice{
			{
				Message:      assistantMessageWithTool("", "call_fc_test", "test_func", `{}`),
				FinishReason: "tool_calls",
			},
		},
	}, "resp_fctest", nil)
	require.NoError(t, err)
	var found bool
	for _, out := range resp2.Output {
		if out.Type == responsesOutputTypeFunctionCall {
			assert.True(t, strings.HasPrefix(out.ID, "fc_"), "function_call ID %q", out.ID)
			assert.Equal(t, "call_fc_test", out.CallId)
			found = true
		}
	}
	require.True(t, found, "expected a function_call output item")
}

func TestChatCompletionsResponseToResponsesEchoesRequestFields(t *testing.T) {
	temp := 0.7
	topP := 0.9
	maxTokens := uint(1024)
	origReq := &dto.OpenAIResponsesRequest{
		Instructions:    json.RawMessage(`"You are a helpful assistant"`),
		Temperature:     &temp,
		TopP:            &topP,
		MaxOutputTokens: &maxTokens,
		ToolChoice:      json.RawMessage(`"auto"`),
		User:            json.RawMessage(`"user-123"`),
		Metadata:        json.RawMessage(`{"key":"value"}`),
		Store:           json.RawMessage(`true`),
		Truncation:      json.RawMessage(`"auto"`),
	}

	resp, _, err := ChatCompletionsResponseToResponsesResponse(&dto.OpenAITextResponse{
		Id:    "chatcmpl-echo",
		Model: "gpt-test",
		Choices: []dto.OpenAITextResponseChoice{
			{Message: dto.Message{Role: "assistant", Content: "echo test"}, FinishReason: "stop"},
		},
	}, "resp_echo", origReq)
	require.NoError(t, err)

	assert.Equal(t, `"You are a helpful assistant"`, string(resp.Instructions))
	assert.Equal(t, 0.7, resp.Temperature)
	assert.Equal(t, 0.9, resp.TopP)
	assert.Equal(t, 1024, resp.MaxOutputTokens)
	assert.Equal(t, `"auto"`, string(resp.ToolChoice))
	assert.Equal(t, `"user-123"`, string(resp.User))
	assert.Equal(t, `{"key":"value"}`, string(resp.Metadata))
	assert.True(t, resp.Store)
	assert.Equal(t, `"auto"`, string(resp.Truncation))
}

func TestChatCompletionsResponseToResponsesEchoesReasoning(t *testing.T) {
	origReq := &dto.OpenAIResponsesRequest{
		Reasoning: &dto.Reasoning{Effort: "high", Summary: "auto"},
	}
	resp, _, err := ChatCompletionsResponseToResponsesResponse(&dto.OpenAITextResponse{
		Id:    "chatcmpl-reasoning",
		Model: "gpt-test",
		Choices: []dto.OpenAITextResponseChoice{
			{Message: dto.Message{Role: "assistant", Content: "reasoned answer"}, FinishReason: "stop"},
		},
	}, "resp_reasoning", origReq)
	require.NoError(t, err)
	require.NotNil(t, resp.Reasoning)
	assert.Equal(t, "high", resp.Reasoning.Effort)
	assert.Equal(t, "auto", resp.Reasoning.Summary)
}

func TestChatCompletionsResponseToResponsesParsesFunctionArgumentsString(t *testing.T) {
	resp, _, err := ChatCompletionsResponseToResponsesResponse(&dto.OpenAITextResponse{
		Id:    "chatcmpl-tool123",
		Model: "gpt-test",
		Choices: []dto.OpenAITextResponseChoice{
			{
				Message:      assistantMessageWithTool("", "call_abc", "get_weather", `{"city":"Tokyo"}`),
				FinishReason: "tool_calls",
			},
		},
	}, "resp_tool_test", nil)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Output)
	out := resp.Output[0]
	if out.Type != responsesOutputTypeFunctionCall {
		out = resp.Output[len(resp.Output)-1]
	}
	var argsStr string
	require.NoError(t, kitutil.Unmarshal(out.Arguments, &argsStr))
	var args map[string]string
	require.NoError(t, kitutil.Unmarshal([]byte(argsStr), &args))
	assert.Equal(t, "Tokyo", args["city"])
}
