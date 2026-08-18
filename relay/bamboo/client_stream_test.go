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

func TestBlocksToStreamEventsKeepsToolUse(t *testing.T) {
	blocks := []bamboosdk.ContentBlock{
		bamboosdk.NewTextBlock("hi"),
		bamboosdk.NewToolUseBlockWithRawInput("call_1", "bash", `{"cmd":"ls"}`),
	}
	events := blocksToStreamEvents(blocks)
	var kinds []bamboosdk.StreamEventType
	for _, ev := range events {
		kinds = append(kinds, ev.Type)
	}
	require.Contains(t, kinds, bamboosdk.EventContentBlockStart)
	var sawTool bool
	for _, ev := range events {
		if ev.Type == bamboosdk.EventContentBlockStart {
			if _, ok := ev.ContentBlock.(*bamboosdk.ToolUseBlock); ok {
				sawTool = true
			}
		}
	}
	assert.True(t, sawTool)
}
