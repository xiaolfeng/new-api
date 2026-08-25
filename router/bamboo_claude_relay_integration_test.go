package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func anthropicSSE(kind, data string) string {
	return "event: " + kind + "\ndata: " + data + "\n\n"
}

// 模拟 Anthropic 上游：工具回调轮 → text + tool_use
func mockAnthropicUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	var sb strings.Builder
	sb.WriteString(anthropicSSE("message_start", `{"type":"message_start","message":{"id":"msg_up_01","type":"message","role":"assistant","model":"claude-sonnet-4-5-20250929","content":[],"usage":{"input_tokens":100,"output_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":10}}}`))
	sb.WriteString(anthropicSSE("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`))
	sb.WriteString(anthropicSSE("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"好的"}}`))
	sb.WriteString(anthropicSSE("content_block_stop", `{"type":"content_block_stop","index":0}`))
	sb.WriteString(anthropicSSE("content_block_start", `{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_02DEF","name":"Read","input":{}}}`))
	sb.WriteString(anthropicSSE("content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"file_path\":\"main.go\"}"}}`))
	sb.WriteString(anthropicSSE("content_block_stop", `{"type":"content_block_stop","index":1}`))
	sb.WriteString(anthropicSSE("message_delta", `{"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":12}}`))
	sb.WriteString(anthropicSSE("message_stop", `{"type":"message_stop"}`))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sb.String())
	}))
	t.Cleanup(srv.Close)
	return srv
}

func setupBambooIntegrationDB(t *testing.T) *httptest.Server {
	t.Helper()

	gin.SetMode(gin.TestMode)
	originalIsMasterNode := common.IsMasterNode
	originalRedisEnabled := common.RedisEnabled
	originalSQLitePath := common.SQLitePath
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()
	originalSQLDSN, hadSQLDSN := os.LookupEnv("SQL_DSN")

	common.IsMasterNode = false
	common.RedisEnabled = false
	common.SQLitePath = fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, os.Setenv("SQL_DSN", "local"))
	require.NoError(t, model.InitDB())
	model.LOG_DB = model.DB
	require.NoError(t, model.DB.AutoMigrate(
		&model.Channel{}, &model.Token{}, &model.User{}, &model.UserSession{},
		&model.AuthFlow{}, &model.ExternalIdentityClaim{}, &model.PasskeyCredential{},
		&model.Option{}, &model.Redemption{}, &model.Ability{}, &model.Log{},
		&model.TokenRecord{}, &model.ToolLog{}, &model.Midjourney{}, &model.TopUp{},
		&model.QuotaData{}, &model.Task{}, &model.Model{}, &model.Vendor{},
		&model.PrefillGroup{}, &model.Setup{}, &model.TwoFA{}, &model.TwoFABackupCode{},
		&model.Checkin{}, &model.SubscriptionOrder{}, &model.UserSubscription{},
		&model.SubscriptionPreConsumeRecord{}, &model.CustomOAuthProvider{},
		&model.UserOAuthBinding{}, &model.PerfMetric{}, &model.SystemInstance{},
		&model.SystemTask{}, &model.SystemTaskLock{}, &model.CasbinRule{}, &model.AuthzRole{},
	))

	// main.go 的初始化步骤
	ratio_setting.InitRatioSettings()
	service.InitHttpClient()

	t.Cleanup(func() {
		if sqlDB, err := model.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
		common.IsMasterNode = originalIsMasterNode
		common.RedisEnabled = originalRedisEnabled
		common.SQLitePath = originalSQLitePath
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		if hadSQLDSN {
			require.NoError(t, os.Setenv("SQL_DSN", originalSQLDSN))
		} else {
			require.NoError(t, os.Unsetenv("SQL_DSN"))
		}
	})

	// mock 上游
	upstream := mockAnthropicUpstream(t)

	// 用户 + token
	user := model.User{
		Username: "bamboo-it-user",
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    1000000,
	}
	require.NoError(t, model.DB.Create(&user).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		UserId:         user.Id,
		Key:            "bambooittestkey",
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)

	// Anthropic 渠道指向 mock 上游
	url := upstream.URL
	channel := model.Channel{
		Name:    "mock-anthropic",
		Type:    constant.ChannelTypeAnthropic,
		Key:     "sk-mock-key",
		BaseURL: &url,
		Models:  "claude-sonnet-4-5-20250929",
		Status:  common.ChannelStatusEnabled,
		Group:   "default",
	}
	require.NoError(t, model.DB.Create(&channel).Error)
	// 渠道-模型映射 Ability
	require.NoError(t, model.DB.Create(&model.Ability{
		Group:     "default",
		Model:     "claude-sonnet-4-5-20250929",
		ChannelId: channel.Id,
		Enabled:   true,
	}).Error)

	// bamboo 中继开 + host 工具开（与用户部署保持一致：Claude Code 自带 web_search 会命中 host 链路）
	bs := model_setting.GetBambooSettings()
	origRelay := bs.EnableBambooRelay
	origHost := bs.EnableHostTools
	bs.EnableBambooRelay = true
	bs.EnableHostTools = true
	t.Cleanup(func() {
		bs.EnableBambooRelay = origRelay
		bs.EnableHostTools = origHost
	})

	// 消费日志详细记录开
	rs := operation_setting.GetRetrySetting()
	origRecord := rs.RecordConsumeLogDetailEnabled
	rs.RecordConsumeLogDetailEnabled = true
	t.Cleanup(func() { rs.RecordConsumeLogDetailEnabled = origRecord })

	return upstream
}

// Claude Code 工具回调轮，走完整 HTTP 链路
func TestBambooFullRelayClaudeCodeToolCallback(t *testing.T) {
	setupBambooIntegrationDB(t)

	body := `{
  "model": "claude-sonnet-4-5-20250929",
  "max_tokens": 64000,
  "stream": true,
  "messages": [
    {"role": "user", "content": "看看仓库结构"},
    {"role": "assistant", "content": [
      {"type": "text", "text": "我先看一下。"},
      {"type": "tool_use", "id": "toolu_01ABC", "name": "Bash", "input": {"command": "ls -la"}}
    ]},
    {"role": "user", "content": [
      {"type": "tool_result", "tool_use_id": "toolu_01ABC", "content": "total 32\ndrwxr-xr-x  file1.go"}
    ]}
  ],
  "tools": [
    {"name": "Bash", "description": "run shell", "input_schema": {"type": "object"}},
    {"name": "web_search", "description": "Search the web", "input_schema": {"type": "object", "properties": {"query": {"type": "string"}}, "required": ["query"]}}
  ]
}`

	engine := gin.New()
	SetRelayRouter(engine)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer sk-bambooittestkey")
	request.Header.Set("User-Agent", "claude-cli/2.1.88 (user, cli)")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("x-api-key", "bambooittestkey")
	request.Header.Set("anthropic-version", "2023-06-01")

	engine.ServeHTTP(recorder, request)
	t.Logf("status=%d body=%s", recorder.Code, recorder.Body.String())
	require.Equal(t, http.StatusOK, recorder.Code)

	// 查日志记录
	var logs []model.Log
	require.NoError(t, model.LOG_DB.Where("user_id = ?", 0).Or("username = ?", "bamboo-it-user").Order("id desc").Limit(5).Find(&logs).Error)
	require.NotEmpty(t, logs, "should have consume log")

	var foundInteraction string
	for _, log := range logs {
		t.Logf("log record=%s", log.Record)
		if strings.TrimSpace(log.Record) != "" {
			_, interactionType, _, _, _ := model.ExtractLogDetailSummaries(log.Record)
			foundInteraction = interactionType
		}
	}
	require.Equal(t, "回调", foundInteraction, "tool-result callback turn must classify as callback")
}

// Claude Code 用户指令轮：user text → assistant tool_use（无 tool_result）
func TestBambooFullRelayClaudeCodeUserTurnToolUse(t *testing.T) {
	setupBambooIntegrationDB(t)

	body := `{
  "model": "claude-sonnet-4-5-20250929",
  "max_tokens": 64000,
  "stream": true,
  "messages": [
    {"role": "user", "content": "看看仓库结构"}
  ],
  "tools": [
    {"name": "Bash", "description": "run shell", "input_schema": {"type": "object"}}
  ]
}`

	engine := gin.New()
	SetRelayRouter(engine)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer sk-bambooittestkey")
	request.Header.Set("User-Agent", "claude-cli/2.1.88 (user, cli)")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("x-api-key", "bambooittestkey")
	request.Header.Set("anthropic-version", "2023-06-01")

	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var logs []model.Log
	require.NoError(t, model.LOG_DB.Where("username = ?", "bamboo-it-user").Order("id desc").Limit(5).Find(&logs).Error)
	require.NotEmpty(t, logs)

	for _, log := range logs {
		t.Logf("log record=%s", log.Record)
	}
	var found string
	for _, log := range logs {
		if strings.TrimSpace(log.Record) != "" {
			_, interactionType, _, _, _ := model.ExtractLogDetailSummaries(log.Record)
			found = interactionType
		}
	}
	require.Equal(t, "回调", found, "user input + assistant tool_use turn must classify as callback")
}