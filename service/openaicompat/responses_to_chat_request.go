package openaicompat

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/samber/lo"
)

func ResponsesRequestToChatCompletionsRequest(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}

	if req.PreviousResponseID != "" {
		return nil, fmt.Errorf("previous_response_id is not supported in Chat Completions compatibility mode")
	}

	// Log unsupported fields that are silently dropped.
	if req.ContextManagement != nil {
		log.Printf("[ResponsesToChat] dropping unsupported field: context_management")
	}
	if req.Include != nil {
		log.Printf("[ResponsesToChat] dropping unsupported field: include")
	}
	if req.Conversation != nil {
		log.Printf("[ResponsesToChat] dropping unsupported field: conversation")
	}
	if req.Truncation != nil {
		log.Printf("[ResponsesToChat] dropping unsupported field: truncation")
	}
	if req.MaxToolCalls != nil {
		log.Printf("[ResponsesToChat] dropping unsupported field: max_tool_calls")
	}
	if req.Preset != nil {
		log.Printf("[ResponsesToChat] dropping unsupported field: preset")
	}

	var messages []dto.Message

	// 1. instructions → system message
	if len(req.Instructions) > 0 {
		var instructionsStr string
		if err := common.Unmarshal(req.Instructions, &instructionsStr); err == nil {
			if strings.TrimSpace(instructionsStr) != "" {
				messages = append(messages, dto.Message{
					Role:    "system",
					Content: instructionsStr,
				})
			}
		}
	}

	// 2. input[] → messages[]
	inputItems, err := parseResponsesInput(req.Input)
	if err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}

	// State scanner: accumulate function_call items into the preceding assistant message.
	var pendingToolCalls []dto.ToolCallRequest

	for _, item := range inputItems {
		itemType, _ := item["type"].(string)

		switch {
		case itemType == "function_call":
			callID, _ := item["call_id"].(string)
			name, _ := item["name"].(string)
			arguments, _ := item["arguments"].(string)
			pendingToolCalls = append(pendingToolCalls, dto.ToolCallRequest{
				ID:   callID,
				Type: "function",
				Function: dto.FunctionRequest{
					Name:      name,
					Arguments: arguments,
				},
			})

		case itemType == "function_call_output":
			// Flush any pending tool calls before adding tool result.
			if len(pendingToolCalls) > 0 {
				messages = append(messages, dto.Message{
					Role:    "assistant",
					Content: nil,
				})
				messages[len(messages)-1].SetToolCalls(pendingToolCalls)
				pendingToolCalls = nil
			}

			callID, _ := item["call_id"].(string)
			output := serializeContentPart(item["output"])

			messages = append(messages, dto.Message{
				Role:       "tool",
				ToolCallId: callID,
				Content:    output,
			})

		default:
			// Role-based item: user, assistant, developer, system
			role, _ := item["role"].(string)
			if role == "" {
				role = "user"
			}
			// Responses API "developer" role maps to Chat Completions "system" role.
			// In Responses API, "developer" is equivalent to "system" in Chat Completions
			// (used for o-series and gpt-5 models). See also: chat_to_responses.go line 128-129
			// which does the reverse mapping.
			if role == "developer" {
				role = "system"
			}

			// Flush pending tool calls if the role changes away from assistant context.
			if len(pendingToolCalls) > 0 && role != "assistant" {
				messages = append(messages, dto.Message{
					Role:    "assistant",
					Content: nil,
				})
				messages[len(messages)-1].SetToolCalls(pendingToolCalls)
				pendingToolCalls = nil
			}

			content := convertResponsesContentToChatContent(item["content"], role)

			if role == "assistant" {
				msg := dto.Message{
					Role:    role,
					Content: content,
				}
				messages = append(messages, msg)
				// Pending tool calls will be appended after this assistant message.
			} else {
				messages = append(messages, dto.Message{
					Role:    role,
					Content: content,
				})
			}
		}
	}

	// Flush remaining pending tool calls.
	if len(pendingToolCalls) > 0 {
		// Check if last message is an assistant message we can attach to.
		if len(messages) > 0 && messages[len(messages)-1].Role == "assistant" {
			messages[len(messages)-1].SetToolCalls(pendingToolCalls)
		} else {
			messages = append(messages, dto.Message{
				Role:    "assistant",
				Content: nil,
			})
			messages[len(messages)-1].SetToolCalls(pendingToolCalls)
		}
	}

	// 3. tools conversion
	var tools []dto.ToolCallRequest
	if len(req.Tools) > 0 {
		var rawTools []map[string]any
		if err := common.Unmarshal(req.Tools, &rawTools); err == nil {
			for _, t := range rawTools {
				tType, _ := t["type"].(string)
				if tType == "function" {
					name, _ := t["name"].(string)
					desc, _ := t["description"].(string)
					params := t["parameters"]
					tools = append(tools, dto.ToolCallRequest{
						Type: "function",
						Function: dto.FunctionRequest{
							Name:        name,
							Description: desc,
							Parameters:  params,
						},
					})
				} else {
					// Best-effort: keep original shape for unknown types.
					var tool dto.ToolCallRequest
					if b, err := common.Marshal(t); err == nil {
						_ = common.Unmarshal(b, &tool)
					}
					tools = append(tools, tool)
				}
			}
		}
	}

	// 4. tool_choice conversion
	var toolChoice any
	if len(req.ToolChoice) > 0 {
		var raw map[string]any
		if err := common.Unmarshal(req.ToolChoice, &raw); err == nil {
			t, _ := raw["type"].(string)
			if t == "function" {
				// Responses: {"type":"function","name":"..."}
				// Chat: {"type":"function","function":{"name":"..."}}
				name, _ := raw["name"].(string)
				if name != "" {
					toolChoice = map[string]any{
						"type": "function",
						"function": map[string]any{
							"name": name,
						},
					}
				} else {
					toolChoice = raw
				}
			} else {
				// String values like "auto", "none", "required"
				toolChoice = raw
			}
		} else {
			// Might be a plain string like "auto"
			var s string
			if err := common.Unmarshal(req.ToolChoice, &s); err == nil {
				toolChoice = s
			}
		}
	}

	// 5. text.format → response_format
	responseFormat := convertResponsesTextToChatResponseFormat(req.Text)

	// 6. Direct field mapping
	out := &dto.GeneralOpenAIRequest{
		Model:       req.Model,
		Messages:    messages,
		Stream:      req.Stream,
		StreamOptions: req.StreamOptions,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Tools:       tools,
		ToolChoice:  toolChoice,
		User:        req.User,
		Store:       req.Store,
		Metadata:    req.Metadata,
		TopLogProbs: req.TopLogProbs,
		ResponseFormat: responseFormat,
	}

	if req.MaxOutputTokens != nil {
		out.MaxCompletionTokens = lo.ToPtr(*req.MaxOutputTokens)
	}

	if req.Reasoning != nil && req.Reasoning.Effort != "" {
		out.ReasoningEffort = req.Reasoning.Effort
	}

	if len(req.ParallelToolCalls) > 0 {
		var ptc bool
		if err := common.Unmarshal(req.ParallelToolCalls, &ptc); err == nil {
			out.ParallelTooCalls = &ptc
		}
	}

	return out, nil
}

// parseResponsesInput parses the raw input field which can be a string, or an array of items.
func parseResponsesInput(raw json.RawMessage) ([]map[string]any, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	// Input can be a plain string.
	trimmed := strings.TrimSpace(string(raw))
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var s string
		if err := common.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		return []map[string]any{
			{"role": "user", "content": s},
		}, nil
	}

	var items []map[string]any
	if err := common.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// convertResponsesContentToChatContent converts a Responses content field to Chat message content.
// Content can be a string, an array of content parts, or nil.
func convertResponsesContentToChatContent(content any, role string) any {
	if content == nil {
		return nil
	}

	switch v := content.(type) {
	case string:
		return v
	case []any:
		parts := make([]dto.MediaContent, 0, len(v))
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			partType, _ := m["type"].(string)
			switch partType {
			case "input_text":
				text, _ := m["text"].(string)
				parts = append(parts, dto.MediaContent{
					Type: "text",
					Text: text,
				})
			case "output_text":
				text, _ := m["text"].(string)
				parts = append(parts, dto.MediaContent{
					Type: "text",
					Text: text,
				})
			case "input_image":
				imageURL := m["image_url"]
				parts = append(parts, dto.MediaContent{
					Type:     "image_url",
					ImageUrl: imageURL,
				})
			case "input_audio":
				parts = append(parts, dto.MediaContent{
					Type:       "input_audio",
					InputAudio: m["input_audio"],
				})
			case "input_file":
				parts = append(parts, dto.MediaContent{
					Type: "file",
					File: m["file"],
				})
			case "input_video":
				parts = append(parts, dto.MediaContent{
					Type:     "video_url",
					VideoUrl: m["video_url"],
				})
			default:
				// Pass through unknown content types as text if they have text.
				if text, ok := m["text"].(string); ok {
					parts = append(parts, dto.MediaContent{
						Type: "text",
						Text: text,
					})
				}
			}
		}
		if len(parts) == 0 {
			return nil
		}
		return parts
	default:
		return fmt.Sprintf("%v", v)
	}
}

// serializeContentPart converts a content value to a string for tool output.
func serializeContentPart(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	default:
		b, err := common.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(b)
	}
}

// convertResponsesTextToChatResponseFormat reverses convertChatResponseFormatToResponsesText.
// Responses text.format: {"type":"text"} or {"type":"json_object"} or {"type":"json_schema", "name":"...", "schema":{...}, ...}
// Chat response_format: {"type":"text"} or {"type":"json_object"} or {"type":"json_schema", "json_schema":{"name":"...", "schema":{...}, ...}}
func convertResponsesTextToChatResponseFormat(textRaw json.RawMessage) *dto.ResponseFormat {
	if len(textRaw) == 0 {
		return nil
	}

	var textObj map[string]json.RawMessage
	if err := common.Unmarshal(textRaw, &textObj); err != nil {
		return nil
	}

	formatRaw, ok := textObj["format"]
	if !ok || len(formatRaw) == 0 {
		return nil
	}

	var format map[string]any
	if err := common.Unmarshal(formatRaw, &format); err != nil {
		return nil
	}

	formatType, _ := format["type"].(string)
	if formatType == "" {
		return nil
	}

	result := &dto.ResponseFormat{
		Type: formatType,
	}

	if formatType == "json_schema" {
		// Extract json_schema fields from the flat Responses format.
		schema := map[string]any{}
		for key, value := range format {
			if key == "type" {
				continue
			}
			schema[key] = value
		}
		if len(schema) > 0 {
			schemaRaw, _ := common.Marshal(schema)
			result.JsonSchema = schemaRaw
		}
	}

	return result
}
