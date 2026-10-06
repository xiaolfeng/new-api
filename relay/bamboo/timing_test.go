package bamboo

import (
	"net/http/httptest"
	"testing"
	"time"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	_ "github.com/bamboo-services/bamboo-messages/bamboo/codec/openai"
	"github.com/bamboo-services/bamboo-messages/provider"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestBambooTimingCollectorWithTimingAnchor(t *testing.T) {
	baseTime := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	requestSentAt := baseTime
	responseHeaderAt := baseTime.Add(500 * time.Millisecond)

	collector := newBambooTimingCollector()

	// 1. message_start 事件携带 TimingAnchor
	collector.observe(bamboosdk.StreamEvent{
		Type:       bamboosdk.EventMessageStart,
		ReceivedAt: responseHeaderAt,
		Timing: &provider.TimingAnchor{
			RequestSentAt:    requestSentAt,
			ResponseHeaderAt: responseHeaderAt,
		},
	})

	// 2. 经过 8 秒的上游 Prefill 与思考，到达思考块 start
	thinkingStart := baseTime.Add(8000 * time.Millisecond)
	collector.observe(bamboosdk.StreamEvent{
		Type:         bamboosdk.EventContentBlockStart,
		ReceivedAt:   thinkingStart,
		ContentBlock: bamboosdk.NewThinkingBlock("", ""),
	})

	// 3. 首个内容 delta（首字时间打点源）
	firstDeltaTime := baseTime.Add(8100 * time.Millisecond)
	collector.observe(bamboosdk.StreamEvent{
		Type:       bamboosdk.EventContentBlockDelta,
		ReceivedAt: firstDeltaTime,
		Delta: &bamboosdk.StreamDelta{
			Type:     bamboosdk.DeltaThinkingDelta,
			Thinking: "Let me think...",
		},
	})

	// 4. 正文块 start
	contentStart := baseTime.Add(8500 * time.Millisecond)
	collector.observe(bamboosdk.StreamEvent{
		Type:         bamboosdk.EventContentBlockStart,
		ReceivedAt:   contentStart,
		ContentBlock: bamboosdk.NewTextBlock(""),
	})

	// 5. 正文 delta
	contentDeltaTime := baseTime.Add(8700 * time.Millisecond)
	collector.observe(bamboosdk.StreamEvent{
		Type:       bamboosdk.EventContentBlockDelta,
		ReceivedAt: contentDeltaTime,
		Delta: &bamboosdk.StreamDelta{
			Type: bamboosdk.DeltaTextDelta,
			Text: "Hello world!",
		},
	})

	// 6. message_stop
	stopTime := baseTime.Add(9000 * time.Millisecond)
	collector.observe(bamboosdk.StreamEvent{
		Type:       bamboosdk.EventMessageStop,
		ReceivedAt: stopTime,
	})

	res := collector.result()

	// 验证核心指标与物理不变量：
	// TotalDuration 必须锚定 requestSentAt（9000ms），绝不能是仅首帧到尾帧的时间
	assert.Equal(t, 9000*time.Millisecond, res.Stats.TotalDuration)
	// FirstByteDuration 必须锚定 requestSentAt（8100ms），包含网络建连与 Prefill
	assert.Equal(t, 8100*time.Millisecond, res.Stats.FirstByteDuration)
	assert.True(t, res.Stats.FirstByteDuration <= res.Stats.TotalDuration, "TTFT <= TotalDuration 必须恒成立")

	// 思考耗时与正文耗时
	assert.Equal(t, 500*time.Millisecond, res.Stats.ThinkingDuration) // 8000ms -> 8500ms
	assert.Equal(t, 500*time.Millisecond, res.Stats.ContentDuration)  // 8500ms -> 9000ms

	// 耗时 >= 100ms，TPS 应为正常正数
	assert.True(t, res.Rates.ThinkingTokensPerSec > 0)
	assert.True(t, res.Rates.OutputTokensPerSec > 0)
}

func TestBambooTimingCollectorFallbackWithoutAnchor(t *testing.T) {
	baseTime := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	collector := newBambooTimingCollector()

	// 无 TimingAnchor
	collector.observe(bamboosdk.StreamEvent{
		Type:       bamboosdk.EventMessageStart,
		ReceivedAt: baseTime,
	})

	firstDelta := baseTime.Add(200 * time.Millisecond)
	collector.observe(bamboosdk.StreamEvent{
		Type:       bamboosdk.EventContentBlockDelta,
		ReceivedAt: firstDelta,
		Delta: &bamboosdk.StreamDelta{
			Type: bamboosdk.DeltaTextDelta,
			Text: "Hello",
		},
	})

	stopTime := baseTime.Add(600 * time.Millisecond)
	collector.observe(bamboosdk.StreamEvent{
		Type:       bamboosdk.EventMessageStop,
		ReceivedAt: stopTime,
	})

	res := collector.result()
	// 无锚点时平滑回退到 startTime
	assert.Equal(t, 600*time.Millisecond, res.Stats.TotalDuration)
	assert.Equal(t, 200*time.Millisecond, res.Stats.FirstByteDuration)
}

func TestBambooTimingCollectorTPSReliabilityThreshold(t *testing.T) {
	baseTime := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	collector := newBambooTimingCollector()

	collector.observe(bamboosdk.StreamEvent{
		Type:       bamboosdk.EventMessageStart,
		ReceivedAt: baseTime,
	})

	// 模拟 TCP 缓冲倒灌：内容块在 20ms 内刷完（< 100ms 阈值）
	collector.observe(bamboosdk.StreamEvent{
		Type:         bamboosdk.EventContentBlockStart,
		ReceivedAt:   baseTime.Add(10 * time.Millisecond),
		ContentBlock: bamboosdk.NewTextBlock(""),
	})
	collector.observe(bamboosdk.StreamEvent{
		Type:       bamboosdk.EventContentBlockDelta,
		ReceivedAt: baseTime.Add(20 * time.Millisecond),
		Delta: &bamboosdk.StreamDelta{
			Type: bamboosdk.DeltaTextDelta,
			Text: "This is a fast buffer flush with lots of tokens",
		},
	})
	collector.observe(bamboosdk.StreamEvent{
		Type:       bamboosdk.EventMessageStop,
		ReceivedAt: baseTime.Add(30 * time.Millisecond),
	})

	res := collector.result()
	// 内容耗时为 20ms (30ms - 10ms)，低于 minReliableDuration (100ms)
	assert.True(t, res.Stats.ContentDuration < minReliableDuration)
	// TPS 必须被标记为负数不可靠
	assert.True(t, res.Rates.OutputTokensPerSec < 0, "低于 100ms 的暴冲 TPS 必须为负数标记")
}

func TestClientStreamFirstResponseTimeCalibration(t *testing.T) {
	codec, err := bamboocodec.Get(bamboocodec.FormatOpenAI)
	require.NoError(t, err)

	relayInfo := &relaycommon.RelayInfo{
		StartTime: time.Now(),
	}
	relayInfo.InitFirstResponse()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	cs := newClientStream(c, codec, relayInfo, "test-model")

	// 1. 发送 EventMessageStart：信封到达，此时不应打点首字响应时间
	ok := cs.forward(bamboosdk.StreamEvent{
		Type: bamboosdk.EventMessageStart,
	})
	require.True(t, ok)
	assert.True(t, relayInfo.FirstResponseTime.IsZero(), "EventMessageStart 时不应打上首字时间")

	// 空内容块不是有效内容，不能提前打点。
	ok = cs.forward(bamboosdk.StreamEvent{
		Type:         bamboosdk.EventContentBlockStart,
		Index:        0,
		ContentBlock: bamboosdk.NewTextBlock(""),
	})
	require.True(t, ok)
	assert.True(t, relayInfo.FirstResponseTime.IsZero(), "空 content_block_start 不应打点")
	require.True(t, cs.forward(bamboosdk.StreamEvent{
		Type:  bamboosdk.EventContentBlockDelta,
		Delta: &bamboosdk.StreamDelta{Type: bamboosdk.DeltaTextDelta, Text: "hello"},
	}))
	assert.False(t, relayInfo.FirstResponseTime.IsZero(), "有效内容成功写出后必须打点")
}
