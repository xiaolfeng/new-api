package bamboo

import (
	"fmt"
	"math"
	"time"
	"unicode"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"

	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

type collectorPhase int

const (
	phaseInit     collectorPhase = iota
	phaseThinking
	phaseContent
	phaseTool
)

type charCounter struct {
	cjk   int64
	latin int64
	other int64
}

func (c *charCounter) add(text string) {
	for _, r := range text {
		switch {
		case isCJKRune(r):
			c.cjk++
		case isLatinAlnumRune(r):
			c.latin++
		default:
			c.other++
		}
	}
}

func (c *charCounter) estimateTokens() int64 {
	return c.cjk + c.latin/4 + c.other/2
}

func isCJKRune(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Hangul, r)
}

func isLatinAlnumRune(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9')
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

type bambooTimingCollector struct {
	startTime     time.Time
	firstByteTime time.Time
	stopTime      time.Time
	lastEventTime time.Time

	thinkingStart time.Time
	thinkingEnd   time.Time
	contentStart  time.Time
	contentEnd    time.Time
	toolStart     time.Time

	phase collectorPhase

	thinkingChars charCounter
	outputChars   charCounter
	toolChars     charCounter
}

func newBambooTimingCollector() *bambooTimingCollector {
	return &bambooTimingCollector{phase: phaseInit}
}

func (tc *bambooTimingCollector) observe(event bamboosdk.StreamEvent) {
	now := time.Now()
	tc.lastEventTime = now

	if tc.startTime.IsZero() {
		tc.startTime = now
	}

	switch event.Type {
	case bamboosdk.EventContentBlockStart:
		tc.handleBlockStart(event, now)
	case bamboosdk.EventContentBlockDelta:
		tc.handleDelta(event, now)
	case bamboosdk.EventMessageStop:
		tc.stopTime = now
	}
}

func (tc *bambooTimingCollector) handleBlockStart(event bamboosdk.StreamEvent, now time.Time) {
	if event.ContentBlock == nil {
		return
	}
	switch event.ContentBlock.BlockType() {
	case bamboosdk.ContentBlockThinking:
		if tc.thinkingStart.IsZero() {
			tc.thinkingStart = now
		}
		tc.phase = phaseThinking

	case bamboosdk.ContentBlockText:
		if tc.contentStart.IsZero() {
			tc.contentStart = now
		}
		if tc.thinkingEnd.IsZero() && !tc.thinkingStart.IsZero() {
			tc.thinkingEnd = now
		}
		tc.phase = phaseContent

	case bamboosdk.ContentBlockToolUse:
		if tc.toolStart.IsZero() {
			tc.toolStart = now
		}
		if tc.contentEnd.IsZero() && !tc.contentStart.IsZero() {
			tc.contentEnd = now
		}
		tc.phase = phaseTool
	}
}

func (tc *bambooTimingCollector) handleDelta(event bamboosdk.StreamEvent, now time.Time) {
	if tc.firstByteTime.IsZero() {
		tc.firstByteTime = now
	}

	delta, ok := event.Delta.(*bamboosdk.StreamDelta)
	if !ok || delta == nil {
		return
	}

	switch delta.Type {
	case bamboosdk.DeltaThinkingDelta:
		if tc.thinkingStart.IsZero() {
			tc.thinkingStart = now
		}
		if delta.Thinking != "" {
			tc.thinkingChars.add(delta.Thinking)
		}

	case bamboosdk.DeltaTextDelta:
		if tc.contentStart.IsZero() {
			tc.contentStart = now
		}
		if tc.contentEnd.IsZero() && !tc.thinkingStart.IsZero() && tc.thinkingEnd.IsZero() {
			tc.thinkingEnd = now
		}
		if delta.Text != "" {
			tc.outputChars.add(delta.Text)
		}

	case bamboosdk.DeltaInputJSON:
		if tc.toolStart.IsZero() {
			tc.toolStart = now
		}
		if tc.contentEnd.IsZero() && !tc.contentStart.IsZero() {
			tc.contentEnd = now
		}
		tc.phase = phaseTool
		if delta.PartialJSON != "" {
			tc.toolChars.add(delta.PartialJSON)
		}
	}
}

func (tc *bambooTimingCollector) result() relaycommon.BambooTimingResult {
	var stats relaycommon.BambooTimingStats

	// 兜底：observe() 入口会设置 startTime，但 eventCh 提前关闭时仍可能为零。
	// 用 lastEventTime 退化回填，避免 TotalDuration 为零被 IsZero() 过滤。
	if tc.startTime.IsZero() && !tc.lastEventTime.IsZero() {
		tc.startTime = tc.lastEventTime
	}

	endTime := tc.stopTime
	if endTime.IsZero() {
		endTime = tc.lastEventTime
	}

	// 兜底：无 delta 事件时 firstByteTime 为零，用 startTime 回填使
	// FirstByteDuration 退化为 0 而非缺失，保证下游消费一致性。
	if tc.firstByteTime.IsZero() && !tc.startTime.IsZero() {
		tc.firstByteTime = tc.startTime
	}

	if !tc.startTime.IsZero() && !endTime.IsZero() {
		stats.TotalDuration = endTime.Sub(tc.startTime)
	}
	if !tc.startTime.IsZero() && !tc.firstByteTime.IsZero() {
		stats.FirstByteDuration = tc.firstByteTime.Sub(tc.startTime)
	}

	if !tc.thinkingStart.IsZero() {
		end := tc.thinkingEnd
		if end.IsZero() {
			if !tc.contentStart.IsZero() {
				end = tc.contentStart
			} else if !tc.toolStart.IsZero() {
				end = tc.toolStart
			} else {
				end = endTime
			}
		}
		if !end.IsZero() {
			stats.ThinkingDuration = end.Sub(tc.thinkingStart)
		}
	}

	if !tc.contentStart.IsZero() {
		end := tc.contentEnd
		if end.IsZero() {
			if !tc.toolStart.IsZero() {
				end = tc.toolStart
			} else {
				end = endTime
			}
		}
		if !end.IsZero() {
			stats.ContentDuration = end.Sub(tc.contentStart)
		}
	}

	if !tc.toolStart.IsZero() && !endTime.IsZero() {
		stats.ToolDuration = endTime.Sub(tc.toolStart)
	}

	// 诊断日志：零值时警告，帮助定位上游事件缺失问题。
	if stats.TotalDuration == 0 {
		logger.LogWarn(nil, fmt.Sprintf(
			"[bamboo-timing] TotalDuration is zero: startTime=%v, stopTime=%v, lastEventTime=%v",
			tc.startTime, tc.stopTime, tc.lastEventTime,
		))
	}
	if stats.FirstByteDuration == 0 {
		logger.LogWarn(nil, fmt.Sprintf(
			"[bamboo-timing] FirstByteDuration is zero: firstByteTime=%v, startTime=%v",
			tc.firstByteTime, tc.startTime,
		))
	}

	var rates relaycommon.BambooTokenRates
	var counts relaycommon.BambooTokenCounts

	counts.ThinkingTokens = tc.thinkingChars.estimateTokens()
	counts.OutputTokens = tc.outputChars.estimateTokens()
	counts.ToolTokens = tc.toolChars.estimateTokens()

	rates.ThinkingTokensPerSec = computeRate(counts.ThinkingTokens, stats.ThinkingDuration, tc.thinkingStart)
	rates.OutputTokensPerSec = computeRate(counts.OutputTokens, stats.ContentDuration, tc.contentStart)
	rates.ToolTokensPerSec = computeRate(counts.ToolTokens, stats.ToolDuration, tc.toolStart)

	return relaycommon.BambooTimingResult{Stats: stats, Rates: rates, Tokens: counts}
}

// minReliableDuration 最小可信耗时阈值。
// 低于此值时，阶段内事件密集到达（channel buffer 导致时间戳几乎相同），
// 计算出的 token/s 严重失真（如 5k+ tok/s），用负号标记不可靠。
const minReliableDuration = time.Millisecond

// computeRate 计算 token/s 速率，含不可靠标记逻辑。
// 阶段已发生（start != zero）但耗时低于 minReliableDuration 时，
// 用 minReliableDuration 作为分母估算参考值并取负标记不可靠。
func computeRate(tokens int64, duration time.Duration, phaseStart time.Time) float64 {
	if tokens == 0 || phaseStart.IsZero() {
		return 0
	}
	if duration < minReliableDuration {
		return -round2(float64(tokens) / minReliableDuration.Seconds())
	}
	return round2(float64(tokens) / duration.Seconds())
}
