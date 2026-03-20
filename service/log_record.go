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
func buildPromptRecordFromOpenAI(req *dto.GeneralOpenAIRequest) map[string]interface{} {
	if req == nil {
		return nil
	}

	result := make(map[string]interface{})

	// 记录 messages
	if len(req.Messages) > 0 {
		messages := make([]map[string]interface{}, 0, len(req.Messages))
		for _, msg := range req.Messages {
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
			messages = append(messages, m)
		}
		result["messages"] = messages
	}

	return result
}

// buildPromptRecordFromClaude 从 Claude 格式请求中构建 prompt 记录
func buildPromptRecordFromClaude(req *dto.ClaudeRequest) map[string]interface{} {
	if req == nil {
		return nil
	}

	result := make(map[string]interface{})

	// 记录 system
	if req.System != nil {
		if req.IsStringSystem() {
			result["system"] = req.GetStringSystem()
		} else if sysMedia := req.ParseSystem(); len(sysMedia) > 0 {
			textParts := make([]string, 0)
			for _, media := range sysMedia {
				if media.Type == "text" {
					textParts = append(textParts, media.GetText())
				}
			}
			if len(textParts) > 0 {
				result["system"] = strings.Join(textParts, "\n")
			}
		}
	}

	// 记录 messages
	if len(req.Messages) > 0 {
		messages := make([]map[string]interface{}, 0, len(req.Messages))
		for _, msg := range req.Messages {
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
			messages = append(messages, m)
		}
		result["messages"] = messages
	}

	return result
}

// buildPromptRecordFromGemini 从 Gemini 格式请求中构建 prompt 记录
func buildPromptRecordFromGemini(req *dto.GeminiChatRequest) map[string]interface{} {
	if req == nil {
		return nil
	}

	result := make(map[string]interface{})

	// 记录 systemInstruction
	if req.SystemInstructions != nil && len(req.SystemInstructions.Parts) > 0 {
		textParts := make([]string, 0)
		for _, part := range req.SystemInstructions.Parts {
			if part.Text != "" {
				textParts = append(textParts, part.Text)
			}
		}
		if len(textParts) > 0 {
			result["systemInstruction"] = strings.Join(textParts, "\n")
		}
	}

	// 记录 contents
	if len(req.Contents) > 0 {
		messages := make([]map[string]interface{}, 0, len(req.Contents))
		for _, content := range req.Contents {
			m := make(map[string]interface{})
			m["role"] = content.Role
			if len(content.Parts) > 0 {
				textParts := make([]string, 0)
				for _, part := range content.Parts {
					if part.Text != "" {
						textParts = append(textParts, part.Text)
					}
				}
				if len(textParts) > 0 {
					m["content"] = strings.Join(textParts, "\n")
				}
			}
			messages = append(messages, m)
		}
		result["messages"] = messages
	}

	return result
}

// buildPromptRecordFromResponses 从 OpenAI Responses API 格式请求中构建 prompt 记录
func buildPromptRecordFromResponses(req *dto.OpenAIResponsesRequest) map[string]interface{} {
	if req == nil {
		return nil
	}

	result := make(map[string]interface{})

	// 记录 instructions
	if len(req.Instructions) > 0 {
		result["instructions"] = string(req.Instructions)
	}

	// 记录 input (解析为文本列表)
	if req.Input != nil {
		inputs := req.ParseInput()
		textParts := make([]string, 0, len(inputs))
		for _, input := range inputs {
			if input.Text != "" {
				textParts = append(textParts, input.Text)
			}
		}
		if len(textParts) > 0 {
			result["input"] = strings.Join(textParts, "\n")
		}
	}

	// 记录 prompt (如果有)
	if len(req.Prompt) > 0 {
		result["prompt"] = string(req.Prompt)
	}

	return result
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
