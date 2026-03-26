package model

import (
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TokenRecord struct {
	Id               int    `json:"id"`
	BucketStartAt    int64  `json:"bucket_start_at" gorm:"bigint;uniqueIndex:idx_token_record_bucket_model,priority:1;index:idx_token_record_bucket_start,priority:1"`
	BucketEndAt      int64  `json:"bucket_end_at" gorm:"bigint"`
	ModelName        string `json:"model_name" gorm:"size:255;default:'';uniqueIndex:idx_token_record_bucket_model,priority:2;index:idx_token_record_model_name"`
	RequestCount     int64  `json:"request_count" gorm:"default:0"`
	PromptTokens     int64  `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens int64  `json:"completion_tokens" gorm:"default:0"`
	TotalTokens      int64  `json:"total_tokens" gorm:"default:0"`
	TotalUseTime     int64  `json:"total_use_time" gorm:"default:0"`
	FirstUsedAt      int64  `json:"first_used_at" gorm:"bigint"`
	LastUsedAt       int64  `json:"last_used_at" gorm:"bigint"`
	CreatedAt        int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt        int64  `json:"updated_at" gorm:"bigint"`
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
	BucketStartAt    int64   `json:"bucket_start_at"`
	BucketEndAt      int64   `json:"bucket_end_at"`
	RequestCount     int64   `json:"request_count"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	TotalUseTime     int64   `json:"total_use_time"`
	AvgTPS           float64 `json:"avg_tps"`
	IsCurrent        bool    `json:"is_current"`
}

type TokenRecordSummary struct {
	RequestCount     int64   `json:"request_count"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	TotalUseTime     int64   `json:"total_use_time"`
	AvgTPS           float64 `json:"avg_tps"`
}

type TokenRecordRecentItem struct {
	ModelName string                `json:"model_name"`
	Summary   TokenRecordSummary    `json:"summary"`
	Cells     []TokenRecordHourCell `json:"cells"`
}

type TokenRecordRecentSnapshot struct {
	Hours []TokenRecordHourMeta   `json:"hours"`
	Items []TokenRecordRecentItem `json:"items"`
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

func calcTokenRecordAvgTPS(totalTokens int64, totalUseTime int64) float64 {
	if totalTokens <= 0 || totalUseTime <= 0 {
		return 0
	}
	return math.Round((float64(totalTokens)/float64(totalUseTime))*100) / 100
}

func buildTokenRecordIncrementExpr(column string, delta interface{}) clause.Expr {
	return gorm.Expr("? + ?", clause.Column{Table: "token_record", Name: column}, delta)
}

func RecordTokenRecord(modelName string, promptTokens int, completionTokens int, useTimeSeconds int, createdAt int64) error {
	if LOG_DB == nil {
		return errors.New("log db is not initialized")
	}

	modelName = normalizeTokenRecordModelName(modelName)
	bucketStartAt, bucketEndAt := getTokenRecordHourBucket(createdAt)
	totalTokens := int64(promptTokens + completionTokens)
	totalUseTime := int64(useTimeSeconds)
	if totalUseTime < 0 {
		totalUseTime = 0
	}

	record := TokenRecord{
		BucketStartAt:    bucketStartAt,
		BucketEndAt:      bucketEndAt,
		ModelName:        modelName,
		RequestCount:     1,
		PromptTokens:     int64(promptTokens),
		CompletionTokens: int64(completionTokens),
		TotalTokens:      totalTokens,
		TotalUseTime:     totalUseTime,
		FirstUsedAt:      createdAt,
		LastUsedAt:       createdAt,
		CreatedAt:        createdAt,
		UpdatedAt:        createdAt,
	}

	return LOG_DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "bucket_start_at"},
			{Name: "model_name"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"bucket_end_at":     bucketEndAt,
			"request_count":     buildTokenRecordIncrementExpr("request_count", 1),
			"prompt_tokens":     buildTokenRecordIncrementExpr("prompt_tokens", promptTokens),
			"completion_tokens": buildTokenRecordIncrementExpr("completion_tokens", completionTokens),
			"total_tokens":      buildTokenRecordIncrementExpr("total_tokens", totalTokens),
			"total_use_time":    buildTokenRecordIncrementExpr("total_use_time", totalUseTime),
			"last_used_at":      createdAt,
			"updated_at":        createdAt,
		}),
	}).Create(&record).Error
}

func buildTokenRecordHours(rangeStartAt int64, currentBucketStartAt int64) []TokenRecordHourMeta {
	hours := make([]TokenRecordHourMeta, 0, 24)
	for i := 0; i < 24; i++ {
		bucketStartAt := rangeStartAt + int64(i*3600)
		bucketEndAt := bucketStartAt + 3599
		hours = append(hours, TokenRecordHourMeta{
			BucketStartAt: bucketStartAt,
			BucketEndAt:   bucketEndAt,
			Label:         time.Unix(bucketStartAt, 0).Format("15:00"),
			IsCurrent:     bucketStartAt == currentBucketStartAt,
		})
	}
	return hours
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

func GetRecentTokenRecordSnapshot(currentTimestamp int64) (TokenRecordRecentSnapshot, error) {
	if LOG_DB == nil {
		return TokenRecordRecentSnapshot{}, errors.New("log db is not initialized")
	}

	currentBucketStartAt, _ := getTokenRecordHourBucket(currentTimestamp)
	rangeStartAt := currentBucketStartAt - 23*3600
	hours := buildTokenRecordHours(rangeStartAt, currentBucketStartAt)

	var records []TokenRecord
	err := LOG_DB.Model(&TokenRecord{}).
		Where("bucket_start_at >= ? AND bucket_start_at <= ?", rangeStartAt, currentBucketStartAt).
		Order("bucket_start_at ASC, total_tokens DESC, model_name ASC").
		Find(&records).Error
	if err != nil {
		common.SysError("failed to query token records: " + err.Error())
		return TokenRecordRecentSnapshot{}, errors.New("查询模型日志失败")
	}

	hourIndexMap := make(map[int64]int, len(hours))
	for i, hour := range hours {
		hourIndexMap[hour.BucketStartAt] = i
	}

	itemsMap := make(map[string]*TokenRecordRecentItem)
	for _, record := range records {
		hourIndex, ok := hourIndexMap[record.BucketStartAt]
		if !ok {
			continue
		}

		item, ok := itemsMap[record.ModelName]
		if !ok {
			item = &TokenRecordRecentItem{
				ModelName: record.ModelName,
				Cells:     buildEmptyTokenRecordCells(hours),
			}
			itemsMap[record.ModelName] = item
		}

		item.Cells[hourIndex] = TokenRecordHourCell{
			BucketStartAt:    record.BucketStartAt,
			BucketEndAt:      record.BucketEndAt,
			RequestCount:     record.RequestCount,
			PromptTokens:     record.PromptTokens,
			CompletionTokens: record.CompletionTokens,
			TotalTokens:      record.TotalTokens,
			TotalUseTime:     record.TotalUseTime,
			AvgTPS:           calcTokenRecordAvgTPS(record.TotalTokens, record.TotalUseTime),
			IsCurrent:        hours[hourIndex].IsCurrent,
		}

		item.Summary.RequestCount += record.RequestCount
		item.Summary.PromptTokens += record.PromptTokens
		item.Summary.CompletionTokens += record.CompletionTokens
		item.Summary.TotalTokens += record.TotalTokens
		item.Summary.TotalUseTime += record.TotalUseTime
	}

	items := make([]TokenRecordRecentItem, 0, len(itemsMap))
	for _, item := range itemsMap {
		item.Summary.AvgTPS = calcTokenRecordAvgTPS(item.Summary.TotalTokens, item.Summary.TotalUseTime)
		items = append(items, *item)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Summary.TotalTokens == items[j].Summary.TotalTokens {
			return items[i].ModelName < items[j].ModelName
		}
		return items[i].Summary.TotalTokens > items[j].Summary.TotalTokens
	})

	return TokenRecordRecentSnapshot{
		Hours: hours,
		Items: items,
	}, nil
}
