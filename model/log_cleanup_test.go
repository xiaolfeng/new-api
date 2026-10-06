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
For commercial licensing, please contact support@quantumnous.com
*/

package model

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupLogCleanupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	oldDB, oldLogDB := DB, LOG_DB
	DB = db
	LOG_DB = db
	require.NoError(t, db.AutoMigrate(&Log{}, &ToolLog{}))
	t.Cleanup(func() {
		DB = oldDB
		LOG_DB = oldLogDB
	})

	return db
}

// TestDeleteOldLogBatchRespectsBatchSize 非 MySQL 数据库同样必须按批次删除，
// 一次清理调用不得越过 limit 删除全部历史。
func TestDeleteOldLogBatchRespectsBatchSize(t *testing.T) {
	db := setupLogCleanupTestDB(t)

	target := int64(1711454400)
	oldLogs := make([]Log, 0, 150)
	for i := 0; i < 150; i++ {
		oldLogs = append(oldLogs, Log{CreatedAt: target - 100, Type: LogTypeConsume, Content: "old"})
	}
	newLogs := make([]Log, 0, 10)
	for i := 0; i < 10; i++ {
		newLogs = append(newLogs, Log{CreatedAt: target + 100, Type: LogTypeConsume, Content: "new"})
	}
	require.NoError(t, db.Create(&oldLogs).Error)
	require.NoError(t, db.Create(&newLogs).Error)

	deleted, err := DeleteOldLogBatch(context.Background(), target, 100)
	require.NoError(t, err)
	assert.EqualValues(t, 100, deleted, "单批删除必须受 limit 约束")

	remaining, err := CountOldLog(context.Background(), target)
	require.NoError(t, err)
	assert.EqualValues(t, 50, remaining, "未删除的历史日志必须保留给后续批次")

	var totalCount int64
	require.NoError(t, db.Model(&Log{}).Count(&totalCount).Error)
	assert.EqualValues(t, 60, totalCount, "limit 之外的新日志不能被误删")
}
