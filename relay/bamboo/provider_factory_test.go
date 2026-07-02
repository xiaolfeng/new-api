package bamboo

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// makeInfo 构造一个最小可用的 RelayInfo（补 apiKey/baseURL 避免 provider 空值 panic）。
// 注意：RelayInfo 通过指针嵌入 *ChannelMeta（relay_info.go:198），
// 必须显式初始化 ChannelMeta，否则访问 ApiType/ApiKey 等提升字段会 nil 解引用 panic。
func makeInfo(apiType int) *relaycommon.RelayInfo {
	return makeInfoWithBaseURL(apiType, "https://api.example.com")
}

// makeInfoWithBaseURL 构造指定 baseURL 的 RelayInfo，用于测试 coding-plan 快捷 URL。
func makeInfoWithBaseURL(apiType int, baseURL string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiType:        apiType,
			ApiKey:         "test-key",
			ChannelBaseUrl: baseURL,
		},
	}
}

func TestNewProvider_SupportedOpenAICompatible(t *testing.T) {
	// OpenAI 兼容渠道应返回非 nil provider 且无错误
	supportedTypes := []int{
		constant.APITypeOpenAI,
		constant.APITypeDeepSeek,
		constant.APITypeMoonshot,
		constant.APITypeSiliconFlow,
		constant.APITypeMistral,
		constant.APITypeXai,
		constant.APITypeZhipuV4,
		constant.APITypePerplexity,
		constant.APITypeCohere,
		constant.APITypeMiniMax,
		constant.APITypeBaiduV2,
		constant.APITypeOpenRouter,
		constant.APITypeXinference,
	}
	for _, apiType := range supportedTypes {
		info := makeInfo(apiType)
		p, _, err := newProvider(nil, info)
		if err != nil {
			t.Errorf("APIType %d: expected nil err, got %v", apiType, err)
			continue
		}
		if p == nil {
			t.Errorf("APIType %d: expected non-nil provider", apiType)
		}
	}
}

func TestNewProvider_SupportedNativeProtocols(t *testing.T) {
	nativeTypes := []int{
		constant.APITypeAnthropic,
		constant.APITypeGemini,
		constant.APITypeCodex,
	}
	for _, apiType := range nativeTypes {
		info := makeInfo(apiType)
		p, _, err := newProvider(nil, info)
		if err != nil {
			t.Errorf("APIType %d: expected nil err, got %v", apiType, err)
			continue
		}
		if p == nil {
			t.Errorf("APIType %d: expected non-nil provider", apiType)
		}
	}
}

func TestNewProvider_UnsupportedReturnsFallback(t *testing.T) {
	// AWS/讯飞/腾讯等未覆盖渠道应返回包裹 ErrUnsupportedProvider 的错误
	unsupportedTypes := []int{
		constant.APITypeAws,
		constant.APITypeXunfei,
		constant.APITypeTencent,
		constant.APITypeZhipu, // 智谱 v3 JWT
		constant.APITypeCoze,
		constant.APITypeDify,
		constant.APITypeBaidu, // 千帆 v1
		constant.APITypeAli,   // DashScope
	}
	for _, apiType := range unsupportedTypes {
		info := makeInfo(apiType)
		p, _, err := newProvider(nil, info)
		if p != nil {
			t.Errorf("APIType %d: expected nil provider for unsupported", apiType)
			continue
		}
		if err == nil {
			t.Errorf("APIType %d: expected non-nil err for unsupported", apiType)
			continue
		}
		// *types.NewAPIError 实现了 Unwrap，errors.Is 链可达 ErrUnsupportedProvider
		if !errors.Is(err, ErrUnsupportedProvider) {
			t.Errorf("APIType %d: expected ErrUnsupportedProvider in chain, got %v", apiType, err)
		}
	}
}

// === S1: coding-plan 快捷 URL 映射 — OpenAI 格式 ===

func TestResolveBaseURL_CodingPlanOpenAI(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		format   types.RelayFormat
		expected string
	}{
		{
			name:     "glm-coding-plan OpenAI",
			baseURL:  "glm-coding-plan",
			format:   types.RelayFormatOpenAI,
			expected: "https://open.bigmodel.cn/api/coding/paas/v4",
		},
		{
			name:     "kimi-coding-plan OpenAI",
			baseURL:  "kimi-coding-plan",
			format:   types.RelayFormatOpenAI,
			expected: "https://api.kimi.com/coding/v1",
		},
		{
			name:     "doubao-coding-plan OpenAI",
			baseURL:  "doubao-coding-plan",
			format:   types.RelayFormatOpenAI,
			expected: "https://ark.cn-beijing.volces.com/api/coding/v3",
		},
		{
			name:     "glm-coding-plan-international OpenAI",
			baseURL:  "glm-coding-plan-international",
			format:   types.RelayFormatOpenAI,
			expected: "https://api.z.ai/api/coding/paas/v4",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := makeInfoWithBaseURL(constant.APITypeZhipuV4, tt.baseURL)
			info.RelayFormat = tt.format
			got := resolveBaseURL(info)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// === S2: coding-plan 快捷 URL 映射 — Claude 格式 ===

func TestResolveBaseURL_CodingPlanClaude(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		expected string
	}{
		{
			name:     "glm-coding-plan Claude",
			baseURL:  "glm-coding-plan",
			expected: "https://open.bigmodel.cn/api/anthropic",
		},
		{
			name:     "kimi-coding-plan Claude",
			baseURL:  "kimi-coding-plan",
			expected: "https://api.kimi.com/coding/",
		},
		{
			name:     "doubao-coding-plan Claude",
			baseURL:  "doubao-coding-plan",
			expected: "https://ark.cn-beijing.volces.com/api/coding",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := makeInfoWithBaseURL(constant.APITypeZhipuV4, tt.baseURL)
			info.RelayFormat = types.RelayFormatClaude
			info.ChannelOtherSettings.BambooUpstreamFormat = "anthropic"
			got := resolveBaseURL(info)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// === Bug 修复核心：入口=Claude 但显式指定上游=OpenAI ===

func TestResolveBaseURL_ExplicitUpstreamOpenAI(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		expected string
	}{
		{"glm-coding-plan", "glm-coding-plan", "https://open.bigmodel.cn/api/coding/paas/v4"},
		{"kimi-coding-plan", "kimi-coding-plan", "https://api.kimi.com/coding/v1"},
		{"doubao-coding-plan", "doubao-coding-plan", "https://ark.cn-beijing.volces.com/api/coding/v3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := makeInfoWithBaseURL(constant.APITypeAnthropic, tt.baseURL)
			info.RelayFormat = types.RelayFormatClaude
			info.ChannelOtherSettings.BambooUpstreamFormat = "openai"
			got := resolveBaseURL(info)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// 入口=OpenAI 但显式指定上游=Anthropic

func TestResolveBaseURL_ExplicitUpstreamAnthropic(t *testing.T) {
	info := makeInfoWithBaseURL(constant.APITypeOpenAI, "glm-coding-plan")
	info.RelayFormat = types.RelayFormatOpenAI
	info.ChannelOtherSettings.BambooUpstreamFormat = "anthropic"
	got := resolveBaseURL(info)
	assert.Equal(t, "https://open.bigmodel.cn/api/anthropic", got)
}

// === S4: 普通 URL 不受影响 ===

func TestResolveBaseURL_NormalURL(t *testing.T) {
	info := makeInfoWithBaseURL(constant.APITypeOpenAI, "https://api.openai.com")
	info.RelayFormat = types.RelayFormatOpenAI
	got := resolveBaseURL(info)
	assert.Equal(t, "https://api.openai.com", got)
}

func TestResolveBaseURL_PlainURL(t *testing.T) {
	info := makeInfoWithBaseURL(constant.APITypeOpenAI, "https://custom.example.com")
	info.RelayFormat = types.RelayFormatOpenAI
	got := resolveBaseURL(info)
	assert.Equal(t, "https://custom.example.com", got)
}

func TestResolveBaseURL_EmptyURL(t *testing.T) {
	info := makeInfoWithBaseURL(constant.APITypeOpenAI, "")
	info.RelayFormat = types.RelayFormatOpenAI
	got := resolveBaseURL(info)
	assert.Equal(t, "", got)
}

// === S3: 自定义 header 透传 ===

func TestNewProvider_CustomHeadersForwarded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)

	info := makeInfo(constant.APITypeOpenAI)
	info.HeadersOverride = map[string]any{
		"X-Tenant-Id": "tenant-123",
		"X-Trace-Id":  "trace-456",
	}

	p, _, err := newProvider(c, info)
	if err != nil {
		t.Fatalf("expected no error, got: %v (type: %T)", err, err)
	}
	assert.NotNil(t, p)
}

func TestBambooUpstreamFormatToRelayFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    dto.BambooUpstreamFormatType
		expected types.RelayFormat
	}{
		{"OpenAI", dto.BambooUpstreamFormatOpenAI, types.RelayFormatOpenAI},
		{"Anthropic", dto.BambooUpstreamFormatAnthropic, types.RelayFormatClaude},
		{"Gemini", dto.BambooUpstreamFormatGemini, types.RelayFormatGemini},
		{"Responses", dto.BambooUpstreamFormatResponses, types.RelayFormatOpenAIResponses},
		{"Auto(empty)", dto.BambooUpstreamFormatAuto, ""},
		{"Unknown", "unknown_format", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bambooUpstreamFormatToRelayFormat(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestApiTypeToRelayFormat(t *testing.T) {
	tests := []struct {
		name     string
		apiType  int
		expected types.RelayFormat
	}{
		{"Anthropic", constant.APITypeAnthropic, types.RelayFormatClaude},
		{"Gemini", constant.APITypeGemini, types.RelayFormatGemini},
		{"Codex(Responses)", constant.APITypeCodex, types.RelayFormatOpenAIResponses},
		{"OpenAI", constant.APITypeOpenAI, types.RelayFormatOpenAI},
		{"Xai", constant.APITypeXai, types.RelayFormatOpenAI},
		{"DeepSeek", constant.APITypeDeepSeek, types.RelayFormatOpenAI},
		{"Moonshot", constant.APITypeMoonshot, types.RelayFormatOpenAI},
		{"SiliconFlow", constant.APITypeSiliconFlow, types.RelayFormatOpenAI},
		{"Mistral", constant.APITypeMistral, types.RelayFormatOpenAI},
		{"ZhipuV4", constant.APITypeZhipuV4, types.RelayFormatOpenAI},
		{"Perplexity", constant.APITypePerplexity, types.RelayFormatOpenAI},
		{"Cohere", constant.APITypeCohere, types.RelayFormatOpenAI},
		{"MiniMax", constant.APITypeMiniMax, types.RelayFormatOpenAI},
		{"BaiduV2", constant.APITypeBaiduV2, types.RelayFormatOpenAI},
		{"OpenRouter", constant.APITypeOpenRouter, types.RelayFormatOpenAI},
		{"Xinference", constant.APITypeXinference, types.RelayFormatOpenAI},
		{"Unknown(9999)", 9999, ""},
		{"Aws(unsupported)", constant.APITypeAws, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := apiTypeToRelayFormat(tt.apiType)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestResolveUpstreamRelayFormat(t *testing.T) {
	tests := []struct {
		name           string
		bambooUpstream string
		apiType        int
		expected       types.RelayFormat
	}{
		{"manual OpenAI", "openai", constant.APITypeAnthropic, types.RelayFormatOpenAI},
		{"manual Anthropic", "anthropic", constant.APITypeOpenAI, types.RelayFormatClaude},
		{"manual Gemini", "gemini", constant.APITypeOpenAI, types.RelayFormatGemini},
		{"manual Responses", "responses", constant.APITypeOpenAI, types.RelayFormatOpenAIResponses},
		{"auto empty + OpenAI ApiType", "", constant.APITypeOpenAI, types.RelayFormatOpenAI},
		{"auto + Anthropic ApiType", "auto", constant.APITypeAnthropic, types.RelayFormatClaude},
		{"auto + Gemini ApiType", "", constant.APITypeGemini, types.RelayFormatGemini},
		{"auto + Codex ApiType", "", constant.APITypeCodex, types.RelayFormatOpenAIResponses},
		{"auto + Legacy(DeepSeek)", "", constant.APITypeDeepSeek, types.RelayFormatOpenAI},
		{"auto + Unknown ApiType", "", 9999, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := makeInfo(tt.apiType)
			info.ChannelOtherSettings.BambooUpstreamFormat = tt.bambooUpstream
			got := resolveUpstreamRelayFormat(info)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// === ensureOpenAIBaseURL: /v1 自动补全 ===

func TestEnsureOpenAIBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		expected string
	}{
		// 纯域名 → 自动补 /v1
		{"plain domain", "https://ai.akass.cn", "https://ai.akass.cn/v1"},
		{"plain domain with trailing slash", "https://ai.akass.cn/", "https://ai.akass.cn/v1"},

		// 已有版本号 → 原样返回
		{"has /v1", "https://api.openai.com/v1", "https://api.openai.com/v1"},
		{"has /v4", "https://open.bigmodel.cn/api/coding/paas/v4", "https://open.bigmodel.cn/api/coding/paas/v4"},
		{"has /v3", "https://ark.cn-beijing.volces.com/api/coding/v3", "https://ark.cn-beijing.volces.com/api/coding/v3"},
		{"has /v1 with trailing slash", "https://api.openai.com/v1/", "https://api.openai.com/v1/"},

		// 空字符串 → 原样返回
		{"empty string", "", ""},

		// http 协议
		{"http plain domain", "http://localhost:8080", "http://localhost:8080/v1"},
		{"http with port and /v1", "http://localhost:8080/v1", "http://localhost:8080/v1"},

		// 带路径但无版本号 → 补 /v1
		{"path without version", "https://api.example.com/openai", "https://api.example.com/openai/v1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ensureOpenAIBaseURL(tt.baseURL)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestNewAnthropicProvider_LegacyCompat(t *testing.T) {
	// legacyCompat=true 不应 panic 并返回非 nil provider
	p := newAnthropicProvider("test-key", "https://api.example.com", nil, true, nil)
	assert.NotNil(t, p)

	// legacyCompat=false 也应返回非 nil provider
	p = newAnthropicProvider("test-key", "https://api.example.com", nil, false, nil)
	assert.NotNil(t, p)
}

func TestNewProvider_BambooLegacyCompat(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name         string
		apiType      int
		upstreamFmt  string
		legacyCompat *bool
	}{
		{"nil default fallback to false", constant.APITypeAnthropic, "", nil},
		{"force legacy true", constant.APITypeAnthropic, "", boolPtr(true)},
		{"force legacy false", constant.APITypeAnthropic, "", boolPtr(false)},
		{"manual anthropic format with legacy", constant.APITypeOpenAI, "anthropic", boolPtr(true)},
		{"manual openai format with legacy", constant.APITypeOpenAI, "openai", boolPtr(true)},
		{"auto openai apitype with legacy", constant.APITypeOpenAI, "", boolPtr(true)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := makeInfo(tt.apiType)
			info.ChannelOtherSettings.BambooLegacyCompat = tt.legacyCompat
			info.ChannelOtherSettings.BambooUpstreamFormat = tt.upstreamFmt

			p, _, err := newProvider(nil, info)
			if err != nil {
				t.Fatalf("newProvider returned error: %v (type: %T, msg: %q)", err, err, err.Error())
			}
			assert.NotNil(t, p)
		})
	}
}
