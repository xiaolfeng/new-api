package openai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func ChatCompletionsStreamToResponsesHandler(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	resp *http.Response,
	origReq *dto.OpenAIResponsesRequest,
) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	defer service.CloseResponseBodyGracefully(resp)

	responseID := fmt.Sprintf("resp_%s", c.GetString(common.RequestIdKey))
	messageItemID := fmt.Sprintf("msg_%s", c.GetString(common.RequestIdKey))
	createdAt := time.Now().Unix()
	model := info.UpstreamModelName

	var (
		usage       = &dto.Usage{}
		usageText   strings.Builder
		streamErr   *types.NewAPIError

		sentResponseCreated bool
		hasOpenMessageItem  bool
		currentMode         string
		outputIndex         int
		contentIndex        int
		accumulatedText     strings.Builder

		toolCallItemIDs         = make(map[int]string)
		toolCallArgAccumulators = make(map[int]string)
		toolCallNames           = make(map[int]string)
		toolCallIDs             = make(map[int]string)
	)

	sendEvent := func(eventType string, payload interface{}) bool {
		data, err := common.Marshal(payload)
		if err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
			return false
		}
		helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: eventType}, string(data))
		return true
	}

	emitResponseCreated := func() bool {
		if sentResponseCreated {
			return true
		}
		status := "in_progress"
		respObj := &dto.OpenAIResponsesResponse{
			ID:        responseID,
			Object:    "response",
			CreatedAt: int(createdAt),
			Model:     model,
			Status:    json.RawMessage(`"` + status + `"`),
			Output:    []dto.ResponsesOutput{},
		}
		if origReq != nil {
			respObj.Metadata = origReq.Metadata
		}
		if !sendEvent("response.created", &dto.ResponsesStreamResponse{
			Type:     "response.created",
			Response: respObj,
		}) {
			return false
		}
		sentResponseCreated = true
		return true
	}

	emitMessageItemAdded := func() bool {
		if hasOpenMessageItem {
			return true
		}
		if !emitResponseCreated() {
			return false
		}
		oi := outputIndex
		ci := contentIndex
		if !sendEvent("response.output_item.added", &dto.ResponsesStreamResponse{
			Type:        "response.output_item.added",
			OutputIndex: &oi,
			Item: &dto.ResponsesOutput{
				Type: "message",
				ID:   messageItemID,
				Role: "assistant",
				Content: []dto.ResponsesOutputContent{
					{
						Type: "output_text",
						Text: "",
					},
				},
			},
		}) {
			return false
		}
		if !sendEvent("response.content_part.added", &dto.ResponsesStreamResponse{
			Type:         "response.content_part.added",
			OutputIndex:  &oi,
			ContentIndex: &ci,
			Part: &dto.ResponsesReasoningSummaryPart{
				Type: "output_text",
				Text: "",
			},
		}) {
			return false
		}
		hasOpenMessageItem = true
		return true
	}

	closeMessageItem := func() bool {
		if !hasOpenMessageItem {
			return true
		}
		oi := outputIndex
		ci := contentIndex
		text := accumulatedText.String()

		if !sendEvent("response.output_text.done", &dto.ResponsesStreamResponse{
			Type:         "response.output_text.done",
			OutputIndex:  &oi,
			ContentIndex: &ci,
			Text:         text,
		}) {
			return false
		}
		if !sendEvent("response.content_part.done", &dto.ResponsesStreamResponse{
			Type:         "response.content_part.done",
			OutputIndex:  &oi,
			ContentIndex: &ci,
			Part: &dto.ResponsesReasoningSummaryPart{
				Type: "output_text",
				Text: text,
			},
		}) {
			return false
		}
		if !sendEvent("response.output_item.done", &dto.ResponsesStreamResponse{
			Type:        "response.output_item.done",
			OutputIndex: &oi,
			Item: &dto.ResponsesOutput{
				Type: "message",
				ID:   messageItemID,
				Role: "assistant",
				Content: []dto.ResponsesOutputContent{
					{
						Type: "output_text",
						Text: text,
					},
				},
			},
		}) {
			return false
		}

		outputIndex++
		contentIndex = 0
		hasOpenMessageItem = false
		accumulatedText.Reset()
		return true
	}

	closeToolCall := func(toolIdx int) bool {
		itemID, ok := toolCallItemIDs[toolIdx]
		if !ok {
			return true
		}
		oi := toolIdx + outputIndex
		args := toolCallArgAccumulators[toolIdx]

		if !sendEvent("response.function_call_arguments.done", &dto.ResponsesStreamResponse{
			Type:        "response.function_call_arguments.done",
			OutputIndex: &oi,
			ItemID:      itemID,
			Arguments:   args,
		}) {
			return false
		}
		if !sendEvent("response.output_item.done", &dto.ResponsesStreamResponse{
			Type:        "response.output_item.done",
			OutputIndex: &oi,
			Item: &dto.ResponsesOutput{
				Type:      "function_call",
				ID:        itemID,
				CallId:    toolCallIDs[toolIdx],
				Name:      toolCallNames[toolIdx],
				Arguments: json.RawMessage(args),
			},
		}) {
			return false
		}
		return true
	}

	emitResponseCompleted := func(status string) bool {
		if !closeMessageItem() {
			return false
		}
		for idx := range toolCallItemIDs {
			if !closeToolCall(idx) {
				return false
			}
		}

		respObj := &dto.OpenAIResponsesResponse{
			ID:        responseID,
			Object:    "response",
			CreatedAt: int(createdAt),
			Model:     model,
			Status:    json.RawMessage(`"` + status + `"`),
			Output:    []dto.ResponsesOutput{},
			Usage:     usage,
		}
		if origReq != nil {
			respObj.Metadata = origReq.Metadata
		}
		if !sendEvent("response.completed", &dto.ResponsesStreamResponse{
			Type:     "response.completed",
			Response: respObj,
		}) {
			return false
		}
		return true
	}

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		if streamErr != nil {
			sr.Stop(streamErr)
			return
		}

		var chatChunk dto.ChatCompletionsStreamResponse
		if err := common.UnmarshalJsonStr(data, &chatChunk); err != nil {
			logger.LogError(c, "failed to unmarshal chat completions stream chunk: "+err.Error())
			sr.Error(err)
			return
		}

		if chatChunk.Usage != nil {
			usage = chatChunk.Usage
		}
		if chatChunk.Model != "" {
			model = chatChunk.Model
		}

		if len(chatChunk.Choices) == 0 {
			return
		}

		choice := &chatChunk.Choices[0]
		delta := &choice.Delta

		if !sentResponseCreated && delta.Role == "assistant" {
			if !emitResponseCreated() {
				sr.Stop(streamErr)
				return
			}
			return
		}

		if reasoning := delta.GetReasoningContent(); reasoning != "" {
			if !emitResponseCreated() {
				sr.Stop(streamErr)
				return
			}
			if currentMode != "reasoning" && currentMode != "text" {
				currentMode = "reasoning"
			}
			usageText.WriteString(reasoning)
			oi := outputIndex
			si := 0
			if !sendEvent("response.reasoning_summary_text.delta", &dto.ResponsesStreamResponse{
				Type:         "response.reasoning_summary_text.delta",
				OutputIndex:  &oi,
				SummaryIndex: &si,
				Delta:        reasoning,
			}) {
				sr.Stop(streamErr)
				return
			}
			return
		}

		if content := delta.GetContentString(); content != "" {
			if !emitMessageItemAdded() {
				sr.Stop(streamErr)
				return
			}
			currentMode = "text"
			accumulatedText.WriteString(content)
			usageText.WriteString(content)
			oi := outputIndex
			ci := contentIndex
			if !sendEvent("response.output_text.delta", &dto.ResponsesStreamResponse{
				Type:         "response.output_text.delta",
				OutputIndex:  &oi,
				ContentIndex: &ci,
				Delta:        content,
			}) {
				sr.Stop(streamErr)
				return
			}
			return
		}

		if len(delta.ToolCalls) > 0 {
			for _, tc := range delta.ToolCalls {
				toolIdx := 0
				if tc.Index != nil {
					toolIdx = *tc.Index
				}

				if _, exists := toolCallItemIDs[toolIdx]; !exists {
					if !closeMessageItem() {
						sr.Stop(streamErr)
						return
					}

					fcID := fmt.Sprintf("fc_%s_%d", c.GetString(common.RequestIdKey), toolIdx)
					callID := tc.ID
					if callID == "" {
						callID = fcID
					}
					toolCallItemIDs[toolIdx] = fcID
					toolCallIDs[toolIdx] = callID
					toolCallArgAccumulators[toolIdx] = ""
					currentMode = "tool_call"

					oi := toolIdx + outputIndex
					if !sendEvent("response.output_item.added", &dto.ResponsesStreamResponse{
						Type:        "response.output_item.added",
						OutputIndex: &oi,
						Item: &dto.ResponsesOutput{
							Type:   "function_call",
							ID:     fcID,
							CallId: callID,
							Name:   tc.Function.Name,
							Status: "in_progress",
						},
					}) {
						sr.Stop(streamErr)
						return
					}

					if tc.Function.Name != "" {
						toolCallNames[toolIdx] = tc.Function.Name
					}
				}

				if tc.Function.Arguments != "" {
					toolCallArgAccumulators[toolIdx] += tc.Function.Arguments
					usageText.WriteString(tc.Function.Arguments)
					oi := toolIdx + outputIndex
					if !sendEvent("response.function_call_arguments.delta", &dto.ResponsesStreamResponse{
						Type:        "response.function_call_arguments.delta",
						OutputIndex: &oi,
						ItemID:      toolCallItemIDs[toolIdx],
						Delta:       tc.Function.Arguments,
					}) {
						sr.Stop(streamErr)
						return
					}
				}
			}
			return
		}

		if choice.FinishReason != nil {
			finishReason := *choice.FinishReason
			switch finishReason {
			case "stop":
				if !emitResponseCompleted("completed") {
					sr.Stop(streamErr)
					return
				}
				sr.Done()
				return
			case "tool_calls":
				if !emitResponseCompleted("completed") {
					sr.Stop(streamErr)
					return
				}
				sr.Done()
				return
			case "length":
				if !emitResponseCompleted("incomplete") {
					sr.Stop(streamErr)
					return
				}
				sr.Done()
				return
			default:
				if !emitResponseCompleted("completed") {
					sr.Stop(streamErr)
					return
				}
				sr.Done()
				return
			}
		}
	})

	if streamErr != nil {
		_ = emitResponseCompleted("failed")
		return nil, streamErr
	}

	if sentResponseCreated {
		if usage.TotalTokens == 0 {
			usage = service.ResponseText2Usage(c, usageText.String(), info.UpstreamModelName, info.GetEstimatePromptTokens())
		}
		_ = emitResponseCompleted("completed")
	} else {
		_ = emitResponseCreated()
		if usage.TotalTokens == 0 {
			usage = service.ResponseText2Usage(c, "", info.UpstreamModelName, info.GetEstimatePromptTokens())
		}
		_ = emitResponseCompleted("completed")
	}

	info.CompletionText = usageText.String()

	return usage, nil
}
