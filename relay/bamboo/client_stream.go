package bamboo

import (
	"time"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	bamboorelay "github.com/bamboo-services/bamboo-messages/bamboo/relay"
	pkgErrors "github.com/bamboo-services/bamboo-messages/pkg/errors"
	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/clientprofile"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

const clientStreamDeltaRunes = 512

// clientStream 是一次 Bamboo 流式响应的唯一出口。
// 识图前缀、host 围栏、主 hop / hop2 必须共用同一把 serializer，
// 只写一次 message_start，结束时只 Flush 一次。
type clientStream struct {
	c         *gin.Context
	info      *relaycommon.RelayInfo
	ser       bamboocodec.StreamSerializer
	writeSSE  func([]byte) bool
	onStart   func()
	startPing func() func()
	stopPing  func()
	headers   bool
	started   bool
	finished  bool
	ok        bool
	prefixN   int
	openIdx    int
	hasOpen    bool
	sawDelta   bool
	sawToolUse bool
	frames     []string
	format    bamboocodec.FormatType
	model     string
	lastUsage *bamboosdk.Usage
}

func newClientStream(c *gin.Context, entryCodec bamboocodec.Codec, info *relaycommon.RelayInfo, model string) *clientStream {
	if entryCodec == nil {
		return nil
	}
	cs := &clientStream{
		c:      c,
		info:   info,
		ser:    entryCodec.NewSerializer(model),
		ok:     true,
		format: entryCodec.Format(),
		model:  model,
	}
	cs.writeSSE = func(data []byte) bool {
		if c == nil || c.Writer == nil {
			return false
		}
		data = clientprofile.NormalizeClaudeSSEFrame(info, data)
		if len(data) == 0 {
			return true
		}
		data = clientprofile.NormalizeResponsesSSEFrame(info, data)
		if len(data) == 0 {
			return true
		}
		if _, err := c.Writer.Write(data); err != nil {
			return false
		}
		c.Writer.Flush()
		return true
	}
	return cs
}

func (s *clientStream) HasHeaders() bool {
	return s != nil && s.headers
}

func (s *clientStream) Started() bool {
	return s != nil && s.started
}

func (s *clientStream) PrefixN() int {
	if s == nil {
		return 0
	}
	return s.prefixN
}

func (s *clientStream) Frames() []string {
	if s == nil {
		return nil
	}
	return s.frames
}

func (s *clientStream) ensureHeaders() {
	if s == nil || s.headers {
		return
	}
	if s.c != nil {
		writeStreamHeaders(s.c)
	}
	s.headers = true
	s.resumePing()
}

func (s *clientStream) resumePing() {
	if s == nil || s.startPing == nil || s.stopPing != nil {
		return
	}
	s.stopPing = s.startPing()
}

func (s *clientStream) pausePing() {
	if s == nil || s.stopPing == nil {
		return
	}
	s.stopPing()
	s.stopPing = nil
}

func (s *clientStream) ensureStart() {
	if s == nil {
		return
	}
	s.ensureHeaders()
	if s.started {
		return
	}
	s.started = true
	s.pausePing()
	if s.onStart != nil {
		s.onStart()
	}
	if s.info != nil {
		s.info.SetFirstResponseTime()
	}
	s.emit(bamboosdk.StreamEvent{
		Type:    bamboosdk.EventMessageStart,
		Message: &bamboosdk.BambooMessage{Role: bamboosdk.RoleAssistant},
	})
}

func (s *clientStream) rememberUsage(ev bamboosdk.StreamEvent) {
	if s == nil || ev.Usage == nil {
		return
	}
	u := ev.Usage
	if u.InputTokens == 0 && u.OutputTokens == 0 &&
		u.CacheReadInputTokens == 0 && u.CacheCreationInputTokens == 0 &&
		u.ReasoningTokens == 0 {
		return
	}
	cp := *u
	s.lastUsage = &cp
}

func (s *clientStream) emit(ev bamboosdk.StreamEvent) bool {
	if s == nil || !s.ok || s.ser == nil {
		return false
	}
	s.rememberUsage(ev)
	data, err := s.ser.Serialize(ev)
	if err != nil || data == nil {
		return s.ok
	}
	for _, frame := range bamboorelay.SplitSSEFrames(data) {
		s.frames = append(s.frames, string(frame))
		if !s.writeSSE(frame) {
			s.ok = false
			return false
		}
	}
	return true
}

func (s *clientStream) openText() int {
	s.ensureStart()
	idx := s.prefixN
	s.emit(bamboosdk.StreamEvent{
		Type:         bamboosdk.EventContentBlockStart,
		Index:        idx,
		ContentBlock: bamboosdk.NewTextBlock(""),
	})
	s.hasOpen = true
	s.openIdx = idx
	return idx
}

func (s *clientStream) openThinking() int {
	s.ensureStart()
	idx := s.prefixN
	s.emit(bamboosdk.StreamEvent{
		Type:         bamboosdk.EventContentBlockStart,
		Index:        idx,
		ContentBlock: bamboosdk.NewThinkingBlock("", ""),
	})
	s.hasOpen = true
	s.openIdx = idx
	return idx
}

func (s *clientStream) textDelta(idx int, text string) {
	if text == "" {
		return
	}
	for _, part := range splitClientRunes(text, clientStreamDeltaRunes) {
		s.emit(bamboosdk.StreamEvent{
			Type:  bamboosdk.EventContentBlockDelta,
			Index: idx,
			Delta: &bamboosdk.StreamDelta{Type: bamboosdk.DeltaTextDelta, Text: part},
		})
	}
}

func (s *clientStream) thinkingDelta(idx int, text string) {
	if text == "" {
		return
	}
	for _, part := range splitClientRunes(text, clientStreamDeltaRunes) {
		s.emit(bamboosdk.StreamEvent{
			Type:  bamboosdk.EventContentBlockDelta,
			Index: idx,
			Delta: &bamboosdk.StreamDelta{Type: bamboosdk.DeltaThinkingDelta, Thinking: part},
		})
	}
}

func (s *clientStream) closeBlock(idx int) {
	s.emit(bamboosdk.StreamEvent{Type: bamboosdk.EventContentBlockStop, Index: idx})
	if idx >= s.prefixN {
		s.prefixN = idx + 1
	}
	if s.hasOpen && s.openIdx == idx {
		s.hasOpen = false
	}
}

func (s *clientStream) emitToolUse(id, name, input string) {
	if s == nil {
		return
	}
	s.forward(bamboosdk.StreamEvent{
		Type:         bamboosdk.EventContentBlockStart,
		Index:        0,
		ContentBlock: bamboosdk.NewToolUseBlockWithRawInput(id, name, ""),
	})
	if input != "" {
		s.forward(bamboosdk.StreamEvent{
			Type:  bamboosdk.EventContentBlockDelta,
			Index: 0,
			Delta: &bamboosdk.StreamDelta{Type: bamboosdk.DeltaInputJSON, PartialJSON: input},
		})
	}
	s.forward(bamboosdk.StreamEvent{Type: bamboosdk.EventContentBlockStop, Index: 0})
}

func (s *clientStream) emitOpenAIToolOutput(index int, id, output string) {
	if s == nil || !s.ok || s.format != bamboocodec.FormatOpenAI {
		return
	}
	s.ensureStart()
	model := s.model
	if model == "" && s.info != nil {
		model = s.info.OriginModelName
	}
	payload := map[string]any{
		"id":      "chatcmpl-host-tool",
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []any{
			map[string]any{
				"index": 0,
				"delta": map[string]any{
					"tool_calls": []any{
						map[string]any{
							"index": index,
							"id":    id,
							"type":  "function",
							"function": map[string]any{
								"output": output,
							},
						},
					},
				},
			},
		},
	}
	data, err := common.Marshal(payload)
	if err != nil || len(data) == 0 {
		return
	}
	frame := append([]byte("data: "), data...)
	frame = append(frame, '\n', '\n')
	s.frames = append(s.frames, string(frame))
	if !s.writeSSE(frame) {
		s.ok = false
	}
}

func (s *clientStream) emitClosedText(text string) {
	if s == nil || text == "" {
		return
	}
	idx := s.openText()
	s.textDelta(idx, text)
	s.closeBlock(idx)
}

func (s *clientStream) emitClosedThinking(text string) {
	if s == nil || text == "" {
		return
	}
	idx := s.openThinking()
	s.thinkingDelta(idx, text)
	s.closeBlock(idx)
}

func (s *clientStream) Begin() bool {
	if s == nil {
		return false
	}
	s.openThinking()
	s.thinkingDelta(s.openIdx, relaycommon.ImageRecognizeFenceStart+"\n")
	return s.ok
}

func (s *clientStream) OnDelta(text string) {
	if s == nil || !s.hasOpen || text == "" {
		return
	}
	s.thinkingDelta(s.openIdx, text)
}

func (s *clientStream) End() {
	if s == nil || !s.hasOpen {
		return
	}
	s.thinkingDelta(s.openIdx, "\n"+relaycommon.ImageRecognizeFenceEnd+"\n")
	s.closeBlock(s.openIdx)
}

func (s *clientStream) forward(ev bamboosdk.StreamEvent) bool {
	if s == nil {
		return false
	}
	switch ev.Type {
	case bamboosdk.EventMessageStart:
		s.ensureHeaders()
		if s.started {
			return s.ok
		}
		s.started = true
		s.pausePing()
		if s.onStart != nil {
			s.onStart()
		}
		if s.info != nil {
			s.info.SetFirstResponseTime()
		}
		return s.emit(ev)
	case bamboosdk.EventMessageStop:
		return s.ok
	case bamboosdk.EventContentBlockStart:
		s.ensureStart()
		if ev.ContentBlock != nil && ev.ContentBlock.BlockType() == bamboosdk.ContentBlockToolUse {
			s.sawToolUse = true
		}
		ev.Index += s.prefixN
		return s.emit(ev)
	case bamboosdk.EventContentBlockDelta, bamboosdk.EventContentBlockStop:
		s.ensureStart()
		ev.Index += s.prefixN
		return s.emit(ev)
	case bamboosdk.EventMessageDelta:
		s.ensureStart()
		s.sawDelta = true
		return s.emit(ev)
	default:
		s.ensureStart()
		return s.emit(ev)
	}
}

func (s *clientStream) finish() {
	if s == nil || s.finished {
		return
	}
	s.finished = true
	if s.started && !s.sawDelta {
		stopReason := bamboosdk.FinishReasonEndTurn
		if s.sawToolUse {
			stopReason = bamboosdk.FinishReasonToolUse
		}
		s.emit(bamboosdk.StreamEvent{
			Type:  bamboosdk.EventMessageDelta,
			Delta: &bamboosdk.MessageDelta{StopReason: stopReason},
			Usage: s.lastUsage,
		})
	}
	if s.started {
		s.emit(bamboosdk.StreamEvent{Type: bamboosdk.EventMessageStop})
	}
	if s.ser == nil {
		return
	}
	tail, _ := s.ser.Flush()
	if len(tail) == 0 {
		return
	}
	for _, frame := range bamboorelay.SplitSSEFrames(tail) {
		s.frames = append(s.frames, string(frame))
		if !s.writeSSE(frame) {
			s.ok = false
			return
		}
	}
}

// emitHeld 补发 tee 模式下被 commit point 遮住的内容块事件。
// 只补发 content 块（start/delta/stop），message 级终止语义仍由 finish 统一补发；
// 走 forward 保证索引与已透传块连续衔接。
func (s *clientStream) emitHeld(held []bamboosdk.StreamEvent) {
	if s == nil {
		return
	}
	for _, ev := range held {
		switch ev.Type {
		case bamboosdk.EventContentBlockStart, bamboosdk.EventContentBlockDelta, bamboosdk.EventContentBlockStop, bamboosdk.EventMessageDelta:
			if !s.forward(ev) {
				return
			}
		}
	}
}

func (s *clientStream) emitError(err error) {
	if s == nil {
		return
	}
	s.ensureHeaders()
	msg := "stream error"
	if err != nil {
		msg = err.Error()
	}
	s.emit(bamboosdk.StreamEvent{
		Type: bamboosdk.EventError,
		Error: &pkgErrors.BambooError{
			Category:   "api",
			Message:    msg,
			StatusCode: 500,
		},
	})
	s.finish()
}

func splitClientRunes(s string, size int) []string {
	if s == "" {
		return nil
	}
	if size <= 0 {
		size = clientStreamDeltaRunes
	}
	runes := []rune(s)
	if len(runes) <= size {
		return []string{s}
	}
	out := make([]string, 0, (len(runes)+size-1)/size)
	for i := 0; i < len(runes); i += size {
		end := i + size
		if end > len(runes) {
			end = len(runes)
		}
		out = append(out, string(runes[i:end]))
	}
	return out
}
