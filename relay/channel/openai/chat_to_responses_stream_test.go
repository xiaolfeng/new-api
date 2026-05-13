package openai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

func init() {
	constant.StreamingTimeout = 60
}

// buildSSEBody constructs an io.ReadCloser from SSE lines.
// Each element in `chunks` is the JSON payload for a `data:` line.
// A `[DONE]` sentinel is appended automatically.
func buildSSEBody(chunks []string) io.ReadCloser {
	var buf bytes.Buffer
	for _, chunk := range chunks {
		buf.WriteString("data: ")
		buf.WriteString(chunk)
		buf.WriteString("\n")
	}
	buf.WriteString("data: [DONE]\n")
	return io.NopCloser(&buf)
}

// setupGinContext creates a gin.Context backed by an httptest recorder.
func setupGinContext(requestID string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(common.RequestIdKey, requestID)
	return c, w
}

// setupRelayInfo creates a minimal RelayInfo for testing.
func setupRelayInfo(modelName string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: modelName,
		},
	}
}

// parseSSEEvents extracts event type and data pairs from the SSE output.
func parseSSEEvents(body string) []struct {
	EventType string
	Data      string
} {
	var events []struct {
		EventType string
		Data      string
	}
	scanner := bufio.NewScanner(strings.NewReader(body))
	var currentEvent string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			events = append(events, struct {
				EventType string
				Data      string
			}{
				EventType: currentEvent,
				Data:      strings.TrimPrefix(line, "data: "),
			})
		}
	}
	return events
}

// TestPlainTextStream verifies a plain text SSE stream is converted correctly.
func TestPlainTextStream(t *testing.T) {
	content := "Hello"
	chunks := []string{
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Id:      "chatcmpl-test",
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   "gpt-4o",
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
										Role: "assistant",
					},
				},
			},
		}),
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Id:      "chatcmpl-test",
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   "gpt-4o",
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
						Content: ptrStr(content),
					},
				},
			},
		}),
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Id:      "chatcmpl-test",
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   "gpt-4o",
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index:        0,
					FinishReason: ptrStr("stop"),
					Delta:        dto.ChatCompletionsStreamResponseChoiceDelta{},
				},
			},
		}),
	}

	c, w := setupGinContext("test-req-123")
	info := setupRelayInfo("gpt-4o")
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       buildSSEBody(chunks),
	}

	usage, apiErr := ChatCompletionsStreamToResponsesHandler(c, info, resp, nil)
	if apiErr != nil {
		t.Fatalf("unexpected error: %v", apiErr)
	}
	if usage == nil {
		t.Fatal("expected non-nil usage")
	}

	events := parseSSEEvents(w.Body.String())

	// Should have: response.created, output_item.added, content_part.added,
	// output_text.delta, output_text.done, content_part.done, output_item.done, response.completed
	eventTypes := make([]string, len(events))
	for i, e := range events {
		eventTypes[i] = e.EventType
	}

	assertContains(t, eventTypes, "response.created")
	assertContains(t, eventTypes, "response.output_item.added")
	assertContains(t, eventTypes, "response.content_part.added")
	assertContains(t, eventTypes, "response.output_text.delta")
	assertContains(t, eventTypes, "response.output_text.done")
	assertContains(t, eventTypes, "response.content_part.done")
	assertContains(t, eventTypes, "response.output_item.done")
	assertContains(t, eventTypes, "response.completed")

	// Verify the text delta content
	for _, e := range events {
		if e.EventType == "response.output_text.delta" {
			var sseResp dto.ResponsesStreamResponse
			if err := common.UnmarshalJsonStr(e.Data, &sseResp); err != nil {
				t.Fatalf("failed to unmarshal delta: %v", err)
			}
			if sseResp.Delta != content {
				t.Errorf("expected delta %q, got %q", content, sseResp.Delta)
			}
		}
	}
}

// TestToolCallStream verifies tool call SSE events are converted correctly.
func TestToolCallStream(t *testing.T) {
	idx := 0
	chunks := []string{
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Id:      "chatcmpl-tc",
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   "gpt-4o",
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
										Role: "assistant",
					},
				},
			},
		}),
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Id:      "chatcmpl-tc",
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   "gpt-4o",
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
						ToolCalls: []dto.ToolCallResponse{
							{
								ID:   "call_tc_1",
								Type: "function",
								Function: dto.FunctionResponse{
									Name:      "get_weather",
									Arguments: `{"ci`,
								},
							},
						},
					},
				},
			},
		}),
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Id:      "chatcmpl-tc",
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   "gpt-4o",
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
						ToolCalls: []dto.ToolCallResponse{
							{
								Index: &idx,
								Function: dto.FunctionResponse{
									Arguments: `ty":"SF"}`,
								},
							},
						},
					},
				},
			},
		}),
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Id:      "chatcmpl-tc",
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   "gpt-4o",
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index:        0,
					FinishReason: ptrStr("tool_calls"),
					Delta:        dto.ChatCompletionsStreamResponseChoiceDelta{},
				},
			},
		}),
	}

	c, w := setupGinContext("tc-req-123")
	info := setupRelayInfo("gpt-4o")
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       buildSSEBody(chunks),
	}

	usage, apiErr := ChatCompletionsStreamToResponsesHandler(c, info, resp, nil)
	if apiErr != nil {
		t.Fatalf("unexpected error: %v", apiErr)
	}
	if usage == nil {
		t.Fatal("expected non-nil usage")
	}

	events := parseSSEEvents(w.Body.String())
	eventTypes := make([]string, len(events))
	for i, e := range events {
		eventTypes[i] = e.EventType
	}

	assertContains(t, eventTypes, "response.output_item.added")
	assertContains(t, eventTypes, "response.function_call_arguments.delta")
	assertContains(t, eventTypes, "response.function_call_arguments.done")
	assertContains(t, eventTypes, "response.output_item.done")
	assertContains(t, eventTypes, "response.completed")

	// Verify the function_call output_item.added has correct name
	for _, e := range events {
		if e.EventType == "response.output_item.added" {
			var sseResp dto.ResponsesStreamResponse
			if err := common.UnmarshalJsonStr(e.Data, &sseResp); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}
			if sseResp.Item != nil && sseResp.Item.Type == "function_call" {
				if sseResp.Item.Name != "get_weather" {
					t.Errorf("expected function name 'get_weather', got %q", sseResp.Item.Name)
				}
				if sseResp.Item.CallId != "call_tc_1" {
					t.Errorf("expected CallId 'call_tc_1', got %q", sseResp.Item.CallId)
				}
			}
		}
	}
}

// TestReasoningStream verifies reasoning_content SSE events.
func TestReasoningStream(t *testing.T) {
	chunks := []string{
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Id:      "chatcmpl-reason",
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   "gpt-4o",
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
										Role: "assistant",
					},
				},
			},
		}),
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Id:      "chatcmpl-reason",
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   "gpt-4o",
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
						ReasoningContent: ptrStr("Let me think..."),
					},
				},
			},
		}),
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Id:      "chatcmpl-reason",
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   "gpt-4o",
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
						Content: ptrStr("The answer is 42."),
					},
				},
			},
		}),
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Id:      "chatcmpl-reason",
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   "gpt-4o",
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index:        0,
					FinishReason: ptrStr("stop"),
					Delta:        dto.ChatCompletionsStreamResponseChoiceDelta{},
				},
			},
		}),
	}

	c, w := setupGinContext("reason-req-123")
	info := setupRelayInfo("gpt-4o")
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       buildSSEBody(chunks),
	}

	_, apiErr := ChatCompletionsStreamToResponsesHandler(c, info, resp, nil)
	if apiErr != nil {
		t.Fatalf("unexpected error: %v", apiErr)
	}

	events := parseSSEEvents(w.Body.String())
	eventTypes := make([]string, len(events))
	for i, e := range events {
		eventTypes[i] = e.EventType
	}

	assertContains(t, eventTypes, "response.reasoning_summary_text.delta")
	assertContains(t, eventTypes, "response.output_text.delta")
	assertContains(t, eventTypes, "response.completed")

	// Verify reasoning delta content
	for _, e := range events {
		if e.EventType == "response.reasoning_summary_text.delta" {
			var sseResp dto.ResponsesStreamResponse
			if err := common.UnmarshalJsonStr(e.Data, &sseResp); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}
			if sseResp.Delta != "Let me think..." {
				t.Errorf("expected reasoning delta 'Let me think...', got %q", sseResp.Delta)
			}
		}
	}
}

// TestMixedContentStream verifies text + reasoning mixed content.
func TestMixedContentStream(t *testing.T) {
	chunks := []string{
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Id:      "chatcmpl-mix",
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   "gpt-4o",
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
										Role: "assistant",
					},
				},
			},
		}),
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
						ReasoningContent: ptrStr("step 1"),
					},
				},
			},
		}),
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
						ReasoningContent: ptrStr("step 2"),
					},
				},
			},
		}),
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
						Content: ptrStr("final answer"),
					},
				},
			},
		}),
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index:        0,
					FinishReason: ptrStr("stop"),
					Delta:        dto.ChatCompletionsStreamResponseChoiceDelta{},
				},
			},
		}),
	}

	c, w := setupGinContext("mix-req-123")
	info := setupRelayInfo("gpt-4o")
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       buildSSEBody(chunks),
	}

	_, apiErr := ChatCompletionsStreamToResponsesHandler(c, info, resp, nil)
	if apiErr != nil {
		t.Fatalf("unexpected error: %v", apiErr)
	}

	events := parseSSEEvents(w.Body.String())

	reasoningCount := 0
	textCount := 0
	for _, e := range events {
		if e.EventType == "response.reasoning_summary_text.delta" {
			reasoningCount++
		}
		if e.EventType == "response.output_text.delta" {
			textCount++
		}
	}

	if reasoningCount != 2 {
		t.Errorf("expected 2 reasoning deltas, got %d", reasoningCount)
	}
	if textCount != 1 {
		t.Errorf("expected 1 text delta, got %d", textCount)
	}
}

// TestMultiToolCallStream verifies multiple tool calls with correct indexing.
func TestMultiToolCallStream(t *testing.T) {
	idx0, idx1 := 0, 1
	chunks := []string{
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
										Role: "assistant",
					},
				},
			},
		}),
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
						ToolCalls: []dto.ToolCallResponse{
							{
								ID:    "call_1",
								Type:  "function",
								Index: &idx0,
								Function: dto.FunctionResponse{
									Name:      "func_a",
									Arguments: `{"a":1}`,
								},
							},
						},
					},
				},
			},
		}),
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
						ToolCalls: []dto.ToolCallResponse{
							{
								ID:    "call_2",
								Type:  "function",
								Index: &idx1,
								Function: dto.FunctionResponse{
									Name:      "func_b",
									Arguments: `{"b":2}`,
								},
							},
						},
					},
				},
			},
		}),
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index:        0,
					FinishReason: ptrStr("tool_calls"),
					Delta:        dto.ChatCompletionsStreamResponseChoiceDelta{},
				},
			},
		}),
	}

	c, w := setupGinContext("multi-tc-req")
	info := setupRelayInfo("gpt-4o")
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       buildSSEBody(chunks),
	}

	_, apiErr := ChatCompletionsStreamToResponsesHandler(c, info, resp, nil)
	if apiErr != nil {
		t.Fatalf("unexpected error: %v", apiErr)
	}

	events := parseSSEEvents(w.Body.String())

	// Count function_call output_item.added events
	fcAddedCount := 0
	fcNames := make(map[string]bool)
	for _, e := range events {
		if e.EventType == "response.output_item.added" {
			var sseResp dto.ResponsesStreamResponse
			if err := common.UnmarshalJsonStr(e.Data, &sseResp); err != nil {
				continue
			}
			if sseResp.Item != nil && sseResp.Item.Type == "function_call" {
				fcAddedCount++
				fcNames[sseResp.Item.Name] = true
			}
		}
	}

	if fcAddedCount != 2 {
		t.Errorf("expected 2 function_call output_item.added events, got %d", fcAddedCount)
	}
	if !fcNames["func_a"] {
		t.Error("expected func_a in tool call names")
	}
	if !fcNames["func_b"] {
		t.Error("expected func_b in tool call names")
	}
}

// TestEmptyStream verifies that empty delta chunks are skipped.
func TestEmptyStream(t *testing.T) {
	chunks := []string{
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
										Role: "assistant",
					},
				},
			},
		}),
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{},
				},
			},
		}),
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
						Content: ptrStr(""),
					},
				},
			},
		}),
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index:        0,
					FinishReason: ptrStr("stop"),
					Delta:        dto.ChatCompletionsStreamResponseChoiceDelta{},
				},
			},
		}),
	}

	c, w := setupGinContext("empty-req")
	info := setupRelayInfo("gpt-4o")
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       buildSSEBody(chunks),
	}

	_, apiErr := ChatCompletionsStreamToResponsesHandler(c, info, resp, nil)
	if apiErr != nil {
		t.Fatalf("unexpected error: %v", apiErr)
	}

	events := parseSSEEvents(w.Body.String())
	eventTypes := make([]string, len(events))
	for i, e := range events {
		eventTypes[i] = e.EventType
	}

	assertContains(t, eventTypes, "response.created")
	assertContains(t, eventTypes, "response.completed")

	// Empty content deltas should not produce output_text.delta events
	for _, et := range eventTypes {
		if et == "response.output_text.delta" {
			t.Error("empty delta should not produce output_text.delta event")
		}
	}
}

// TestUsageInCompleted verifies that usage data appears in response.completed.
func TestUsageInCompleted(t *testing.T) {
	chunks := []string{
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
										Role: "assistant",
					},
				},
			},
		}),
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
						Content: ptrStr("hi"),
					},
				},
			},
		}),
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index:        0,
					FinishReason: ptrStr("stop"),
					Delta:        dto.ChatCompletionsStreamResponseChoiceDelta{},
				},
			},
			Usage: &dto.Usage{
				PromptTokens:     50,
				CompletionTokens: 10,
				TotalTokens:      60,
			},
		}),
	}

	c, w := setupGinContext("usage-req")
	info := setupRelayInfo("gpt-4o")
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       buildSSEBody(chunks),
	}

	usage, apiErr := ChatCompletionsStreamToResponsesHandler(c, info, resp, nil)
	if apiErr != nil {
		t.Fatalf("unexpected error: %v", apiErr)
	}

	if usage.PromptTokens != 50 {
		t.Errorf("expected PromptTokens 50, got %d", usage.PromptTokens)
	}
	if usage.CompletionTokens != 10 {
		t.Errorf("expected CompletionTokens 10, got %d", usage.CompletionTokens)
	}

	// Also verify usage in the response.completed event
	events := parseSSEEvents(w.Body.String())
	for _, e := range events {
		if e.EventType == "response.completed" {
			var sseResp dto.ResponsesStreamResponse
			if err := common.UnmarshalJsonStr(e.Data, &sseResp); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}
			if sseResp.Response == nil {
				t.Fatal("expected Response in response.completed")
			}
			if sseResp.Response.Usage == nil {
				t.Fatal("expected Usage in response.completed")
			}
			if sseResp.Response.Usage.PromptTokens != 50 {
				t.Errorf("expected PromptTokens 50 in completed, got %d", sseResp.Response.Usage.PromptTokens)
			}
		}
	}
}

// TestErrorStream verifies that upstream errors are handled.
func TestErrorStream(t *testing.T) {
	// Send an invalid JSON chunk that will cause unmarshal error
	chunks := []string{
		`{invalid json`,
	}

	c, w := setupGinContext("err-req")
	info := setupRelayInfo("gpt-4o")
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       buildSSEBody(chunks),
	}

	_, _ = ChatCompletionsStreamToResponsesHandler(c, info, resp, nil)

	events := parseSSEEvents(w.Body.String())
	eventTypes := make([]string, len(events))
	for i, e := range events {
		eventTypes[i] = e.EventType
	}

	// Should still emit response.created and response.completed (with "failed" or "completed")
	assertContains(t, eventTypes, "response.created")
	assertContains(t, eventTypes, "response.completed")
}

// TestNilResponse verifies nil response handling.
func TestNilResponse(t *testing.T) {
	c, _ := setupGinContext("nil-req")
	info := setupRelayInfo("gpt-4o")

	_, apiErr := ChatCompletionsStreamToResponsesHandler(c, info, nil, nil)
	if apiErr == nil {
		t.Fatal("expected error for nil response")
	}
}

// TestStreamWithMetadata verifies that metadata from origReq is passed through.
func TestStreamWithMetadata(t *testing.T) {
	chunks := []string{
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
										Role: "assistant",
					},
				},
			},
		}),
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
						Content: ptrStr("test"),
					},
				},
			},
		}),
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index:        0,
					FinishReason: ptrStr("stop"),
					Delta:        dto.ChatCompletionsStreamResponseChoiceDelta{},
				},
			},
		}),
	}

	c, w := setupGinContext("meta-req")
	info := setupRelayInfo("gpt-4o")
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       buildSSEBody(chunks),
	}

	origReq := &dto.OpenAIResponsesRequest{
		Metadata: json.RawMessage(`{"session":"abc"}`),
	}

	_, apiErr := ChatCompletionsStreamToResponsesHandler(c, info, resp, origReq)
	if apiErr != nil {
		t.Fatalf("unexpected error: %v", apiErr)
	}

	events := parseSSEEvents(w.Body.String())
	for _, e := range events {
		if e.EventType == "response.created" {
			var sseResp dto.ResponsesStreamResponse
			if err := common.UnmarshalJsonStr(e.Data, &sseResp); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}
			if sseResp.Response == nil {
				t.Fatal("expected Response in response.created")
			}
			if string(sseResp.Response.Metadata) != `{"session":"abc"}` {
				t.Errorf("expected metadata echoed, got %q", string(sseResp.Response.Metadata))
			}
		}
	}
}

// TestIncompleteStream verifies finish_reason="length" produces "incomplete" status.
func TestIncompleteStream(t *testing.T) {
	chunks := []string{
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
										Role: "assistant",
					},
				},
			},
		}),
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
						Content: ptrStr("truncated..."),
					},
				},
			},
		}),
		mustMarshal(dto.ChatCompletionsStreamResponse{
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index:        0,
					FinishReason: ptrStr("length"),
					Delta:        dto.ChatCompletionsStreamResponseChoiceDelta{},
				},
			},
		}),
	}

	c, w := setupGinContext("inc-stream-req")
	info := setupRelayInfo("gpt-4o")
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       buildSSEBody(chunks),
	}

	_, apiErr := ChatCompletionsStreamToResponsesHandler(c, info, resp, nil)
	if apiErr != nil {
		t.Fatalf("unexpected error: %v", apiErr)
	}

	events := parseSSEEvents(w.Body.String())
	foundIncomplete := false
	for _, e := range events {
		if e.EventType == "response.completed" {
			var sseResp dto.ResponsesStreamResponse
			if err := common.UnmarshalJsonStr(e.Data, &sseResp); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}
			if sseResp.Response == nil {
				t.Fatal("expected Response in response.completed")
			}
			if string(sseResp.Response.Status) == `"incomplete"` {
				foundIncomplete = true
			}
		}
	}
	if !foundIncomplete {
		t.Error("expected at least one response.completed with status \"incomplete\"")
	}
}

// --- helpers ---

func ptrStr(s string) *string { return &s }

func mustMarshal(v any) string {
	data, err := common.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("mustMarshal failed: %v", err))
	}
	return string(data)
}

func assertContains(t *testing.T, slice []string, target string) {
	t.Helper()
	for _, s := range slice {
		if s == target {
			return
		}
	}
	t.Errorf("expected slice to contain %q, got %v", target, slice)
}
