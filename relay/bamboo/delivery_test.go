package bamboo

import (
	"context"
	"errors"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/bamboo-services/bamboo-messages/provider"
)

func TestDeliveryContentAcrossProtocols(t *testing.T) {
	for _, tc := range []struct {
		name   string
		format bamboocodec.FormatType
		stream bool
		body   string
		want   bool
	}{
		{"text", bamboocodec.FormatAnthropic, true, `{"type":"content_block_delta","delta":{"type":"text_delta","text":"hello"}}`, true},
		{"thinking", bamboocodec.FormatAnthropic, true, `{"type":"content_block_delta","delta":{"type":"thinking_delta","thinking":"think"}}`, true},
		{"empty_block", bamboocodec.FormatAnthropic, true, `{"type":"content_block_start","content_block":{"type":"text","text":""}}`, false},
		{"tool_declaration", bamboocodec.FormatAnthropic, true, `{"type":"content_block_start","content_block":{"type":"tool_use","name":"search","input":{}}}`, true},
		{"tool_id_only", bamboocodec.FormatAnthropic, true, `{"type":"content_block_start","content_block":{"type":"tool_use","id":"tool_1"}}`, false},
		{"signature", bamboocodec.FormatAnthropic, true, `{"type":"content_block_delta","delta":{"type":"signature_delta","signature":"opaque"}}`, false},
		{"ping", bamboocodec.FormatAnthropic, true, `{"type":"ping"}`, false},
		{"usage", bamboocodec.FormatOpenAI, true, `{"choices":[],"usage":{"completion_tokens":20}}`, false},
		{"role", bamboocodec.FormatOpenAI, true, `{"choices":[{"delta":{"role":"assistant"}}]}`, false},
		{"openai_text", bamboocodec.FormatOpenAI, true, `{"choices":[{"delta":{"content":"answer"}}]}`, true},
		{"openai_thinking", bamboocodec.FormatOpenAI, true, `{"choices":[{"delta":{"reasoning_content":"think"}}]}`, true},
		{"openai_tool", bamboocodec.FormatOpenAI, true, `{"choices":[{"delta":{"tool_calls":[{"function":{"name":"search"}}]}}]}`, true},
		{"openai_args", bamboocodec.FormatOpenAI, true, `{"choices":[{"delta":{"tool_calls":[{"function":{"arguments":"{}"}}]}}]}`, true},
		{"openai_tool_result", bamboocodec.FormatOpenAI, true, `{"choices":[{"delta":{"tool_calls":[{"function":{"output":"result"}}]}}]}`, true},
		{"responses_envelope", bamboocodec.FormatResponses, true, `{"type":"response.created","response":{"output":[]}}`, false},
		{"responses_text", bamboocodec.FormatResponses, true, `{"type":"response.output_text.delta","delta":"answer"}`, true},
		{"responses_reasoning", bamboocodec.FormatResponses, true, `{"type":"response.reasoning_summary_text.delta","delta":"think"}`, true},
		{"responses_tool", bamboocodec.FormatResponses, true, `{"type":"response.output_item.added","item":{"type":"function_call","name":"search"}}`, true},
		{"responses_args", bamboocodec.FormatResponses, true, `{"type":"response.function_call_arguments.delta","delta":"{}"}`, true},
		{"responses_encrypted", bamboocodec.FormatResponses, true, `{"type":"response.output_item.done","item":{"type":"reasoning","encrypted_content":"opaque"}}`, false},
		{"gemini_tool", bamboocodec.FormatGemini, true, `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"search","args":{}}}]}}]}`, true},
		{"gemini_thought", bamboocodec.FormatGemini, true, `{"candidates":[{"content":{"parts":[{"text":"think","thought":true}]}}]}`, true},
		{"gemini_usage", bamboocodec.FormatGemini, true, `{"usageMetadata":{"candidatesTokenCount":2}}`, false},
		{"complete_openai", bamboocodec.FormatOpenAI, false, `{"choices":[{"message":{"content":"answer"}}]}`, true},
		{"complete_anthropic", bamboocodec.FormatAnthropic, false, `{"content":[{"type":"tool_use","name":"search"}]}`, true},
		{"complete_responses", bamboocodec.FormatResponses, false, `{"output":[{"type":"message","content":[{"type":"output_text","text":"answer"}]}]}`, true},
		{"complete_bamboo", bamboocodec.FormatBamboo, false, `{"content":[{"type":"text","text":"answer"}]}`, true},
		{"empty_complete", bamboocodec.FormatOpenAI, false, `{"choices":[{"message":{"content":""}}],"usage":{"completion_tokens":2}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(tc.body)
			if tc.stream {
				body = []byte("data: " + tc.body + "\n\n")
			}
			assert.Equal(t, tc.want, deliveryHasContent(body, tc.format, tc.stream))
		})
	}
}

type deliveryTestResponseWriter struct {
	gin.ResponseWriter
	write func([]byte) (int, error)
	flush func()
}

func (w *deliveryTestResponseWriter) Write(p []byte) (int, error) { return w.write(p) }
func (w *deliveryTestResponseWriter) Flush() {
	if w.flush != nil {
		w.flush()
	}
}

func TestDeliveryWriteBoundaryAndFailure(t *testing.T) {
	base := time.Unix(100, 0)
	for _, tc := range []struct {
		name  string
		short bool
		fail  bool
	}{
		{"success", false, false}, {"short", true, false}, {"error", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			clock := base.Add(time.Second)
			flushed := false
			c.Writer = &deliveryTestResponseWriter{ResponseWriter: c.Writer, write: func(p []byte) (int, error) {
				clock = base.Add(7079 * time.Millisecond)
				if tc.fail {
					return 0, errors.New("closed")
				}
				if tc.short {
					return len(p) - 1, nil
				}
				return len(p), nil
			}, flush: func() { flushed = true; clock = base.Add(7109 * time.Millisecond) }}
			info := &relaycommon.RelayInfo{StartTime: base}
			w := newDeliveryWriter(c, info, bamboocodec.FormatOpenAI)
			w.now = func() time.Time { return clock }
			ok := w.writeSSE([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"answer\"}}]}\n\n"))
			assert.Equal(t, !tc.short && !tc.fail, ok)
			clock = base.Add(7200 * time.Millisecond)
			w.finish("completed")
			r, valid := info.DeliveryTiming.Result()
			require.True(t, valid)
			require.NotNil(t, r)
			if tc.short || tc.fail {
				assert.Nil(t, r.TTFTMs)
				assert.Equal(t, "write_error", r.Status)
				assert.False(t, flushed)
			} else {
				require.NotNil(t, r.TTFTMs)
				assert.Equal(t, int64(7109), *r.TTFTMs)
				assert.Equal(t, int64(7200), r.TotalMs)
				assert.Equal(t, base.Add(7109*time.Millisecond), info.FirstResponseTime)
			}
		})
	}
}

type deliveryBufferSerializer struct{ frame []byte }

func (s *deliveryBufferSerializer) Serialize(bamboosdk.StreamEvent) ([]byte, error) { return nil, nil }
func (s *deliveryBufferSerializer) Flush() ([]byte, error)                          { return s.frame, nil }

func TestDeliveryBufferedContentAndNoContent(t *testing.T) {
	base := time.Unix(100, 0)
	for _, content := range []bool{false, true} {
		t.Run(map[bool]string{false: "no_content", true: "buffered_tool"}[content], func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			codec, err := bamboocodec.Get(bamboocodec.FormatOpenAI)
			require.NoError(t, err)
			info := &relaycommon.RelayInfo{StartTime: base}
			cs := newClientStream(c, codec, info, "model")
			clock := base.Add(7 * time.Second)
			cs.writer.now = func() time.Time { return clock }
			ser := &deliveryBufferSerializer{}
			if content {
				ser.frame = []byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"function\":{\"name\":\"search\"}}]}}]}\n\n")
			}
			cs.ser = ser
			require.True(t, cs.forward(bamboosdk.StreamEvent{Type: bamboosdk.EventContentBlockDelta, Delta: &bamboosdk.StreamDelta{Type: bamboosdk.DeltaInputJSON, PartialJSON: "{}"}}))
			assert.True(t, info.FirstResponseTime.IsZero())
			clock = base.Add(8 * time.Second)
			cs.finish()
			r, valid := info.DeliveryTiming.Result()
			require.True(t, valid)
			require.NotNil(t, r)
			assert.Equal(t, int64(8000), r.TotalMs)
			if content {
				require.NotNil(t, r.TTFTMs)
				assert.Equal(t, int64(8000), *r.TTFTMs)
			} else {
				assert.Nil(t, r.TTFTMs)
			}
		})
	}
}

func TestDeliveryConcurrentPingAndContent(t *testing.T) {
	base := time.Unix(100, 0)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{StartTime: base}
	w := newDeliveryWriter(c, info, bamboocodec.FormatOpenAI)
	w.now = func() time.Time { return base.Add(time.Second) }
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); w.writeSSE([]byte("data: {\"choices\":[],\"usage\":{}}\n\n")) }()
	go func() {
		defer wg.Done()
		w.writeSSE([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"answer\"}}]}\n\n"))
	}()
	wg.Wait()
	w.now = func() time.Time { return base.Add(2 * time.Second) }
	w.finish("completed")
	r, valid := info.DeliveryTiming.Result()
	require.True(t, valid)
	require.NotNil(t, r.TTFTMs)
	assert.Equal(t, int64(1000), *r.TTFTMs)
	assert.Equal(t, int64(2000), r.TotalMs)
}

func TestDeliveryNormalizationDoesNotCountDroppedFrames(t *testing.T) {
	settings := model_setting.GetBambooSettings()
	old := settings.EnableClientStrictEgress
	enabled := true
	settings.EnableClientStrictEgress = &enabled
	t.Cleanup(func() { settings.EnableClientStrictEgress = old })
	base := time.Unix(100, 0)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{StartTime: base, ClientProfile: common.ClientProfileClaudeCode, RelayFormat: types.RelayFormatClaude}
	w := newDeliveryWriter(c, info, bamboocodec.FormatAnthropic)
	w.now = func() time.Time { return base.Add(time.Second) }
	require.True(t, w.writeSSE([]byte("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")))
	require.True(t, w.writeSSE([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{}\"}}\n\n")))
	assert.True(t, info.FirstResponseTime.IsZero())
	w.finish("completed")
	r, valid := info.DeliveryTiming.Result()
	require.True(t, valid)
	assert.Nil(t, r.TTFTMs)
}

func TestDeliveryHopsRetainRequestStartAndFirstWrittenContent(t *testing.T) {
	base := time.Unix(100, 0)
	c := newBridgeTestGinContext()
	codec, err := bamboocodec.Get(bamboocodec.FormatOpenAI)
	require.NoError(t, err)
	info := &relaycommon.RelayInfo{StartTime: base}
	cs := newClientStream(c, codec, info, "model")
	clock := base.Add(4 * time.Second)
	cs.writer.now = func() time.Time { return clock }
	hop := 0
	client := &mockBridgeClient{chatFn: func(context.Context) (<-chan bamboosdk.StreamEvent, error) {
		start := base.Add(time.Duration(hop) * 5 * time.Second)
		hop++
		ch := make(chan bamboosdk.StreamEvent, 3)
		ch <- bamboosdk.StreamEvent{Type: bamboosdk.EventMessageStart, ReceivedAt: start.Add(time.Second), Timing: &provider.TimingAnchor{RequestSentAt: start}}
		ch <- bamboosdk.StreamEvent{Type: bamboosdk.EventContentBlockDelta, ReceivedAt: start.Add(2 * time.Second), Delta: &bamboosdk.StreamDelta{Type: bamboosdk.DeltaTextDelta, Text: "answer"}}
		ch <- bamboosdk.StreamEvent{Type: bamboosdk.EventMessageStop, ReceivedAt: start.Add(3 * time.Second)}
		close(ch)
		return ch, nil
	}}
	req := &bamboocodec.RelayRequest{Config: &bamboosdk.RequestConfig{Model: "model"}}
	_, apiErr := collectStreamHop(context.Background(), c, info, client, req, false, cs)
	require.Nil(t, apiErr)
	first := info.FirstResponseTime
	clock = base.Add(9 * time.Second)
	_, apiErr = collectStreamHop(context.Background(), c, info, client, req, false, cs)
	require.Nil(t, apiErr)
	clock = base.Add(10 * time.Second)
	cs.finish()
	require.Len(t, info.BambooTimingHops, 2)
	assert.Equal(t, 3*time.Second, info.BambooTimingHops[0].Stats.TotalDuration)
	assert.Equal(t, 3*time.Second, info.BambooTimingHops[1].Stats.TotalDuration)
	assert.Equal(t, first, info.FirstResponseTime)
	r, valid := info.DeliveryTiming.Result()
	require.True(t, valid)
	require.NotNil(t, r)
	assert.Equal(t, int64(4000), *r.TTFTMs)
	assert.Equal(t, int64(10000), r.TotalMs)
}

func TestDeliveryFrameBatchAndMultilineData(t *testing.T) {
	batch := []byte("data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"answer\"}}]}\n\n")
	assert.True(t, deliveryHasContent(batch, bamboocodec.FormatOpenAI, true))
	multiline := []byte("data: {\"choices\":\r\ndata: [{\"delta\":{\"content\":\"answer\"}}]}\r\n\r\n")
	assert.True(t, deliveryHasContent(multiline, bamboocodec.FormatOpenAI, true))
}
