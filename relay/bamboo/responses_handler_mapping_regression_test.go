package bamboo_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/bamboo-services/bamboo-messages/bamboo/codec/responses"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
)

// TestResponsesHelperBambooBranchSendsMappedModel 直接驱动 /v1/responses 的
// ResponsesHelper：渠道配置 model_mapping 且 bamboo 中继开启时，上游收到的
// 请求体必须携带映射后的模型名。
//
// 回归背景：bamboo 分支曾先于 ModelMappedHelper 执行，上游收到未映射原名
// gemini-3.8-flash 后返回 "unknown provider for model gemini-3.8-flash"。
// 失败路径（上游 500）不涉计费结算，无需 billing 夹具。
func TestResponsesHelperBambooBranchSendsMappedModel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedModel string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		_ = json.Unmarshal(body, &parsed)
		if m, ok := parsed["model"].(string); ok {
			capturedModel = m
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":500,"message":"unknown provider for model gemini-3.8-flash","status":"INTERNAL"}}`))
	}))
	defer upstream.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	// 模拟 distributor 注入的渠道上下文（Gemini 渠道 → 原生 Gemini provider，
	// 请求体中的 model 字段即上游收到的模型名）
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeGemini)
	common.SetContextKey(c, constant.ContextKeyChannelId, 1)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gemini-3.8-flash")
	c.Set("model_mapping", `{"gemini-3.8-flash":"gemini-3.8-flash-high"}`)

	info := &relaycommon.RelayInfo{
		OriginModelName: "gemini-3.8-flash",
		RelayMode:       relayconstant.RelayModeResponses,
		Request: &dto.OpenAIResponsesRequest{
			Model: "gemini-3.8-flash",
			Input: json.RawMessage(`"hi"`),
		},
	}

	// 开启 bamboo 中继（测试后恢复，避免污染全局设置）
	settings := model_setting.GetBambooSettings()
	prev := settings.EnableBambooRelay
	settings.EnableBambooRelay = true
	defer func() { settings.EnableBambooRelay = prev }()

	apiErr := relay.ResponsesHelper(c, info)
	if apiErr == nil {
		t.Fatal("expected upstream 500 error, got nil")
	}

	wantPath := "/v1beta/models/gemini-3.8-flash-high:generateContent"
	if capturedModel != wantPath {
		t.Fatalf("upstream path = %q, want mapped %q", capturedModel, wantPath)
	}
}
