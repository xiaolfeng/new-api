package hosttool

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type responsesWebSearchSource struct {
	Type  string `json:"type"`
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
}

type responsesWebSearchAction struct {
	Type    string                     `json:"type"`
	Query   string                     `json:"query,omitempty"`
	URL     string                     `json:"url,omitempty"`
	Pattern string                     `json:"pattern,omitempty"`
	Sources []responsesWebSearchSource `json:"sources,omitempty"`
}

type responsesWebSearchCall struct {
	Type   string                   `json:"type"`
	ID     string                   `json:"id"`
	Status string                   `json:"status"`
	Action responsesWebSearchAction `json:"action"`
}

type responsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type responsesObject struct {
	ID        string         `json:"id"`
	Object    string         `json:"object"`
	CreatedAt int64          `json:"created_at"`
	Status    string         `json:"status"`
	Model     string         `json:"model"`
	Output    []any          `json:"output"`
	Usage     responsesUsage `json:"usage"`
}

func buildWebSearchCall(id string, result ExecResult) responsesWebSearchCall {
	call := responsesWebSearchCall{
		Type:   "web_search_call",
		ID:     id,
		Status: "failed",
		Action: responsesWebSearchAction{Type: "search"},
	}
	switch result.Kind {
	case "fetch":
		call.Action.Type = "open_page"
		call.Action.URL = result.URL
		call.Action.Pattern = result.Pattern
	default:
		call.Action.Type = "search"
		call.Action.Query = result.Query
	}
	if result.OK {
		call.Status = "completed"
		if result.Kind != "fetch" {
			call.Action.Sources = sourcesFromResult(result)
		}
		return call
	}
	return call
}

func sourcesFromResult(result ExecResult) []responsesWebSearchSource {
	if len(result.Hits) == 0 {
		return nil
	}
	out := make([]responsesWebSearchSource, 0, len(result.Hits))
	for _, hit := range result.Hits {
		if strings.TrimSpace(hit.URL) == "" {
			continue
		}
		out = append(out, responsesWebSearchSource{
			Type:  "url",
			URL:   hit.URL,
			Title: hit.Title,
		})
	}
	return out
}

func builtinResponseID(requestID string) string {
	if strings.TrimSpace(requestID) == "" {
		return "resp_host_web_search"
	}
	return "resp_" + requestID
}

func builtinCallID(requestID string) string {
	if strings.TrimSpace(requestID) == "" {
		return "ws_host_web_search"
	}
	return "ws_" + requestID
}

func MarshalBuiltinComplete(model, requestID string, createdAt int64, result ExecResult) ([]byte, error) {
	call := buildWebSearchCall(builtinCallID(requestID), result)
	body := responsesObject{
		ID:        builtinResponseID(requestID),
		Object:    "response",
		CreatedAt: createdAt,
		Status:    "completed",
		Model:     model,
		Output:    []any{call},
		Usage:     responsesUsage{},
	}
	if !result.OK {
		body.Status = "incomplete"
	}
	return common.Marshal(body)
}

func marshalBuiltinSSE(eventType string, seq int, payload map[string]any) ([]byte, error) {
	if payload == nil {
		payload = map[string]any{}
	}
	payload["type"] = eventType
	payload["sequence_number"] = seq
	data, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, data)), nil
}

func BuiltinStreamFrames(model, requestID string, createdAt int64, result ExecResult) ([][]byte, error) {
	respID := builtinResponseID(requestID)
	callID := builtinCallID(requestID)
	call := buildWebSearchCall(callID, result)
	pending := call
	pending.Status = "in_progress"
	pending.Action.Sources = nil

	finalStatus := "completed"
	if !result.OK {
		finalStatus = "incomplete"
	}
	base := responsesObject{
		ID:        respID,
		Object:    "response",
		CreatedAt: createdAt,
		Status:    "in_progress",
		Model:     model,
		Output:    []any{},
		Usage:     responsesUsage{},
	}
	seq := 0
	next := func(eventType string, payload map[string]any) ([]byte, error) {
		seq++
		payload["response_id"] = respID
		return marshalBuiltinSSE(eventType, seq, payload)
	}

	created := base
	var frames [][]byte
	frame, err := next("response.created", map[string]any{"response": created})
	if err != nil {
		return nil, err
	}
	frames = append(frames, frame)

	frame, err = next("response.in_progress", map[string]any{"response": base})
	if err != nil {
		return nil, err
	}
	frames = append(frames, frame)

	frame, err = next("response.output_item.added", map[string]any{
		"output_index": 0,
		"item":         pending,
	})
	if err != nil {
		return nil, err
	}
	frames = append(frames, frame)

	frame, err = next("response.output_item.done", map[string]any{
		"output_index": 0,
		"item":         call,
	})
	if err != nil {
		return nil, err
	}
	frames = append(frames, frame)

	done := base
	done.Status = finalStatus
	done.Output = []any{call}
	frame, err = next("response.completed", map[string]any{"response": done})
	if err != nil {
		return nil, err
	}
	frames = append(frames, frame)
	return frames, nil
}
