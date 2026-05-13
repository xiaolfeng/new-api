package openaicompat

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

// generateID creates a prefixed random ID (e.g. "resp_xxx", "msg_xxx", "fc_xxx").
func generateID(prefix string, length int) string {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return prefix + string(b)
}

// ChatCompletionsResponseToResponsesResponse converts a Chat Completions non-stream
// response into a Responses API non-stream response.
func ChatCompletionsResponseToResponsesResponse(
	chatResp *dto.OpenAITextResponse,
	origReq *dto.OpenAIResponsesRequest,
	id string,
) (*dto.OpenAIResponsesResponse, *dto.Usage, error) {
	if chatResp == nil {
		return nil, nil, fmt.Errorf("chat response is nil")
	}

	// --- Determine status from finish_reason ---
	status := json.RawMessage(`"completed"`)
	var incompleteDetails *dto.IncompleteDetails
	if len(chatResp.Choices) > 0 && chatResp.Choices[0].FinishReason == "length" {
		status = json.RawMessage(`"incomplete"`)
		incompleteDetails = &dto.IncompleteDetails{
			Reasoning: "max_output_tokens",
		}
	}

	// --- Build output[] ---
	output := make([]dto.ResponsesOutput, 0)

	if len(chatResp.Choices) > 0 {
		choice := chatResp.Choices[0]
		msg := choice.Message

		// Text content → message output item
		textContent := msg.StringContent()
		if textContent != "" || len(msg.ParseToolCalls()) == 0 {
			output = append(output, dto.ResponsesOutput{
				Type:   "message",
				ID:     generateID("msg_", 24),
				Status: "completed",
				Role:   "assistant",
				Content: []dto.ResponsesOutputContent{
					{
						Type:        "output_text",
						Text:        textContent,
						Annotations: []interface{}{},
					},
				},
			})
		}

		// Tool calls → function_call output items
		for _, tc := range msg.ParseToolCalls() {
			argsRaw, _ := common.Marshal(tc.Function.Arguments)
			// If arguments is already a JSON string, keep it as-is; otherwise wrap.
			var testStr string
			if common.UnmarshalJsonStr(tc.Function.Arguments, &testStr) != nil {
				argsRaw = json.RawMessage(tc.Function.Arguments)
			}
			output = append(output, dto.ResponsesOutput{
				Type:      "function_call",
				ID:        generateID("fc_", 24),
				Status:    "completed",
				CallId:    tc.ID,
				Name:      tc.Function.Name,
				Arguments: argsRaw,
			})
		}
	}

	// --- Usage mapping ---
	usage := &dto.Usage{}
	usage.PromptTokens = chatResp.Usage.PromptTokens
	usage.CompletionTokens = chatResp.Usage.CompletionTokens
	usage.TotalTokens = chatResp.Usage.TotalTokens
	usage.InputTokens = chatResp.Usage.PromptTokens
	usage.OutputTokens = chatResp.Usage.CompletionTokens

	// --- Echo request fields from origReq ---
	var instructions json.RawMessage
	var tools []map[string]any
	var toolChoice json.RawMessage
	var temperature float64
	var topP float64
	var parallelToolCalls bool
	var maxOutputTokens int
	var reasoning *dto.Reasoning
	var user json.RawMessage
	var metadata json.RawMessage
	var store json.RawMessage
	var truncation json.RawMessage

	if origReq != nil {
		instructions = origReq.Instructions
		tools = origReq.GetToolsMap()
		toolChoice = origReq.ToolChoice
		if origReq.Temperature != nil {
			temperature = *origReq.Temperature
		}
		if origReq.TopP != nil {
			topP = *origReq.TopP
		}
		if len(origReq.ParallelToolCalls) > 0 {
			_ = common.Unmarshal(origReq.ParallelToolCalls, &parallelToolCalls)
		}
		if origReq.MaxOutputTokens != nil {
			maxOutputTokens = int(*origReq.MaxOutputTokens)
		}
		reasoning = origReq.Reasoning
		user = origReq.User
		metadata = origReq.Metadata
		store = origReq.Store
		truncation = origReq.Truncation
	}

	created := time.Now().Unix()
	if chatResp.Created != nil {
		switch v := chatResp.Created.(type) {
		case int:
			created = int64(v)
		case int64:
			created = v
		case float64:
			created = int64(v)
		}
	}

	resp := &dto.OpenAIResponsesResponse{
		ID:                 id,
		Object:             "response",
		CreatedAt:          int(created),
		Status:             status,
		IncompleteDetails:  incompleteDetails,
		Instructions:       instructions,
		MaxOutputTokens:    maxOutputTokens,
		Model:              chatResp.Model,
		Output:             output,
		ParallelToolCalls:  parallelToolCalls,
		Reasoning:          reasoning,
		Store:              string(store) == "true",
		Temperature:        temperature,
		ToolChoice:         toolChoice,
		Tools:              tools,
		TopP:               topP,
		Truncation:         truncation,
		Usage:              usage,
		User:               user,
		Metadata:           metadata,
	}

	return resp, usage, nil
}
