/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
For commercial licensing, please contact support@quantumnous.com
*/

package service

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const maxCompletionLength = 5000

// safeTruncateUTF8 安全截断 UTF-8 字符串，避免在多字节字符中间截断
func safeTruncateUTF8(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	// 按 rune 截断，保证不会截断多字节字符
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen])
	}
	return s
}

// BuildLogRecord 构建消费日志详细记录
// relayInfo: 中继信息（包含请求和响应内容）
func BuildLogRecord(relayInfo *relaycommon.RelayInfo) string {
	if !operation_setting.IsRecordConsumeLogDetailEnabled() {
		return ""
	}

	record := model.LogDetailRecord{}

	// 1. Prompt (从 relayInfo.Request 获取 messages)
	if relayInfo != nil && relayInfo.Request != nil {
		switch req := relayInfo.Request.(type) {
		case *dto.GeneralOpenAIRequest:
			if req != nil {
				record.Prompt = buildPromptRecordFromOpenAI(req)
			}
		case *dto.ClaudeRequest:
			if req != nil {
				record.Prompt = buildPromptRecordFromClaude(req)
			}
		case *dto.GeminiChatRequest:
			if req != nil {
				record.Prompt = buildPromptRecordFromGemini(req)
			}
		case *dto.OpenAIResponsesRequest:
			if req != nil {
				record.Prompt = buildPromptRecordFromResponses(req)
			}
		}
	}

	// 2. Completion (从 relayInfo.CompletionText 获取，安全截断到 5000 字符)
	if relayInfo != nil && relayInfo.CompletionText != "" {
		record.Completion = safeTruncateUTF8(relayInfo.CompletionText, maxCompletionLength)
	}

	// 3. Headers (排除敏感信息)
	if relayInfo != nil && relayInfo.RequestHeaders != nil {
		record.Headers = filterSensitiveHeaders(relayInfo.RequestHeaders)
	}

	// 如果所有字段都为空，返回空字符串
	if len(record.Prompt) == 0 && record.Completion == "" && len(record.Headers) == 0 {
		return ""
	}

	jsonBytes, err := common.Marshal(record)
	if err != nil {
		return ""
	}
	return string(jsonBytes)
}

// buildPromptRecordFromOpenAI 从 OpenAI 格式请求中构建 prompt 记录
// 只提取最后一个用户消息，避免存储过多内容
func buildPromptRecordFromOpenAI(req *dto.GeneralOpenAIRequest) map[string]interface{} {
	if req == nil {
		return nil
	}

	result := make(map[string]interface{})

	// 只提取最后一个用户消息
	if len(req.Messages) > 0 {
		// 从后向前查找最后一个用户消息
		for i := len(req.Messages) - 1; i >= 0; i-- {
			msg := req.Messages[i]
			if msg.Role == "user" {
				m := make(map[string]interface{})
				m["role"] = msg.Role
				if msg.IsStringContent() {
					m["content"] = msg.StringContent()
				} else if contents := msg.ParseContent(); len(contents) > 0 {
					// 对于多模态内容，只记录文本部分
					textParts := make([]string, 0)
					for _, c := range contents {
						if c.Type == dto.ContentTypeText && c.Text != "" {
							textParts = append(textParts, c.Text)
						}
					}
					if len(textParts) > 0 {
						m["content"] = strings.Join(textParts, "\n")
					}
				}
				result["lastUserMessage"] = m
				break
			}
		}
	}

	return result
}

// buildPromptRecordFromClaude 从 Claude 格式请求中构建 prompt 记录
// 只提取最后一个用户消息，避免存储过多内容
func buildPromptRecordFromClaude(req *dto.ClaudeRequest) map[string]interface{} {
	if req == nil {
		return nil
	}

	result := make(map[string]interface{})

	// 只提取最后一个用户消息
	if len(req.Messages) > 0 {
		// 从后向前查找最后一个用户消息
		for i := len(req.Messages) - 1; i >= 0; i-- {
			msg := req.Messages[i]
			if msg.Role == "user" {
				m := make(map[string]interface{})
				m["role"] = msg.Role
				if msg.IsStringContent() {
					m["content"] = msg.GetStringContent()
				} else if contents, _ := msg.ParseContent(); len(contents) > 0 {
					textParts := make([]string, 0)
					for _, c := range contents {
						if c.Text != nil && *c.Text != "" {
							textParts = append(textParts, *c.Text)
						}
					}
					if len(textParts) > 0 {
						m["content"] = strings.Join(textParts, "\n")
					}
				}
				result["lastUserMessage"] = m
				break
			}
		}
	}

	return result
}

// buildPromptRecordFromGemini 从 Gemini 格式请求中构建 prompt 记录
// 只提取最后一个用户消息，避免存储过多内容
func buildPromptRecordFromGemini(req *dto.GeminiChatRequest) map[string]interface{} {
	if req == nil {
		return nil
	}

	result := make(map[string]interface{})

	// 只提取最后一个用户消息
	if len(req.Contents) > 0 {
		// 从后向前查找最后一个用户消息（role 为 "user" 或空）
		for i := len(req.Contents) - 1; i >= 0; i-- {
			content := req.Contents[i]
			// Gemini 中用户消息的 role 可能是 "user" 或空字符串
			if content.Role == "user" || content.Role == "" {
				if len(content.Parts) > 0 {
					textParts := make([]string, 0)
					for _, part := range content.Parts {
						if part.Text != "" {
							textParts = append(textParts, part.Text)
						}
					}
					if len(textParts) > 0 {
						result["lastUserMessage"] = map[string]interface{}{
							"role":    "user",
							"content": strings.Join(textParts, "\n"),
						}
						break
					}
				}
			}
		}
	}

	return result
}

// buildPromptRecordFromResponses 从 OpenAI Responses API 格式请求中构建 prompt 记录
// 只提取最后一个用户消息，避免存储全量上下文
func buildPromptRecordFromResponses(req *dto.OpenAIResponsesRequest) map[string]interface{} {
	if req == nil {
		return nil
	}

	result := make(map[string]interface{})

	// 只提取最后一个用户消息，与其他格式保持一致
	if text := extractLastUserMessageTextFromResponsesInput(req.Input); text != "" {
		result["lastUserMessage"] = map[string]interface{}{
			"role":    "user",
			"content": text,
		}
	}

	return result
}

// extractLastUserMessageTextFromResponsesInput 从 Responses API input 字段中提取最后一个用户消息的文本
func extractLastUserMessageTextFromResponsesInput(input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}

	// input 为字符串时，视为当前用户输入
	if common.GetJsonType(input) == "string" {
		var s string
		_ = common.Unmarshal(input, &s)
		return strings.TrimSpace(s)
	}

	if common.GetJsonType(input) != "array" {
		return ""
	}

	var items []map[string]interface{}
	if err := common.Unmarshal(input, &items); err != nil {
		return ""
	}

	// 优先：逆序找最后一个 role=user 的消息
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		role := strings.TrimSpace(common.Interface2String(item["role"]))
		if role != "user" {
			continue
		}

		switch content := item["content"].(type) {
		case string:
			if s := strings.TrimSpace(content); s != "" {
				return s
			}
		case []interface{}:
			textParts := make([]string, 0, len(content))
			for _, p := range content {
				part, ok := p.(map[string]interface{})
				if !ok {
					continue
				}
				t := strings.TrimSpace(common.Interface2String(part["type"]))
				if t != "" && t != "input_text" && t != "text" {
					continue
				}
				if txt := strings.TrimSpace(common.Interface2String(part["text"])); txt != "" {
					textParts = append(textParts, txt)
				}
			}
			if len(textParts) > 0 {
				return strings.Join(textParts, "\n")
			}
		}
	}

	// 兜底：兼容无 role 的简化数组 [{type:"input_text",text:"..."}]
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		t := strings.TrimSpace(common.Interface2String(item["type"]))
		if t == "input_text" || t == "text" {
			if txt := strings.TrimSpace(common.Interface2String(item["text"])); txt != "" {
				return txt
			}
		}
	}

	return ""
}

// filterSensitiveHeaders 过滤敏感请求头
func filterSensitiveHeaders(headers map[string]string) map[string]string {
	if headers == nil {
		return nil
	}

	filtered := make(map[string]string)
	for key, value := range headers {
		lowerKey := strings.ToLower(key)
		if model.SensitiveHeaders[lowerKey] {
			continue
		}
		filtered[key] = value
	}

	if len(filtered) == 0 {
		return nil
	}
	return filtered
}
