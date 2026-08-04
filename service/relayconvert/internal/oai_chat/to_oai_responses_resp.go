package oaichat

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

const (
	chatFinishReasonLength        = "length"
	chatFinishReasonContentFilter = "content_filter"

	responsesEventCreated                  = "response.created"
	responsesEventCompleted                = "response.completed"
	responsesEventIncomplete               = "response.incomplete"
	responsesEventOutputTextDelta          = "response.output_text.delta"
	responsesEventOutputItemAdded          = "response.output_item.added"
	responsesEventOutputItemDone           = "response.output_item.done"
	responsesEventFunctionArgsDelta        = "response.function_call_arguments.delta"
	responsesEventFunctionArgsDone         = "response.function_call_arguments.done"
	responsesEventReasoningSummaryDelta    = "response.reasoning_summary_text.delta"
	responsesEventReasoningSummaryDone     = "response.reasoning_summary_text.done"
	responsesOutputTypeFunctionCall        = "function_call"
	responsesOutputTypeMessage             = "message"
	responsesOutputTypeReasoning           = "reasoning"
	responsesIncompleteReasonContentFilter = "content_filter"
	responsesIncompleteReasonMaxTokens     = "max_output_tokens"
)

func ChatCompletionsResponseToResponsesResponse(resp *dto.OpenAITextResponse, id string, origReq *dto.OpenAIResponsesRequest) (*dto.OpenAIResponsesResponse, *dto.Usage, error) {
	if resp == nil {
		return nil, nil, errors.New("response is nil")
	}

	usage := UsageFromChatUsage(&resp.Usage)
	out := &dto.OpenAIResponsesResponse{
		ID:        id,
		Object:    "response",
		CreatedAt: chatCreatedAt(resp.Created),
		Status:    []byte(`"completed"`),
		Model:     resp.Model,
		Output:    make([]dto.ResponsesOutput, 0),
		Usage:     usage,
	}

	// 回显原请求参数（Responses API 语义：response.created 需携带客户端传入的
	// instructions/temperature/tool_choice 等配置字段，保证客户端拿到的响应与
	// 请求一致）。origReq 为 nil 时跳过，兼容无请求上下文的转换路径（如
	// response_registry 注册的通用格式转换器）。
	echoResponsesRequestFields(out, origReq)

	if len(resp.Choices) == 0 {
		return out, usage, nil
	}

	choice := resp.Choices[0]
	if status, details := ResponsesStatusFromChatFinishReason(choice.FinishReason); status != "" {
		out.Status = []byte(fmt.Sprintf("%q", status))
		out.IncompleteDetails = details
	}

	if text := choice.Message.StringContent(); text != "" {
		out.Output = append(out.Output, dto.ResponsesOutput{
			Type:   responsesOutputTypeMessage,
			ID:     fmt.Sprintf("%s_msg_0", id),
			Status: responseOutputStatus(out),
			Role:   "assistant",
			Content: []dto.ResponsesOutputContent{
				{
					Type:        "output_text",
					Text:        text,
					Annotations: []interface{}{},
				},
			},
		})
	}
	if reasoning := choice.Message.GetReasoningContent(); reasoning != "" {
		// 三槽位对齐官方 schema（与 bamboo-messages v0.9.0 一致）：
		//   content           — reasoning_text 承载原始思考全文
		//   summary           — 启发式摘要；chat 转换层不具备摘要能力，给空数组
		//   encrypted_content — chat 路径无上游签名，留空
		out.Output = append(out.Output, dto.ResponsesOutput{
			Type:   responsesOutputTypeReasoning,
			ID:     fmt.Sprintf("%s_reasoning_0", id),
			Status: responseOutputStatus(out),
			Content: []dto.ResponsesOutputContent{
				{
					Type: "reasoning_text",
					Text: reasoning,
				},
			},
			Summary: []dto.ResponsesReasoningSummaryPart{},
		})
	}

	for i, toolCall := range choice.Message.ParseToolCalls() {
		toolOutput, err := chatToolCallToResponsesOutput(toolCall, id, i, responseOutputStatus(out))
		if err != nil {
			return nil, nil, err
		}
		out.Output = append(out.Output, toolOutput)
	}

	return out, usage, nil
}

// echoResponsesRequestFields 将原 Responses 请求的参数回显到响应中。
//
// Responses API 规范要求 response.created 回显客户端传入的请求字段
// （instructions、max_output_tokens、temperature、top_p、tool_choice、
// store、truncation、user、metadata、reasoning 等），使客户端拿到的响应
// 与请求配置一致。origReq 为 nil 或字段缺失时保持响应零值，不覆盖。
func echoResponsesRequestFields(out *dto.OpenAIResponsesResponse, origReq *dto.OpenAIResponsesRequest) {
	if origReq == nil {
		return
	}
	if len(origReq.Instructions) > 0 {
		out.Instructions = origReq.Instructions
	}
	if origReq.MaxOutputTokens != nil {
		out.MaxOutputTokens = int(*origReq.MaxOutputTokens)
	}
	if origReq.Temperature != nil {
		out.Temperature = *origReq.Temperature
	}
	if origReq.TopP != nil {
		out.TopP = *origReq.TopP
	}
	if len(origReq.ToolChoice) > 0 {
		out.ToolChoice = origReq.ToolChoice
	}
	if len(origReq.User) > 0 {
		out.User = origReq.User
	}
	if len(origReq.Metadata) > 0 {
		out.Metadata = origReq.Metadata
	}
	if len(origReq.Truncation) > 0 {
		out.Truncation = origReq.Truncation
	}
	// Store / ParallelToolCalls 在请求中为 JSON 布尔值（"true"/"false"），
	// 解析后回显到响应布尔字段；解析失败时保持默认值。
	if len(origReq.Store) > 0 {
		var store bool
		if err := common.Unmarshal(origReq.Store, &store); err == nil {
			out.Store = store
		}
	}
	if len(origReq.ParallelToolCalls) > 0 {
		var parallel bool
		if err := common.Unmarshal(origReq.ParallelToolCalls, &parallel); err == nil {
			out.ParallelToolCalls = parallel
		}
	}
	if origReq.PreviousResponseID != "" {
		if raw, err := common.Marshal(origReq.PreviousResponseID); err == nil {
			out.PreviousResponseID = raw
		}
	}
	if origReq.Reasoning != nil {
		out.Reasoning = origReq.Reasoning
	}
	if len(origReq.Tools) > 0 {
		var tools []map[string]any
		if err := common.Unmarshal(origReq.Tools, &tools); err == nil && tools != nil {
			out.Tools = tools
		}
	}
}

func ResponsesStatusFromChatFinishReason(finishReason string) (string, *dto.IncompleteDetails) {
	switch strings.TrimSpace(finishReason) {
	case chatFinishReasonLength:
		return "incomplete", &dto.IncompleteDetails{Reason: responsesIncompleteReasonMaxTokens}
	case chatFinishReasonContentFilter:
		return "incomplete", &dto.IncompleteDetails{Reason: responsesIncompleteReasonContentFilter}
	default:
		return "completed", nil
	}
}

func UsageFromChatUsage(src *dto.Usage) *dto.Usage {
	usage := &dto.Usage{}
	if src == nil {
		return usage
	}
	usage.UsageSemantic = src.UsageSemantic
	usage.UsageSource = src.UsageSource
	usage.BillingUsage = dto.CloneBillingUsage(src.BillingUsage)
	if usage.BillingUsage == nil {
		usage.BillingUsage = dto.NewOpenAIChatBillingUsage(src)
	}
	usage.Cost = src.Cost
	if src.PromptTokens != 0 {
		usage.PromptTokens = src.PromptTokens
		usage.InputTokens = src.PromptTokens
	}
	if src.CompletionTokens != 0 {
		usage.CompletionTokens = src.CompletionTokens
		usage.OutputTokens = src.CompletionTokens
	}
	if src.TotalTokens != 0 {
		usage.TotalTokens = src.TotalTokens
	} else {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}
	if src.PromptTokensDetails.CachedTokens != 0 ||
		src.PromptTokensDetails.ImageTokens != 0 ||
		src.PromptTokensDetails.AudioTokens != 0 ||
		src.PromptTokensDetails.CachedCreationTokens != 0 ||
		src.PromptTokensDetails.CacheWriteTokens != 0 ||
		src.PromptTokensDetails.TextTokens != 0 {
		details := src.PromptTokensDetails
		usage.InputTokensDetails = &details
	}
	if src.CompletionTokenDetails.ReasoningTokens != 0 ||
		src.CompletionTokenDetails.TextTokens != 0 ||
		src.CompletionTokenDetails.AudioTokens != 0 ||
		src.CompletionTokenDetails.ImageTokens != 0 {
		usage.CompletionTokenDetails = src.CompletionTokenDetails
	}
	usage.ClaudeCacheCreation5mTokens = src.ClaudeCacheCreation5mTokens
	usage.ClaudeCacheCreation1hTokens = src.ClaudeCacheCreation1hTokens
	return usage
}

func responseOutputStatus(resp *dto.OpenAIResponsesResponse) string {
	if resp == nil || responseStatusString(resp) != "incomplete" {
		return "completed"
	}
	return "incomplete"
}

func responseStatusString(resp *dto.OpenAIResponsesResponse) string {
	if resp == nil || len(resp.Status) == 0 {
		return ""
	}
	var status string
	_ = common.Unmarshal(resp.Status, &status)
	return strings.TrimSpace(status)
}

func chatToolCallToResponsesOutput(toolCall dto.ToolCallRequest, responseID string, index int, status string) (dto.ResponsesOutput, error) {
	callID := strings.TrimSpace(toolCall.ID)
	if callID == "" {
		callID = fmt.Sprintf("%s_call_%d", responseID, index)
	}
	if toolCall.Type == "" || toolCall.Type == "function" {
		return dto.ResponsesOutput{
			Type:      responsesOutputTypeFunctionCall,
			ID:        callID,
			Status:    status,
			CallId:    callID,
			Name:      toolCall.Function.Name,
			Arguments: chatArgumentsRawMessage(toolCall.Function.Arguments),
		}, nil
	}
	return dto.ResponsesOutput{
		Type:      toolCall.Type,
		ID:        callID,
		Status:    status,
		CallId:    callID,
		Arguments: toolCall.Custom,
	}, nil
}

func chatArgumentsRawMessage(arguments string) []byte {
	raw, err := common.Marshal(arguments)
	if err != nil {
		return []byte(`""`)
	}
	return raw
}

func chatCreatedAt(created any) int {
	switch v := created.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	case string:
		if parsed := common.String2Int(v); parsed != 0 {
			return parsed
		}
	}
	return int(time.Now().Unix())
}

func responsesStreamEvent(eventType string, payload dto.ResponsesStreamResponse) ChatToResponsesStreamEvent {
	payload.Type = eventType
	return ChatToResponsesStreamEvent{
		Type:    eventType,
		Payload: payload,
	}
}

func intPtr(v int) *int {
	return &v
}
