package bamboo

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
)

// hostToolFetchPlan 构造仅声明 web_fetch 的最小 host-tool 计划，
// 工具执行用 127.0.0.1 触发 SSRF 保护快速失败，避免测试联网。
func hostToolFetchPlan() *relaycommon.HostToolPlan {
	return &relaycommon.HostToolPlan{
		Enabled: true,
		Mode:    "loop",
		Decls: []relaycommon.HostToolDecl{
			{OriginalName: "web_fetch", Canonical: relaycommon.HostToolCanonicalFetch},
		},
	}
}

// lastOpenAIUsageChunk 返回 flow 中最后一次出现的 usage chunk（openai 流式格式）。
func lastOpenAIUsageChunk(frames []string) map[string]any {
	var last map[string]any
	for _, frame := range frames {
		line := strings.TrimSpace(frame)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			continue
		}
		var obj map[string]any
		if err := common.Unmarshal([]byte(payload), &obj); err != nil {
			continue
		}
		if usage, ok := obj["usage"].(map[string]any); ok {
			last = usage
		}
	}
	return last
}

// TestDoHostStreamRelayMergesHopUsageIntoFinalChunk 验证多 hop 流式路径的
// 出口 usage chunk 合并 hop1+hop2 总消耗，而非只反映最后 hop。
func TestDoHostStreamRelayMergesHopUsageIntoFinalChunk(t *testing.T) {
	setupToolLogDB(t)
	codec := testCodec(t, bamboocodec.FormatOpenAI)
	c, _ := newHostTestContext(t)
	info, err := relaycommon.GenRelayInfo(c, types.RelayFormatOpenAI, &dto.BaseRequest{}, nil)
	require.NoError(t, err)
	info.ChannelMeta = &relaycommon.ChannelMeta{ChannelId: 1}
	info.HostToolPlan = hostToolFetchPlan()
	cs := newClientStream(c, codec, info, "test-model")

	hop1 := []bamboosdk.StreamEvent{
		{Type: bamboosdk.EventMessageStart, Message: &bamboosdk.BambooMessage{Role: bamboosdk.RoleAssistant}, Usage: &bamboosdk.Usage{InputTokens: 100}},
		{Type: bamboosdk.EventContentBlockStart, Index: 0, ContentBlock: bamboosdk.NewToolUseBlockWithRawInput("call_1", "web_fetch", `{"url":"http://127.0.0.1"}`)},
		{Type: bamboosdk.EventContentBlockStop, Index: 0},
		{Type: bamboosdk.EventMessageDelta, Delta: &bamboosdk.MessageDelta{StopReason: bamboosdk.FinishReasonToolUse}},
		{Type: bamboosdk.EventMessageStop},
	}
	hop2 := []bamboosdk.StreamEvent{
		{Type: bamboosdk.EventMessageStart, Message: &bamboosdk.BambooMessage{Role: bamboosdk.RoleAssistant}},
		{Type: bamboosdk.EventContentBlockStart, Index: 0, ContentBlock: bamboosdk.NewTextBlock("")},
		{Type: bamboosdk.EventContentBlockDelta, Index: 0, Delta: &bamboosdk.StreamDelta{Type: bamboosdk.DeltaTextDelta, Text: "final"}},
		{Type: bamboosdk.EventContentBlockStop, Index: 0},
		{Type: bamboosdk.EventMessageDelta, Delta: &bamboosdk.MessageDelta{StopReason: bamboosdk.FinishReasonEndTurn}},
		{Type: bamboosdk.EventMessageDelta, Delta: &bamboosdk.MessageDelta{}, Usage: &bamboosdk.Usage{InputTokens: 150, OutputTokens: 30, CacheReadInputTokens: 5}},
		{Type: bamboosdk.EventMessageStop},
	}
	client := &fakeBambooClient{hops: [][]bamboosdk.StreamEvent{hop1, hop2}}

	usage, apiErr := doHostStreamRelay(c, info, client, codec, codec.Format(), &bamboocodec.RelayRequest{
		Messages: []bamboosdk.BambooMessage{{Role: bamboosdk.RoleUser, Content: []bamboosdk.ContentBlock{bamboosdk.NewTextBlock("fetch")}}},
		Config:   &bamboosdk.RequestConfig{Model: "test-model"},
		IsStream: true,
	}, cs)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	// 内部计费口径（非缓存输入合并）：hop1=100，hop2=150-5(缓存读取)=145。
	require.Equal(t, 245, usage.PromptTokens)
	require.Equal(t, 30, usage.CompletionTokens)

	// 出口最终 usage chunk 必须是 hop1+hop2 合并值。
	lastUsage := lastOpenAIUsageChunk(cs.Frames())
	require.NotNil(t, lastUsage, "missing final usage chunk in stream output")
	require.EqualValues(t, 250, lastUsage["prompt_tokens"])
	require.EqualValues(t, 30, lastUsage["completion_tokens"])
	require.EqualValues(t, 280, lastUsage["total_tokens"])
	details, ok := lastUsage["prompt_tokens_details"].(map[string]any)
	require.True(t, ok, "missing prompt_tokens_details")
	require.EqualValues(t, 5, details["cached_tokens"])
}

// TestDoHostCompleteRelayMergesRawUsageIntoResponse 验证非流式多 hop 出口
// 响应的 usage 为两 hop 原始用量之和（客户端可见 total 反映完整消耗）。
func TestDoHostCompleteRelayMergesRawUsageIntoResponse(t *testing.T) {
	setupToolLogDB(t)
	codec := testCodec(t, bamboocodec.FormatOpenAI)
	c, w := newHostTestContext(t)
	info, err := relaycommon.GenRelayInfo(c, types.RelayFormatOpenAI, &dto.BaseRequest{}, nil)
	require.NoError(t, err)
	info.ChannelMeta = &relaycommon.ChannelMeta{ChannelId: 1}
	info.HostToolPlan = hostToolFetchPlan()

	client := &fakeBambooClient{completes: []*bamboosdk.Response{
		{
			ID:      "m1",
			Model:   "test-model",
			Content: []bamboosdk.ContentBlock{bamboosdk.NewToolUseBlockWithRawInput("call_1", "web_fetch", `{"url":"http://127.0.0.1"}`)},
			Usage:   bamboosdk.Usage{InputTokens: 100, OutputTokens: 20, CacheReadInputTokens: 10},
		},
		{
			ID:      "m2",
			Model:   "test-model",
			Content: []bamboosdk.ContentBlock{bamboosdk.NewTextBlock("final answer")},
			Usage:   bamboosdk.Usage{InputTokens: 150, OutputTokens: 30, CacheReadInputTokens: 5},
		},
	}}

	usage, apiErr := doHostCompleteRelay(c, info, client, codec, codec.Format(), &bamboocodec.RelayRequest{
		Messages: []bamboosdk.BambooMessage{{Role: bamboosdk.RoleUser, Content: []bamboosdk.ContentBlock{bamboosdk.NewTextBlock("fetch")}}},
		Config:   &bamboosdk.RequestConfig{Model: "test-model"},
	})
	require.Nil(t, apiErr)
	require.NotNil(t, usage)

	var resp struct {
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
			TotalTokens      int64 `json:"total_tokens"`
			PromptTokensDetails struct {
				CachedTokens int64 `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
		} `json:"usage"`
	}
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, int64(250), resp.Usage.PromptTokens)
	require.Equal(t, int64(50), resp.Usage.CompletionTokens)
	require.Equal(t, int64(300), resp.Usage.TotalTokens)
	require.Equal(t, int64(15), resp.Usage.PromptTokensDetails.CachedTokens)
}