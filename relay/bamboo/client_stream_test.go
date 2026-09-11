package bamboo

import (
	"strings"
	"testing"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	_ "github.com/bamboo-services/bamboo-messages/bamboo/codec/anthropic"
	_ "github.com/bamboo-services/bamboo-messages/bamboo/codec/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func testClientStream(t *testing.T, format bamboocodec.FormatType) *clientStream {
	t.Helper()
	codec, err := bamboocodec.Get(format)
	require.NoError(t, err)
	cs := newClientStream(nil, codec, &relaycommon.RelayInfo{}, "test-model")
	require.NotNil(t, cs)
	cs.writeSSE = func([]byte) bool { return true }
	return cs
}

func TestClientStreamImageRecognizeUsesThinking(t *testing.T) {
	cs := testClientStream(t, bamboocodec.FormatOpenAI)
	require.True(t, cs.Begin())
	cs.OnDelta("[Image 1]\nA cat")
	cs.End()
	joined := strings.Join(cs.Frames(), "")
	assert.Contains(t, joined, "A cat")
	assert.Contains(t, joined, "reasoning_content")
	assert.NotContains(t, joined, `"content":"<<<image_recognition>>>`)
}

func TestClientStreamRemapsIndexAfterPrefix(t *testing.T) {
	cs := testClientStream(t, bamboocodec.FormatAnthropic)
	require.True(t, cs.Begin())
	cs.OnDelta("a cat")
	cs.End()
	require.Equal(t, 1, cs.PrefixN())

	ok := cs.forward(bamboosdk.StreamEvent{
		Type:         bamboosdk.EventContentBlockStart,
		Index:        0,
		ContentBlock: bamboosdk.NewTextBlock(""),
	})
	require.True(t, ok)
	joined := strings.Join(cs.Frames(), "")
	assert.Contains(t, joined, `"index":1`)
}

func TestClientStreamDropsSecondMessageStart(t *testing.T) {
	cs := testClientStream(t, bamboocodec.FormatOpenAI)
	cs.ensureStart()
	before := len(cs.Frames())
	require.True(t, cs.forward(bamboosdk.StreamEvent{
		Type:    bamboosdk.EventMessageStart,
		Message: &bamboosdk.BambooMessage{Role: bamboosdk.RoleAssistant},
	}))
	assert.Equal(t, before, len(cs.Frames()))
}

func TestClientStreamFlushOnce(t *testing.T) {
	cs := testClientStream(t, bamboocodec.FormatOpenAI)
	cs.ensureStart()
	cs.emitClosedText("hello")
	cs.finish()
	joined := strings.Join(cs.Frames(), "")
	assert.Equal(t, 1, strings.Count(joined, "data: [DONE]"))
	cs.finish()
	joined = strings.Join(cs.Frames(), "")
	assert.Equal(t, 1, strings.Count(joined, "data: [DONE]"))
}

func TestClientStreamSplitsLongText(t *testing.T) {
	cs := testClientStream(t, bamboocodec.FormatOpenAI)
	long := strings.Repeat("汉", clientStreamDeltaRunes+8)
	cs.emitClosedText(long)
	cs.finish()
	joined := strings.Join(cs.Frames(), "")
	assert.GreaterOrEqual(t, strings.Count(joined, `"content"`), 2)
}

func TestClientStreamEmitsOpenAIToolCall(t *testing.T) {
	cs := testClientStream(t, bamboocodec.FormatOpenAI)
	cs.emitToolUse("call_1", "WebSearch", `{"query":"cats"}`)
	cs.emitOpenAIToolOutput(0, "call_1", `{"content":[{"type":"text","text":"ok"}],"isError":false}`)
	cs.finish()
	joined := strings.Join(cs.Frames(), "")
	assert.Contains(t, joined, `"tool_calls"`)
	assert.Contains(t, joined, `"WebSearch"`)
	assert.Contains(t, joined, `"arguments"`)
	assert.Contains(t, joined, `"output"`)
	assert.Contains(t, joined, "isError")
	assert.Contains(t, joined, `"object":"chat.completion.chunk"`)
	assert.Contains(t, joined, `"model":"test-model"`)
	assert.Regexp(t, `"created":\d+`, joined)
}

func TestClientStreamFinish_ToolUseStopReasonWhenToolBlockEmitted(t *testing.T) {
	cs := testClientStream(t, bamboocodec.FormatAnthropic)
	cs.forward(bamboosdk.StreamEvent{
		Type:         bamboosdk.EventContentBlockStart,
		Index:        0,
		ContentBlock: bamboosdk.NewToolUseBlock("call_1", "bash", nil),
	})
	cs.finish()
	joined := strings.Join(cs.Frames(), "")
	assert.Contains(t, joined, `"stop_reason":"tool_use"`)
	assert.NotContains(t, joined, `"stop_reason":"end_turn"`)
}

func TestClientStreamFinish_EndTurnStopReasonWhenNoToolBlock(t *testing.T) {
	cs := testClientStream(t, bamboocodec.FormatAnthropic)
	cs.forward(bamboosdk.StreamEvent{
		Type:         bamboosdk.EventContentBlockStart,
		Index:        0,
		ContentBlock: bamboosdk.NewTextBlock("hello"),
	})
	cs.finish()
	joined := strings.Join(cs.Frames(), "")
	assert.Contains(t, joined, `"stop_reason":"end_turn"`)
	assert.NotContains(t, joined, `"stop_reason":"tool_use"`)
}
