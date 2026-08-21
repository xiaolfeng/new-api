package model

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRecordConsumeLogUsesUpstreamModelForTokenRecord(t *testing.T) {
	db := setupTokenRecordTestDB(t)
	require.NoError(t, db.AutoMigrate(&Log{}))

	oldLogConsumeEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = true
	t.Cleanup(func() {
		common.LogConsumeEnabled = oldLogConsumeEnabled
	})

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req, err := http.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	require.NoError(t, err)
	ctx.Request = req
	ctx.Set("username", "tester")
	ctx.Set(common.RequestIdKey, "req-1")

	originModel := "claude-opus-4-1"
	upstreamModel := "glm-4.5"

	RecordConsumeLog(ctx, 1001, RecordConsumeLogParams{
		ChannelId:        1,
		PromptTokens:     120,
		CompletionTokens: 80,
		ModelName:        originModel,
		TokenName:        "t1",
		Quota:            10,
		Content:          "test",
		TokenId:          2002,
		UseTimeSeconds:   5,
		IsStream:         false,
		Group:            "default",
		Other: map[string]interface{}{
			"is_model_mapped":     true,
			"upstream_model_name": upstreamModel,
		},
	})

	var logs []Log
	require.NoError(t, LOG_DB.Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Equal(t, originModel, logs[0].ModelName)

	var records []TokenRecord
	require.NoError(t, LOG_DB.Find(&records).Error)
	require.Len(t, records, 1)
	require.Equal(t, upstreamModel, records[0].ModelName)
	require.EqualValues(t, 1, records[0].RequestCount)
	require.EqualValues(t, 80, records[0].TotalTokens)
}

// TestRecordConsumeLogInternalSkipsTokenRecord 验证内部工具合成请求
// （host-tool builtin，Internal=true）仍写入消费日志，但跳过 TokenRecord
// 模型日志统计，避免 0-token 内部请求稀释 Model Log 的 TPS / 请求数。
func TestRecordConsumeLogInternalSkipsTokenRecord(t *testing.T) {
	db := setupTokenRecordTestDB(t)
	require.NoError(t, db.AutoMigrate(&Log{}))

	oldLogConsumeEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = true
	t.Cleanup(func() {
		common.LogConsumeEnabled = oldLogConsumeEnabled
	})

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req, err := http.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	require.NoError(t, err)
	ctx.Request = req
	ctx.Set("username", "tester")
	ctx.Set(common.RequestIdKey, "req-internal-1")

	RecordConsumeLog(ctx, 1002, RecordConsumeLogParams{
		ChannelId:        1,
		PromptTokens:     0,
		CompletionTokens: 0,
		ModelName:        "claude-sonnet-4-5",
		TokenName:        "t1",
		Quota:            0,
		Content:          "host tool internal",
		TokenId:          2002,
		UseTimeSeconds:   5,
		IsStream:         false,
		Group:            "default",
		Internal:         true,
		Other: map[string]interface{}{
			"host_tool_internal": true,
		},
	})

	var logs []Log
	require.NoError(t, LOG_DB.Find(&logs).Error)
	require.Len(t, logs, 1)

	var records []TokenRecord
	require.NoError(t, LOG_DB.Find(&records).Error)
	require.Len(t, records, 0)
}

// TestRecordConsumeLogNonInternalStillRecordsTokenRecord 回归对照：
// 正常请求（Internal=false）必须继续写入 TokenRecord。
func TestRecordConsumeLogNonInternalStillRecordsTokenRecord(t *testing.T) {
	db := setupTokenRecordTestDB(t)
	require.NoError(t, db.AutoMigrate(&Log{}))

	oldLogConsumeEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = true
	t.Cleanup(func() {
		common.LogConsumeEnabled = oldLogConsumeEnabled
	})

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req, err := http.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	require.NoError(t, err)
	ctx.Request = req
	ctx.Set("username", "tester")
	ctx.Set(common.RequestIdKey, "req-normal-1")

	RecordConsumeLog(ctx, 1003, RecordConsumeLogParams{
		ChannelId:        1,
		PromptTokens:     120,
		CompletionTokens: 80,
		ModelName:        "claude-opus-4-1",
		TokenName:        "t1",
		Quota:            10,
		Content:          "test",
		TokenId:          2003,
		UseTimeSeconds:   5,
		IsStream:         false,
		Group:            "default",
	})

	var records []TokenRecord
	require.NoError(t, LOG_DB.Find(&records).Error)
	require.Len(t, records, 1)
	require.Equal(t, "claude-opus-4-1", records[0].ModelName)
	require.EqualValues(t, 80, records[0].TotalTokens)
}
