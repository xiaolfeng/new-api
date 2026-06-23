package common

import "time"

// BambooTimingStats 分阶段流式耗时统计。
//
// 所有 Duration 字段在对应阶段未发生时为零值。
type BambooTimingStats struct {
	TotalDuration      time.Duration
	FirstByteDuration  time.Duration
	ThinkingDuration   time.Duration
	ContentDuration    time.Duration
	ToolDuration       time.Duration
}

// BambooTokenRates 分阶段 Token 生成速率（.2f 精度）。
//
// Token 数按 CJK ≈ 1 char/token、Latin ≈ 4 chars/token 估算。
type BambooTokenRates struct {
	ThinkingTokensPerSec float64
	OutputTokensPerSec   float64
}

// BambooTimingResult 完整计时结果，供 RelayInfo 存储和日志层消费。
type BambooTimingResult struct {
	Stats BambooTimingStats
	Rates BambooTokenRates
}

// IsZero 判断是否有有效的计时数据（总耗时大于零）。
func (r *BambooTimingResult) IsZero() bool {
	return r == nil || r.Stats.TotalDuration == 0
}
