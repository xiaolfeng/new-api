package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupToolLogTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	oldDB, oldLogDB := DB, LOG_DB
	DB, LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&ToolLog{}))
	t.Cleanup(func() {
		DB, LOG_DB = oldDB, oldLogDB
	})
}

func TestRecordAndQueryToolLogs(t *testing.T) {
	setupToolLogTestDB(t)

	RecordToolLogs([]*ToolLog{
		{
			UserId:       1,
			Username:     "alice",
			TokenName:    "code",
			ModelName:    "deepseek-v4-flash",
			RequestId:    "req-1",
			OriginalName: "web_search",
			Canonical:    "host.web_search",
			Kind:         "search",
			Mode:         "loop",
			Backend:      "searxng",
			Query:        "筱锋",
			Result:       `[web_search] query="筱锋"`,
		},
		{
			UserId:       1,
			Username:     "alice",
			OriginalName: "web_search",
			Canonical:    "host.web_search",
			Kind:         "search",
			Query:        "筱锋",
			ErrorCode:    "invalid_input",
			RequestId:    "req-1",
		},
		{
			UserId:       2,
			Username:     "bob",
			OriginalName: "web_fetch",
			Canonical:    "host.web_fetch",
			Kind:         "fetch",
			URL:          "https://example.com",
			RequestId:    "req-2",
		},
	})

	all, total, err := GetAllToolLogs(ToolLogQuery{Num: 20})
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, all, 3)

	mine, mineTotal, err := GetUserToolLogs(1, ToolLogQuery{Num: 20})
	require.NoError(t, err)
	require.EqualValues(t, 2, mineTotal)
	require.Len(t, mine, 2)
	for _, log := range mine {
		assert.Equal(t, 1, log.UserId)
	}

	failed, failedTotal, err := GetAllToolLogs(ToolLogQuery{ErrorFilter: "invalid_input", Num: 20})
	require.NoError(t, err)
	require.EqualValues(t, 1, failedTotal)
	require.Equal(t, "invalid_input", failed[0].ErrorCode)

	fetched, fetchTotal, err := GetAllToolLogs(ToolLogQuery{Kind: "fetch", Num: 20})
	require.NoError(t, err)
	require.EqualValues(t, 1, fetchTotal)
	require.Equal(t, "https://example.com", fetched[0].URL)

	byQuery, qTotal, err := GetAllToolLogs(ToolLogQuery{Q: "筱锋", Num: 20})
	require.NoError(t, err)
	require.EqualValues(t, 2, qTotal)
	require.NotEmpty(t, byQuery)
}

func TestClickHouseToolLogCreateSQL(t *testing.T) {
	sql := clickHouseToolLogCreateTableSQL(0)
	assert.Contains(t, sql, "CREATE TABLE IF NOT EXISTS tool_logs")
	assert.Contains(t, sql, "ENGINE = MergeTree()")
	assert.NotContains(t, sql, "TTL ")
	sqlTTL := clickHouseToolLogCreateTableSQL(7)
	assert.Contains(t, sqlTTL, "INTERVAL 7 DAY DELETE")
}
