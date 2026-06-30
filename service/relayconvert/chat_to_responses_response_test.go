package openaicompat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

// TestTextResponseConversion verifies that a plain text Chat Completions response
// is correctly converted to a Responses API response.
func TestTextResponseConversion(t *testing.T) {
	chatResp := &dto.OpenAITextResponse{
		Id:      "chatcmpl-abc123",
		Model:   "gpt-4o",
		Object:  "chat.completion",
		Created: 1700000000,
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index: 0,
				Message: dto.Message{
					Role:    "assistant",
					Content: "Hello, world!",
				},
				FinishReason: "stop",
			},
		},
		Usage: dto.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	resp, usage, err := ChatCompletionsResponseToResponsesResponse(chatResp, nil, "resp_test123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify basic fields
	if resp.ID != "resp_test123" {
		t.Errorf("expected ID 'resp_test123', got %q", resp.ID)
	}
	if resp.Object != "response" {
		t.Errorf("expected Object 'response', got %q", resp.Object)
	}
	if resp.Model != "gpt-4o" {
		t.Errorf("expected Model 'gpt-4o', got %q", resp.Model)
	}

	// Verify status is "completed"
	if string(resp.Status) != `"completed"` {
		t.Errorf("expected status \"completed\", got %s", string(resp.Status))
	}

	// Verify output contains one message item
	if len(resp.Output) != 1 {
		t.Fatalf("expected 1 output item, got %d", len(resp.Output))
	}
	out := resp.Output[0]
	if out.Type != "message" {
		t.Errorf("expected output type 'message', got %q", out.Type)
	}
	if out.Role != "assistant" {
		t.Errorf("expected role 'assistant', got %q", out.Role)
	}
	if out.Status != "completed" {
		t.Errorf("expected output status 'completed', got %q", out.Status)
	}

	// Verify text content
	if len(out.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(out.Content))
	}
	if out.Content[0].Type != "output_text" {
		t.Errorf("expected content type 'output_text', got %q", out.Content[0].Type)
	}
	if out.Content[0].Text != "Hello, world!" {
		t.Errorf("expected text 'Hello, world!', got %q", out.Content[0].Text)
	}

	// Verify usage
	if usage == nil {
		t.Fatal("expected non-nil usage")
	}
	if usage.PromptTokens != 10 {
		t.Errorf("expected PromptTokens 10, got %d", usage.PromptTokens)
	}
	if usage.CompletionTokens != 5 {
		t.Errorf("expected CompletionTokens 5, got %d", usage.CompletionTokens)
	}
}

// TestToolCallResponseConversion verifies that tool_calls in a Chat Completions response
// are correctly converted to function_call output items.
func TestToolCallResponseConversion(t *testing.T) {
	toolCallsJSON, _ := json.Marshal([]dto.ToolCallRequest{
		{
			ID:   "call_abc",
			Type: "function",
			Function: dto.FunctionRequest{
				Name:      "get_weather",
				Arguments: `{"city":"Tokyo"}`,
			},
		},
	})

	chatResp := &dto.OpenAITextResponse{
		Id:      "chatcmpl-tool123",
		Model:   "gpt-4o",
		Object:  "chat.completion",
		Created: 1700000000,
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index: 0,
				Message: dto.Message{
					Role:      "assistant",
					Content:   nil,
					ToolCalls: toolCallsJSON,
				},
				FinishReason: "tool_calls",
			},
		},
		Usage: dto.Usage{
			PromptTokens:     20,
			CompletionTokens: 10,
			TotalTokens:      30,
		},
	}

	resp, _, err := ChatCompletionsResponseToResponsesResponse(chatResp, nil, "resp_tool_test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 1 function_call output item (no text content since content is nil and tool_calls exist)
	if len(resp.Output) != 1 {
		t.Fatalf("expected 1 output item, got %d", len(resp.Output))
	}

	out := resp.Output[0]
	if out.Type != "function_call" {
		t.Errorf("expected output type 'function_call', got %q", out.Type)
	}
	if out.CallId != "call_abc" {
		t.Errorf("expected CallId 'call_abc', got %q", out.CallId)
	}
	if out.Name != "get_weather" {
		t.Errorf("expected Name 'get_weather', got %q", out.Name)
	}
	if out.Status != "completed" {
		t.Errorf("expected output status 'completed', got %q", out.Status)
	}

	// Verify arguments
	var args map[string]string
	if err := common.Unmarshal(out.Arguments, &args); err != nil {
		t.Fatalf("failed to unmarshal arguments: %v", err)
	}
	if args["city"] != "Tokyo" {
		t.Errorf("expected city 'Tokyo', got %q", args["city"])
	}
}

// TestUsageMapping verifies that prompt_tokens/completion_tokens are correctly
// mapped to input_tokens/output_tokens.
func TestUsageMapping(t *testing.T) {
	chatResp := &dto.OpenAITextResponse{
		Id:      "chatcmpl-usage",
		Model:   "gpt-4o",
		Object:  "chat.completion",
		Created: 1700000000,
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index: 0,
				Message: dto.Message{
					Role:    "assistant",
					Content: "hi",
				},
				FinishReason: "stop",
			},
		},
		Usage: dto.Usage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}

	_, usage, err := ChatCompletionsResponseToResponsesResponse(chatResp, nil, "resp_usage")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if usage.InputTokens != 100 {
		t.Errorf("expected InputTokens 100, got %d", usage.InputTokens)
	}
	if usage.OutputTokens != 50 {
		t.Errorf("expected OutputTokens 50, got %d", usage.OutputTokens)
	}
	if usage.PromptTokens != 100 {
		t.Errorf("expected PromptTokens 100, got %d", usage.PromptTokens)
	}
	if usage.CompletionTokens != 50 {
		t.Errorf("expected CompletionTokens 50, got %d", usage.CompletionTokens)
	}
	if usage.TotalTokens != 150 {
		t.Errorf("expected TotalTokens 150, got %d", usage.TotalTokens)
	}
}

// TestIDGeneration verifies that output items have correct ID prefixes.
func TestIDGeneration(t *testing.T) {
	chatResp := &dto.OpenAITextResponse{
		Id:      "chatcmpl-idtest",
		Model:   "gpt-4o",
		Object:  "chat.completion",
		Created: 1700000000,
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index: 0,
				Message: dto.Message{
					Role:    "assistant",
					Content: "test",
				},
				FinishReason: "stop",
			},
		},
		Usage: dto.Usage{},
	}

	resp, _, err := ChatCompletionsResponseToResponsesResponse(chatResp, nil, "resp_idtest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Response ID should be the one passed in
	if resp.ID != "resp_idtest" {
		t.Errorf("expected response ID 'resp_idtest', got %q", resp.ID)
	}

	// Message output item should have msg_ prefix
	if len(resp.Output) < 1 {
		t.Fatal("expected at least 1 output item")
	}
	if !strings.HasPrefix(resp.Output[0].ID, "msg_") {
		t.Errorf("expected message ID with 'msg_' prefix, got %q", resp.Output[0].ID)
	}

	// Now test with tool calls for fc_ prefix
	toolCallsJSON, _ := json.Marshal([]dto.ToolCallRequest{
		{
			ID:   "call_fc_test",
			Type: "function",
			Function: dto.FunctionRequest{
				Name:      "test_func",
				Arguments: `{}`,
			},
		},
	})
	chatResp2 := &dto.OpenAITextResponse{
		Id:      "chatcmpl-fctest",
		Model:   "gpt-4o",
		Object:  "chat.completion",
		Created: 1700000000,
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index: 0,
				Message: dto.Message{
					Role:      "assistant",
					Content:   nil,
					ToolCalls: toolCallsJSON,
				},
				FinishReason: "tool_calls",
			},
		},
		Usage: dto.Usage{},
	}

	resp2, _, err := ChatCompletionsResponseToResponsesResponse(chatResp2, nil, "resp_fctest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the function_call output item
	for _, out := range resp2.Output {
		if out.Type == "function_call" {
			if !strings.HasPrefix(out.ID, "fc_") {
				t.Errorf("expected function_call ID with 'fc_' prefix, got %q", out.ID)
			}
			return
		}
	}
	t.Error("expected a function_call output item but found none")
}

// TestEchoFields verifies that request fields are echoed back in the response.
func TestEchoFields(t *testing.T) {
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

	chatResp := &dto.OpenAITextResponse{
		Id:      "chatcmpl-echo",
		Model:   "gpt-4o",
		Object:  "chat.completion",
		Created: 1700000000,
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index: 0,
				Message: dto.Message{
					Role:    "assistant",
					Content: "echo test",
				},
				FinishReason: "stop",
			},
		},
		Usage: dto.Usage{},
	}

	resp, _, err := ChatCompletionsResponseToResponsesResponse(chatResp, origReq, "resp_echo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify echoed fields
	if string(resp.Instructions) != `"You are a helpful assistant"` {
		t.Errorf("expected instructions echoed, got %q", string(resp.Instructions))
	}
	if resp.Temperature != 0.7 {
		t.Errorf("expected Temperature 0.7, got %f", resp.Temperature)
	}
	if resp.TopP != 0.9 {
		t.Errorf("expected TopP 0.9, got %f", resp.TopP)
	}
	if resp.MaxOutputTokens != 1024 {
		t.Errorf("expected MaxOutputTokens 1024, got %d", resp.MaxOutputTokens)
	}
	if string(resp.ToolChoice) != `"auto"` {
		t.Errorf("expected ToolChoice echoed, got %q", string(resp.ToolChoice))
	}
	if string(resp.User) != `"user-123"` {
		t.Errorf("expected User echoed, got %q", string(resp.User))
	}
	if string(resp.Metadata) != `{"key":"value"}` {
		t.Errorf("expected Metadata echoed, got %q", string(resp.Metadata))
	}
	if !resp.Store {
		t.Errorf("expected Store true, got false")
	}
	if string(resp.Truncation) != `"auto"` {
		t.Errorf("expected Truncation echoed, got %q", string(resp.Truncation))
	}
}

// TestIncompleteResponse verifies that finish_reason="length" produces
// an "incomplete" status with IncompleteDetails.
func TestIncompleteResponse(t *testing.T) {
	chatResp := &dto.OpenAITextResponse{
		Id:      "chatcmpl-inc",
		Model:   "gpt-4o",
		Object:  "chat.completion",
		Created: 1700000000,
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index: 0,
				Message: dto.Message{
					Role:    "assistant",
					Content: "truncated text...",
				},
				FinishReason: "length",
			},
		},
		Usage: dto.Usage{},
	}

	resp, _, err := ChatCompletionsResponseToResponsesResponse(chatResp, nil, "resp_inc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(resp.Status) != `"incomplete"` {
		t.Errorf("expected status \"incomplete\", got %s", string(resp.Status))
	}
	if resp.IncompleteDetails == nil {
		t.Fatal("expected IncompleteDetails to be non-nil")
	}
	if resp.IncompleteDetails.Reasoning != "max_output_tokens" {
		t.Errorf("expected IncompleteDetails.Reasoning 'max_output_tokens', got %q", resp.IncompleteDetails.Reasoning)
	}
}

// TestEmptyResponse verifies handling of empty content responses.
func TestEmptyResponse(t *testing.T) {
	tests := []struct {
		name     string
		chatResp *dto.OpenAITextResponse
	}{
		{
			name: "empty string content",
			chatResp: &dto.OpenAITextResponse{
				Id:      "chatcmpl-empty1",
				Model:   "gpt-4o",
				Object:  "chat.completion",
				Created: 1700000000,
				Choices: []dto.OpenAITextResponseChoice{
					{
						Index: 0,
						Message: dto.Message{
							Role:    "assistant",
							Content: "",
						},
						FinishReason: "stop",
					},
				},
				Usage: dto.Usage{},
			},
		},
		{
			name: "nil content",
			chatResp: &dto.OpenAITextResponse{
				Id:      "chatcmpl-empty2",
				Model:   "gpt-4o",
				Object:  "chat.completion",
				Created: 1700000000,
				Choices: []dto.OpenAITextResponseChoice{
					{
						Index: 0,
						Message: dto.Message{
							Role:    "assistant",
							Content: nil,
						},
						FinishReason: "stop",
					},
				},
				Usage: dto.Usage{},
			},
		},
		{
			name: "no choices",
			chatResp: &dto.OpenAITextResponse{
				Id:      "chatcmpl-empty3",
				Model:   "gpt-4o",
				Object:  "chat.completion",
				Created: 1700000000,
				Choices: []dto.OpenAITextResponseChoice{},
				Usage:   dto.Usage{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, _, err := ChatCompletionsResponseToResponsesResponse(tt.chatResp, nil, "resp_empty")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp == nil {
				t.Fatal("expected non-nil response")
			}
			if resp.ID != "resp_empty" {
				t.Errorf("expected ID 'resp_empty', got %q", resp.ID)
			}
			// Status should still be "completed" for empty content
			if string(resp.Status) != `"completed"` {
				t.Errorf("expected status \"completed\", got %s", string(resp.Status))
			}
		})
	}
}

// TestNilChatResponse verifies that a nil chat response returns an error.
func TestNilChatResponse(t *testing.T) {
	resp, usage, err := ChatCompletionsResponseToResponsesResponse(nil, nil, "resp_nil")
	if err == nil {
		t.Fatal("expected error for nil chat response")
	}
	if resp != nil {
		t.Error("expected nil response")
	}
	if usage != nil {
		t.Error("expected nil usage")
	}
}

// TestToolCallWithText verifies that a response with both text and tool calls
// produces both a message output item and function_call output items.
func TestToolCallWithText(t *testing.T) {
	toolCallsJSON, _ := json.Marshal([]dto.ToolCallRequest{
		{
			ID:   "call_mixed",
			Type: "function",
			Function: dto.FunctionRequest{
				Name:      "search",
				Arguments: `{"q":"test"}`,
			},
		},
	})

	chatResp := &dto.OpenAITextResponse{
		Id:      "chatcmpl-mixed",
		Model:   "gpt-4o",
		Object:  "chat.completion",
		Created: 1700000000,
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index: 0,
				Message: dto.Message{
					Role:      "assistant",
					Content:   "Let me search for that.",
					ToolCalls: toolCallsJSON,
				},
				FinishReason: "tool_calls",
			},
		},
		Usage: dto.Usage{
			PromptTokens:     15,
			CompletionTokens: 8,
			TotalTokens:      23,
		},
	}

	resp, _, err := ChatCompletionsResponseToResponsesResponse(chatResp, nil, "resp_mixed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 2 output items: message + function_call
	if len(resp.Output) != 2 {
		t.Fatalf("expected 2 output items, got %d", len(resp.Output))
	}

	// First should be message
	if resp.Output[0].Type != "message" {
		t.Errorf("expected first output type 'message', got %q", resp.Output[0].Type)
	}
	if resp.Output[0].Content[0].Text != "Let me search for that." {
		t.Errorf("expected text 'Let me search for that.', got %q", resp.Output[0].Content[0].Text)
	}

	// Second should be function_call
	if resp.Output[1].Type != "function_call" {
		t.Errorf("expected second output type 'function_call', got %q", resp.Output[1].Type)
	}
	if resp.Output[1].Name != "search" {
		t.Errorf("expected function name 'search', got %q", resp.Output[1].Name)
	}
}

// TestMultipleToolCalls verifies handling of multiple tool calls.
func TestMultipleToolCalls(t *testing.T) {
	toolCallsJSON, _ := json.Marshal([]dto.ToolCallRequest{
		{
			ID:   "call_multi_1",
			Type: "function",
			Function: dto.FunctionRequest{
				Name:      "get_weather",
				Arguments: `{"city":"Tokyo"}`,
			},
		},
		{
			ID:   "call_multi_2",
			Type: "function",
			Function: dto.FunctionRequest{
				Name:      "get_time",
				Arguments: `{"tz":"JST"}`,
			},
		},
	})

	chatResp := &dto.OpenAITextResponse{
		Id:      "chatcmpl-multi",
		Model:   "gpt-4o",
		Object:  "chat.completion",
		Created: 1700000000,
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index: 0,
				Message: dto.Message{
					Role:      "assistant",
					Content:   nil,
					ToolCalls: toolCallsJSON,
				},
				FinishReason: "tool_calls",
			},
		},
		Usage: dto.Usage{},
	}

	resp, _, err := ChatCompletionsResponseToResponsesResponse(chatResp, nil, "resp_multi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 2 function_call output items
	if len(resp.Output) != 2 {
		t.Fatalf("expected 2 output items, got %d", len(resp.Output))
	}

	names := []string{"get_weather", "get_time"}
	callIDs := []string{"call_multi_1", "call_multi_2"}
	for i, out := range resp.Output {
		if out.Type != "function_call" {
			t.Errorf("output[%d]: expected type 'function_call', got %q", i, out.Type)
		}
		if out.Name != names[i] {
			t.Errorf("output[%d]: expected name %q, got %q", i, names[i], out.Name)
		}
		if out.CallId != callIDs[i] {
			t.Errorf("output[%d]: expected CallId %q, got %q", i, callIDs[i], out.CallId)
		}
	}
}

// TestCreatedField verifies that the Created field is properly handled
// from different types in the chat response.
func TestCreatedField(t *testing.T) {
	tests := []struct {
		name           string
		created        any
		expectNonZero  bool
	}{
		{
			name:          "int created",
			created:       1700000000,
			expectNonZero: true,
		},
		{
			name:          "int64 created",
			created:       int64(1700000000),
			expectNonZero: true,
		},
		{
			name:          "float64 created",
			created:       float64(1700000000),
			expectNonZero: true,
		},
		{
			name:          "nil created",
			created:       nil,
			expectNonZero: false, // will use time.Now().Unix()
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chatResp := &dto.OpenAITextResponse{
				Id:      "chatcmpl-created",
				Model:   "gpt-4o",
				Object:  "chat.completion",
				Created: tt.created,
				Choices: []dto.OpenAITextResponseChoice{
					{
						Index: 0,
						Message: dto.Message{
							Role:    "assistant",
							Content: "test",
						},
						FinishReason: "stop",
					},
				},
				Usage: dto.Usage{},
			}

			resp, _, err := ChatCompletionsResponseToResponsesResponse(chatResp, nil, "resp_created")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.CreatedAt <= 0 {
				t.Error("expected positive CreatedAt")
			}
		})
	}
}

// TestReasoningEcho verifies that the reasoning field is echoed from the request.
func TestReasoningEcho(t *testing.T) {
	origReq := &dto.OpenAIResponsesRequest{
		Reasoning: &dto.Reasoning{
			Effort:  "high",
			Summary: "auto",
		},
	}

	chatResp := &dto.OpenAITextResponse{
		Id:      "chatcmpl-reasoning",
		Model:   "gpt-4o",
		Object:  "chat.completion",
		Created: 1700000000,
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index: 0,
				Message: dto.Message{
					Role:    "assistant",
					Content: "reasoned answer",
				},
				FinishReason: "stop",
			},
		},
		Usage: dto.Usage{},
	}

	resp, _, err := ChatCompletionsResponseToResponsesResponse(chatResp, origReq, "resp_reasoning")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Reasoning == nil {
		t.Fatal("expected non-nil Reasoning")
	}
	if resp.Reasoning.Effort != "high" {
		t.Errorf("expected Reasoning.Effort 'high', got %q", resp.Reasoning.Effort)
	}
	if resp.Reasoning.Summary != "auto" {
		t.Errorf("expected Reasoning.Summary 'auto', got %q", resp.Reasoning.Summary)
	}
}
