package bamboo

import (
	"math"
	"time"
	"unicode"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"

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
	}
}

func (tc *bambooTimingCollector) result() relaycommon.BambooTimingResult {
	var stats relaycommon.BambooTimingStats

	endTime := tc.stopTime
	if endTime.IsZero() {
		endTime = tc.lastEventTime
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

	var rates relaycommon.BambooTokenRates
	if stats.ThinkingDuration > 0 {
		tokens := tc.thinkingChars.estimateTokens()
		rates.ThinkingTokensPerSec = round2(float64(tokens) / stats.ThinkingDuration.Seconds())
	}
	if stats.ContentDuration > 0 {
		tokens := tc.outputChars.estimateTokens()
		rates.OutputTokensPerSec = round2(float64(tokens) / stats.ContentDuration.Seconds())
	}

	return relaycommon.BambooTimingResult{Stats: stats, Rates: rates}
}
