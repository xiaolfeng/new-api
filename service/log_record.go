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
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
)

const maxCompletionLength = 5000
const maxLoggedJSONValueLength = 200

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

func summarizeLongUTF8(s string, maxLen int) string {
	if maxLen <= 0 || utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	if maxLen <= 6 {
		return safeTruncateUTF8(s, maxLen)
	}

	runes := []rune(s)
	remaining := maxLen - 6
	headLen := remaining / 2
	tailLen := remaining - headLen
	return string(runes[:headLen]) + "......" + string(runes[len(runes)-tailLen:])
}

// BuildLogRecord 构建消费日志详细记录
// relayInfo: 中继信息（包含请求和响应内容）
func BuildLogRecord(relayInfo *relaycommon.RelayInfo) string {
	if !operation_setting.IsRecordConsumeLogDetailEnabled() {
		return ""
	}

	record := model.LogDetailRecord{}
	isClaudeRecord := isClaudeStructuredRecord(relayInfo)
	isResponsesRecord := isResponsesStructuredRecord(relayInfo)

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
				if isClaudeRecord {
					record.ClaudeRequestBlocks = buildClaudeRequestBlocks(req)
					record.ClaudeToolResponses = buildClaudeToolResponseBlocks(req)
				}
			}
		case *dto.GeminiChatRequest:
			if req != nil {
				record.Prompt = buildPromptRecordFromGemini(req)
			}
		case *dto.OpenAIResponsesRequest:
			if req != nil {
				record.Prompt = buildPromptRecordFromResponses(req)
				if isResponsesRecord {
					record.ResponsesRequestBlocks = buildResponsesRequestBlocks(req)
					record.ResponsesToolResponses = buildResponsesToolResponseBlocks(req)
				}
			}
		}
	}

	// 2. Completion (从 relayInfo.CompletionText 获取，安全截断到 5000 字符)
	if relayInfo != nil && relayInfo.CompletionText != "" {
		record.Completion = safeTruncateUTF8(relayInfo.CompletionText, maxCompletionLength)
	}

	if shouldRecordClaudeResponseBlocks(relayInfo) {
		record.ClaudeResponseBlocks = buildClaudeResponseBlocks(relayInfo)
	}
	if shouldRecordResponsesResponseBlocks(relayInfo) {
		record.ResponsesResponseBlocks = buildResponsesResponseBlocksFromSSE(relayInfo.ResponseBody)
	}

	// 3. Headers (排除敏感信息)
	if relayInfo != nil && relayInfo.RequestHeaders != nil {
		record.Headers = filterSensitiveHeaders(relayInfo.RequestHeaders)
	}

	// 4. Tool invokes (Claude/Anthropic tool_use + tool_result)
	if len(record.ClaudeResponseBlocks) > 0 {
		record.ToolInvokes = buildClaudeToolInvokeRecordsFromBlocks(record.ClaudeResponseBlocks)
	} else if len(record.ResponsesResponseBlocks) > 0 {
		record.ToolInvokes = buildResponsesToolInvokeRecordsFromBlocks(record.ResponsesResponseBlocks)
	} else if relayInfo != nil {
		record.ToolInvokes = buildToolInvokeRecords(relayInfo)
	}

	// 如果所有字段都为空，返回空字符串
	if len(record.Prompt) == 0 &&
		record.Completion == "" &&
		len(record.Headers) == 0 &&
		len(record.ToolInvokes) == 0 &&
		len(record.ClaudeRequestBlocks) == 0 &&
		len(record.ClaudeToolResponses) == 0 &&
		len(record.ClaudeResponseBlocks) == 0 &&
		len(record.ResponsesRequestBlocks) == 0 &&
		len(record.ResponsesToolResponses) == 0 &&
		len(record.ResponsesResponseBlocks) == 0 {
		return ""
	}

	jsonBytes, err := common.Marshal(record)
	if err != nil {
		return ""
	}
	return string(jsonBytes)
}

func isClaudeStructuredRecord(relayInfo *relaycommon.RelayInfo) bool {
	if relayInfo == nil {
		return false
	}
	return relayInfo.GetFinalRequestRelayFormat() == types.RelayFormatClaude
}

func isResponsesStructuredRecord(relayInfo *relaycommon.RelayInfo) bool {
	if relayInfo == nil {
		return false
	}
	return relayInfo.GetFinalRequestRelayFormat() == types.RelayFormatOpenAIResponses
}

func shouldRecordClaudeResponseBlocks(relayInfo *relaycommon.RelayInfo) bool {
	if !isClaudeStructuredRecord(relayInfo) || relayInfo == nil || strings.TrimSpace(relayInfo.ResponseBody) == "" {
		return false
	}
	return true
}

func shouldRecordResponsesResponseBlocks(relayInfo *relaycommon.RelayInfo) bool {
	if !isResponsesStructuredRecord(relayInfo) || relayInfo == nil || !relayInfo.IsStream || strings.TrimSpace(relayInfo.ResponseBody) == "" {
		return false
	}
	return true
}

type claudeResponseBlockState struct {
	Block        model.ClaudeResponseBlock
	Content      strings.Builder
	ToolInputRaw strings.Builder
}

func buildClaudeResponseBlocks(relayInfo *relaycommon.RelayInfo) []model.ClaudeResponseBlock {
	if relayInfo == nil {
		return nil
	}
	responseBody := strings.TrimSpace(relayInfo.ResponseBody)
	if responseBody == "" {
		return nil
	}
	if relayInfo.IsStream {
		return buildClaudeResponseBlocksFromSSE(responseBody)
	}
	return buildClaudeResponseBlocksFromMessageBody(responseBody)
}

func buildClaudeResponseBlocksFromMessageBody(responseBody string) []model.ClaudeResponseBlock {
	responseBody = strings.TrimSpace(responseBody)
	if responseBody == "" {
		return nil
	}

	var claudeResponse dto.ClaudeResponse
	if err := common.UnmarshalJsonStr(responseBody, &claudeResponse); err != nil {
		return nil
	}
	if len(claudeResponse.Content) == 0 {
		return nil
	}

	result := make([]model.ClaudeResponseBlock, 0, len(claudeResponse.Content))
	for _, content := range claudeResponse.Content {
		switch strings.TrimSpace(content.Type) {
		case "thinking":
			if content.Thinking == nil {
				continue
			}
			thinkingText := safeTruncateUTF8(*content.Thinking, maxCompletionLength)
			if strings.TrimSpace(thinkingText) == "" {
				continue
			}
			result = append(result, model.ClaudeResponseBlock{
				Type:    "thinking",
				Content: thinkingText,
			})
		case "text":
			text := safeTruncateUTF8(content.GetText(), maxCompletionLength)
			if strings.TrimSpace(text) == "" {
				continue
			}
			result = append(result, model.ClaudeResponseBlock{
				Type:    "text",
				Content: text,
			})
		case "tool_use":
			toolID := strings.TrimSpace(content.Id)
			toolName := strings.TrimSpace(content.Name)
			if toolID == "" && toolName == "" && content.Input == nil {
				continue
			}
			result = append(result, model.ClaudeResponseBlock{
				ID:    toolID,
				Type:  "tool_use",
				Name:  toolName,
				Input: sanitizeToolLogValue(content.Input),
			})
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func buildClaudeResponseBlocksFromSSE(responseBody string) []model.ClaudeResponseBlock {
	responseBody = strings.TrimSpace(responseBody)
	if responseBody == "" {
		return nil
	}

	states := make(map[int]*claudeResponseBlockState)
	stateOrder := make([]int, 0)
	result := make([]model.ClaudeResponseBlock, 0)

	upsertState := func(index int) *claudeResponseBlockState {
		if state, ok := states[index]; ok {
			return state
		}
		state := &claudeResponseBlockState{}
		states[index] = state
		stateOrder = append(stateOrder, index)
		return state
	}

	flushState := func(index int) {
		state, ok := states[index]
		if !ok {
			return
		}
		if block, ok := finalizeClaudeResponseBlock(state); ok {
			result = append(result, block)
		}
		delete(states, index)
	}

	lines := strings.Split(responseBody, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "[DONE]" {
			continue
		}

		var claudeResponse dto.ClaudeResponse
		if err := common.UnmarshalJsonStr(line, &claudeResponse); err != nil {
			continue
		}

		index := claudeResponse.GetIndex()
		switch claudeResponse.Type {
		case "content_block_start":
			flushState(index)
			if claudeResponse.ContentBlock == nil {
				continue
			}
			state := upsertState(index)
			seedClaudeResponseBlockState(state, claudeResponse.ContentBlock)
		case "content_block_delta":
			if claudeResponse.Delta == nil {
				continue
			}
			state := upsertState(index)
			applyClaudeResponseDelta(state, claudeResponse.Delta)
		case "content_block_stop":
			flushState(index)
		}
	}

	for _, index := range stateOrder {
		flushState(index)
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func seedClaudeResponseBlockState(state *claudeResponseBlockState, contentBlock *dto.ClaudeMediaMessage) {
	if state == nil || contentBlock == nil {
		return
	}

	state.Block.ID = strings.TrimSpace(contentBlock.Id)
	state.Block.Type = strings.TrimSpace(contentBlock.Type)

	switch state.Block.Type {
	case "thinking":
		if contentBlock.Thinking != nil {
			state.Content.WriteString(*contentBlock.Thinking)
		}
	case "text":
		if contentBlock.Text != nil {
			state.Content.WriteString(*contentBlock.Text)
		}
	case "tool_use":
		state.Block.Name = strings.TrimSpace(contentBlock.Name)
		if contentBlock.Input != nil {
			state.Block.Input = sanitizeToolLogValue(contentBlock.Input)
		}
	}
}

func applyClaudeResponseDelta(state *claudeResponseBlockState, delta *dto.ClaudeMediaMessage) {
	if state == nil || delta == nil {
		return
	}

	switch delta.Type {
	case "thinking_delta":
		if state.Block.Type == "" {
			state.Block.Type = "thinking"
		}
		if delta.Thinking != nil {
			state.Content.WriteString(*delta.Thinking)
		}
	case "text_delta":
		if state.Block.Type == "" {
			state.Block.Type = "text"
		}
		if delta.Text != nil {
			state.Content.WriteString(*delta.Text)
		}
	case "input_json_delta":
		if state.Block.Type == "" {
			state.Block.Type = "tool_use"
		}
		if delta.PartialJson != nil {
			state.ToolInputRaw.WriteString(*delta.PartialJson)
		}
	}
}

func finalizeClaudeResponseBlock(state *claudeResponseBlockState) (model.ClaudeResponseBlock, bool) {
	if state == nil {
		return model.ClaudeResponseBlock{}, false
	}

	block := state.Block
	switch block.Type {
	case "thinking", "text":
		block.Content = safeTruncateUTF8(state.Content.String(), maxCompletionLength)
		if strings.TrimSpace(block.Content) == "" {
			return model.ClaudeResponseBlock{}, false
		}
		return block, true
	case "tool_use":
		raw := strings.TrimSpace(state.ToolInputRaw.String())
		if raw != "" {
			var input any
			if err := common.UnmarshalJsonStr(raw, &input); err != nil {
				input = map[string]any{"_raw": raw}
			}
			block.Input = sanitizeToolLogValue(input)
		}
		if strings.TrimSpace(block.Name) == "" && block.Input == nil && strings.TrimSpace(block.ID) == "" {
			return model.ClaudeResponseBlock{}, false
		}
		return block, true
	default:
		return model.ClaudeResponseBlock{}, false
	}
}

func buildClaudeToolInvokeRecordsFromBlocks(blocks []model.ClaudeResponseBlock) []model.LogToolInvokeRecord {
	if len(blocks) == 0 {
		return nil
	}

	records := make([]model.LogToolInvokeRecord, 0, len(blocks))
	for _, block := range blocks {
		if block.Type != "tool_use" {
			continue
		}
		if strings.TrimSpace(block.Name) == "" && block.Input == nil && strings.TrimSpace(block.ID) == "" {
			continue
		}
		records = append(records, model.LogToolInvokeRecord{
			ID:    block.ID,
			Name:  block.Name,
			Input: block.Input,
		})
	}
	if len(records) == 0 {
		return nil
	}
	return records
}

type responsesResponseBlockState struct {
	Block        model.ResponsesResponseBlock
	Content      strings.Builder
	ArgumentsRaw strings.Builder
}

func buildResponsesResponseBlocksFromSSE(responseBody string) []model.ResponsesResponseBlock {
	responseBody = strings.TrimSpace(responseBody)
	if responseBody == "" {
		return nil
	}

	states := make(map[string]*responsesResponseBlockState)
	stateOrder := make([]string, 0)
	itemKeysByItemID := make(map[string][]string)

	upsertState := func(key string, initializer func(*responsesResponseBlockState)) *responsesResponseBlockState {
		if key == "" {
			return nil
		}
		if state, ok := states[key]; ok {
			if initializer != nil {
				initializer(state)
			}
			return state
		}
		state := &responsesResponseBlockState{}
		if initializer != nil {
			initializer(state)
		}
		states[key] = state
		stateOrder = append(stateOrder, key)
		if itemID := strings.TrimSpace(state.Block.ID); itemID != "" {
			itemKeysByItemID[itemID] = append(itemKeysByItemID[itemID], key)
		}
		return state
	}

	lines := strings.Split(responseBody, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "[DONE]" {
			continue
		}

		var streamResp dto.ResponsesStreamResponse
		if err := common.UnmarshalJsonStr(line, &streamResp); err != nil {
			continue
		}

		switch streamResp.Type {
		case "response.content_part.added":
			if streamResp.Part == nil || strings.TrimSpace(streamResp.Part.Type) != "output_text" {
				continue
			}
			key := buildResponsesTextBlockKey(streamResp.ItemID, streamResp.OutputIndex, streamResp.ContentIndex)
			state := upsertState(key, func(state *responsesResponseBlockState) {
				state.Block.ID = strings.TrimSpace(streamResp.ItemID)
				state.Block.Type = "output_text"
			})
			if state != nil && strings.TrimSpace(streamResp.Part.Text) != "" {
				state.Content.WriteString(streamResp.Part.Text)
			}
		case "response.output_text.delta":
			key := buildResponsesTextBlockKey(streamResp.ItemID, streamResp.OutputIndex, streamResp.ContentIndex)
			state := upsertState(key, func(state *responsesResponseBlockState) {
				state.Block.ID = strings.TrimSpace(streamResp.ItemID)
				state.Block.Type = "output_text"
			})
			if state != nil && streamResp.Delta != "" {
				state.Content.WriteString(streamResp.Delta)
			}
		case "response.output_text.done":
			key := buildResponsesTextBlockKey(streamResp.ItemID, streamResp.OutputIndex, streamResp.ContentIndex)
			state := upsertState(key, func(state *responsesResponseBlockState) {
				state.Block.ID = strings.TrimSpace(streamResp.ItemID)
				state.Block.Type = "output_text"
			})
			if state != nil {
				mergeResponsesText(&state.Content, streamResp.Text)
			}
		case "response.output_item.added", "response.output_item.done":
			if streamResp.Item == nil {
				continue
			}
			if strings.TrimSpace(streamResp.Item.Type) == "function_call" {
				key := buildResponsesFunctionCallBlockKey(streamResp.Item.ID, streamResp.Item.CallId)
				state := upsertState(key, func(state *responsesResponseBlockState) {
					state.Block.ID = strings.TrimSpace(streamResp.Item.ID)
					state.Block.Type = "function_call"
					state.Block.CallID = strings.TrimSpace(streamResp.Item.CallId)
					state.Block.Name = strings.TrimSpace(streamResp.Item.Name)
				})
				if state != nil {
					if state.Block.CallID == "" {
						state.Block.CallID = strings.TrimSpace(streamResp.Item.ID)
					}
					if state.Block.Name == "" {
						state.Block.Name = strings.TrimSpace(streamResp.Item.Name)
					}
					mergeResponsesText(&state.ArgumentsRaw, streamResp.Item.Arguments)
				}
				continue
			}

			if strings.TrimSpace(streamResp.Item.Type) == "message" {
				itemID := strings.TrimSpace(streamResp.Item.ID)
				for _, key := range itemKeysByItemID[itemID] {
					state := states[key]
					if state == nil || state.Block.Type != "output_text" {
						continue
					}
					for _, content := range streamResp.Item.Content {
						if strings.TrimSpace(content.Type) != "output_text" {
							continue
						}
						mergeResponsesText(&state.Content, content.Text)
						break
					}
				}
			}
		case "response.function_call_arguments.delta":
			key := buildResponsesFunctionCallBlockKey(streamResp.ItemID, "")
			state := upsertState(key, func(state *responsesResponseBlockState) {
				state.Block.ID = strings.TrimSpace(streamResp.ItemID)
				state.Block.Type = "function_call"
				state.Block.CallID = strings.TrimSpace(streamResp.ItemID)
			})
			if state != nil && streamResp.Delta != "" {
				state.ArgumentsRaw.WriteString(streamResp.Delta)
			}
		case "response.function_call_arguments.done":
			key := buildResponsesFunctionCallBlockKey(streamResp.ItemID, "")
			state := upsertState(key, func(state *responsesResponseBlockState) {
				state.Block.ID = strings.TrimSpace(streamResp.ItemID)
				state.Block.Type = "function_call"
				state.Block.CallID = strings.TrimSpace(streamResp.ItemID)
			})
			if state != nil {
				mergeResponsesText(&state.ArgumentsRaw, streamResp.Arguments)
			}
		}
	}

	result := make([]model.ResponsesResponseBlock, 0, len(stateOrder))
	for _, key := range stateOrder {
		state := states[key]
		if state == nil {
			continue
		}

		block := state.Block
		switch block.Type {
		case "output_text":
			block.Content = safeTruncateUTF8(state.Content.String(), maxCompletionLength)
			if strings.TrimSpace(block.Content) == "" {
				continue
			}
		case "function_call":
			rawArguments := strings.TrimSpace(state.ArgumentsRaw.String())
			if rawArguments != "" {
				block.Arguments = sanitizeResponsesJSONLikeValue(rawArguments)
			}
			if strings.TrimSpace(block.CallID) == "" {
				block.CallID = strings.TrimSpace(block.ID)
			}
			if strings.TrimSpace(block.Name) == "" && block.Arguments == nil && strings.TrimSpace(block.CallID) == "" {
				continue
			}
		default:
			continue
		}

		result = append(result, block)
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func buildResponsesTextBlockKey(itemID string, outputIndex, contentIndex *int) string {
	key := strings.TrimSpace(itemID)
	if key == "" {
		key = "text"
	}
	if outputIndex != nil {
		key += "|output:" + strconv.Itoa(*outputIndex)
	}
	if contentIndex != nil {
		key += "|content:" + strconv.Itoa(*contentIndex)
	}
	return "text|" + key
}

func buildResponsesFunctionCallBlockKey(itemID, callID string) string {
	callID = strings.TrimSpace(callID)
	itemID = strings.TrimSpace(itemID)
	if itemID == "" && callID == "" {
		return ""
	}
	if itemID == "" {
		itemID = callID
	}
	return "call|" + itemID
}

func mergeResponsesText(builder *strings.Builder, next string) {
	if builder == nil {
		return
	}
	if next == "" {
		return
	}
	current := builder.String()
	if current == "" {
		builder.WriteString(next)
		return
	}
	if strings.HasPrefix(next, current) {
		builder.WriteString(next[len(current):])
		return
	}
	if !strings.Contains(current, next) {
		builder.WriteString(next)
	}
}

func sanitizeResponsesJSONLikeValue(value any) any {
	if value == nil {
		return nil
	}

	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return nil
		}
		var parsed any
		if err := common.UnmarshalJsonStr(trimmed, &parsed); err == nil {
			return sanitizeToolLogValue(parsed)
		}
		return summarizeLongUTF8(trimmed, maxLoggedJSONValueLength)
	default:
		return sanitizeToolLogValue(value)
	}
}

func buildResponsesToolInvokeRecordsFromBlocks(blocks []model.ResponsesResponseBlock) []model.LogToolInvokeRecord {
	if len(blocks) == 0 {
		return nil
	}

	records := make([]model.LogToolInvokeRecord, 0, len(blocks))
	for _, block := range blocks {
		if block.Type != "function_call" {
			continue
		}
		id := strings.TrimSpace(block.CallID)
		if id == "" {
			id = strings.TrimSpace(block.ID)
		}
		if id == "" && strings.TrimSpace(block.Name) == "" && block.Arguments == nil {
			continue
		}
		records = append(records, model.LogToolInvokeRecord{
			ID:    id,
			Name:  block.Name,
			Input: block.Arguments,
		})
	}
	if len(records) == 0 {
		return nil
	}
	return records
}

func BuildFullLogRecord(relayInfo *relaycommon.RelayInfo) string {
	if !operation_setting.IsFullLogConsumeEnabled() || relayInfo == nil {
		return ""
	}

	record := model.FullLogRecord{}
	requestHeaders := filterSensitiveHeaders(relayInfo.RequestHeaders)
	requestBody := buildFullLogRequestBody(relayInfo)
	responseBody := parseLoggedBody(relayInfo.ResponseBody)

	if requestHeaders != nil || requestBody != nil {
		record.Request = &model.FullLogRequest{
			Headers: requestHeaders,
			Body:    requestBody,
		}
	}

	if responseBody != nil {
		record.Response = &model.FullLogResponse{
			Body: responseBody,
		}
	}

	if meta := buildFullLogMeta(relayInfo); meta != nil {
		record.Meta = meta
	}

	if record.Request == nil && record.Response == nil && record.Meta == nil {
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

func buildFullLogRequestBody(relayInfo *relaycommon.RelayInfo) interface{} {
	if relayInfo == nil || relayInfo.Request == nil {
		return nil
	}

	requestBytes, err := common.Marshal(relayInfo.Request)
	if err != nil || len(requestBytes) == 0 {
		return nil
	}
	return parseLoggedBody(string(requestBytes))
}

func buildFullLogMeta(relayInfo *relaycommon.RelayInfo) *model.FullLogMeta {
	if relayInfo == nil {
		return nil
	}

	meta := &model.FullLogMeta{
		RequestID:          strings.TrimSpace(relayInfo.RequestId),
		RequestPath:        sanitizeRequestPath(relayInfo.RequestURLPath),
		IsStream:           relayInfo.IsStream,
		RelayFormat:        string(relayInfo.RelayFormat),
		FinalRequestFormat: string(relayInfo.GetFinalRequestRelayFormat()),
		RetryIndex:         relayInfo.RetryIndex,
	}

	if meta.RequestID == "" &&
		meta.RequestPath == "" &&
		meta.RelayFormat == "" &&
		meta.FinalRequestFormat == "" &&
		meta.RetryIndex == 0 &&
		!meta.IsStream {
		return nil
	}
	return meta
}

func sanitizeRequestPath(requestURLPath string) string {
	path := strings.TrimSpace(requestURLPath)
	if path == "" {
		return ""
	}
	if idx := strings.Index(path, "?"); idx != -1 {
		return path[:idx]
	}
	return path
}

func parseLoggedBody(body string) interface{} {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}

	var parsed interface{}
	if err := common.UnmarshalJsonStr(body, &parsed); err == nil {
		return parsed
	}
	return body
}

// buildPromptRecordFromClaude 从 Claude 格式请求中构建 prompt 记录
// 只提取最后一个用户消息，避免存储过多内容
func buildPromptRecordFromClaude(req *dto.ClaudeRequest) map[string]interface{} {
	if req == nil {
		return nil
	}

	result := make(map[string]interface{})
	lastUserMessage, requestBlocks := extractLastClaudeUserPrompt(req)
	if lastUserMessage == nil {
		return nil
	}

	result["lastUserMessage"] = lastUserMessage
	if len(requestBlocks) > 0 {
		result["claudeRequestBlocks"] = requestBlocks
	}
	return result
}

func buildClaudeRequestBlocks(req *dto.ClaudeRequest) []model.ClaudeRequestBlock {
	_, requestBlocks := extractLastClaudeUserPrompt(req)
	if len(requestBlocks) == 0 {
		return nil
	}
	return requestBlocks
}

func buildClaudeToolResponseBlocks(req *dto.ClaudeRequest) []model.ClaudeToolResponseBlock {
	if req == nil || len(req.Messages) == 0 {
		return nil
	}

	lastUserMessage := findLastClaudeUserMessage(req)
	if lastUserMessage == nil || lastUserMessage.IsStringContent() {
		return nil
	}

	toolNameMap := buildClaudeToolNameMap(req)
	contents, err := lastUserMessage.ParseContent()
	if err != nil || len(contents) == 0 {
		return nil
	}

	blocks := make([]model.ClaudeToolResponseBlock, 0)
	for _, content := range contents {
		if strings.TrimSpace(content.Type) != "tool_result" {
			continue
		}
		toolUseID := strings.TrimSpace(content.ToolUseId)
		if toolUseID == "" {
			continue
		}
		name := strings.TrimSpace(content.Name)
		if name == "" {
			name = toolNameMap[toolUseID]
		}
		blocks = append(blocks, model.ClaudeToolResponseBlock{
			ToolUseID: toolUseID,
			Name:      name,
			Type:      "tool_result",
			Role:      strings.TrimSpace(lastUserMessage.Role),
		})
	}

	if len(blocks) == 0 {
		return nil
	}
	return blocks
}

func extractLastClaudeUserPrompt(req *dto.ClaudeRequest) (map[string]interface{}, []model.ClaudeRequestBlock) {
	lastUserMessage := findLastClaudeUserMessage(req)
	if lastUserMessage == nil {
		return nil, nil
	}

	requestBlocks := make([]model.ClaudeRequestBlock, 0)
	messageRecord := map[string]interface{}{
		"role": strings.TrimSpace(lastUserMessage.Role),
	}

	if lastUserMessage.IsStringContent() {
		content := safeTruncateUTF8(lastUserMessage.GetStringContent(), maxCompletionLength)
		if strings.TrimSpace(content) == "" {
			return nil, nil
		}
		requestBlocks = append(requestBlocks, model.ClaudeRequestBlock{
			Type: "text",
			Text: content,
		})
		messageRecord["content"] = content
		messageRecord["contentList"] = requestBlocks
		return messageRecord, requestBlocks
	}

	contents, err := lastUserMessage.ParseContent()
	if err != nil || len(contents) == 0 {
		return nil, nil
	}

	textParts := make([]string, 0, len(contents))
	for _, content := range contents {
		block, ok := buildClaudeRequestBlock(content)
		if !ok {
			continue
		}
		requestBlocks = append(requestBlocks, block)
		if strings.TrimSpace(block.Text) != "" {
			textParts = append(textParts, block.Text)
		}
	}

	if len(requestBlocks) == 0 {
		return nil, nil
	}
	if len(textParts) > 0 {
		messageRecord["content"] = strings.Join(textParts, "\n")
	}
	messageRecord["contentList"] = requestBlocks
	return messageRecord, requestBlocks
}

func buildClaudeRequestBlock(content dto.ClaudeMediaMessage) (model.ClaudeRequestBlock, bool) {
	blockType := strings.TrimSpace(content.Type)
	text := ""
	if content.Text != nil {
		text = safeTruncateUTF8(*content.Text, maxCompletionLength)
	}
	if blockType == "" {
		blockType = "text"
	}
	if blockType == "tool_result" {
		return model.ClaudeRequestBlock{}, false
	}
	if strings.TrimSpace(blockType) == "" {
		return model.ClaudeRequestBlock{}, false
	}
	if blockType == "text" && strings.TrimSpace(text) == "" {
		return model.ClaudeRequestBlock{}, false
	}
	return model.ClaudeRequestBlock{
		Type: blockType,
		Text: text,
	}, true
}

func findLastClaudeUserMessage(req *dto.ClaudeRequest) *dto.ClaudeMessage {
	if req == nil || len(req.Messages) == 0 {
		return nil
	}
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			return &req.Messages[i]
		}
	}
	return nil
}

func buildClaudeToolNameMap(req *dto.ClaudeRequest) map[string]string {
	if req == nil || len(req.Messages) == 0 {
		return nil
	}

	toolNameMap := make(map[string]string)
	for _, message := range req.Messages {
		contents, err := message.ParseContent()
		if err != nil || len(contents) == 0 {
			continue
		}
		for _, content := range contents {
			if strings.TrimSpace(content.Type) != "tool_use" {
				continue
			}
			toolUseID := strings.TrimSpace(content.Id)
			name := strings.TrimSpace(content.Name)
			if toolUseID == "" || name == "" {
				continue
			}
			toolNameMap[toolUseID] = name
		}
	}
	return toolNameMap
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
// 记录 input 的结构化内容，避免直接落完整原始上下文
func buildPromptRecordFromResponses(req *dto.OpenAIResponsesRequest) map[string]interface{} {
	if req == nil {
		return nil
	}

	recordData := parseResponsesInputForRecord(req.Input)
	if len(recordData.PromptInput) == 0 && recordData.LastUserText == "" {
		return nil
	}

	result := make(map[string]interface{}, 2)
	if recordData.LastUserText != "" {
		result["lastUserMessage"] = map[string]interface{}{
			"role":    "user",
			"content": recordData.LastUserText,
		}
	}
	if len(recordData.PromptInput) > 0 {
		result["input"] = recordData.PromptInput
	}

	return result
}

func buildResponsesRequestBlocks(req *dto.OpenAIResponsesRequest) []model.ResponsesRequestBlock {
	if req == nil {
		return nil
	}
	recordData := parseResponsesInputForRecord(req.Input)
	if len(recordData.RequestBlocks) == 0 {
		return nil
	}
	return recordData.RequestBlocks
}

func buildResponsesToolResponseBlocks(req *dto.OpenAIResponsesRequest) []model.ResponsesToolResponseBlock {
	if req == nil {
		return nil
	}
	recordData := parseResponsesInputForRecord(req.Input)
	if len(recordData.ToolResponses) == 0 {
		return nil
	}
	return recordData.ToolResponses
}

type responsesInputRecordData struct {
	PromptInput   []map[string]interface{}
	RequestBlocks []model.ResponsesRequestBlock
	ToolResponses []model.ResponsesToolResponseBlock
	LastUserText  string
}

type responsesInputSegmentItem struct {
	Type      string
	Role      string
	Text      string
	CallID    string
	Name      string
	Arguments any
}

func parseResponsesInputForRecord(input json.RawMessage) responsesInputRecordData {
	result := responsesInputRecordData{}
	if len(input) == 0 {
		return result
	}

	if common.GetJsonType(input) == "string" {
		var text string
		_ = common.Unmarshal(input, &text)
		text = safeTruncateUTF8(text, maxCompletionLength)
		if strings.TrimSpace(text) == "" {
			return result
		}
		result.PromptInput = append(result.PromptInput, map[string]interface{}{
			"type": "input_text",
			"text": text,
		})
		result.RequestBlocks = append(result.RequestBlocks, model.ResponsesRequestBlock{
			Type: "input_text",
			Text: text,
		})
		result.LastUserText = text
		return result
	}

	if common.GetJsonType(input) != "array" {
		return result
	}

	var items []map[string]interface{}
	if err := common.Unmarshal(input, &items); err != nil {
		return result
	}

	toolNameByCallID := make(map[string]string)
	flatItems := make([]responsesInputSegmentItem, 0)

	for _, item := range items {
		itemType := strings.TrimSpace(common.Interface2String(item["type"]))
		switch itemType {
		case "message":
			role := strings.TrimSpace(common.Interface2String(item["role"]))
			flatItems = append(flatItems, buildResponsesPromptMessageContent(item["content"], role)...)
		case "function_call":
			callID := strings.TrimSpace(common.Interface2String(item["call_id"]))
			name := strings.TrimSpace(common.Interface2String(item["name"]))
			if callID != "" && name != "" {
				toolNameByCallID[callID] = name
			}
			flatItems = append(flatItems, responsesInputSegmentItem{
				Type:      "function_call",
				CallID:    callID,
				Name:      name,
				Arguments: sanitizeResponsesJSONLikeValue(item["arguments"]),
			})
		case "function_call_output":
			callID := strings.TrimSpace(common.Interface2String(item["call_id"]))
			name := strings.TrimSpace(common.Interface2String(item["name"]))
			if name == "" {
				name = toolNameByCallID[callID]
			}
			flatItems = append(flatItems, responsesInputSegmentItem{
				Type:   "function_call_output",
				CallID: callID,
				Name:   name,
			})
		case "input_text", "text", "output_text":
			text := safeTruncateUTF8(common.Interface2String(item["text"]), maxCompletionLength)
			if strings.TrimSpace(text) == "" {
				continue
			}
			flatItems = append(flatItems, responsesInputSegmentItem{
				Type: itemType,
				Text: text,
			})
		case "reasoning":
			continue
		}
	}

	segmentItems := selectResponsesInputSegment(flatItems)
	for _, item := range segmentItems {
		switch item.Type {
		case "input_text", "text", "output_text":
			promptItem := map[string]interface{}{
				"type": item.Type,
				"text": item.Text,
			}
			if item.Role != "" {
				promptItem["role"] = item.Role
			}
			result.PromptInput = append(result.PromptInput, promptItem)
			if item.Type == "input_text" || item.Type == "text" {
				result.RequestBlocks = append(result.RequestBlocks, model.ResponsesRequestBlock{
					Type: item.Type,
					Role: item.Role,
					Text: item.Text,
				})
				result.LastUserText = item.Text
			}
		case "function_call":
			promptItem := map[string]interface{}{
				"type": "function_call",
			}
			if item.CallID != "" {
				promptItem["call_id"] = item.CallID
			}
			if item.Name != "" {
				promptItem["name"] = item.Name
			}
			if item.Arguments != nil {
				promptItem["arguments"] = item.Arguments
			}
			result.PromptInput = append(result.PromptInput, promptItem)
		case "function_call_output":
			promptItem := map[string]interface{}{
				"type": "function_call_output",
			}
			if item.CallID != "" {
				promptItem["call_id"] = item.CallID
			}
			if item.Name != "" {
				promptItem["name"] = item.Name
			}
			result.PromptInput = append(result.PromptInput, promptItem)
			result.ToolResponses = append(result.ToolResponses, model.ResponsesToolResponseBlock{
				CallID: item.CallID,
				Name:   item.Name,
				Type:   "function_call_output",
			})
		}
	}

	return result
}

func buildResponsesPromptMessageContent(content any, role string) []responsesInputSegmentItem {
	switch typed := content.(type) {
	case string:
		text := safeTruncateUTF8(typed, maxCompletionLength)
		if strings.TrimSpace(text) == "" {
			return nil
		}
		return []responsesInputSegmentItem{
			{
				Type: "input_text",
				Role: role,
				Text: text,
			},
		}
	case []interface{}:
		records := make([]responsesInputSegmentItem, 0, len(typed))
		for _, partAny := range typed {
			part, ok := partAny.(map[string]interface{})
			if !ok {
				continue
			}

			partType := strings.TrimSpace(common.Interface2String(part["type"]))
			if partType == "" {
				partType = "input_text"
			}
			if partType != "input_text" && partType != "text" && partType != "output_text" {
				continue
			}

			text := safeTruncateUTF8(common.Interface2String(part["text"]), maxCompletionLength)
			if strings.TrimSpace(text) == "" {
				continue
			}

			records = append(records, responsesInputSegmentItem{
				Type: partType,
				Role: role,
				Text: text,
			})
		}
		return records
	default:
		return nil
	}
}

func selectResponsesInputSegment(items []responsesInputSegmentItem) []responsesInputSegmentItem {
	if len(items) == 0 {
		return nil
	}

	lastIndex := len(items) - 1
	lastType := strings.TrimSpace(items[lastIndex].Type)
	if lastType == "" {
		return items
	}

	switch lastType {
	case "output_text":
		return selectResponsesItemsUntilBoundary(items, lastIndex, map[string]bool{
			"function_call": true,
			"input_text":    true,
			"text":          true,
		})
	case "function_call_output":
		return selectResponsesItemsUntilBoundary(items, lastIndex, map[string]bool{
			"function_call": true,
			"input_text":    true,
			"text":          true,
			"output_text":   true,
		})
	case "input_text", "text":
		start := lastIndex
		for start >= 0 {
			itemType := strings.TrimSpace(items[start].Type)
			if itemType != "input_text" && itemType != "text" {
				break
			}
			start--
		}
		return append([]responsesInputSegmentItem(nil), items[start+1:lastIndex+1]...)
	case "function_call":
		return selectResponsesItemsUntilBoundary(items, lastIndex, map[string]bool{
			"function_call_output": true,
			"input_text":           true,
			"text":                 true,
			"output_text":          true,
		})
	default:
		return append([]responsesInputSegmentItem(nil), items...)
	}
}

func selectResponsesItemsUntilBoundary(items []responsesInputSegmentItem, lastIndex int, boundaries map[string]bool) []responsesInputSegmentItem {
	start := lastIndex
	for start >= 0 {
		itemType := strings.TrimSpace(items[start].Type)
		if start != lastIndex && boundaries[itemType] {
			break
		}
		start--
	}
	return append([]responsesInputSegmentItem(nil), items[start+1:lastIndex+1]...)
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

func buildToolInvokeRecords(relayInfo *relaycommon.RelayInfo) []model.LogToolInvokeRecord {
	if relayInfo == nil {
		return nil
	}

	toolMap := make(map[string]*model.LogToolInvokeRecord)
	toolOrder := make([]string, 0)

	upsertTool := func(id string) *model.LogToolInvokeRecord {
		if id == "" {
			id = "tool-" + common.GetUUID()
		}
		if record, ok := toolMap[id]; ok {
			return record
		}
		record := &model.LogToolInvokeRecord{ID: id}
		toolMap[id] = record
		toolOrder = append(toolOrder, id)
		return record
	}

	for _, invoke := range relayInfo.ToolInvokes {
		record := upsertTool(strings.TrimSpace(invoke.ID))
		if record.Name == "" {
			record.Name = strings.TrimSpace(invoke.Name)
		}
		if record.Input == nil && invoke.Input != nil {
			record.Input = sanitizeToolLogValue(invoke.Input)
		}
		if record.Result == nil && invoke.Result != nil {
			record.Result = sanitizeToolLogValue(invoke.Result)
		}
		if record.ResultText == "" && strings.TrimSpace(invoke.ResultText) != "" {
			record.ResultText = invoke.ResultText
		}
		if record.IsError == nil && invoke.IsError != nil {
			record.IsError = invoke.IsError
		}
		if record.StopReason == "" {
			record.StopReason = strings.TrimSpace(invoke.StopReason)
		}
		if record.ResponseRole == "" {
			record.ResponseRole = strings.TrimSpace(invoke.ResponseRole)
		}
	}

	if claudeReq, ok := relayInfo.Request.(*dto.ClaudeRequest); ok && claudeReq != nil {
		for _, invoke := range extractClaudeToolResults(claudeReq) {
			record := upsertTool(invoke.ID)
			if record.Name == "" {
				record.Name = invoke.Name
			}
			if record.Result == nil && invoke.Result != nil {
				record.Result = invoke.Result
			}
			if record.ResultText == "" {
				record.ResultText = invoke.ResultText
			}
			if record.IsError == nil {
				record.IsError = invoke.IsError
			}
		}
	}

	if len(toolOrder) == 0 {
		return nil
	}

	result := make([]model.LogToolInvokeRecord, 0, len(toolOrder))
	for _, id := range toolOrder {
		if record, ok := toolMap[id]; ok {
			result = append(result, *record)
		}
	}
	return result
}

func extractClaudeToolResults(req *dto.ClaudeRequest) []model.LogToolInvokeRecord {
	if req == nil || len(req.Messages) == 0 {
		return nil
	}

	results := make([]model.LogToolInvokeRecord, 0)
	for _, message := range req.Messages {
		contents, err := message.ParseContent()
		if err != nil || len(contents) == 0 {
			continue
		}
		for _, content := range contents {
			if content.Type != "tool_result" || strings.TrimSpace(content.ToolUseId) == "" {
				continue
			}
			results = append(results, model.LogToolInvokeRecord{
				ID:         strings.TrimSpace(content.ToolUseId),
				Name:       strings.TrimSpace(content.Name),
				Result:     sanitizeToolLogValue(content.Content),
				ResultText: extractClaudeToolResultText(content.Content),
				IsError:    content.IsError,
			})
		}
	}
	return results
}

func extractClaudeToolResultText(content any) string {
	switch value := content.(type) {
	case string:
		return value
	case []any:
		textParts := make([]string, 0, len(value))
		for _, item := range value {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			itemType := strings.TrimSpace(common.Interface2String(itemMap["type"]))
			if itemType != "" && itemType != "text" {
				continue
			}
			text := strings.TrimSpace(common.Interface2String(itemMap["text"]))
			if text != "" {
				textParts = append(textParts, text)
			}
		}
		return strings.Join(textParts, "\n")
	default:
		jsonBytes, err := common.Marshal(value)
		if err != nil {
			return ""
		}
		return string(jsonBytes)
	}
}

func sanitizeToolLogValue(value any) any {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case json.RawMessage:
		if len(typed) == 0 {
			return nil
		}
		var decoded any
		if err := common.Unmarshal(typed, &decoded); err == nil {
			return sanitizeToolLogValue(decoded)
		}
		return summarizeLongUTF8(string(typed), maxLoggedJSONValueLength)
	case []byte:
		if len(typed) == 0 {
			return nil
		}
		var decoded any
		if err := common.Unmarshal(typed, &decoded); err == nil {
			return sanitizeToolLogValue(decoded)
		}
		return summarizeLongUTF8(string(typed), maxLoggedJSONValueLength)
	case string:
		return summarizeLongUTF8(typed, maxLoggedJSONValueLength)
	}

	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return nil
	}

	switch rv.Kind() {
	case reflect.Interface, reflect.Pointer:
		if rv.IsNil() {
			return nil
		}
		return sanitizeToolLogValue(rv.Elem().Interface())
	case reflect.Map:
		sanitizedMap := reflect.MakeMapWithSize(rv.Type(), rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			key := iter.Key()
			sanitizedValue := sanitizeToolLogValue(iter.Value().Interface())
			valueToSet := reflect.ValueOf(sanitizedValue)
			if !valueToSet.IsValid() {
				valueToSet = reflect.Zero(rv.Type().Elem())
			} else if !valueToSet.Type().AssignableTo(rv.Type().Elem()) {
				if valueToSet.Type().ConvertibleTo(rv.Type().Elem()) {
					valueToSet = valueToSet.Convert(rv.Type().Elem())
				} else {
					valueToSet = iter.Value()
				}
			}
			sanitizedMap.SetMapIndex(key, valueToSet)
		}
		return sanitizedMap.Interface()
	case reflect.Slice:
		sanitizedSlice := reflect.MakeSlice(rv.Type(), rv.Len(), rv.Len())
		for i := 0; i < rv.Len(); i++ {
			sanitizedValue := sanitizeToolLogValue(rv.Index(i).Interface())
			valueToSet := reflect.ValueOf(sanitizedValue)
			if !valueToSet.IsValid() {
				valueToSet = reflect.Zero(rv.Type().Elem())
			} else if !valueToSet.Type().AssignableTo(rv.Type().Elem()) {
				if valueToSet.Type().ConvertibleTo(rv.Type().Elem()) {
					valueToSet = valueToSet.Convert(rv.Type().Elem())
				} else {
					valueToSet = rv.Index(i)
				}
			}
			sanitizedSlice.Index(i).Set(valueToSet)
		}
		return sanitizedSlice.Interface()
	case reflect.Array:
		sanitizedArray := reflect.New(rv.Type()).Elem()
		for i := 0; i < rv.Len(); i++ {
			sanitizedValue := sanitizeToolLogValue(rv.Index(i).Interface())
			valueToSet := reflect.ValueOf(sanitizedValue)
			if !valueToSet.IsValid() {
				valueToSet = reflect.Zero(rv.Type().Elem())
			} else if !valueToSet.Type().AssignableTo(rv.Type().Elem()) {
				if valueToSet.Type().ConvertibleTo(rv.Type().Elem()) {
					valueToSet = valueToSet.Convert(rv.Type().Elem())
				} else {
					valueToSet = rv.Index(i)
				}
			}
			sanitizedArray.Index(i).Set(valueToSet)
		}
		return sanitizedArray.Interface()
	default:
		return value
	}
}
