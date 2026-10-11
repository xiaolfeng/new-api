package service

import (
	"testing"
	"time"

	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

func TestAppendUsageActivityTagsFromSuccessfulHostToolsAndImageRecognize(t *testing.T) {
	info := &relaycommon.RelayInfo{
		HostToolPlan: &relaycommon.HostToolPlan{
			Execs: []relaycommon.HostToolExecRecord{
				{Canonical: relaycommon.HostToolCanonicalSearch},
				{Canonical: relaycommon.HostToolCanonicalFetch},
				{Canonical: relaycommon.HostToolCanonicalFetch, ErrorCode: "ssrf_blocked"},
			},
		},
		ImageRecognizePlan: &relaycommon.ImageRecognizePlan{
			Enabled:    true,
			ImageCount: 2,
		},
	}
	other := model.NewLogOther()
	appendUsageActivityTags(info, other)
	snap := other.Snapshot()

	require.Equal(t, []string{"web_search", "web_fetch", "image_recognize"}, snap["usage_tags"])
	assert.Equal(t, true, snap["web_search"])
	assert.Equal(t, 1, snap["web_search_call_count"])
	assert.Equal(t, true, snap["web_fetch"])
	assert.Equal(t, 1, snap["web_fetch_call_count"])
	assert.Equal(t, true, snap["image_recognize"])
	assert.Equal(t, 2, snap["image_recognize_image_count"])
}

func TestAppendUsageActivityTagsUsesBuiltInSearchCount(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ResponsesUsageInfo: &relaycommon.ResponsesUsageInfo{
			BuiltInTools: map[string]*relaycommon.BuildInToolInfo{
				dto.BuildInToolWebSearch: {CallCount: 3},
			},
		},
	}
	other := model.NewLogOther()
	appendUsageActivityTags(info, other)
	snap := other.Snapshot()
	assert.Equal(t, []string{"web_search"}, snap["usage_tags"])
	assert.Equal(t, 3, snap["web_search_call_count"])
	assert.NotContains(t, snap, "web_fetch")
	assert.NotContains(t, snap, "image_recognize")
}

func TestAppendUsageActivityTagsSkipsFailedOrDisabled(t *testing.T) {
	info := &relaycommon.RelayInfo{
		HostToolPlan: &relaycommon.HostToolPlan{
			Execs: []relaycommon.HostToolExecRecord{
				{Canonical: relaycommon.HostToolCanonicalFetch, ErrorCode: "timeout"},
			},
		},
		ImageRecognizePlan: &relaycommon.ImageRecognizePlan{
			Enabled:       false,
			SkippedReason: relaycommon.ImageRecognizeSkipNoLatestUserImage,
		},
	}
	other := model.NewLogOther()
	appendUsageActivityTags(info, other)
	snap := other.Snapshot()
	assert.NotContains(t, snap, "usage_tags")
	assert.NotContains(t, snap, "web_fetch")
	assert.NotContains(t, snap, "image_recognize")
}

func TestAppendClientProfileAdminInfo(t *testing.T) {
	admin := model.NewLogOther()
	appendClientProfileAdminInfo(&relaycommon.RelayInfo{
		ClientProfile:    common.ClientProfileClaudeCode,
		ClientProfileHit: "claude-cli",
	}, admin)
	adminMap := admin.Snapshot()["admin_info"].(map[string]any)
	assert.Equal(t, "claude_code", adminMap["client_profile"])
	assert.Equal(t, "claude-cli", adminMap["client_profile_hit"])
	assert.NotContains(t, adminMap, "return_profile")

	empty := model.NewLogOther()
	appendClientProfileAdminInfo(&relaycommon.RelayInfo{ClientProfile: common.ClientProfileGeneric}, empty)
	emptyMap := empty.Snapshot()["admin_info"].(map[string]any)
	assert.Equal(t, "generic", emptyMap["client_profile"])
	assert.NotContains(t, emptyMap, "client_profile_hit")
	assert.NotContains(t, emptyMap, "return_profile")
}

func TestBuildTokenRecordTiming(t *testing.T) {
	info := &relaycommon.RelayInfo{
		BambooTiming: &relaycommon.BambooTimingResult{
			Stats: relaycommon.BambooTimingStats{
				ThinkingDuration: 1500 * time.Millisecond,
				ContentDuration:  2 * time.Second,
				ToolDuration:     750 * time.Millisecond,
			},
			Rates: relaycommon.BambooTokenRates{
				ThinkingTokensPerSec: 20,
				OutputTokensPerSec:   20,
				ToolTokensPerSec:     20,
			},
			Tokens: relaycommon.BambooTokenCounts{
				ThinkingTokens: 30,
				OutputTokens:   40,
				ToolTokens:     15,
			},
		},
	}

	require.Equal(t, model.TokenRecordTiming{
		ThinkingTokens:     30,
		ThinkingDurationMs: 1500,
		OutputTokens:       40,
		OutputDurationMs:   2000,
		ToolTokens:         15,
		ToolDurationMs:     750,
	}, BuildTokenRecordTiming(info))
	require.Equal(t, model.TokenRecordTiming{}, BuildTokenRecordTiming(nil))

	// Unreliable phases (< 100ms or negative '~' rate) are excluded individually without dropping valid sibling phases
	partial := &relaycommon.RelayInfo{
		BambooTiming: &relaycommon.BambooTimingResult{
			Stats: relaycommon.BambooTimingStats{
				ThinkingDuration: 50 * time.Millisecond,  // < 100ms
				ContentDuration:  100 * time.Millisecond, // exact 100ms boundary, valid
				ToolDuration:     500 * time.Millisecond, // negative rate (~), invalid
			},
			Rates: relaycommon.BambooTokenRates{
				ThinkingTokensPerSec: -200,
				OutputTokensPerSec:   150,
				ToolTokensPerSec:     -30,
			},
			Tokens: relaycommon.BambooTokenCounts{
				ThinkingTokens: 10,
				OutputTokens:   15,
				ToolTokens:     15,
			},
		},
	}
	require.Equal(t, model.TokenRecordTiming{
		OutputTokens:     15,
		OutputDurationMs: 100,
	}, BuildTokenRecordTiming(partial))
}

func TestBuildTokenRecordTimingForConsumeEligibility(t *testing.T) {
	gin.SetMode(gin.TestMode)
	start := time.Unix(1000, 0)

	makeStreamRelay := func(genDuration time.Duration) *relaycommon.RelayInfo {
		dt := relaycommon.NewDeliveryTiming(start)
		dt.RecordContent(start.Add(200 * time.Millisecond))
		dt.Finish(start.Add(200*time.Millisecond+genDuration), "completed")
		return &relaycommon.RelayInfo{
			StartTime:      start,
			IsStream:       true,
			DeliveryTiming: dt,
			BambooTiming: &relaycommon.BambooTimingResult{
				Stats:  relaycommon.BambooTimingStats{ContentDuration: 500 * time.Millisecond},
				Rates:  relaycommon.BambooTokenRates{OutputTokensPerSec: 100},
				Tokens: relaycommon.BambooTokenCounts{OutputTokens: 50},
			},
		}
	}

	confirmedUsage := &dto.Usage{
		CompletionTokens: 80,
		BillingUsage: &dto.BillingUsage{
			Source:      dto.BillingUsageSourceOAIChat,
			Semantic:    dto.BillingUsageSemanticOpenAI,
			OpenAIUsage: &dto.Usage{PromptTokens: 10, CompletionTokens: 80, TotalTokens: 90},
			Estimated:   false,
		},
	}

	// 1. Valid stream sample (generation = 1000ms >= 100ms)
	validTiming := BuildTokenRecordTimingForConsume(nil, makeStreamRelay(time.Second), confirmedUsage, 80, 1)
	assert.EqualValues(t, 1, validTiming.TpsSampleCount)
	assert.EqualValues(t, 80, validTiming.TpsCompletionTokens)
	assert.EqualValues(t, 1000, validTiming.TpsDurationMs)
	assert.EqualValues(t, 50, validTiming.OutputTokens)

	// 2. Boundary 99ms vs 100ms generation duration
	underThreshold := BuildTokenRecordTimingForConsume(nil, makeStreamRelay(99*time.Millisecond), confirmedUsage, 80, 1)
	assert.EqualValues(t, 0, underThreshold.TpsSampleCount)
	atThreshold := BuildTokenRecordTimingForConsume(nil, makeStreamRelay(100*time.Millisecond), confirmedUsage, 80, 1)
	assert.EqualValues(t, 1, atThreshold.TpsSampleCount)
	assert.EqualValues(t, 100, atThreshold.TpsDurationMs)

	// 3. Estimated fallback usage excluded from verified AvgTPS, while valid Bamboo phase estimation remains in phase trend
	estimatedUsage := &dto.Usage{
		CompletionTokens: 80,
		BillingUsage: &dto.BillingUsage{
			Source:      dto.BillingUsageSourceOAIChat,
			Semantic:    dto.BillingUsageSemanticOpenAI,
			OpenAIUsage: &dto.Usage{PromptTokens: 10, CompletionTokens: 80, TotalTokens: 90},
			Estimated:   true,
		},
	}
	estTiming := BuildTokenRecordTimingForConsume(nil, makeStreamRelay(time.Second), estimatedUsage, 80, 1)
	assert.EqualValues(t, 0, estTiming.TpsSampleCount, "estimated usage must not enter verified AvgTPS")
	assert.EqualValues(t, 50, estTiming.OutputTokens, "valid phase estimation still enters phase trend")

	// 4. Failed relay context excluded from verified AvgTPS
	rec := httptest.NewRecorder()
	errCtx, _ := gin.CreateTestContext(rec)
	common.SetContextKey(errCtx, constant.ContextKeyRelayError, true)
	failedTiming := BuildTokenRecordTimingForConsume(errCtx, makeStreamRelay(time.Second), confirmedUsage, 80, 1)
	assert.EqualValues(t, 0, failedTiming.TpsSampleCount)
}

func TestAppendBambooTimingAlignsFrt(t *testing.T) {
	info := &relaycommon.RelayInfo{
		BambooTiming: &relaycommon.BambooTimingResult{
			Stats: relaycommon.BambooTimingStats{
				TotalDuration:     8795 * time.Millisecond,
				FirstByteDuration: 8756 * time.Millisecond,
				ThinkingDuration:  500 * time.Millisecond,
				ContentDuration:   500 * time.Millisecond,
			},
			Rates: relaycommon.BambooTokenRates{
				ThinkingTokensPerSec: -96.6,
				OutputTokensPerSec:   45.2,
			},
			Tokens: relaycommon.BambooTokenCounts{
				ThinkingTokens: 100,
				OutputTokens:   200,
			},
		},
	}

	other := model.NewLogOther()
	// 模拟外部初始 frt 为 0 或旧值
	other.SetPublic("frt", float64(0))

	appendBambooTiming(info, other)
	snap := other.Snapshot()

	// 验证 frt 被 FirstByteDuration 对齐回填
	assert.Equal(t, float64(8756), snap["frt"])

	timing, ok := snap["bamboo_timing"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, int64(8795), timing["total_ms"])
	assert.Equal(t, int64(8756), timing["ttft_ms"])
	assert.Equal(t, -96.6, timing["thinking_tps"])
}

func TestDeliveryTimingLogUsesWrittenContentNotUpstreamTTFT(t *testing.T) {
	start := time.Unix(100, 0)
	info := &relaycommon.RelayInfo{
		StartTime: start, IsStream: true,
		FirstResponseTime: start.Add(9 * time.Second),
		DeliveryTiming:    relaycommon.NewDeliveryTiming(start),
		BambooTiming:      &relaycommon.BambooTimingResult{Stats: relaycommon.BambooTimingStats{TotalDuration: 7109 * time.Millisecond, FirstByteDuration: 7079 * time.Millisecond}},
	}
	info.BambooTimingHops = []relaycommon.BambooTimingResult{*info.BambooTiming, *info.BambooTiming}
	info.DeliveryTiming.RecordContent(start.Add(8 * time.Second))
	info.DeliveryTiming.Finish(start.Add(9*time.Second), "completed")
	other := model.NewLogOther()
	other.SetPublic("frt", float64(9000))
	appendBambooTiming(info, other)
	assert.Equal(t, float64(9000), other.Snapshot()["frt"], "上游明细不能覆盖交付首字")
	appendDeliveryTiming(nil, info, other)
	snap := other.Snapshot()
	assert.Equal(t, float64(8000), snap["frt"])
	result, ok := snap["delivery_timing"].(*relaycommon.DeliveryTimingResult)
	require.True(t, ok)
	assert.Equal(t, int64(9000), result.TotalMs)
	assert.Equal(t, int64(8000), *result.TTFTMs)
	hops, ok := snap["bamboo_timing_hops"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, hops, 2)
	assert.Equal(t, 1, hops[0]["hop_index"])
	assert.Equal(t, 2, hops[1]["hop_index"])
	tps, valid := calculateTextLogTPS(info, 100, 7)
	require.True(t, valid)
	assert.Equal(t, float64(100), tps)
}

func TestDeliveryTimingLogMissingAndZeroFirstContent(t *testing.T) {
	start := time.Unix(100, 0)
	for _, hasContent := range []bool{false, true} {
		info := &relaycommon.RelayInfo{StartTime: start, IsStream: true, DeliveryTiming: relaycommon.NewDeliveryTiming(start)}
		if hasContent {
			info.DeliveryTiming.RecordContent(start)
		}
		info.DeliveryTiming.Finish(start.Add(time.Second), "completed")
		other := model.NewLogOther()
		appendDeliveryTiming(nil, info, other)
		data, err := common.Marshal(other.Snapshot())
		require.NoError(t, err)
		var decoded map[string]any
		require.NoError(t, common.Unmarshal(data, &decoded))
		result := decoded["delivery_timing"].(map[string]any)
		assert.Equal(t, float64(0), decoded["frt"])
		if hasContent {
			assert.Equal(t, float64(0), result["ttft_ms"])
		} else {
			assert.Nil(t, result["ttft_ms"])
		}
	}
}

func TestGenerateTextOtherInfoWithZeroFirstResponseTimeDoesNotEmitNegativeFRT(t *testing.T) {
	start := time.Unix(100, 0)
	info := &relaycommon.RelayInfo{
		StartTime:   start,
		IsStream:    true,
		ChannelMeta: &relaycommon.ChannelMeta{},
		// FirstResponseTime is zero (time.Time{})
	}
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	other := GenerateTextOtherInfo(ctx, info, 1, 1, 1, 0, 0, 0, 1)
	snap := other.Snapshot()
	assert.Equal(t, float64(0), snap["frt"], "frt must not be negative year-1 offset")
}

func TestCalculateTPSWithNegativeOrZeroFRTDoesNotCorruptGenerationTime(t *testing.T) {
	// With 10 tokens and 5 seconds, but negative frt (due to zero FirstResponseTime)
	tps, valid := CalculateTPS(10, 5, -63875596800000, true)
	require.True(t, valid)
	assert.Equal(t, float64(2), tps, "negative frt should fall back to useTimeSeconds rather than producing ~0 tps")

	// calculateTextLogTPS with zero FirstResponseTime
	start := time.Unix(100, 0)
	info := &relaycommon.RelayInfo{
		StartTime: start,
		IsStream:  true,
		// FirstResponseTime is zero
	}
	tps2, valid2 := calculateTextLogTPS(info, 10, 5)
	require.True(t, valid2)
	assert.Equal(t, float64(2), tps2)
}

func TestGenerateTextOtherInfoWithValidFRTAndNilChannelMeta(t *testing.T) {
	start := time.Unix(100, 0)
	info := &relaycommon.RelayInfo{
		StartTime:         start,
		FirstResponseTime: start.Add(1250 * time.Millisecond),
		IsStream:          true,
		ChannelMeta:       nil, // verify no nil dereference
	}
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	other := GenerateTextOtherInfo(ctx, info, 1, 1, 1, 0, 0, 0, 1)
	snap := other.Snapshot()
	assert.Equal(t, float64(1250), snap["frt"])
}

func TestCalculateTPSNormalStreamCalculation(t *testing.T) {
	// 100 tokens, 5s total, 1000ms (1s) FRT -> generation = 4s -> TPS = 25
	tps, valid := CalculateTPS(100, 5, 1000, true)
	require.True(t, valid)
	assert.Equal(t, float64(25), tps)
}
