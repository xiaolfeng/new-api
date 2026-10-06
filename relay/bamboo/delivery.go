package bamboo

import (
	"bytes"
	"encoding/json"
	"io"
	"time"

	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	bamboorelay "github.com/bamboo-services/bamboo-messages/bamboo/relay"
	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/clientprofile"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// deliveryWriter 在规范化之后的实际写出边界计时，与所有心跳共用请求级写锁。
type deliveryWriter struct {
	c      *gin.Context
	info   *relaycommon.RelayInfo
	format bamboocodec.FormatType
	now    func() time.Time
}

func newDeliveryWriter(c *gin.Context, info *relaycommon.RelayInfo, format bamboocodec.FormatType) *deliveryWriter {
	if info.DeliveryTiming == nil {
		info.DeliveryTiming = relaycommon.NewDeliveryTiming(info.StartTime)
	}
	return &deliveryWriter{c: c, info: info, format: format, now: time.Now}
}

func (w *deliveryWriter) headers() {
	t := w.info.DeliveryTiming
	t.WriteMu.Lock()
	defer t.WriteMu.Unlock()
	if w.c != nil && w.c.Writer != nil && !t.Finished() {
		writeStreamHeaders(w.c)
		t.MarkCommitted()
	}
}

func (w *deliveryWriter) write(data []byte, stream bool) bool {
	t := w.info.DeliveryTiming
	t.WriteMu.Lock()
	defer t.WriteMu.Unlock()
	if t.Finished() {
		return false
	}
	if w.c == nil || w.c.Writer == nil {
		t.Fail("write_error")
		return false
	}
	if stream {
		data = clientprofile.NormalizeClaudeSSEFrame(w.info, data)
		data = clientprofile.NormalizeResponsesSSEFrame(w.info, data)
	} else {
		data = clientprofile.NormalizeResponsesPayload(w.info, data)
		w.c.Writer.Header().Set("Content-Type", "application/json")
	}
	if len(data) == 0 {
		return true
	}
	n, err := w.c.Writer.Write(data)
	if n > 0 || w.c.Writer.Written() {
		t.MarkCommitted()
	}
	if err != nil || n != len(data) {
		t.Fail("write_error")
		return false
	}
	if stream {
		w.c.Writer.Flush()
	}
	at := w.now()
	if deliveryHasContent(data, w.format, stream) && t.RecordContent(at) {
		// 保留旧消费者字段，但不再以空内容块或上游到达时间提前打点。
		w.info.FirstResponseTime = at
	}
	return true
}

func (w *deliveryWriter) writeSSE(data []byte) bool { return w.write(data, true) }

func (w *deliveryWriter) finish(status string) {
	t := w.info.DeliveryTiming
	t.WriteMu.Lock()
	defer t.WriteMu.Unlock()
	if w.c != nil && w.c.Request != nil && w.c.Request.Context().Err() != nil {
		status = "cancelled"
	} else if ss := w.info.StreamStatus; ss != nil {
		switch ss.EndReason {
		case relaycommon.StreamEndReasonClientGone:
			status = "cancelled"
		case relaycommon.StreamEndReasonScannerErr:
			status = "upstream_error"
		}
	}
	t.Finish(w.now(), status)
}

func writeDeliveryJSON(c *gin.Context, info *relaycommon.RelayInfo, format bamboocodec.FormatType, body []byte) error {
	w := newDeliveryWriter(c, info, format)
	defer w.finish("completed")
	if !w.write(body, false) {
		return io.ErrShortWrite
	}
	return nil
}

// deliveryPart 是各出口协议内容的只读投影，签名、用量及信封字段不参与判定。
type deliveryPart struct {
	Type         string          `json:"type"`
	Text         string          `json:"text"`
	Thinking     string          `json:"thinking"`
	Name         string          `json:"name"`
	PartialJSON  string          `json:"partial_json"`
	Content      json.RawMessage `json:"content"`
	Output       json.RawMessage `json:"output"`
	Summary      []deliveryPart  `json:"summary"`
	FunctionCall *struct {
		Name string `json:"name"`
	} `json:"functionCall"`
	FunctionResponse *struct {
		Name string `json:"name"`
	} `json:"functionResponse"`
}

type deliveryTool struct {
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
		Output    string `json:"output"`
	} `json:"function"`
}

type deliveryMessage struct {
	Content          json.RawMessage `json:"content"`
	Reasoning        string          `json:"reasoning"`
	ReasoningContent string          `json:"reasoning_content"`
	ToolCalls        []deliveryTool  `json:"tool_calls"`
	FunctionCall     *struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function_call"`
}

func (m deliveryMessage) hasContent() bool {
	if deliveryRawContent(m.Content, 0) || m.Reasoning != "" || m.ReasoningContent != "" {
		return true
	}
	for _, tool := range m.ToolCalls {
		if tool.Function.Name != "" || tool.Function.Arguments != "" || tool.Function.Output != "" {
			return true
		}
	}
	return m.FunctionCall != nil && (m.FunctionCall.Name != "" || m.FunctionCall.Arguments != "")
}

func deliveryRawContent(raw json.RawMessage, depth int) bool {
	if len(raw) == 0 || depth > 16 {
		return false
	}
	var text string
	if common.Unmarshal(raw, &text) == nil {
		return text != ""
	}
	var parts []deliveryPart
	if common.Unmarshal(raw, &parts) != nil {
		return false
	}
	for _, part := range parts {
		if part.hasContent(depth + 1) {
			return true
		}
	}
	return false
}

func (p deliveryPart) hasContent(depth int) bool {
	if depth > 16 {
		return false
	}
	if p.FunctionCall != nil && p.FunctionCall.Name != "" {
		return true
	}
	if p.FunctionResponse != nil && p.FunctionResponse.Name != "" {
		return true
	}
	switch p.Type {
	case "", "text", "output_text", "reasoning_text", "summary_text", "text_delta", "thinking", "thinking_delta":
		return p.Text != "" || p.Thinking != ""
	case "tool_use", "server_tool_use", "function_call", "web_search_call":
		return p.Name != "" || p.Type == "web_search_call"
	case "input_json_delta":
		return p.PartialJSON != ""
	case "tool_result", "web_search_tool_result", "web_fetch_tool_result":
		return deliveryRawContent(p.Content, depth+1)
	case "function_call_output":
		return deliveryRawContent(p.Output, depth+1)
	case "reasoning", "message":
		if deliveryRawContent(p.Content, depth+1) {
			return true
		}
		for _, item := range p.Summary {
			if item.hasContent(depth + 1) {
				return true
			}
		}
	}
	return false
}

func deliveryHasContent(raw []byte, format bamboocodec.FormatType, stream bool) bool {
	if !stream {
		return deliveryPayloadHasContent(raw, format, false)
	}
	for _, frame := range bamboorelay.SplitSSEFrames(raw) {
		var data [][]byte
		for _, line := range bytes.Split(frame, []byte("\n")) {
			line = bytes.TrimSuffix(line, []byte("\r"))
			if value, ok := bytes.CutPrefix(line, []byte("data:")); ok {
				data = append(data, bytes.TrimPrefix(value, []byte(" ")))
			}
		}
		if deliveryPayloadHasContent(bytes.Join(data, []byte("\n")), format, true) {
			return true
		}
	}
	return false
}

func deliveryPayloadHasContent(raw []byte, format bamboocodec.FormatType, stream bool) bool {
	var event struct {
		Type         string          `json:"type"`
		Delta        json.RawMessage `json:"delta"`
		ContentBlock deliveryPart    `json:"content_block"`
		Part         deliveryPart    `json:"part"`
		Item         deliveryPart    `json:"item"`
		Content      json.RawMessage `json:"content"`
		Output       []deliveryPart  `json:"output"`
		Response     *struct {
			Output []deliveryPart `json:"output"`
		} `json:"response"`
		Choices []struct {
			Delta   deliveryMessage `json:"delta"`
			Message deliveryMessage `json:"message"`
		} `json:"choices"`
		Candidates []struct {
			Content struct {
				Parts []deliveryPart `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if common.Unmarshal(raw, &event) != nil {
		return false
	}
	switch format {
	case bamboocodec.FormatAnthropic, bamboocodec.FormatBamboo:
		if !stream {
			return deliveryRawContent(event.Content, 0)
		}
		switch event.Type {
		case "content_block_start":
			return event.ContentBlock.hasContent(0)
		case "content_block_delta":
			var part deliveryPart
			return common.Unmarshal(event.Delta, &part) == nil && part.hasContent(0)
		}
	case bamboocodec.FormatOpenAI:
		for _, choice := range event.Choices {
			if stream && choice.Delta.hasContent() || !stream && choice.Message.hasContent() {
				return true
			}
		}
	case bamboocodec.FormatResponses:
		if !stream {
			for _, part := range event.Output {
				if part.hasContent(0) {
					return true
				}
			}
			return false
		}
		switch event.Type {
		case "response.output_text.delta", "response.reasoning_text.delta", "response.reasoning_summary_text.delta", "response.function_call_arguments.delta":
			var text string
			return common.Unmarshal(event.Delta, &text) == nil && text != ""
		case "response.output_item.added", "response.output_item.done":
			return event.Item.hasContent(0)
		case "response.content_part.added", "response.content_part.done", "response.reasoning_summary_part.added", "response.reasoning_summary_part.done":
			return event.Part.hasContent(0)
		case "response.completed":
			if event.Response != nil {
				for _, part := range event.Response.Output {
					if part.hasContent(0) {
						return true
					}
				}
			}
		}
	case bamboocodec.FormatGemini:
		for _, candidate := range event.Candidates {
			for _, part := range candidate.Content.Parts {
				if part.hasContent(0) {
					return true
				}
			}
		}
	}
	return false
}
