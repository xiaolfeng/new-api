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

package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupLogDetailTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	oldDB, oldLogDB := model.DB, model.LOG_DB
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.User{}))
	t.Cleanup(func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
	})

	return db
}

func runGetLogDetail(t *testing.T, db *gorm.DB, role int, userId int, requestId string, rowKey ...string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	targetURL := "/api/log/detail?request_id=" + requestId
	if len(rowKey) > 0 && rowKey[0] != "" {
		targetURL += "&row_key=" + rowKey[0]
	}
	c.Request = httptest.NewRequest(http.MethodGet, targetURL, nil)
	c.Set("role", role)
	c.Set("id", userId)
	GetLogDetail(c)
	return recorder
}

// TestGetLogDetailOwnershipAndPayload 详情端点按需返回大字段：
// 管理员可查任意日志，普通用户只能查自己的；request_id 是跨库稳定的定位键。
func TestGetLogDetailOwnershipAndPayload(t *testing.T) {
	db := setupLogDetailTestDB(t)

	target := model.Log{
		UserId: 7, CreatedAt: 1711454400, Type: model.LogTypeConsume,
		RequestId: "req-target", Record: `{"completion":"hi"}`, FullLog: `{"meta":{}}`,
	}
	other := model.Log{
		UserId: 8, CreatedAt: 1711454401, Type: model.LogTypeConsume,
		RequestId: "req-other",
	}
	require.NoError(t, db.Create(&target).Error)
	require.NoError(t, db.Create(&other).Error)

	t.Run("admin fetches any log with detail payload", func(t *testing.T) {
		recorder := runGetLogDetail(t, db, common.RoleAdminUser, 999, "req-target")
		require.Equal(t, http.StatusOK, recorder.Code)
		var resp struct {
			Success bool      `json:"success"`
			Data    model.Log `json:"data"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
		assert.True(t, resp.Success)
		assert.Equal(t, `{"completion":"hi"}`, resp.Data.Record)
		assert.Equal(t, `{"meta":{}}`, resp.Data.FullLog)
		assert.True(t, resp.Data.HasDetail)
		assert.True(t, resp.Data.HasRecord)
		assert.True(t, resp.Data.HasFullLog)
	})

	t.Run("user fetches own log", func(t *testing.T) {
		recorder := runGetLogDetail(t, db, common.RoleCommonUser, 8, "req-other")
		require.Equal(t, http.StatusOK, recorder.Code)
		var resp struct {
			Success bool `json:"success"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
		assert.True(t, resp.Success)
	})

	t.Run("user cannot fetch others log", func(t *testing.T) {
		recorder := runGetLogDetail(t, db, common.RoleCommonUser, 8, "req-target")
		require.Equal(t, http.StatusOK, recorder.Code)
		var resp struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
		assert.False(t, resp.Success)
		assert.NotEmpty(t, resp.Message)
	})

	t.Run("missing request id rejected", func(t *testing.T) {
		recorder := runGetLogDetail(t, db, common.RoleAdminUser, 999, "")
		var resp struct {
			Success bool `json:"success"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
		assert.False(t, resp.Success)
	})
}

// TestLogListOmitsDetailPayload 列表接口不得携带 record/full_log 大字段，
// 详情按需经 /log/detail 获取。
func TestLogListOmitsDetailPayload(t *testing.T) {
	db := setupLogDetailTestDB(t)
	user := model.User{Id: 7, Username: "detail-code-user", Role: common.RoleCodeUser}
	require.NoError(t, db.Create(&user).Error)

	cases := []struct {
		name    string
		record  string
		fullLog string
	}{
		{name: "record", record: `{"completion":"x"}`},
		{name: "full-log", fullLog: `{"meta":{}}`},
		{name: "both", record: `{"completion":"x"}`, fullLog: `{"meta":{}}`},
		{name: "empty"},
	}
	for i := range cases {
		tc := cases[i]
		log := model.Log{
			UserId: 7, CreatedAt: int64(1711454400 + i), Type: model.LogTypeConsume,
			RequestId: "req-" + tc.name, Record: tc.record, FullLog: tc.fullLog,
			Other: `{"client_source":"OpenCode"}`,
		}
		require.NoError(t, db.Create(&log).Error)
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.name, func(t *testing.T) {
			requestID := "req-" + tc.name
			adminLogs, _, err := model.GetAllLogs(model.LogTypeConsume, 0, 0, "", "", "", 0, 10, 0, "", requestID, "")
			require.NoError(t, err)
			selfLogs, _, err := model.GetUserLogs(7, model.LogTypeConsume, 0, 0, "", "", 0, 10, "", requestID, "")
			require.NoError(t, err)
			for _, rows := range [][]*model.Log{adminLogs, selfLogs} {
				require.Len(t, rows, 1)
				row := rows[0]
				assert.Empty(t, row.Record, "列表响应必须剥离 record")
				assert.Empty(t, row.FullLog, "列表响应必须剥离 full_log")
				assert.NotEmpty(t, row.RowKey)
				assert.Equal(t, tc.record != "" || tc.fullLog != "", row.HasDetail)
				assert.Equal(t, tc.record != "", row.HasRecord)
				assert.Equal(t, tc.fullLog != "", row.HasFullLog)
			}

			for _, role := range []int{common.RoleAdminUser, common.RoleRootUser, common.RoleCodeUser} {
				recorder := runGetLogDetail(t, db, role, 7, requestID)
				var resp struct {
					Success bool      `json:"success"`
					Data    model.Log `json:"data"`
				}
				require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
				require.True(t, resp.Success)
				assert.Equal(t, tc.record, resp.Data.Record)
				assert.Equal(t, tc.fullLog, resp.Data.FullLog)
				assert.Equal(t, tc.record != "" || tc.fullLog != "", resp.Data.HasDetail)
				assert.Equal(t, tc.record != "", resp.Data.HasRecord)
				assert.Equal(t, tc.fullLog != "", resp.Data.HasFullLog)
			}
		})
	}

	// Common users must not be offered details that the detail endpoint hides.
	require.NoError(t, db.Model(&user).Update("role", common.RoleCommonUser).Error)
	selfLogs, _, err := model.GetUserLogs(7, model.LogTypeConsume, 0, 0, "", "", 0, 10, "", "", "")
	require.NoError(t, err)
	require.Len(t, selfLogs, len(cases))
	for _, row := range selfLogs {
		assert.Empty(t, row.Record)
		assert.Empty(t, row.FullLog)
		assert.False(t, row.HasDetail)
		assert.False(t, row.HasRecord)
		assert.False(t, row.HasFullLog)
	}
	recorder := runGetLogDetail(t, db, common.RoleCommonUser, 7, "req-both")
	var resp struct {
		Success bool      `json:"success"`
		Data    model.Log `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	assert.Empty(t, resp.Data.Record)
	assert.Empty(t, resp.Data.FullLog)
	assert.False(t, resp.Data.HasDetail)
	assert.False(t, resp.Data.HasRecord)
	assert.False(t, resp.Data.HasFullLog)

	for _, column := range []string{"has_detail", "has_record", "has_full_log"} {
		assert.False(t, db.Migrator().HasColumn(&model.Log{}, column))
	}
}

func TestGetLogDetailDisambiguatesSharedRequestIdByRowKey(t *testing.T) {
	db := setupLogDetailTestDB(t)

	errLog1 := model.Log{
		UserId: 7, CreatedAt: 1711454400, Type: model.LogTypeError,
		RequestId: "req-shared-retry", LogEntryId: "le_err_1",
		Content: "attempt 1 failed", FullLog: `{"attempt":1}`,
	}
	errLog2 := model.Log{
		UserId: 7, CreatedAt: 1711454400, Type: model.LogTypeError,
		RequestId: "req-shared-retry", LogEntryId: "",
		Content: "attempt 2 legacy row", FullLog: `{"attempt":2}`,
	}
	consumeLog := model.Log{
		UserId: 7, CreatedAt: 1711454401, Type: model.LogTypeConsume,
		RequestId: "req-shared-retry", LogEntryId: "le_consume_3",
		Record: `{"completion":"final answer"}`, FullLog: `{"attempt":3}`,
	}
	require.NoError(t, db.Create(&errLog1).Error)
	require.NoError(t, db.Create(&errLog2).Error)
	require.NoError(t, db.Create(&consumeLog).Error)

	// Fetch exact error log 1 by its log_entry_id row_key
	rec1 := runGetLogDetail(t, db, common.RoleAdminUser, 999, "req-shared-retry", "le_err_1")
	require.Equal(t, http.StatusOK, rec1.Code)
	var resp1 struct {
		Success bool      `json:"success"`
		Data    model.Log `json:"data"`
	}
	require.NoError(t, common.Unmarshal(rec1.Body.Bytes(), &resp1))
	require.True(t, resp1.Success)
	assert.Equal(t, `{"attempt":1}`, resp1.Data.FullLog)

	// Fetch legacy relational row by legacy-id:<pk>
	rec2 := runGetLogDetail(t, db, common.RoleAdminUser, 999, "req-shared-retry", "legacy-id:"+strconv.Itoa(errLog2.Id))
	require.Equal(t, http.StatusOK, rec2.Code)
	var resp2 struct {
		Success bool      `json:"success"`
		Data    model.Log `json:"data"`
	}
	require.NoError(t, common.Unmarshal(rec2.Body.Bytes(), &resp2))
	require.True(t, resp2.Success)
	assert.Equal(t, `{"attempt":2}`, resp2.Data.FullLog)

	// Legacy caller without row_key still receives latest entry for request_id
	recLatest := runGetLogDetail(t, db, common.RoleAdminUser, 999, "req-shared-retry")
	require.Equal(t, http.StatusOK, recLatest.Code)
	var respLatest struct {
		Success bool      `json:"success"`
		Data    model.Log `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recLatest.Body.Bytes(), &respLatest))
	require.True(t, respLatest.Success)
	assert.Equal(t, `{"attempt":3}`, respLatest.Data.FullLog)

	// Synthetic row_key cannot bypass ownership check for another user
	recDenied := runGetLogDetail(t, db, common.RoleCommonUser, 8, "req-shared-retry", "le_err_1")
	require.Equal(t, http.StatusOK, recDenied.Code)
	var respDenied struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recDenied.Body.Bytes(), &respDenied))
	assert.False(t, respDenied.Success)

	// Unmatched or malformed non-ClickHouse row_key must not fall back to latest entry
	for _, badRowKey := range []string{"le_nonexistent", "legacy-id:999999", "legacy-id:notanumber"} {
		recMissing := runGetLogDetail(t, db, common.RoleAdminUser, 999, "req-shared-retry", badRowKey)
		require.Equal(t, http.StatusOK, recMissing.Code)
		var respMissing struct {
			Success bool `json:"success"`
		}
		require.NoError(t, common.Unmarshal(recMissing.Body.Bytes(), &respMissing))
		assert.False(t, respMissing.Success, "expected failure for row_key=%s", badRowKey)
	}
}
