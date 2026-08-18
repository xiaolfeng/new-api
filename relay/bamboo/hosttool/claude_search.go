package hosttool

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

// IsClaudeWebSearchHelper 识别 Claude Code WebSearch helper 请求。
// 扫描入口 tools[].type 前缀 web_search_，且没有非 host 工具。不读 UA。
func IsClaudeWebSearchHelper(entryFormat types.RelayFormat, entryBytes []byte) bool {
	search, _ := claudeHelperFlags(entryFormat, entryBytes)
	return search
}

// IsClaudeWebFetchHelper 识别 Claude Code / Anthropic server-side WebFetch helper。
// 扫描入口 tools[].type 前缀 web_fetch_，且没有非 host 工具。不读 UA。
func IsClaudeWebFetchHelper(entryFormat types.RelayFormat, entryBytes []byte) bool {
	_, fetch := claudeHelperFlags(entryFormat, entryBytes)
	return fetch
}

// IsClaudeServerToolHelper 识别 Claude Code 服务端 web_search_* / web_fetch_* helper。
func IsClaudeServerToolHelper(entryFormat types.RelayFormat, entryBytes []byte) bool {
	search, fetch := claudeHelperFlags(entryFormat, entryBytes)
	return search || fetch
}

func claudeHelperFlags(entryFormat types.RelayFormat, entryBytes []byte) (search, fetch bool) {
	if entryFormat != types.RelayFormatClaude || len(entryBytes) == 0 {
		return false, false
	}
	var root map[string]any
	if err := common.Unmarshal(entryBytes, &root); err != nil {
		return false, false
	}
	tools := asMapSlice(root["tools"])
	if len(tools) == 0 {
		return false, false
	}
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		typ, _ := tool["type"].(string)
		name := firstNonEmpty(asString(tool["name"]))
		if fn, ok := tool["function"].(map[string]any); ok {
			if n := asString(fn["name"]); n != "" {
				name = n
			}
		}
		if strings.HasPrefix(typ, "web_search_") {
			search = true
			continue
		}
		if strings.HasPrefix(typ, "web_fetch_") {
			fetch = true
			continue
		}
		if IsHostToolName(name) || IsHostToolType(typ) {
			continue
		}
		return false, false
	}
	return search, fetch
}

func claudeServerToolID(requestID string) string {
	if strings.TrimSpace(requestID) == "" {
		return "srvtoolu_host_web_search"
	}
	return "srvtoolu_" + requestID
}

func claudeMessageID(requestID string) string {
	if strings.TrimSpace(requestID) == "" {
		return "msg_host_web_search"
	}
	return "msg_" + requestID
}

func claudeSearchResultContent(result ExecResult) any {
	if !result.OK {
		code := result.ErrorCode
		if code == "" {
			code = ErrInvalidInput
		}
		return map[string]any{"error_code": code}
	}
	src := resolvedHits(result)
	hits := make([]map[string]any, 0, len(src))
	for _, hit := range src {
		if strings.TrimSpace(hit.URL) == "" {
			continue
		}
		hits = append(hits, map[string]any{
			"type":  "web_search_result",
			"title": hit.Title,
			"url":   hit.URL,
		})
	}
	return hits
}

func MarshalClaudeServerSearch(model, requestID string, result ExecResult) ([]byte, error) {
	toolID := claudeServerToolID(requestID)
	query := result.Query
	body := map[string]any{
		"id":    claudeMessageID(requestID),
		"type":  "message",
		"role":  "assistant",
		"model": model,
		"content": []any{
			map[string]any{
				"type":  "server_tool_use",
				"id":    toolID,
				"name":  "web_search",
				"input": map[string]any{"query": query},
			},
			map[string]any{
				"type":        "web_search_tool_result",
				"tool_use_id": toolID,
				"content":     claudeSearchResultContent(result),
			},
		},
		"stop_reason": "end_turn",
		"usage": map[string]any{
			"input_tokens":  0,
			"output_tokens": 0,
		},
	}
	return common.Marshal(body)
}

func claudeFetchToolID(requestID string) string {
	if strings.TrimSpace(requestID) == "" {
		return "srvtoolu_host_web_fetch"
	}
	return "srvtoolu_fetch_" + requestID
}

func claudeFetchMessageID(requestID string) string {
	if strings.TrimSpace(requestID) == "" {
		return "msg_host_web_fetch"
	}
	return "msg_fetch_" + requestID
}

func claudeFetchResultContent(result ExecResult) any {
	if !result.OK {
		code := result.ErrorCode
		if code == "" {
			code = ErrInvalidInput
		}
		return map[string]any{"error_code": code}
	}
	body := result.Body
	if max := model_setting.GetBambooSettings().ClampMaxResultRunes(); max > 0 {
		body, _ = truncateRunes(body, max)
	}
	retrievedAt := time.Now().UTC().Format(time.RFC3339)
	return map[string]any{
		"type":         "web_fetch_result",
		"url":          result.URL,
		"retrieved_at": retrievedAt,
		"content": map[string]any{
			"type": "document",
			"source": map[string]any{
				"type":       "text",
				"media_type": "text/plain",
				"data":       body,
			},
		},
	}
}

func MarshalClaudeServerFetch(model, requestID string, result ExecResult) ([]byte, error) {
	toolID := claudeFetchToolID(requestID)
	body := map[string]any{
		"id":    claudeFetchMessageID(requestID),
		"type":  "message",
		"role":  "assistant",
		"model": model,
		"content": []any{
			map[string]any{
				"type":  "server_tool_use",
				"id":    toolID,
				"name":  "web_fetch",
				"input": map[string]any{"url": result.URL},
			},
			map[string]any{
				"type":        "web_fetch_tool_result",
				"tool_use_id": toolID,
				"content":     claudeFetchResultContent(result),
			},
		},
		"stop_reason": "end_turn",
		"usage": map[string]any{
			"input_tokens":  0,
			"output_tokens": 0,
		},
	}
	return common.Marshal(body)
}

func marshalClaudeSSE(eventType string, payload map[string]any) ([]byte, error) {
	if payload == nil {
		payload = map[string]any{}
	}
	if _, ok := payload["type"]; !ok {
		payload["type"] = eventType
	}
	data, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, data)), nil
}

func ClaudeServerSearchStreamFrames(model, requestID string, result ExecResult) ([][]byte, error) {
	msgID := claudeMessageID(requestID)
	toolID := claudeServerToolID(requestID)
	queryJSON, err := common.Marshal(map[string]any{"query": result.Query})
	if err != nil {
		return nil, err
	}

	var frames [][]byte
	appendFrame := func(eventType string, payload map[string]any) error {
		frame, ferr := marshalClaudeSSE(eventType, payload)
		if ferr != nil {
			return ferr
		}
		frames = append(frames, frame)
		return nil
	}

	if err := appendFrame("message_start", map[string]any{
		"message": map[string]any{
			"id":      msgID,
			"type":    "message",
			"role":    "assistant",
			"model":   model,
			"content": []any{},
			"usage": map[string]any{
				"input_tokens":  0,
				"output_tokens": 0,
			},
		},
	}); err != nil {
		return nil, err
	}

	if err := appendFrame("content_block_start", map[string]any{
		"index": 0,
		"content_block": map[string]any{
			"type":  "server_tool_use",
			"id":    toolID,
			"name":  "web_search",
			"input": "",
		},
	}); err != nil {
		return nil, err
	}

	if err := appendFrame("content_block_delta", map[string]any{
		"index": 0,
		"delta": map[string]any{
			"type":         "input_json_delta",
			"partial_json": string(queryJSON),
		},
	}); err != nil {
		return nil, err
	}

	if err := appendFrame("content_block_stop", map[string]any{"index": 0}); err != nil {
		return nil, err
	}

	if err := appendFrame("content_block_start", map[string]any{
		"index": 1,
		"content_block": map[string]any{
			"type":        "web_search_tool_result",
			"tool_use_id": toolID,
			"content":     claudeSearchResultContent(result),
		},
	}); err != nil {
		return nil, err
	}

	if err := appendFrame("content_block_stop", map[string]any{"index": 1}); err != nil {
		return nil, err
	}

	if err := appendFrame("message_delta", map[string]any{
		"delta": map[string]any{"stop_reason": "end_turn"},
		"usage": map[string]any{"output_tokens": 0},
	}); err != nil {
		return nil, err
	}

	if err := appendFrame("message_stop", map[string]any{}); err != nil {
		return nil, err
	}
	return frames, nil
}

func ClaudeServerFetchStreamFrames(model, requestID string, result ExecResult) ([][]byte, error) {
	msgID := claudeFetchMessageID(requestID)
	toolID := claudeFetchToolID(requestID)
	urlJSON, err := common.Marshal(map[string]any{"url": result.URL})
	if err != nil {
		return nil, err
	}

	var frames [][]byte
	appendFrame := func(eventType string, payload map[string]any) error {
		frame, ferr := marshalClaudeSSE(eventType, payload)
		if ferr != nil {
			return ferr
		}
		frames = append(frames, frame)
		return nil
	}

	if err := appendFrame("message_start", map[string]any{
		"message": map[string]any{
			"id":      msgID,
			"type":    "message",
			"role":    "assistant",
			"model":   model,
			"content": []any{},
			"usage": map[string]any{
				"input_tokens":  0,
				"output_tokens": 0,
			},
		},
	}); err != nil {
		return nil, err
	}

	if err := appendFrame("content_block_start", map[string]any{
		"index": 0,
		"content_block": map[string]any{
			"type":  "server_tool_use",
			"id":    toolID,
			"name":  "web_fetch",
			"input": "",
		},
	}); err != nil {
		return nil, err
	}

	if err := appendFrame("content_block_delta", map[string]any{
		"index": 0,
		"delta": map[string]any{
			"type":         "input_json_delta",
			"partial_json": string(urlJSON),
		},
	}); err != nil {
		return nil, err
	}

	if err := appendFrame("content_block_stop", map[string]any{"index": 0}); err != nil {
		return nil, err
	}

	if err := appendFrame("content_block_start", map[string]any{
		"index": 1,
		"content_block": map[string]any{
			"type":        "web_fetch_tool_result",
			"tool_use_id": toolID,
			"content":     claudeFetchResultContent(result),
		},
	}); err != nil {
		return nil, err
	}

	if err := appendFrame("content_block_stop", map[string]any{"index": 1}); err != nil {
		return nil, err
	}

	if err := appendFrame("message_delta", map[string]any{
		"delta": map[string]any{"stop_reason": "end_turn"},
		"usage": map[string]any{"output_tokens": 0},
	}); err != nil {
		return nil, err
	}

	if err := appendFrame("message_stop", map[string]any{}); err != nil {
		return nil, err
	}
	return frames, nil
}
