package model

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TokenRecord struct {
	Id                      int    `json:"id"`
	BucketStartAt           int64  `json:"bucket_start_at" gorm:"bigint;uniqueIndex:idx_token_record_bucket_model,priority:1;index:idx_token_record_bucket_start,priority:1"`
	BucketEndAt             int64  `json:"bucket_end_at" gorm:"bigint"`
	ModelName               string `json:"model_name" gorm:"size:255;default:'';uniqueIndex:idx_token_record_bucket_model,priority:2;index:idx_token_record_model_name"`
	RequestCount            int64  `json:"request_count" gorm:"default:0"`
	PromptTokens            int64  `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens        int64  `json:"completion_tokens" gorm:"default:0"`
	TotalTokens             int64  `json:"total_tokens" gorm:"default:0"`
	TotalUseTime            int64  `json:"total_use_time" gorm:"default:0"`
	TpsSampleCount          int64  `json:"tps_sample_count" gorm:"column:tps_sample_count;default:0"`
	TpsCompletionTokens     int64  `json:"tps_completion_tokens" gorm:"column:tps_completion_tokens;default:0"`
	TpsDurationMs           int64  `json:"tps_duration_ms" gorm:"column:tps_duration_ms;default:0"`
	ThinkingSampleCount     int64  `json:"thinking_sample_count" gorm:"column:thinking_sample_count;default:0"`
	ThinkingTokens          int64  `json:"thinking_tokens" gorm:"default:0"`
	ThinkingDurationMs      int64  `json:"thinking_duration_ms" gorm:"default:0"`
	ThinkingValidTokens     int64  `json:"thinking_valid_tokens" gorm:"column:thinking_valid_tokens;default:0"`
	ThinkingValidDurationMs int64  `json:"thinking_valid_duration_ms" gorm:"column:thinking_valid_duration_ms;default:0"`
	OutputSampleCount       int64  `json:"output_sample_count" gorm:"column:output_sample_count;default:0"`
	OutputTokens            int64  `json:"output_tokens" gorm:"default:0"`
	OutputDurationMs        int64  `json:"output_duration_ms" gorm:"default:0"`
	OutputValidTokens       int64  `json:"output_valid_tokens" gorm:"column:output_valid_tokens;default:0"`
	OutputValidDurationMs   int64  `json:"output_valid_duration_ms" gorm:"column:output_valid_duration_ms;default:0"`
	ToolSampleCount         int64  `json:"tool_sample_count" gorm:"column:tool_sample_count;default:0"`
	ToolTokens              int64  `json:"tool_tokens" gorm:"default:0"`
	ToolDurationMs          int64  `json:"tool_duration_ms" gorm:"default:0"`
	ToolValidTokens         int64  `json:"tool_valid_tokens" gorm:"column:tool_valid_tokens;default:0"`
	ToolValidDurationMs     int64  `json:"tool_valid_duration_ms" gorm:"column:tool_valid_duration_ms;default:0"`
	FailedCount             int64  `json:"failed_count" gorm:"default:0"`
	FailedDetail            string `json:"failed_detail" gorm:"type:text"` // JSON: {"429":2,"500":1}
	FirstUsedAt             int64  `json:"first_used_at" gorm:"bigint"`
	LastUsedAt              int64  `json:"last_used_at" gorm:"bigint"`
	CreatedAt               int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt               int64  `json:"updated_at" gorm:"bigint"`
}

func (TokenRecord) TableName() string {
	return "token_record"
}

type TokenRecordHourMeta struct {
	BucketStartAt int64  `json:"bucket_start_at"`
	BucketEndAt   int64  `json:"bucket_end_at"`
	Label         string `json:"label"`
	IsCurrent     bool   `json:"is_current"`
}

type TokenRecordHourCell struct {
	BucketStartAt       int64            `json:"bucket_start_at"`
	BucketEndAt         int64            `json:"bucket_end_at"`
	RequestCount        int64            `json:"request_count"`
	PromptTokens        int64            `json:"prompt_tokens"`
	CompletionTokens    int64            `json:"completion_tokens"`
	TotalTokens         int64            `json:"total_tokens"`
	TotalUseTime        int64            `json:"total_use_time"`
	TpsSampleCount      int64            `json:"tps_sample_count"`
	TpsCompletionTokens int64            `json:"tps_completion_tokens"`
	TpsDurationMs       int64            `json:"tps_duration_ms"`
	AvgTPS              *float64         `json:"avg_tps"`
	ThinkingSampleCount int64            `json:"thinking_sample_count"`
	ThinkingTokens      int64            `json:"thinking_tokens"`
	ThinkingDurationMs  int64            `json:"thinking_duration_ms"`
	AvgThinkingTPS      *float64         `json:"avg_thinking_tps"`
	OutputSampleCount   int64            `json:"output_sample_count"`
	OutputTokens        int64            `json:"output_tokens"`
	OutputDurationMs    int64            `json:"output_duration_ms"`
	AvgOutputTPS        *float64         `json:"avg_output_tps"`
	ToolSampleCount     int64            `json:"tool_sample_count"`
	ToolTokens          int64            `json:"tool_tokens"`
	ToolDurationMs      int64            `json:"tool_duration_ms"`
	AvgToolTPS          *float64         `json:"avg_tool_tps"`
	FailedCount         int64            `json:"failed_count"`
	FailedDetail        map[string]int64 `json:"failed_detail"`
	IsCurrent           bool             `json:"is_current"`
}

type TokenRecordSummary struct {
	RequestCount        int64            `json:"request_count"`
	PromptTokens        int64            `json:"prompt_tokens"`
	CompletionTokens    int64            `json:"completion_tokens"`
	TotalTokens         int64            `json:"total_tokens"`
	TotalUseTime        int64            `json:"total_use_time"`
	TpsSampleCount      int64            `json:"tps_sample_count"`
	TpsCompletionTokens int64            `json:"tps_completion_tokens"`
	TpsDurationMs       int64            `json:"tps_duration_ms"`
	AvgTPS              *float64         `json:"avg_tps"`
	ThinkingSampleCount int64            `json:"thinking_sample_count"`
	ThinkingTokens      int64            `json:"thinking_tokens"`
	ThinkingDurationMs  int64            `json:"thinking_duration_ms"`
	AvgThinkingTPS      *float64         `json:"avg_thinking_tps"`
	OutputSampleCount   int64            `json:"output_sample_count"`
	OutputTokens        int64            `json:"output_tokens"`
	OutputDurationMs    int64            `json:"output_duration_ms"`
	AvgOutputTPS        *float64         `json:"avg_output_tps"`
	ToolSampleCount     int64            `json:"tool_sample_count"`
	ToolTokens          int64            `json:"tool_tokens"`
	ToolDurationMs      int64            `json:"tool_duration_ms"`
	AvgToolTPS          *float64         `json:"avg_tool_tps"`
	FailedCount         int64            `json:"failed_count"`
	FailedRate          float64          `json:"failed_rate"`
	FailedDetail        map[string]int64 `json:"failed_detail"`
}

type TokenRecordRecentItem struct {
	ModelName string                `json:"model_name"`
	Summary   TokenRecordSummary    `json:"summary"`
	Cells     []TokenRecordHourCell `json:"cells"`
}

type TokenRecordRecentSnapshot struct {
	Hours   []TokenRecordHourMeta      `json:"hours"`
	Items   []TokenRecordRecentItem    `json:"items"`
	Summary *TokenRecordOverallSummary `json:"summary"`
}

// TokenRecordOverallSummary 全局聚合摘要（最近24小时所有模型合计）
type TokenRecordOverallSummary struct {
	TotalRequestCount int64 `json:"total_request_count"` // 总成功请求次数
	TotalPromptTokens int64 `json:"total_prompt_tokens"` // 总输入Token
	TotalOutputTokens int64 `json:"total_output_tokens"` // 总输出Token
	ActiveModelCount  int   `json:"active_model_count"`  // 活跃模型数
}

// TokenRecordDailyItem represents daily aggregated data
type TokenRecordDailyItem struct {
	Date             string `json:"date"` // "YYYY-MM-DD"
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
	TotalTokens      int64  `json:"total_tokens"`
}

func normalizeTokenRecordModelName(modelName string) string {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return "unknown"
	}
	return modelName
}

func getTokenRecordHourBucket(createdAt int64) (int64, int64) {
	if createdAt <= 0 {
		createdAt = common.GetTimestamp()
	}
	bucketStartAt := createdAt - (createdAt % 3600)
	return bucketStartAt, bucketStartAt + 3599
}

const minReliablePhaseDurationMs int64 = 100

func calcTokenRecordVerifiedTPS(sampleCount int64, tokens int64, durationMs int64, minDurationMs int64) *float64 {
	if sampleCount <= 0 || tokens <= 0 || durationMs <= 0 || durationMs < minDurationMs {
		return nil
	}
	val := math.Round((float64(tokens)*1000.0/float64(durationMs))*100) / 100
	if math.IsNaN(val) || math.IsInf(val, 0) || val <= 0 {
		return nil
	}
	return &val
}

type TokenRecordTiming struct {
	TpsSampleCount      int64
	TpsCompletionTokens int64
	TpsDurationMs       int64
	ThinkingTokens      int64
	ThinkingDurationMs  int64
	OutputTokens        int64
	OutputDurationMs    int64
	ToolTokens          int64
	ToolDurationMs      int64
}

func sanitizeTokenRecordTiming(timing TokenRecordTiming) TokenRecordTiming {
	if timing.TpsSampleCount <= 0 || timing.TpsCompletionTokens <= 0 || timing.TpsDurationMs <= 0 {
		timing.TpsSampleCount = 0
		timing.TpsCompletionTokens = 0
		timing.TpsDurationMs = 0
	}
	if timing.ThinkingTokens <= 0 || timing.ThinkingDurationMs < minReliablePhaseDurationMs {
		timing.ThinkingTokens = 0
		timing.ThinkingDurationMs = 0
	}
	if timing.OutputTokens <= 0 || timing.OutputDurationMs < minReliablePhaseDurationMs {
		timing.OutputTokens = 0
		timing.OutputDurationMs = 0
	}
	if timing.ToolTokens <= 0 || timing.ToolDurationMs < minReliablePhaseDurationMs {
		timing.ToolTokens = 0
		timing.ToolDurationMs = 0
	}
	return timing
}

func buildTokenRecordIncrementExpr(column string, delta interface{}) clause.Expr {
	return gorm.Expr("? + ?", clause.Column{Table: "token_record", Name: column}, delta)
}

func RecordTokenRecord(modelName string, promptTokens int, completionTokens int, useTimeSeconds int, timing TokenRecordTiming, createdAt int64) error {
	if LOG_DB == nil {
		return errors.New("log db is not initialized")
	}

	modelName = normalizeTokenRecordModelName(modelName)
	timing = sanitizeTokenRecordTiming(timing)
	bucketStartAt, bucketEndAt := getTokenRecordHourBucket(createdAt)
	inputTokens := int64(promptTokens)
	outputTokens := int64(completionTokens)
	totalUseTime := int64(useTimeSeconds)
	if totalUseTime < 0 {
		totalUseTime = 0
	}

	var thinkingSampleCount int64
	if timing.ThinkingTokens > 0 && timing.ThinkingDurationMs >= minReliablePhaseDurationMs {
		thinkingSampleCount = 1
	}
	var outputSampleCount int64
	if timing.OutputTokens > 0 && timing.OutputDurationMs >= minReliablePhaseDurationMs {
		outputSampleCount = 1
	}
	var toolSampleCount int64
	if timing.ToolTokens > 0 && timing.ToolDurationMs >= minReliablePhaseDurationMs {
		toolSampleCount = 1
	}

	record := TokenRecord{
		BucketStartAt:    bucketStartAt,
		BucketEndAt:      bucketEndAt,
		ModelName:        modelName,
		RequestCount:     1,
		PromptTokens:     inputTokens,
		CompletionTokens: outputTokens,
		// model-log 看板中的 total_tokens 口径为“输出 Token”
		TotalTokens:             outputTokens,
		TotalUseTime:            totalUseTime,
		TpsSampleCount:          timing.TpsSampleCount,
		TpsCompletionTokens:     timing.TpsCompletionTokens,
		TpsDurationMs:           timing.TpsDurationMs,
		ThinkingSampleCount:     thinkingSampleCount,
		ThinkingTokens:          timing.ThinkingTokens,
		ThinkingDurationMs:      timing.ThinkingDurationMs,
		ThinkingValidTokens:     timing.ThinkingTokens,
		ThinkingValidDurationMs: timing.ThinkingDurationMs,
		OutputSampleCount:       outputSampleCount,
		OutputTokens:            timing.OutputTokens,
		OutputDurationMs:        timing.OutputDurationMs,
		OutputValidTokens:       timing.OutputTokens,
		OutputValidDurationMs:   timing.OutputDurationMs,
		ToolSampleCount:         toolSampleCount,
		ToolTokens:              timing.ToolTokens,
		ToolDurationMs:          timing.ToolDurationMs,
		ToolValidTokens:         timing.ToolTokens,
		ToolValidDurationMs:     timing.ToolDurationMs,
		FirstUsedAt:             createdAt,
		LastUsedAt:              createdAt,
		CreatedAt:               createdAt,
		UpdatedAt:               createdAt,
	}

	return LOG_DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "bucket_start_at"},
			{Name: "model_name"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"bucket_end_at":              bucketEndAt,
			"request_count":              buildTokenRecordIncrementExpr("request_count", 1),
			"prompt_tokens":              buildTokenRecordIncrementExpr("prompt_tokens", inputTokens),
			"completion_tokens":          buildTokenRecordIncrementExpr("completion_tokens", outputTokens),
			"total_tokens":               buildTokenRecordIncrementExpr("total_tokens", outputTokens),
			"total_use_time":             buildTokenRecordIncrementExpr("total_use_time", totalUseTime),
			"tps_sample_count":           buildTokenRecordIncrementExpr("tps_sample_count", timing.TpsSampleCount),
			"tps_completion_tokens":      buildTokenRecordIncrementExpr("tps_completion_tokens", timing.TpsCompletionTokens),
			"tps_duration_ms":            buildTokenRecordIncrementExpr("tps_duration_ms", timing.TpsDurationMs),
			"thinking_sample_count":      buildTokenRecordIncrementExpr("thinking_sample_count", thinkingSampleCount),
			"thinking_tokens":            buildTokenRecordIncrementExpr("thinking_tokens", timing.ThinkingTokens),
			"thinking_duration_ms":       buildTokenRecordIncrementExpr("thinking_duration_ms", timing.ThinkingDurationMs),
			"thinking_valid_tokens":      buildTokenRecordIncrementExpr("thinking_valid_tokens", timing.ThinkingTokens),
			"thinking_valid_duration_ms": buildTokenRecordIncrementExpr("thinking_valid_duration_ms", timing.ThinkingDurationMs),
			"output_sample_count":        buildTokenRecordIncrementExpr("output_sample_count", outputSampleCount),
			"output_tokens":              buildTokenRecordIncrementExpr("output_tokens", timing.OutputTokens),
			"output_duration_ms":         buildTokenRecordIncrementExpr("output_duration_ms", timing.OutputDurationMs),
			"output_valid_tokens":        buildTokenRecordIncrementExpr("output_valid_tokens", timing.OutputTokens),
			"output_valid_duration_ms":   buildTokenRecordIncrementExpr("output_valid_duration_ms", timing.OutputDurationMs),
			"tool_sample_count":          buildTokenRecordIncrementExpr("tool_sample_count", toolSampleCount),
			"tool_tokens":                buildTokenRecordIncrementExpr("tool_tokens", timing.ToolTokens),
			"tool_duration_ms":           buildTokenRecordIncrementExpr("tool_duration_ms", timing.ToolDurationMs),
			"tool_valid_tokens":          buildTokenRecordIncrementExpr("tool_valid_tokens", timing.ToolTokens),
			"tool_valid_duration_ms":     buildTokenRecordIncrementExpr("tool_valid_duration_ms", timing.ToolDurationMs),
			"last_used_at":               createdAt,
			"updated_at":                 createdAt,
		}),
	}).Create(&record).Error
}

func mergeFailedDetail(existing string, statusCode string) string {
	detail := make(map[string]int64)
	if existing != "" {
		_ = common.UnmarshalJsonStr(existing, &detail)
	}
	detail[statusCode]++
	merged, err := common.Marshal(detail)
	if err != nil {
		return existing
	}
	return string(merged)
}

// recordFailedTokenRecordMaxAttempts 限制乐观重试次数，避免持续竞争时无限循环。
const recordFailedTokenRecordMaxAttempts = 4

func RecordFailedTokenRecord(modelName string, statusCode int, createdAt int64) {
	if LOG_DB == nil {
		return
	}

	modelName = normalizeTokenRecordModelName(modelName)
	bucketStartAt, bucketEndAt := getTokenRecordHourBucket(createdAt)
	statusCodeStr := fmt.Sprintf("%d", statusCode)

	for attempt := 0; attempt < recordFailedTokenRecordMaxAttempts; attempt++ {
		var existing TokenRecord
		err := LOG_DB.Where("bucket_start_at = ? AND model_name = ?", bucketStartAt, modelName).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			failedDetail, _ := common.Marshal(map[string]int64{statusCodeStr: 1})
			record := TokenRecord{
				BucketStartAt: bucketStartAt,
				BucketEndAt:   bucketEndAt,
				ModelName:     modelName,
				FailedCount:   1,
				FailedDetail:  string(failedDetail),
				FirstUsedAt:   createdAt,
				LastUsedAt:    createdAt,
				CreatedAt:     createdAt,
				UpdatedAt:     createdAt,
			}
			// 并发首建碰撞时 DoNothing 保证不报错；RowsAffected==0 说明对手已插入，走重试进入更新路径。
			result := LOG_DB.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "bucket_start_at"}, {Name: "model_name"}},
				DoNothing: true,
			}).Create(&record)
			if result.Error != nil {
				common.SysError("failed to record failed token record: " + result.Error.Error())
				return
			}
			if result.RowsAffected == 1 {
				return
			}
			continue
		}
		if err != nil {
			common.SysError("failed to load token record for failure stats: " + err.Error())
			return
		}

		// 乐观锁：仅当 failed_count 仍等于读取快照时才提交增量，
		// 避免并发失败请求相互覆盖导致丢计数。
		mergedDetail := mergeFailedDetail(existing.FailedDetail, statusCodeStr)
		updateResult := LOG_DB.Model(&TokenRecord{}).
			Where("bucket_start_at = ? AND model_name = ? AND failed_count = ?", bucketStartAt, modelName, existing.FailedCount).
			Updates(map[string]interface{}{
				"failed_count":  gorm.Expr("failed_count + ?", 1),
				"failed_detail": mergedDetail,
				"updated_at":    createdAt,
			})
		if updateResult.Error != nil {
			common.SysError("failed to record failed token record: " + updateResult.Error.Error())
			return
		}
		if updateResult.RowsAffected == 1 {
			return
		}
		// CAS 未命中：并发写已变更快照，重读后重试。
	}
	common.SysError("failed to record failed token record: retry budget exhausted")
}

func buildTokenRecordHours(rangeStartAt int64, currentBucketStartAt int64, hours int64) []TokenRecordHourMeta {
	if hours <= 0 {
		hours = 24
	}
	hoursList := make([]TokenRecordHourMeta, 0, hours)
	for i := int64(0); i < hours; i++ {
		bucketStartAt := rangeStartAt + i*3600
		bucketEndAt := bucketStartAt + 3599
		hoursList = append(hoursList, TokenRecordHourMeta{
			BucketStartAt: bucketStartAt,
			BucketEndAt:   bucketEndAt,
			Label:         time.Unix(bucketStartAt, 0).Format("15:00"),
			IsCurrent:     bucketStartAt == currentBucketStartAt,
		})
	}
	return hoursList
}

func buildEmptyTokenRecordCells(hours []TokenRecordHourMeta) []TokenRecordHourCell {
	cells := make([]TokenRecordHourCell, 0, len(hours))
	for _, hour := range hours {
		cells = append(cells, TokenRecordHourCell{
			BucketStartAt: hour.BucketStartAt,
			BucketEndAt:   hour.BucketEndAt,
			IsCurrent:     hour.IsCurrent,
		})
	}
	return cells
}

func parseFailedDetail(raw string) map[string]int64 {
	detail := make(map[string]int64)
	if raw != "" {
		_ = common.UnmarshalJsonStr(raw, &detail)
	}
	return detail
}

func mergeFailedDetailMaps(a, b map[string]int64) map[string]int64 {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	result := make(map[string]int64, len(a))
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] += v
	}
	return result
}

func calcFailedRate(failedCount, requestCount int64) float64 {
	total := failedCount + requestCount
	if total <= 0 {
		return 0
	}
	return math.Round((float64(failedCount)/float64(total))*10000) / 100
}

func GetRecentTokenRecordSnapshot(currentTimestamp int64, hours int64) (TokenRecordRecentSnapshot, error) {
	if LOG_DB == nil {
		return TokenRecordRecentSnapshot{}, errors.New("log db is not initialized")
	}

	if hours <= 0 {
		hours = 24
	}

	currentBucketStartAt, _ := getTokenRecordHourBucket(currentTimestamp)
	rangeStartAt := currentBucketStartAt - (hours-1)*3600
	hoursList := buildTokenRecordHours(rangeStartAt, currentBucketStartAt, hours)

	var records []TokenRecord
	err := LOG_DB.Model(&TokenRecord{}).
		Where("bucket_start_at >= ? AND bucket_start_at <= ?", rangeStartAt, currentBucketStartAt).
		Order("bucket_start_at ASC, total_tokens DESC, model_name ASC").
		Find(&records).Error
	if err != nil {
		common.SysError("failed to query token records: " + err.Error())
		return TokenRecordRecentSnapshot{}, errors.New("查询模型日志失败")
	}

	hourIndexMap := make(map[int64]int, len(hoursList))
	for i, hour := range hoursList {
		hourIndexMap[hour.BucketStartAt] = i
	}

	type validPhaseTotals struct {
		thinkingTokens     int64
		thinkingDurationMs int64
		outputTokens       int64
		outputDurationMs   int64
		toolTokens         int64
		toolDurationMs     int64
	}
	itemsMap := make(map[string]*TokenRecordRecentItem)
	validPhaseMap := make(map[string]*validPhaseTotals)
	cellValidPhaseMap := make(map[string][]validPhaseTotals)
	for _, record := range records {
		hourIndex, ok := hourIndexMap[record.BucketStartAt]
		if !ok {
			continue
		}

		item, ok := itemsMap[record.ModelName]
		if !ok {
			item = &TokenRecordRecentItem{
				ModelName: record.ModelName,
				Cells:     buildEmptyTokenRecordCells(hoursList),
			}
			itemsMap[record.ModelName] = item
			validPhaseMap[record.ModelName] = &validPhaseTotals{}
			cellValidPhaseMap[record.ModelName] = make([]validPhaseTotals, len(hoursList))
		}
		validSums := validPhaseMap[record.ModelName]
		cellValidSums := &cellValidPhaseMap[record.ModelName][hourIndex]

		cell := &item.Cells[hourIndex]
		cell.BucketStartAt = record.BucketStartAt
		cell.BucketEndAt = hoursList[hourIndex].BucketEndAt
		cell.IsCurrent = hoursList[hourIndex].IsCurrent
		cell.RequestCount += record.RequestCount
		cell.PromptTokens += record.PromptTokens
		cell.CompletionTokens += record.CompletionTokens
		cell.TotalTokens += record.TotalTokens
		cell.TotalUseTime += record.TotalUseTime
		cell.TpsSampleCount += record.TpsSampleCount
		cell.TpsCompletionTokens += record.TpsCompletionTokens
		cell.TpsDurationMs += record.TpsDurationMs
		cell.AvgTPS = calcTokenRecordVerifiedTPS(cell.TpsSampleCount, cell.TpsCompletionTokens, cell.TpsDurationMs, 1)
		cell.ThinkingSampleCount += record.ThinkingSampleCount
		cell.ThinkingTokens += record.ThinkingTokens
		cell.ThinkingDurationMs += record.ThinkingDurationMs
		cellValidSums.thinkingTokens += record.ThinkingValidTokens
		cellValidSums.thinkingDurationMs += record.ThinkingValidDurationMs
		cell.AvgThinkingTPS = calcTokenRecordVerifiedTPS(cell.ThinkingSampleCount, cellValidSums.thinkingTokens, cellValidSums.thinkingDurationMs, minReliablePhaseDurationMs)
		cell.OutputSampleCount += record.OutputSampleCount
		cell.OutputTokens += record.OutputTokens
		cell.OutputDurationMs += record.OutputDurationMs
		cellValidSums.outputTokens += record.OutputValidTokens
		cellValidSums.outputDurationMs += record.OutputValidDurationMs
		cell.AvgOutputTPS = calcTokenRecordVerifiedTPS(cell.OutputSampleCount, cellValidSums.outputTokens, cellValidSums.outputDurationMs, minReliablePhaseDurationMs)
		cell.ToolSampleCount += record.ToolSampleCount
		cell.ToolTokens += record.ToolTokens
		cell.ToolDurationMs += record.ToolDurationMs
		cellValidSums.toolTokens += record.ToolValidTokens
		cellValidSums.toolDurationMs += record.ToolValidDurationMs
		cell.AvgToolTPS = calcTokenRecordVerifiedTPS(cell.ToolSampleCount, cellValidSums.toolTokens, cellValidSums.toolDurationMs, minReliablePhaseDurationMs)
		cell.FailedCount += record.FailedCount
		cell.FailedDetail = mergeFailedDetailMaps(cell.FailedDetail, parseFailedDetail(record.FailedDetail))

		item.Summary.RequestCount += record.RequestCount
		item.Summary.PromptTokens += record.PromptTokens
		item.Summary.CompletionTokens += record.CompletionTokens
		item.Summary.TotalTokens += record.TotalTokens
		item.Summary.TotalUseTime += record.TotalUseTime
		item.Summary.TpsSampleCount += record.TpsSampleCount
		item.Summary.TpsCompletionTokens += record.TpsCompletionTokens
		item.Summary.TpsDurationMs += record.TpsDurationMs
		item.Summary.ThinkingSampleCount += record.ThinkingSampleCount
		item.Summary.ThinkingTokens += record.ThinkingTokens
		item.Summary.ThinkingDurationMs += record.ThinkingDurationMs
		validSums.thinkingTokens += record.ThinkingValidTokens
		validSums.thinkingDurationMs += record.ThinkingValidDurationMs
		item.Summary.OutputSampleCount += record.OutputSampleCount
		item.Summary.OutputTokens += record.OutputTokens
		item.Summary.OutputDurationMs += record.OutputDurationMs
		validSums.outputTokens += record.OutputValidTokens
		validSums.outputDurationMs += record.OutputValidDurationMs
		item.Summary.ToolSampleCount += record.ToolSampleCount
		item.Summary.ToolTokens += record.ToolTokens
		item.Summary.ToolDurationMs += record.ToolDurationMs
		validSums.toolTokens += record.ToolValidTokens
		validSums.toolDurationMs += record.ToolValidDurationMs
		item.Summary.FailedCount += record.FailedCount
		item.Summary.FailedDetail = mergeFailedDetailMaps(item.Summary.FailedDetail, parseFailedDetail(record.FailedDetail))
	}

	items := make([]TokenRecordRecentItem, 0, len(itemsMap))
	for _, item := range itemsMap {
		validSums := validPhaseMap[item.ModelName]
		item.Summary.AvgTPS = calcTokenRecordVerifiedTPS(item.Summary.TpsSampleCount, item.Summary.TpsCompletionTokens, item.Summary.TpsDurationMs, 1)
		item.Summary.AvgThinkingTPS = calcTokenRecordVerifiedTPS(item.Summary.ThinkingSampleCount, validSums.thinkingTokens, validSums.thinkingDurationMs, minReliablePhaseDurationMs)
		item.Summary.AvgOutputTPS = calcTokenRecordVerifiedTPS(item.Summary.OutputSampleCount, validSums.outputTokens, validSums.outputDurationMs, minReliablePhaseDurationMs)
		item.Summary.AvgToolTPS = calcTokenRecordVerifiedTPS(item.Summary.ToolSampleCount, validSums.toolTokens, validSums.toolDurationMs, minReliablePhaseDurationMs)
		item.Summary.FailedRate = calcFailedRate(item.Summary.FailedCount, item.Summary.RequestCount)
		items = append(items, *item)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Summary.TotalTokens == items[j].Summary.TotalTokens {
			return items[i].ModelName < items[j].ModelName
		}
		return items[i].Summary.TotalTokens > items[j].Summary.TotalTokens
	})

	// 计算全局聚合摘要
	var overallSummary TokenRecordOverallSummary
	if len(items) > 0 {
		for _, item := range items {
			overallSummary.TotalRequestCount += item.Summary.RequestCount
			overallSummary.TotalPromptTokens += item.Summary.PromptTokens
			overallSummary.TotalOutputTokens += item.Summary.CompletionTokens
		}
		overallSummary.ActiveModelCount = len(items)
	}

	return TokenRecordRecentSnapshot{
		Hours:   hoursList,
		Items:   items,
		Summary: &overallSummary,
	}, nil
}

// tokenRecordDateExpr 生成每日统计的日期表达式。表达式在日志库上执行，
// 必须跟随日志库方言，不能跟随主库。
func tokenRecordDateExpr(column string) string {
	switch common.LogDatabaseType() {
	case common.DatabaseTypeSQLite:
		return "DATE(ROUND(" + column + "), 'unixepoch')"
	case common.DatabaseTypePostgreSQL:
		return "TO_CHAR(TO_TIMESTAMP(" + column + ")::DATE, 'YYYY-MM-DD')"
	default:
		return "DATE(FROM_UNIXTIME(" + column + "))"
	}
}

func GetDailyTokenSummary(startTime, endTime int64) ([]TokenRecordDailyItem, error) {
	if LOG_DB == nil {
		return nil, errors.New("log db is not initialized")
	}

	dateExpr := tokenRecordDateExpr("bucket_start_at")

	var items []TokenRecordDailyItem
	err := LOG_DB.Model(&TokenRecord{}).
		Select(fmt.Sprintf("%s as date, SUM(prompt_tokens) as prompt_tokens, SUM(completion_tokens) as completion_tokens, SUM(prompt_tokens) + SUM(completion_tokens) as total_tokens", dateExpr)).
		Where("bucket_start_at >= ? AND bucket_start_at <= ?", startTime, endTime).
		Group(dateExpr).
		Order("date ASC").
		Scan(&items).Error

	if err != nil {
		common.SysError("failed to query daily token summary: " + err.Error())
		return nil, errors.New("查询每日使用量失败")
	}

	if items == nil {
		items = []TokenRecordDailyItem{}
	}

	return items, nil
}

func GetUserDailyTokenSummary(userId int, startTime, endTime int64) ([]TokenRecordDailyItem, error) {
	if LOG_DB == nil {
		return nil, errors.New("log db is not initialized")
	}

	dateExpr := tokenRecordDateExpr("created_at")

	var items []TokenRecordDailyItem
	err := LOG_DB.Table("logs").
		Select(fmt.Sprintf("%s as date, SUM(prompt_tokens) as prompt_tokens, SUM(completion_tokens) as completion_tokens, SUM(prompt_tokens) + SUM(completion_tokens) as total_tokens", dateExpr)).
		Where("user_id = ? AND type = ? AND created_at >= ? AND created_at <= ?", userId, LogTypeConsume, startTime, endTime).
		Group(dateExpr).
		Order("date ASC").
		Scan(&items).Error

	if err != nil {
		common.SysError("failed to query user daily token summary: " + err.Error())
		return nil, errors.New("查询用户每日使用量失败")
	}

	if items == nil {
		items = []TokenRecordDailyItem{}
	}

	return items, nil
}
