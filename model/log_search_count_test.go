/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public
License along with this program. If not, see <https://www.gnu.org/licenses/>.
For commercial licensing, please contact support@quantumnous.com.
*/

package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupLogSearchCountTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	oldDB, oldLogDB := DB, LOG_DB
	DB = db
	LOG_DB = db
	require.NoError(t, db.AutoMigrate(&Log{}))
	t.Cleanup(func() {
		DB = oldDB
		LOG_DB = oldLogDB
	})

	return db
}

// TestGetUserLogsCountIsCapped 计数封顶必须真正限制统计扫描量：
// 超过 logSearchCountLimit 时总数封顶返回，而不是全量计数后丢弃精度。
func TestGetUserLogsCountIsCapped(t *testing.T) {
	db := setupLogSearchCountTestDB(t)

	logs := make([]Log, 0, logSearchCountLimit+50)
	for i := 0; i < logSearchCountLimit+50; i++ {
		logs = append(logs, Log{UserId: 7, CreatedAt: int64(1711400000 + i), Type: LogTypeConsume})
	}
	require.NoError(t, db.CreateInBatches(logs, 500).Error)

	_, total, err := GetUserLogs(7, LogTypeUnknown, 0, 0, "", "", 0, 10, "", "", "")
	require.NoError(t, err)
	assert.EqualValues(t, logSearchCountLimit, total, "超过上限时 total 必须封顶在 logSearchCountLimit")

	_, total, err = GetUserLogs(7, LogTypeUnknown, 0, 0, "", "", 0, 10, "", "", "")
	require.NoError(t, err)
	assert.EqualValues(t, logSearchCountLimit, total)
}

// TestGetUserLogsCountExactBelowCap 未达上限时 total 保持精确值。
func TestGetUserLogsCountExactBelowCap(t *testing.T) {
	db := setupLogSearchCountTestDB(t)

	logs := make([]Log, 0, 42)
	for i := 0; i < 42; i++ {
		logs = append(logs, Log{UserId: 9, CreatedAt: int64(1711400000 + i), Type: LogTypeConsume})
	}
	require.NoError(t, db.CreateInBatches(logs, 500).Error)

	logsResult, total, err := GetUserLogs(9, LogTypeUnknown, 0, 0, "", "", 0, 10, "", "", "")
	require.NoError(t, err)
	assert.EqualValues(t, 42, total)
	require.Len(t, logsResult, 10)
}
