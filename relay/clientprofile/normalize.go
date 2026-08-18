package clientprofile

// 调用方禁止预判 ShouldNormalizeResponses。
// 漏匹配观测必须看见 generic Responses；预判门闩会使 TestNormalizeResponsesMissedUAWarn 失败。

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

func ShouldNormalizeResponses(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	if !model_setting.GetBambooSettings().GrokStrictEgressEnabled() {
		return false
	}
	if info.ClientProfile != common.ClientProfileGrokBuild {
		return false
	}
	return info.RelayFormat == types.RelayFormatOpenAIResponses
}

func NormalizeResponsesPayload(info *relaycommon.RelayInfo, payload []byte) []byte {
	return normalizeResponsesJSON(info, payload, "")
}

func NormalizeResponsesSSEData(info *relaycommon.RelayInfo, data []byte) []byte {
	return normalizeResponsesJSON(info, data, "")
}

func NormalizeResponsesSSEFrame(info *relaycommon.RelayInfo, frame []byte) []byte {
	if info == nil || len(frame) == 0 {
		return frame
	}
	if info.RelayFormat != types.RelayFormatOpenAIResponses {
		return frame
	}
	parts := splitSSEFrames(frame)
	if len(parts) == 0 {
		return frame
	}
	var out [][]byte
	changed := false
	for _, part := range parts {
		dataLine, ok := sseJSONData(part)
		if !ok {
			out = append(out, part)
			continue
		}
		next := normalizeResponsesJSON(info, dataLine, eventTypeOfFrame(part))
		if bytes.Equal(next, dataLine) {
			out = append(out, part)
			continue
		}
		changed = true
		out = append(out, replaceSSEData(part, next))
	}
	if !changed {
		return frame
	}
	return bytes.Join(out, nil)
}

func normalizeResponsesJSON(info *relaycommon.RelayInfo, raw []byte, parentEvent string) []byte {
	if info == nil || len(raw) == 0 {
		return raw
	}
	if info.RelayFormat != types.RelayFormatOpenAIResponses {
		return raw
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return raw
	}
	var root map[string]any
	if err := common.Unmarshal(trimmed, &root); err != nil {
		return raw
	}
	observeMissedUA(info, root, parentEvent)
	if !ShouldNormalizeResponses(info) {
		return raw
	}
	st := &responsesWalkState{info: info, eventType: parentEvent}
	walkResponsesValue(root, st)
	if !st.mutated {
		return raw
	}
	out, err := common.Marshal(root)
	if err != nil {
		return raw
	}
	return out
}

type responsesWalkState struct {
	info      *relaycommon.RelayInfo
	eventType string
	fcIdx     int
	mutated   bool
}

func walkResponsesValue(v any, st *responsesWalkState) {
	switch node := v.(type) {
	case map[string]any:
		walkResponsesObject(node, st)
	case []any:
		for _, item := range node {
			walkResponsesValue(item, st)
		}
	}
}

func walkResponsesObject(node map[string]any, st *responsesWalkState) {
	if typ, _ := node["type"].(string); typ != "" && strings.HasPrefix(typ, "response.") {
		st.eventType = typ
	}
	if obj, _ := node["object"].(string); obj == "response" {
		fillResponseEnvelope(node, st)
	}
	if typ, _ := node["type"].(string); typ == "function_call" {
		fillFunctionCall(node, st)
	}
	savedEvent := st.eventType
	for _, child := range node {
		walkResponsesValue(child, st)
		st.eventType = savedEvent
	}
}

func fillResponseEnvelope(node map[string]any, st *responsesWalkState) {
	ts := egressTimestamp(st.info)
	_, hasCreated := node["created"]
	createdNil := !hasCreated || node["created"] == nil
	_, hasCreatedAt := node["created_at"]
	createdAtNil := !hasCreatedAt || node["created_at"] == nil

	if createdNil {
		if !createdAtNil {
			node["created"] = node["created_at"]
		} else {
			node["created"] = ts
		}
		st.mutated = true
		if !st.info.EgressFilledCreated {
			st.info.EgressFilledCreated = true
			logger.LogWarn(context.Background(), fmt.Sprintf("request_id=%s grok-strict filled created", st.info.RequestId))
		}
	}
	if createdAtNil {
		if v, ok := node["created"]; ok && v != nil {
			node["created_at"] = v
		} else {
			node["created_at"] = ts
		}
		st.mutated = true
	}
}

func fillFunctionCall(node map[string]any, st *responsesWalkState) {
	idx := st.fcIdx
	st.fcIdx++
	reqID := strings.TrimSpace(st.info.RequestId)
	if reqID == "" {
		reqID = "unknown"
	}

	id, _ := node["id"].(string)
	if strings.TrimSpace(id) == "" {
		id = fmt.Sprintf("fc_%s_%d", reqID, idx)
		node["id"] = id
		st.mutated = true
	}

	callID, _ := node["call_id"].(string)
	if strings.TrimSpace(callID) == "" {
		if strings.HasPrefix(id, "fc_") {
			node["call_id"] = "call_" + strings.TrimPrefix(id, "fc_")
		} else {
			node["call_id"] = fmt.Sprintf("call_%s_%d", reqID, idx)
		}
		st.mutated = true
	}

	name, _ := node["name"].(string)
	if strings.TrimSpace(name) == "" {
		node["name"] = "unknown"
		st.mutated = true
		logger.LogWarn(context.Background(), fmt.Sprintf("request_id=%s grok-strict filled empty function_call name", st.info.RequestId))
	}

	status, _ := node["status"].(string)
	if strings.TrimSpace(status) == "" {
		if st.eventType == "response.output_item.added" {
			node["status"] = "in_progress"
		} else {
			node["status"] = "completed"
		}
		st.mutated = true
	}

	if input, ok := node["input"]; ok {
		encoded, err := common.Marshal(input)
		if err == nil {
			node["arguments"] = string(encoded)
		} else {
			node["arguments"] = "{}"
		}
		delete(node, "input")
		st.mutated = true
		markFilledArgs(st.info)
	}

	args, hasArgs := node["arguments"]
	if !hasArgs || args == nil {
		node["arguments"] = "{}"
		st.mutated = true
		markFilledArgs(st.info)
		return
	}
	switch typed := args.(type) {
	case string:
		if typed == "" {
			node["arguments"] = "{}"
			st.mutated = true
			markFilledArgs(st.info)
		}
	case map[string]any, []any:
		encoded, err := common.Marshal(typed)
		if err != nil {
			node["arguments"] = "{}"
		} else {
			node["arguments"] = string(encoded)
		}
		st.mutated = true
		markFilledArgs(st.info)
	default:
		encoded, err := common.Marshal(typed)
		if err != nil {
			node["arguments"] = "{}"
		} else {
			node["arguments"] = string(encoded)
		}
		st.mutated = true
		markFilledArgs(st.info)
	}
}

func observeMissedUA(info *relaycommon.RelayInfo, root map[string]any, parentEvent string) {
	if info == nil || info.ClientProfile != common.ClientProfileGeneric || info.EgressMissedUAWarned {
		return
	}
	if responsesLooksIncomplete(root, parentEvent) {
		info.EgressMissedUAWarned = true
		logger.LogWarn(context.Background(), fmt.Sprintf("request_id=%s suspected missed grok ua: incomplete responses function_call/created", info.RequestId))
	}
}

func responsesLooksIncomplete(node map[string]any, parentEvent string) bool {
	if looksIncompleteEnvelope(node) || looksIncompleteFunctionCall(node) {
		return true
	}
	for _, child := range node {
		switch typed := child.(type) {
		case map[string]any:
			if responsesLooksIncomplete(typed, parentEvent) {
				return true
			}
		case []any:
			for _, item := range typed {
				if childMap, ok := item.(map[string]any); ok && responsesLooksIncomplete(childMap, parentEvent) {
					return true
				}
			}
		}
	}
	return false
}

func looksIncompleteEnvelope(node map[string]any) bool {
	obj, _ := node["object"].(string)
	if obj != "response" {
		return false
	}
	_, hasCreated := node["created"]
	_, hasCreatedAt := node["created_at"]
	createdNil := !hasCreated || node["created"] == nil
	createdAtNil := !hasCreatedAt || node["created_at"] == nil
	return createdNil && createdAtNil
}

func looksIncompleteFunctionCall(node map[string]any) bool {
	typ, _ := node["type"].(string)
	if typ != "function_call" {
		return false
	}
	args, ok := node["arguments"]
	return !ok || args == nil
}

func egressTimestamp(info *relaycommon.RelayInfo) int64 {
	if info.EgressCreatedAt != 0 {
		return info.EgressCreatedAt
	}
	ts := info.StartTime.Unix()
	if ts == 0 {
		ts = time.Now().Unix()
	}
	info.EgressCreatedAt = ts
	return ts
}

func markFilledArgs(info *relaycommon.RelayInfo) {
	if info.EgressFilledArgs {
		return
	}
	info.EgressFilledArgs = true
	logger.LogWarn(context.Background(), fmt.Sprintf("request_id=%s grok-strict filled arguments", info.RequestId))
}

func eventTypeOfFrame(frame []byte) string {
	for _, line := range bytes.Split(frame, []byte("\n")) {
		trim := bytes.TrimSpace(line)
		if bytes.HasPrefix(trim, []byte("event:")) {
			return string(bytes.TrimSpace(trim[6:]))
		}
	}
	return ""
}

func replaceSSEData(frame, newData []byte) []byte {
	var b bytes.Buffer
	lines := bytes.Split(frame, []byte("\n"))
	replaced := false
	for i, line := range lines {
		trim := bytes.TrimSpace(line)
		if !replaced && bytes.HasPrefix(trim, []byte("data:")) {
			b.WriteString("data: ")
			b.Write(newData)
			replaced = true
		} else {
			b.Write(line)
		}
		if i != len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.Bytes()
}
