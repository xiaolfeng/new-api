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
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupQuotaDataTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	oldDB := DB
	DB = db
	require.NoError(t, db.AutoMigrate(&QuotaData{}))

	oldCache := CacheQuotaData
	CacheQuotaData = make(map[string]*QuotaData)
	t.Cleanup(func() {
		DB = oldDB
		CacheQuotaData = oldCache
	})

	return db
}

// TestSaveQuotaDataCacheRetainsFailedInserts 落库失败时缓存中的增量必须保留，
// 等待下次刷盘重试；只有全部成功才允许清空。
func TestSaveQuotaDataCacheRetainsFailedInserts(t *testing.T) {
	db := setupQuotaDataTestDB(t)

	LogQuotaData(QuotaDataLogParams{
		UserID: 1, Username: "u1", ModelName: "gpt-5", CreatedAt: 1711454400,
		Quota: 30, TokenUsed: 100, UseGroup: "default", TokenID: 2, ChannelID: 3, NodeName: "n1",
	})

	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(
		"test:fail-quota-create", func(tx *gorm.DB) { tx.AddError(errors.New("db unavailable")) },
	))
	SaveQuotaDataCache()

	assert.Len(t, CacheQuotaData, 1, "落库失败的增量必须保留在缓存中")
	var rowCount int64
	require.NoError(t, db.Table("quota_data").Count(&rowCount).Error)
	assert.Zero(t, rowCount)

	require.NoError(t, db.Callback().Create().Remove("test:fail-quota-create"))
	SaveQuotaDataCache()

	assert.Empty(t, CacheQuotaData, "重试成功后缓存才清空")
	require.NoError(t, db.Table("quota_data").Count(&rowCount).Error)
	assert.EqualValues(t, 1, rowCount)

	var row QuotaData
	require.NoError(t, db.Table("quota_data").First(&row).Error)
	assert.EqualValues(t, 30, row.Quota)
	assert.EqualValues(t, 100, row.TokenUsed)
}

// TestSaveQuotaDataCacheRetainsFailedUpdates 更新路径失败同样必须保留增量，
// 且恢复后按累计差值补齐，不丢不重。
func TestSaveQuotaDataCacheRetainsFailedUpdates(t *testing.T) {
	db := setupQuotaDataTestDB(t)

	seed := &QuotaData{
		UserID: 1, Username: "u1", ModelName: "gpt-5", CreatedAt: 1711454400,
		UseGroup: "default", TokenID: 2, ChannelID: 3, NodeName: "n1",
		Count: 5, Quota: 50, TokenUsed: 500,
	}
	require.NoError(t, db.Table("quota_data").Create(seed).Error)

	LogQuotaData(QuotaDataLogParams{
		UserID: 1, Username: "u1", ModelName: "gpt-5", CreatedAt: 1711454400,
		Quota: 30, TokenUsed: 100, UseGroup: "default", TokenID: 2, ChannelID: 3, NodeName: "n1",
	})

	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(
		"test:fail-quota-update", func(tx *gorm.DB) { tx.AddError(errors.New("db unavailable")) },
	))
	SaveQuotaDataCache()

	assert.Len(t, CacheQuotaData, 1, "更新失败的增量必须保留在缓存中")
	var row QuotaData
	require.NoError(t, db.Table("quota_data").First(&row).Error)
	assert.EqualValues(t, 50, row.Quota, "数据库值不应被部分写入污染")

	require.NoError(t, db.Callback().Update().Remove("test:fail-quota-update"))
	SaveQuotaDataCache()

	assert.Empty(t, CacheQuotaData)
	require.NoError(t, db.Table("quota_data").First(&row).Error)
	assert.EqualValues(t, 80, row.Quota)
	assert.EqualValues(t, 6, row.Count)
	assert.EqualValues(t, 600, row.TokenUsed)
}
