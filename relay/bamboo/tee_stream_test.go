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
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
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
	// tool_use 及其之后的文本都不得实时透传。
	assert.NotContains(t, joined, "web_search")
	assert.NotContains(t, joined, "leaked text")
	// 消息终止语义统一由收尾路径补发，tee 不提前写 stop。
	assert.NotContains(t, joined, "message_stop")
	// commit point 遮住的事件全部暂存，供 passthrough 补发。
	require.Len(t, relay.held, 6)
	assert.Equal(t, bamboosdk.EventContentBlockStart, relay.held[0].Type)
	if tu, ok := relay.held[0].ContentBlock.(*bamboosdk.ToolUseBlock); ok {
		assert.Equal(t, "web_search", tu.Name)
	} else {
		t.Fatalf("held[0] 应为 ToolUseBlock，got %T", relay.held[0].ContentBlock)
	}
	assert.Contains(t, string(relay.held[1].Delta.(*bamboosdk.StreamDelta).PartialJSON), "cats")
	assert.Equal(t, bamboosdk.EventContentBlockStop, relay.held[5].Type)
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

// setupToolLogDB 为 ExecuteCalls 的日志记录路径初始化 sqlite 内存库。
func setupToolLogDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	oldDB, oldLogDB := model.DB, model.LOG_DB
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.ToolLog{}))
	t.Cleanup(func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
	})
}

// fakeBambooClient 按调用次序依次回放预置的 hop 事件流，模拟上游多轮 Chat；
// completes 供非流式 Complete 依次回放响应。
type fakeBambooClient struct {
	hops         [][]bamboosdk.StreamEvent
	completes    []*bamboosdk.Response
	call         int
	completeCall int
}

func (f *fakeBambooClient) Chat(ctx context.Context, _ []bamboosdk.BambooMessage, _ string, _ *bamboosdk.RequestConfig) (<-chan bamboosdk.StreamEvent, error) {
	var events []bamboosdk.StreamEvent
	if f.call < len(f.hops) {
		events = f.hops[f.call]
		f.call++
	}
	ch := make(chan bamboosdk.StreamEvent, len(events))
	go func() {
		defer close(ch)
		for _, ev := range events {
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
	if f.completeCall < len(f.completes) {
		resp := f.completes[f.completeCall]
		f.completeCall++
		return resp, nil
	}
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
	client := &fakeBambooClient{hops: [][]bamboosdk.StreamEvent{{
		{Type: bamboosdk.EventMessageStart, Message: &bamboosdk.BambooMessage{Role: bamboosdk.RoleAssistant}, Usage: &bamboosdk.Usage{InputTokens: 10, OutputTokens: 5}},
		{Type: bamboosdk.EventContentBlockStart, Index: 0, ContentBlock: bamboosdk.NewThinkingBlock("", "")},
		{Type: bamboosdk.EventContentBlockDelta, Index: 0, Delta: &bamboosdk.StreamDelta{Type: bamboosdk.DeltaThinkingDelta, Thinking: "thinking live"}},
		{Type: bamboosdk.EventContentBlockStop, Index: 0},
		{Type: bamboosdk.EventContentBlockStart, Index: 1, ContentBlock: bamboosdk.NewTextBlock("")},
		{Type: bamboosdk.EventContentBlockDelta, Index: 1, Delta: &bamboosdk.StreamDelta{Type: bamboosdk.DeltaTextDelta, Text: "final answer"}},
		{Type: bamboosdk.EventContentBlockStop, Index: 1},
		{Type: bamboosdk.EventMessageDelta, Delta: &bamboosdk.MessageDelta{StopReason: bamboosdk.FinishReasonEndTurn}},
		{Type: bamboosdk.EventMessageStop},
	}}}

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

func TestDoHostStreamRelayResponsesPassthroughKeepsUsage(t *testing.T) {
	codec := testCodec(t, bamboocodec.FormatResponses)
	c, _ := newHostTestContext(t)
	info, err := relaycommon.GenRelayInfo(c, types.RelayFormatOpenAIResponses, &dto.OpenAIResponsesRequest{}, nil)
	require.NoError(t, err)
	cs := newClientStream(c, codec, info, "test-model")
	client := &fakeBambooClient{hops: [][]bamboosdk.StreamEvent{{
		{Type: bamboosdk.EventMessageStart, Message: &bamboosdk.BambooMessage{Role: bamboosdk.RoleAssistant}, Usage: &bamboosdk.Usage{InputTokens: 10}},
		{Type: bamboosdk.EventContentBlockStart, Index: 0, ContentBlock: bamboosdk.NewTextBlock("")},
		{Type: bamboosdk.EventContentBlockDelta, Index: 0, Delta: &bamboosdk.StreamDelta{Type: bamboosdk.DeltaTextDelta, Text: "hello"}},
		{Type: bamboosdk.EventContentBlockStop, Index: 0},
		{Type: bamboosdk.EventPing, Usage: &bamboosdk.Usage{InputTokens: 42, OutputTokens: 18}},
		{Type: bamboosdk.EventMessageDelta, Delta: &bamboosdk.MessageDelta{StopReason: bamboosdk.FinishReasonEndTurn}, Usage: &bamboosdk.Usage{InputTokens: 42, OutputTokens: 18}},
		{Type: bamboosdk.EventMessageStop},
	}}}

	usage, apiErr := doHostStreamRelay(c, info, client, codec, codec.Format(), &bamboocodec.RelayRequest{
		Messages: []bamboosdk.BambooMessage{{Role: bamboosdk.RoleUser, Content: []bamboosdk.ContentBlock{bamboosdk.NewTextBlock("hi")}}},
		Config:   &bamboosdk.RequestConfig{Model: "test-model"},
		IsStream: true,
	}, cs)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	joined := strings.Join(cs.Frames(), "\n")
	assert.Contains(t, joined, `"input_tokens":42`)
	assert.Contains(t, joined, `"output_tokens":18`)
	assert.Contains(t, joined, `"total_tokens":60`)
	assert.NotContains(t, joined, `"total_tokens":0`)
}

// TestDoHostStreamRelayGrokPassthroughForwardsToolUse 复现"无第二轮"回归：
// Grok Build 的 web_search 是客户端 function，走 passthrough 时必须把
// tee 遮住的 function_call 补发给客户端，客户端才能执行搜索并发起第二轮。
func TestDoHostStreamRelayGrokPassthroughForwardsToolUse(t *testing.T) {
	codec := testCodec(t, bamboocodec.FormatAnthropic)
	c, _ := newHostTestContext(t)
	info, err := relaycommon.GenRelayInfo(c, types.RelayFormatClaude, &dto.BaseRequest{}, nil)
	require.NoError(t, err)
	info.ClientProfile = common.ClientProfileGrokBuild
	info.HostToolPlan = &relaycommon.HostToolPlan{
		Enabled: true,
		Mode:    "loop",
	}
	cs := newClientStream(c, codec, info, "test-model")

	hop1 := []bamboosdk.StreamEvent{
		{Type: bamboosdk.EventMessageStart, Message: &bamboosdk.BambooMessage{Role: bamboosdk.RoleAssistant}},
		{Type: bamboosdk.EventContentBlockStart, Index: 0, ContentBlock: bamboosdk.NewThinkingBlock("", "")},
		{Type: bamboosdk.EventContentBlockDelta, Index: 0, Delta: &bamboosdk.StreamDelta{Type: bamboosdk.DeltaThinkingDelta, Thinking: "need latest"}},
		{Type: bamboosdk.EventContentBlockStop, Index: 0},
		{Type: bamboosdk.EventContentBlockStart, Index: 1, ContentBlock: bamboosdk.NewToolUseBlockWithRawInput("call_1", "web_search", `{"query":"latest news"}`)},
		{Type: bamboosdk.EventContentBlockDelta, Index: 1, Delta: &bamboosdk.StreamDelta{Type: bamboosdk.DeltaInputJSON, PartialJSON: `{"query":"latest news"}`}},
		{Type: bamboosdk.EventContentBlockStop, Index: 1},
		{Type: bamboosdk.EventMessageDelta, Delta: &bamboosdk.MessageDelta{StopReason: bamboosdk.FinishReasonToolUse}},
		{Type: bamboosdk.EventMessageStop},
	}
	client := &fakeBambooClient{hops: [][]bamboosdk.StreamEvent{hop1}}

	usage, apiErr := doHostStreamRelay(c, info, client, codec, codec.Format(), &bamboocodec.RelayRequest{
		Messages: []bamboosdk.BambooMessage{{Role: bamboosdk.RoleUser, Content: []bamboosdk.ContentBlock{bamboosdk.NewTextBlock("news?")}}},
		Config:   &bamboosdk.RequestConfig{Model: "test-model"},
		IsStream: true,
	}, cs)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	joined := strings.Join(cs.Frames(), "\n")
	// 思考实时透传。
	assert.Contains(t, joined, "need latest")
	// Grok search 是透传型工具：function_call（web_search）必须补发给客户端。
	assert.Contains(t, joined, "web_search")
	assert.Contains(t, joined, "latest news")
	// 客户端据此发起第二轮，流以正常终止帧收尾。
	assert.Equal(t, 1, strings.Count(joined, "event: message_stop"))
	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
	// 工具未在网关执行（passthrough 不发工具公告）。
	assert.NotContains(t, joined, "ssrf_blocked")
}

// TestDoHostStreamRelayHop2ContinuesAfterToolUse 复现"工具调用后无续流"：
// hop1 思考后调用 web_fetch（host 工具），网关执行后应继续 hop2 并透传最终答案。
// 工具执行用 127.0.0.1 触发 SSRF 保护快速失败，避免测试联网。
func TestDoHostStreamRelayHop2ContinuesAfterToolUse(t *testing.T) {
	setupToolLogDB(t)
	codec := testCodec(t, bamboocodec.FormatAnthropic)
	c, _ := newHostTestContext(t)
	info, err := relaycommon.GenRelayInfo(c, types.RelayFormatClaude, &dto.BaseRequest{}, nil)
	require.NoError(t, err)
	info.ChannelMeta = &relaycommon.ChannelMeta{ChannelId: 1}
	info.HostToolPlan = &relaycommon.HostToolPlan{
		Enabled: true,
		Mode:    "loop",
		Decls: []relaycommon.HostToolDecl{
			{OriginalName: "web_fetch", Canonical: relaycommon.HostToolCanonicalFetch},
		},
	}
	cs := newClientStream(c, codec, info, "test-model")

	// hop1：思考 + web_fetch 工具调用（tee 应透传思考、挡住 tool_use）。
	hop1 := []bamboosdk.StreamEvent{
		{Type: bamboosdk.EventMessageStart, Message: &bamboosdk.BambooMessage{Role: bamboosdk.RoleAssistant}},
		{Type: bamboosdk.EventContentBlockStart, Index: 0, ContentBlock: bamboosdk.NewThinkingBlock("", "")},
		{Type: bamboosdk.EventContentBlockDelta, Index: 0, Delta: &bamboosdk.StreamDelta{Type: bamboosdk.DeltaThinkingDelta, Thinking: "need data"}},
		{Type: bamboosdk.EventContentBlockStop, Index: 0},
		{Type: bamboosdk.EventContentBlockStart, Index: 1, ContentBlock: bamboosdk.NewToolUseBlockWithRawInput("call_1", "web_fetch", `{"url":"http://127.0.0.1:1/","format":"text"}`)},
		{Type: bamboosdk.EventContentBlockStop, Index: 1},
		{Type: bamboosdk.EventMessageDelta, Delta: &bamboosdk.MessageDelta{StopReason: bamboosdk.FinishReasonToolUse}},
		{Type: bamboosdk.EventMessageStop},
	}
	// hop2：模型基于工具结果给出最终答案。
	hop2 := []bamboosdk.StreamEvent{
		{Type: bamboosdk.EventMessageStart, Message: &bamboosdk.BambooMessage{Role: bamboosdk.RoleAssistant}},
		{Type: bamboosdk.EventContentBlockStart, Index: 0, ContentBlock: bamboosdk.NewTextBlock("")},
		{Type: bamboosdk.EventContentBlockDelta, Index: 0, Delta: &bamboosdk.StreamDelta{Type: bamboosdk.DeltaTextDelta, Text: "hop2 final answer"}},
		{Type: bamboosdk.EventContentBlockStop, Index: 0},
		{Type: bamboosdk.EventMessageDelta, Delta: &bamboosdk.MessageDelta{StopReason: bamboosdk.FinishReasonEndTurn}},
		{Type: bamboosdk.EventMessageStop},
	}
	client := &fakeBambooClient{hops: [][]bamboosdk.StreamEvent{hop1, hop2}}

	usage, apiErr := doHostStreamRelay(c, info, client, codec, codec.Format(), &bamboocodec.RelayRequest{
		Messages: []bamboosdk.BambooMessage{{Role: bamboosdk.RoleUser, Content: []bamboosdk.ContentBlock{bamboosdk.NewTextBlock("fetch it")}}},
		Config:   &bamboosdk.RequestConfig{Model: "test-model"},
		IsStream: true,
	}, cs)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	joined := strings.Join(cs.Frames(), "\n")
	// 思考实时透传。
	assert.Contains(t, joined, "need data")
	// 工具公告（host web_fetch）发出，但 hop1 的 tool_use 帧不透传 tool_use 名。
	assert.Contains(t, joined, "web_fetch")
	// hop2 的最终答案必须出现在流中——缺失即复现"直接终止"。
	assert.Contains(t, joined, "hop2 final answer")
	assert.Equal(t, 1, strings.Count(joined, "event: message_stop"))
	// 流正常结束。
	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
}
