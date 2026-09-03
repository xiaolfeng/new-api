package model

import (
	"testing"

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
