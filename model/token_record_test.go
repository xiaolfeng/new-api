package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTokenRecordTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	oldDB := DB
	oldLogDB := LOG_DB

	DB = db
	LOG_DB = db

	require.NoError(t, db.AutoMigrate(&TokenRecord{}))

	t.Cleanup(func() {
		DB = oldDB
		LOG_DB = oldLogDB
	})

	return db
}

func TestRecordTokenRecordAccumulatesWithinSameHour(t *testing.T) {
	setupTokenRecordTestDB(t)

	baseTimestamp := int64(1711454400)

	require.NoError(t, RecordTokenRecord("gpt-5", 10, 20, 5, TokenRecordTiming{}, baseTimestamp))
	require.NoError(t, RecordTokenRecord("gpt-5", 3, 7, 2, TokenRecordTiming{}, baseTimestamp+300))

	var records []TokenRecord
	require.NoError(t, LOG_DB.Find(&records).Error)
	require.Len(t, records, 1)

	record := records[0]
	require.EqualValues(t, baseTimestamp, record.BucketStartAt)
	require.EqualValues(t, baseTimestamp+3599, record.BucketEndAt)
	require.Equal(t, "gpt-5", record.ModelName)
	require.EqualValues(t, 2, record.RequestCount)
	require.EqualValues(t, 13, record.PromptTokens)
	require.EqualValues(t, 27, record.CompletionTokens)
	require.EqualValues(t, 27, record.TotalTokens)
	require.EqualValues(t, 7, record.TotalUseTime)
	require.EqualValues(t, baseTimestamp, record.FirstUsedAt)
	require.EqualValues(t, baseTimestamp+300, record.LastUsedAt)
}

func TestRecordTokenRecordAggregatesPhaseTPS(t *testing.T) {
	setupTokenRecordTestDB(t)

	baseTimestamp := int64(1711454400)
	require.NoError(t, RecordTokenRecord("gpt-5", 10, 20, 5, TokenRecordTiming{
		ThinkingTokens:     10,
		ThinkingDurationMs: 500,
		OutputTokens:       20,
		OutputDurationMs:   1000,
		ToolTokens:         5,
		ToolDurationMs:     250,
	}, baseTimestamp))
	require.NoError(t, RecordTokenRecord("gpt-5", 3, 7, 2, TokenRecordTiming{
		ThinkingTokens:     20,
		ThinkingDurationMs: 500,
		OutputTokens:       10,
		OutputDurationMs:   1000,
	}, baseTimestamp+300))

	snapshot, err := GetRecentTokenRecordSnapshot(baseTimestamp+600, 1)
	require.NoError(t, err)
	require.Len(t, snapshot.Items, 1)

	cell := snapshot.Items[0].Cells[0]
	assert.EqualValues(t, 30, cell.ThinkingTokens)
	assert.EqualValues(t, 1000, cell.ThinkingDurationMs)
	assert.Equal(t, 30.0, cell.AvgThinkingTPS)
	assert.EqualValues(t, 30, cell.OutputTokens)
	assert.EqualValues(t, 2000, cell.OutputDurationMs)
	assert.Equal(t, 15.0, cell.AvgOutputTPS)
	assert.EqualValues(t, 5, cell.ToolTokens)
	assert.EqualValues(t, 250, cell.ToolDurationMs)
	assert.Equal(t, 20.0, cell.AvgToolTPS)

	summary := snapshot.Items[0].Summary
	assert.Equal(t, cell.AvgThinkingTPS, summary.AvgThinkingTPS)
	assert.Equal(t, cell.AvgOutputTPS, summary.AvgOutputTPS)
	assert.Equal(t, cell.AvgToolTPS, summary.AvgToolTPS)
}

func TestRecordTokenRecordCreatesNewHourBucket(t *testing.T) {
	setupTokenRecordTestDB(t)

	baseTimestamp := int64(1711454400)

	require.NoError(t, RecordTokenRecord("gpt-5", 10, 0, 1, TokenRecordTiming{}, baseTimestamp))
	require.NoError(t, RecordTokenRecord("gpt-5", 20, 5, 2, TokenRecordTiming{}, baseTimestamp+3600))

	var records []TokenRecord
	require.NoError(t, LOG_DB.Order("bucket_start_at asc").Find(&records).Error)
	require.Len(t, records, 2)
	require.EqualValues(t, baseTimestamp, records[0].BucketStartAt)
	require.EqualValues(t, baseTimestamp+3600, records[1].BucketStartAt)
}

func TestGetRecentTokenRecordSnapshotBackfillsHours(t *testing.T) {
	setupTokenRecordTestDB(t)

	currentBucketStartAt := int64(1711454400)
	firstBucketStartAt := currentBucketStartAt - 23*3600

	require.NoError(t, RecordTokenRecord("claude-3-7-sonnet", 100, 50, 10, TokenRecordTiming{}, firstBucketStartAt+120))
	require.NoError(t, RecordTokenRecord("claude-3-7-sonnet", 40, 10, 5, TokenRecordTiming{}, currentBucketStartAt+120))
	require.NoError(t, RecordTokenRecord("gpt-5", 70, 30, 0, TokenRecordTiming{}, currentBucketStartAt+300))

	snapshot, err := GetRecentTokenRecordSnapshot(currentBucketStartAt+900, 24)
	require.NoError(t, err)
	require.Len(t, snapshot.Hours, 24)
	require.Len(t, snapshot.Items, 2)
	require.EqualValues(t, firstBucketStartAt, snapshot.Hours[0].BucketStartAt)
	require.True(t, snapshot.Hours[23].IsCurrent)

	var claudeItem *TokenRecordRecentItem
	for i := range snapshot.Items {
		if snapshot.Items[i].ModelName == "claude-3-7-sonnet" {
			claudeItem = &snapshot.Items[i]
			break
		}
	}
	require.NotNil(t, claudeItem)
	require.Len(t, claudeItem.Cells, 24)
	require.EqualValues(t, 50, claudeItem.Cells[0].TotalTokens)
	require.EqualValues(t, 10, claudeItem.Cells[23].TotalTokens)
	require.EqualValues(t, 60, claudeItem.Summary.TotalTokens)
	require.EqualValues(t, 15, claudeItem.Summary.TotalUseTime)
	require.EqualValues(t, 4, claudeItem.Summary.AvgTPS)

	var gptItem *TokenRecordRecentItem
	for i := range snapshot.Items {
		if snapshot.Items[i].ModelName == "gpt-5" {
			gptItem = &snapshot.Items[i]
			break
		}
	}
	require.NotNil(t, gptItem)
	require.EqualValues(t, 30, gptItem.Cells[23].TotalTokens)
	require.EqualValues(t, 0, gptItem.Cells[23].AvgTPS)
}

func TestGetRecentTokenRecordSnapshotHonorsHoursWindow(t *testing.T) {
	setupTokenRecordTestDB(t)

	currentBucketStartAt := int64(1711454400)
	firstBucketStartAt := currentBucketStartAt - 47*3600

	require.NoError(t, RecordTokenRecord("gpt-5", 10, 20, 5, TokenRecordTiming{}, firstBucketStartAt+60))
	require.NoError(t, RecordTokenRecord("gpt-5", 5, 15, 3, TokenRecordTiming{}, currentBucketStartAt+60))

	snapshot, err := GetRecentTokenRecordSnapshot(currentBucketStartAt+900, 48)
	require.NoError(t, err)
	require.Len(t, snapshot.Hours, 48)
	require.Len(t, snapshot.Items, 1)

	item := snapshot.Items[0]
	require.Len(t, item.Cells, 48)
	require.EqualValues(t, 20, item.Cells[0].TotalTokens)
	require.EqualValues(t, 15, item.Cells[47].TotalTokens)
	require.EqualValues(t, 35, item.Summary.TotalTokens)
}

// TestRecordFailedTokenRecordConcurrentIncrementKeepsAllCounts 模拟交错并发：
// 外层调用读到旧值后、写入前，另一个失败请求完成写入。
// 计数与状态码分布都不能丢更新。
func TestRecordFailedTokenRecordConcurrentIncrementKeepsAllCounts(t *testing.T) {
	setupTokenRecordTestDB(t)

	bucketStartAt := int64(1711454400)
	seed := TokenRecord{
		BucketStartAt: bucketStartAt,
		BucketEndAt:   bucketStartAt + 3599,
		ModelName:     "gpt-5",
		FailedCount:   1,
		FailedDetail:  `{"500":1}`,
		FirstUsedAt:   bucketStartAt,
		LastUsedAt:    bucketStartAt,
		CreatedAt:     bucketStartAt,
		UpdatedAt:     bucketStartAt,
	}
	require.NoError(t, LOG_DB.Create(&seed).Error)

	var interleaved bool
	require.NoError(t, LOG_DB.Callback().Query().After("gorm:query").Register(
		"test:interleave-failure-record",
		func(tx *gorm.DB) {
			if interleaved || tx.Statement.Table != (TokenRecord{}).TableName() {
				return
			}
			interleaved = true
			// 外层 First 读取完成后、UPDATE 写入前，插入另一个并发失败记录。
			// 旧实现会基于过期快照覆盖写回，丢失这次并发计数。
			RecordFailedTokenRecord("gpt-5", 402, bucketStartAt+120)
		},
	))
	t.Cleanup(func() {
		LOG_DB.Callback().Query().Remove("test:interleave-failure-record")
	})


	RecordFailedTokenRecord("gpt-5", 429, bucketStartAt+60)

	var got TokenRecord
	require.NoError(t, LOG_DB.Where("bucket_start_at = ? AND model_name = ?", bucketStartAt, "gpt-5").First(&got).Error)
	assert.EqualValues(t, 3, got.FailedCount)
	detail := map[string]int64{}
	require.NoError(t, common.UnmarshalJsonStr(got.FailedDetail, &detail))
	assert.Equal(t, map[string]int64{"500": 1, "402": 1, "429": 1}, detail)
}

// TestUserDailyTokenSummaryUsesLogDatabaseDialect 主库与日志库类型不一致时，
// 每日统计的日期表达式必须跟随实际执行的日志库，否则生成不兼容 SQL。
func TestUserDailyTokenSummaryUsesLogDatabaseDialect(t *testing.T) {
	setupTokenRecordTestDB(t)
	require.NoError(t, LOG_DB.AutoMigrate(&Log{}))
	require.NoError(t, LOG_DB.Create(&Log{UserId: 7, Type: LogTypeConsume, CreatedAt: 1711454400, PromptTokens: 5, CompletionTokens: 3}).Error)

	origMain, origLog := common.MainDatabaseType(), common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypePostgreSQL, common.DatabaseTypeSQLite)
	t.Cleanup(func() { common.SetDatabaseTypes(origMain, origLog) })

	items, err := GetUserDailyTokenSummary(7, 1711454400, 1711540799)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.EqualValues(t, 8, items[0].TotalTokens)
}

func TestTokenRecordDateExprFollowsLogDatabaseType(t *testing.T) {
	origLog := common.LogDatabaseType()
	t.Cleanup(func() { common.SetLogDatabaseType(origLog) })

	common.SetLogDatabaseType(common.DatabaseTypeSQLite)
	assert.Equal(t, "DATE(ROUND(bucket_start_at), 'unixepoch')", tokenRecordDateExpr("bucket_start_at"))
	common.SetLogDatabaseType(common.DatabaseTypePostgreSQL)
	assert.Equal(t, "TO_CHAR(TO_TIMESTAMP(bucket_start_at)::DATE, 'YYYY-MM-DD')", tokenRecordDateExpr("bucket_start_at"))
	common.SetLogDatabaseType(common.DatabaseTypeMySQL)
	assert.Equal(t, "DATE(FROM_UNIXTIME(bucket_start_at))", tokenRecordDateExpr("bucket_start_at"))
}
