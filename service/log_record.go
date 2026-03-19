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

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const maxCompletionLength = 5000

// BuildLogRecord 构建消费日志详细记录
// relayInfo: 中继信息
// completionText: AI 返回的内容文本
func BuildLogRecord(relayInfo *relaycommon.RelayInfo, completionText string) string {
	if !operation_setting.IsRecordConsumeLogDetailEnabled() {
		return ""
	}

	record := model.LogDetailRecord{}

	// 1. Prompt (从 relayInfo.Request 获取 messages)
	if relayInfo != nil && relayInfo.Request != nil {
		if genReq, ok := relayInfo.Request.(*dto.GeneralOpenAIRequest); ok && genReq != nil {
			record.Prompt = buildPromptRecord(genReq)
		}
	}

	// 2. Completion (截断到 5000 字符)
	if completionText != "" {
		if len(completionText) > maxCompletionLength {
			record.Completion = completionText[:maxCompletionLength]
		} else {
			record.Completion = completionText
		}
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

// buildPromptRecord 从请求中构建 prompt 记录
func buildPromptRecord(req *dto.GeneralOpenAIRequest) map[string]interface{} {
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
