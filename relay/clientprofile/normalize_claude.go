package clientprofile

import (
	"bytes"
	"context"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

// 调用方禁止预判 shouldNormalizeClaude。漏匹配 / 非 Claude 入口必须进到 helper 内部再 no-op。

func shouldNormalizeClaude(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	if !model_setting.GetBambooSettings().ClaudeStrictEgressEnabled() {
		return false
	}
	if info.ClientProfile != common.ClientProfileClaudeCode {
		return false
	}
	return info.RelayFormat == types.RelayFormatClaude
}

// NormalizeClaudeSSEData 处理单条 data: JSON。非法 delta / 无 start 的事件返回空切片表示丢弃。
func NormalizeClaudeSSEData(info *relaycommon.RelayInfo, data []byte) []byte {
	if info == nil || len(data) == 0 {
		return data
	}
	if !shouldNormalizeClaude(info) {
		return data
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return data
	}
	var payload map[string]any
	if err := common.Unmarshal(trimmed, &payload); err != nil {
		return data
	}
	if !keepClaudeEvent(info, payload) {
		return nil
	}
	return data
}

// NormalizeClaudeSSEFrame 拆完整 SSE 帧，只改 JSON data 行；ping / 注释字节不变。
func NormalizeClaudeSSEFrame(info *relaycommon.RelayInfo, frame []byte) []byte {
	if info == nil || len(frame) == 0 {
		return frame
	}
	if !shouldNormalizeClaude(info) {
		return frame
	}
	parts := splitSSEFrames(frame)
	if len(parts) == 0 {
		return frame
	}
	var out [][]byte
	changed := false
	for _, part := range parts {
		kept := filterClaudeSSEFrame(info, part)
		if kept == nil {
			changed = true
			continue
		}
		if !bytes.Equal(kept, part) {
			changed = true
		}
		out = append(out, kept)
	}
	if !changed {
		return frame
	}
	if len(out) == 0 {
		return nil
	}
	return bytes.Join(out, nil)
}

func filterClaudeSSEFrame(info *relaycommon.RelayInfo, frame []byte) []byte {
	dataLine, ok := sseJSONData(frame)
	if !ok {
		return frame
	}
	var payload map[string]any
	if err := common.Unmarshal(dataLine, &payload); err != nil {
		return frame
	}
	if keepClaudeEvent(info, payload) {
		return frame
	}
	return nil
}

func keepClaudeEvent(info *relaycommon.RelayInfo, payload map[string]any) bool {
	typ, _ := payload["type"].(string)
	gate := ensureClaudeGate(info)
	switch typ {
	case "message_start":
		gate.SawMessageStart = true
		return true
	case "content_block_start":
		idx, ok := jsonIndex(payload["index"])
		if !ok {
			return true
		}
		block, _ := payload["content_block"].(map[string]any)
		btype := ""
		inputIsObject := false
		if block != nil {
			btype, _ = block["type"].(string)
			if input, exists := block["input"]; exists {
				switch input.(type) {
				case map[string]any, []any:
					inputIsObject = true
				}
			}
		}
		gate.Blocks[idx] = relaycommon.ClaudeStreamBlock{Type: btype, InputIsObject: inputIsObject}
		return true
	case "content_block_delta":
		idx, ok := jsonIndex(payload["index"])
		if !ok {
			warnClaudeDrop(info, "content_block_delta missing index")
			return false
		}
		st, exists := gate.Blocks[idx]
		if !exists {
			warnClaudeDrop(info, fmt.Sprintf("content_block_delta index=%d has no start", idx))
			return false
		}
		delta, _ := payload["delta"].(map[string]any)
		dtype := ""
		if delta != nil {
			dtype, _ = delta["type"].(string)
		}
		switch dtype {
		case "input_json_delta":
			if st.Type != "tool_use" && st.Type != "server_tool_use" {
				warnClaudeDrop(info, fmt.Sprintf("input_json_delta on block type=%s", st.Type))
				return false
			}
			if st.InputIsObject {
				warnClaudeDrop(info, fmt.Sprintf("input_json_delta after object input index=%d", idx))
				return false
			}
		case "text_delta":
			if st.Type != "text" {
				warnClaudeDrop(info, fmt.Sprintf("text_delta on block type=%s", st.Type))
				return false
			}
		case "thinking_delta", "signature_delta":
			if st.Type != "thinking" {
				warnClaudeDrop(info, fmt.Sprintf("%s on block type=%s", dtype, st.Type))
				return false
			}
		}
		return true
	case "content_block_stop":
		if !gate.SawMessageStart {
			warnClaudeDrop(info, "content_block_stop before message_start")
			return false
		}
		idx, ok := jsonIndex(payload["index"])
		if !ok {
			return true
		}
		if _, exists := gate.Blocks[idx]; !exists {
			warnClaudeDrop(info, fmt.Sprintf("content_block_stop index=%d has no start", idx))
			return false
		}
		return true
	default:
		return true
	}
}

func ensureClaudeGate(info *relaycommon.RelayInfo) *relaycommon.ClaudeStreamGate {
	if info.ClaudeStreamGate == nil {
		info.ClaudeStreamGate = &relaycommon.ClaudeStreamGate{
			Blocks: make(map[int]relaycommon.ClaudeStreamBlock),
		}
	}
	if info.ClaudeStreamGate.Blocks == nil {
		info.ClaudeStreamGate.Blocks = make(map[int]relaycommon.ClaudeStreamBlock)
	}
	return info.ClaudeStreamGate
}

func warnClaudeDrop(info *relaycommon.RelayInfo, reason string) {
	reqID := ""
	if info != nil {
		reqID = info.RequestId
	}
	logger.LogWarn(context.Background(), fmt.Sprintf("request_id=%s claude-strict dropped event: %s", reqID, reason))
}

func jsonIndex(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	default:
		return 0, false
	}
}

func sseJSONData(frame []byte) ([]byte, bool) {
	for _, line := range bytes.Split(frame, []byte("\n")) {
		trim := bytes.TrimSpace(line)
		if bytes.HasPrefix(trim, []byte("data:")) {
			payload := bytes.TrimSpace(trim[5:])
			if len(payload) > 0 && payload[0] == '{' {
				return payload, true
			}
		}
	}
	return nil, false
}

func splitSSEFrames(raw []byte) [][]byte {
	if len(raw) == 0 {
		return nil
	}
	var frames [][]byte
	rest := raw
	for {
		idx := bytes.Index(rest, []byte("\n\n"))
		if idx < 0 {
			if len(bytes.TrimSpace(rest)) > 0 {
				frames = append(frames, rest)
			}
			break
		}
		frames = append(frames, rest[:idx+2])
		rest = rest[idx+2:]
		if len(rest) == 0 {
			break
		}
	}
	return frames
}
