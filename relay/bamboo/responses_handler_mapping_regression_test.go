package bamboo_test

import (
	"encoding/json"
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
// ResponsesHelper，回归生产事故「unknown provider for model gemini-3.8-flash」：
//
//  1. bamboo 分支必须先执行 ModelMappedHelper（历史上它先于映射执行，
//     渠道 model_mapping 从未作用于该入口）；
//  2. 映射目标名带 -high 后缀时，需在 thinking_model_blacklist 注册保留，
//     否则 ApplyReasoningModelSuffix 会把 -high 剥成 effort 并把基础名发上游，
//     上游（CLIProxyAPI 只注册变体名）随即报 unknown provider。
//
// 失败路径（上游 500）不涉计费结算，无需 billing 夹具。
func TestResponsesHelperBambooBranchSendsMappedModel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Gemini provider 把模型放在 URL path（/v1beta/models/{model}:generateContent），
		// 与生产事故链路一致，捕获 path 即可判别映射是否生效。
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":500,"message":"unknown provider for model gemini-3.8-flash","status":"INTERNAL"}}`))
	}))
	defer upstream.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	// 模拟 distributor 注入的渠道上下文（Gemini 渠道 → 原生 Gemini provider）
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

	// 生产修复组合之一：开启 bamboo 中继
	bambooSettings := model_setting.GetBambooSettings()
	prevBamboo := bambooSettings.EnableBambooRelay
	bambooSettings.EnableBambooRelay = true
	defer func() { bambooSettings.EnableBambooRelay = prevBamboo }()

	// 生产修复组合之二：thinking 后缀黑名单保留 -high 变体名（测试后恢复）
	globalSettings := model_setting.GetGlobalSettings()
	prevBlacklist := append([]string(nil), globalSettings.ThinkingModelBlacklist...)
	globalSettings.ThinkingModelBlacklist = []string{`re:^gemini-.*-(high|medium|low)$`}
	defer func() { globalSettings.ThinkingModelBlacklist = prevBlacklist }()

	apiErr := relay.ResponsesHelper(c, info)
	if apiErr == nil {
		t.Fatal("expected upstream 500 error, got nil")
	}

	wantPath := "/v1beta/models/gemini-3.8-flash-high:generateContent"
	if capturedPath != wantPath {
		t.Fatalf("upstream path = %q, want mapped %q", capturedPath, wantPath)
	}
}
