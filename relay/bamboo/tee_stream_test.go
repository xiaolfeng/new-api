package bamboo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	_ "github.com/bamboo-services/bamboo-messages/bamboo/codec/anthropic"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
)

// TestTeeRelayForwardsThinkingThenCommitsAtToolUse 验证 tee 的核心规则：
// 工具调用出现前的 thinking/text 逐事件实时转发，tool_use 出现后不再示人。
func TestTeeRelayForwardsThinkingThenCommitsAtToolUse(t *testing.T) {
	cs := testClientStream(t, bamboocodec.FormatAnthropic)
	relay := &teeRelay{openBlocks: make(map[int]bool)}

	events := []bamboosdk.StreamEvent{
		{Type: bamboosdk.EventMessageStart, Message: &bamboosdk.BambooMessage{Role: bamboosdk.RoleAssistant}},
		{Type: bamboosdk.EventContentBlockStart, Index: 0, ContentBlock: bamboosdk.NewThinkingBlock("", "")},
		{Type: bamboosdk.EventContentBlockDelta, Index: 0, Delta: &bamboosdk.StreamDelta{Type: bamboosdk.DeltaThinkingDelta, Thinking: "first think"}},
		{Type: bamboosdk.EventContentBlockStop, Index: 0},
		{Type: bamboosdk.EventContentBlockStart, Index: 1, ContentBlock: bamboosdk.NewToolUseBlockWithRawInput("call_1", "web_search", `{"query":"cats"}`)},
		{Type: bamboosdk.EventContentBlockDelta, Index: 1, Delta: &bamboosdk.StreamDelta{Type: bamboosdk.DeltaInputJSON, PartialJSON: `{"query":"cats"}`}},
		{Type: bamboosdk.EventContentBlockStop, Index: 1},
		// commit 之后的内容不得透传。
		{Type: bamboosdk.EventContentBlockStart, Index: 2, ContentBlock: bamboosdk.NewTextBlock("")},
		{Type: bamboosdk.EventContentBlockDelta, Index: 2, Delta: &bamboosdk.StreamDelta{Type: bamboosdk.DeltaTextDelta, Text: "leaked text"}},
		{Type: bamboosdk.EventContentBlockStop, Index: 2},
	}
	for _, ev := range events {
		require.True(t, relay.forward(cs, ev))
	}

	joined := strings.Join(cs.Frames(), "\n")
	// 工具调用前的 thinking 已实时透传。
	assert.Contains(t, joined, "first think")
	// tool_use 及其之后的文本都不得出现。
	assert.NotContains(t, joined, "web_search")
	assert.NotContains(t, joined, "leaked text")
	// 消息终止语义统一由收尾路径补发，tee 不提前写 stop。
	assert.NotContains(t, joined, "message_stop")
	assert.True(t, relay.emittedContent)
	assert.True(t, relay.committed)
}

// TestTeeRelayDirectToolUseEmitsNothing 验证模型直接发起工具调用（无思考）时
// 客户端收不到任何内容帧，liveEmitted 保持 false，供 hop2 兜底 visible box。
func TestTeeRelayDirectToolUseEmitsNothing(t *testing.T) {
	cs := testClientStream(t, bamboocodec.FormatAnthropic)
	relay := &teeRelay{openBlocks: make(map[int]bool)}

	events := []bamboosdk.StreamEvent{
		{Type: bamboosdk.EventMessageStart, Message: &bamboosdk.BambooMessage{Role: bamboosdk.RoleAssistant}},
		{Type: bamboosdk.EventContentBlockStart, Index: 0, ContentBlock: bamboosdk.NewToolUseBlockWithRawInput("call_1", "web_search", `{"query":"cats"}`)},
		{Type: bamboosdk.EventContentBlockStop, Index: 0},
		{Type: bamboosdk.EventMessageDelta, Delta: &bamboosdk.MessageDelta{StopReason: bamboosdk.FinishReasonToolUse}},
		{Type: bamboosdk.EventMessageStop},
	}
	for _, ev := range events {
		require.True(t, relay.forward(cs, ev))
	}

	// message_start 已写，但没有任何内容块与终止帧。
	joined := strings.Join(cs.Frames(), "\n")
	assert.Contains(t, joined, "message_start")
	assert.NotContains(t, joined, "web_search")
	assert.False(t, relay.emittedContent)
	assert.True(t, relay.committed)
}

// TestTeeRelayHoldsTerminalEvents 验证 message_delta / message_stop / ping
// 不被 tee 透传，终止语义由决策后的 finish 统一补发。
func TestTeeRelayHoldsTerminalEvents(t *testing.T) {
	cs := testClientStream(t, bamboocodec.FormatAnthropic)
	relay := &teeRelay{openBlocks: make(map[int]bool)}

	events := []bamboosdk.StreamEvent{
		{Type: bamboosdk.EventMessageStart, Message: &bamboosdk.BambooMessage{Role: bamboosdk.RoleAssistant}},
		{Type: bamboosdk.EventContentBlockStart, Index: 0, ContentBlock: bamboosdk.NewTextBlock("")},
		{Type: bamboosdk.EventContentBlockDelta, Index: 0, Delta: &bamboosdk.StreamDelta{Type: bamboosdk.DeltaTextDelta, Text: "answer"}},
		{Type: bamboosdk.EventContentBlockStop, Index: 0},
		{Type: bamboosdk.EventMessageDelta, Delta: &bamboosdk.MessageDelta{StopReason: bamboosdk.FinishReasonEndTurn}},
		{Type: bamboosdk.EventMessageStop},
		{Type: bamboosdk.EventPing},
	}
	for _, ev := range events {
		require.True(t, relay.forward(cs, ev))
	}

	joined := strings.Join(cs.Frames(), "\n")
	assert.Contains(t, joined, "answer")
	assert.NotContains(t, joined, "message_stop")
	assert.False(t, relay.committed)
	assert.True(t, relay.emittedContent)
}

// TestTeeRelayClosesOpenBlockAfterCommit 验证 commit 后仍会补发已打开内容块的 stop，
// 避免客户端残留未闭合的 content block。
func TestTeeRelayClosesOpenBlockAfterCommit(t *testing.T) {
	cs := testClientStream(t, bamboocodec.FormatAnthropic)
	relay := &teeRelay{openBlocks: make(map[int]bool)}

	events := []bamboosdk.StreamEvent{
		{Type: bamboosdk.EventMessageStart, Message: &bamboosdk.BambooMessage{Role: bamboosdk.RoleAssistant}},
		{Type: bamboosdk.EventContentBlockStart, Index: 0, ContentBlock: bamboosdk.NewThinkingBlock("", "")},
		{Type: bamboosdk.EventContentBlockStart, Index: 1, ContentBlock: bamboosdk.NewToolUseBlockWithRawInput("call_1", "web_search", ``)},
		// thinking 的 stop 在 tool_use 之后到达，仍需补发。
		{Type: bamboosdk.EventContentBlockStop, Index: 0},
		{Type: bamboosdk.EventContentBlockStop, Index: 1},
	}
	for _, ev := range events {
		require.True(t, relay.forward(cs, ev))
	}

	joined := strings.Join(cs.Frames(), "\n")
	assert.Contains(t, joined, "content_block_stop")
	assert.NotContains(t, joined, "web_search")
	// thinking 块的 stop 被转发后从 openBlocks 移除，tool_use 的 stop 被丢弃。
	assert.Empty(t, relay.openBlocks)
}

// fakeBambooClient 用预置事件流模拟上游 Chat。
type fakeBambooClient struct {
	events []bamboosdk.StreamEvent
}

func (f *fakeBambooClient) Chat(ctx context.Context, _ []bamboosdk.BambooMessage, _ string, _ *bamboosdk.RequestConfig) (<-chan bamboosdk.StreamEvent, error) {
	ch := make(chan bamboosdk.StreamEvent, len(f.events))
	go func() {
		defer close(ch)
		for _, ev := range f.events {
			select {
			case <-ctx.Done():
				return
			case ch <- ev:
			}
		}
	}()
	return ch, nil
}

func (f *fakeBambooClient) Complete(context.Context, []bamboosdk.BambooMessage, string, *bamboosdk.RequestConfig) (*bamboosdk.Response, error) {
	return nil, nil
}

func newHostTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return c, w
}

func testCodec(t *testing.T, format bamboocodec.FormatType) bamboocodec.Codec {
	t.Helper()
	codec, err := bamboocodec.Get(format)
	require.NoError(t, err)
	return codec
}

// TestDoHostStreamRelayPassthroughStreamsWholeAnswer 验证无工具调用的 host-tool
// 流式请求把整个 hop1 实时透传：thinking + text 在终止帧之前到达，
// 流由 finish 统一补发单个 message_stop 收尾。
func TestDoHostStreamRelayPassthroughStreamsWholeAnswer(t *testing.T) {
	codec := testCodec(t, bamboocodec.FormatAnthropic)
	c, w := newHostTestContext(t)
	info, err := relaycommon.GenRelayInfo(c, types.RelayFormatClaude, &dto.BaseRequest{}, nil)
	require.NoError(t, err)
	cs := newClientStream(c, codec, info, "test-model")
	client := &fakeBambooClient{events: []bamboosdk.StreamEvent{
		{Type: bamboosdk.EventMessageStart, Message: &bamboosdk.BambooMessage{Role: bamboosdk.RoleAssistant}, Usage: &bamboosdk.Usage{InputTokens: 10, OutputTokens: 5}},
		{Type: bamboosdk.EventContentBlockStart, Index: 0, ContentBlock: bamboosdk.NewThinkingBlock("", "")},
		{Type: bamboosdk.EventContentBlockDelta, Index: 0, Delta: &bamboosdk.StreamDelta{Type: bamboosdk.DeltaThinkingDelta, Thinking: "thinking live"}},
		{Type: bamboosdk.EventContentBlockStop, Index: 0},
		{Type: bamboosdk.EventContentBlockStart, Index: 1, ContentBlock: bamboosdk.NewTextBlock("")},
		{Type: bamboosdk.EventContentBlockDelta, Index: 1, Delta: &bamboosdk.StreamDelta{Type: bamboosdk.DeltaTextDelta, Text: "final answer"}},
		{Type: bamboosdk.EventContentBlockStop, Index: 1},
		{Type: bamboosdk.EventMessageDelta, Delta: &bamboosdk.MessageDelta{StopReason: bamboosdk.FinishReasonEndTurn}},
		{Type: bamboosdk.EventMessageStop},
	}}

	usage, apiErr := doHostStreamRelay(c, info, client, codec, codec.Format(), &bamboocodec.RelayRequest{
		Messages: []bamboosdk.BambooMessage{{Role: bamboosdk.RoleUser, Content: []bamboosdk.ContentBlock{bamboosdk.NewTextBlock("hi")}}},
		Config:   &bamboosdk.RequestConfig{Model: "test-model"},
		IsStream: true,
	}, cs)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	joined := strings.Join(cs.Frames(), "\n")
	assert.Contains(t, joined, "thinking live")
	assert.Contains(t, joined, "final answer")
	// event 行 + data 行各出现一次 "message_stop"，即只补发了一条终止帧。
	assert.Equal(t, 2, strings.Count(joined, "message_stop"))
	assert.Equal(t, 1, strings.Count(joined, "event: message_stop"))
	// 无工具调用时不写 tool_use 帧。
	assert.NotContains(t, joined, "tool_use")
	// 响应体已记录且流正常结束。
	assert.Contains(t, info.ResponseBody, "final answer")
	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
	// 首字时间在转发期间即已打点（而非回放时）。
	assert.False(t, info.FirstResponseTime.IsZero())
	// 上游字节已实时写出。
	assert.NotEmpty(t, w.Body.String())
}
