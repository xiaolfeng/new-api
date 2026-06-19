package bamboo

import (
	"github.com/gin-gonic/gin"

	bambooanthropic "github.com/bamboo-services/bamboo-messages/provider/anthropic"
	bamboogemini "github.com/bamboo-services/bamboo-messages/provider/gemini"
	bamboocompletions "github.com/bamboo-services/bamboo-messages/provider/openai/completions"
	bambooresponses "github.com/bamboo-services/bamboo-messages/provider/openai/responses"
	"github.com/bamboo-services/bamboo-messages/provider"

	channelconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

// resolveBaseURL 将渠道配置的 BaseURL 解析为实际请求 URL。
//
// 对 coding-plan 快捷键（glm-coding-plan / kimi-coding-plan / doubao-coding-plan 等）
// 查 ChannelSpecialBases 映射表，按入口 RelayFormat 选择对应的 Claude 或 OpenAI 端点。
// 普通完整 URL 直接原样返回。
func resolveBaseURL(info *relaycommon.RelayInfo) string {
	baseURL := info.ChannelBaseUrl
	if baseURL == "" {
		return baseURL
	}
	specialPlan, ok := channelconstant.ChannelSpecialBases[baseURL]
	if !ok {
		return baseURL
	}
	if info.RelayFormat == types.RelayFormatClaude && specialPlan.ClaudeBaseURL != "" {
		return specialPlan.ClaudeBaseURL
	}
	if specialPlan.OpenAIBaseURL != "" {
		return specialPlan.OpenAIBaseURL
	}
	return baseURL
}

// newProvider 根据 RelayInfo.ApiType 构造对应的 bamboo provider。
//
// ApiKey/ChannelBaseUrl 经 *ChannelMeta 嵌入提升访问（relay_info.go:74-92,198）。
// 未覆盖的 ApiType（AWS/讯飞/腾讯/智谱v3/Coze/Dify/百度v1/阿里等）返回包裹
// ErrUnsupportedProvider 的错误，调用方（ChatRelay）据此 fallback 到原生链路。
//
// c 用于解析 header passthrough/placeholder 规则（与原生 DoApiRequest 一致），
// 传入 nil 时仅应用 ChannelsOverride 中的显式 header。
func newProvider(c *gin.Context, info *relaycommon.RelayInfo) (provider.Provider, *types.NewAPIError) {
	apiKey := info.ApiKey
	baseURL := resolveBaseURL(info)

	// 解析自定义 header（与原生链路 api_request.go:325 一致，支持 passthrough/regex/placeholder）
	var headers map[string]string
	if c != nil {
		resolved, err := channel.ResolveHeaderOverride(info, c)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeChannelHeaderOverrideInvalid)
		}
		headers = resolved
	}

	switch info.ApiType {
	case channelconstant.APITypeAnthropic:
		opts := []bambooanthropic.Option{
			bambooanthropic.WithAPIKey(apiKey),
			bambooanthropic.WithBaseURL(baseURL),
		}
		for k, v := range headers {
			opts = append(opts, bambooanthropic.WithHeader(k, v))
		}
		return bambooanthropic.NewProviderWithOptions(opts...), nil

	case channelconstant.APITypeGemini:
		opts := []bamboogemini.Option{
			bamboogemini.WithAPIKey(apiKey),
			bamboogemini.WithBaseURL(baseURL),
		}
		for k, v := range headers {
			opts = append(opts, bamboogemini.WithHeader(k, v))
		}
		return bamboogemini.NewProviderWithOptions(opts...), nil

	case channelconstant.APITypeCodex:
		opts := []bambooresponses.Option{
			bambooresponses.WithAPIKey(apiKey),
			bambooresponses.WithBaseURL(baseURL),
		}
		for k, v := range headers {
			opts = append(opts, bambooresponses.WithHeader(k, v))
		}
		return bambooresponses.NewResponsesProviderWithOptions(opts...), nil

	case channelconstant.APITypeOpenAI, channelconstant.APITypeXai:
		// OpenAI 官方 + xAI grok-3-mini 支持 max_completion_tokens + reasoning_effort，
		// 行为对齐最新 OpenAI 标准（openai/adaptor.go:317-320 / xai/adaptor.go:77-79 均做
		// MaxTokens→MaxCompletionTokens 转换），不需要 legacy 兼容模式。
		return buildCompletionsProvider(apiKey, baseURL, headers, false), nil

	case channelconstant.APITypeDeepSeek, channelconstant.APITypeMoonshot,
		channelconstant.APITypeSiliconFlow, channelconstant.APITypeMistral,
		channelconstant.APITypeZhipuV4,
		channelconstant.APITypePerplexity, channelconstant.APITypeCohere,
		channelconstant.APITypeMiniMax, channelconstant.APITypeBaiduV2,
		channelconstant.APITypeOpenRouter, channelconstant.APITypeXinference:
		// 其余 OpenAI 兼容渠道统一使用 max_tokens（旧字段名），需要 Legacy 兼容模式：
		//   - 使用 max_tokens 而非 max_completion_tokens（各适配器均用 MaxTokens 字段）
		//   - parallel_tool_calls 仅在有工具时发送（避免不支持该参数的端点报错）
		//   - 跳过 reasoning_effort 自动映射（这些服务商均不支持该字段）
		//   - 保留 thinking 透传（DeepSeek-V4 / SiliconFlow 等需要）
		return buildCompletionsProvider(apiKey, baseURL, headers, true), nil

	default:
		// AWS/讯飞/腾讯/智谱v3/Coze/Dify 等特殊协议，bamboo 不覆盖
		return nil, types.NewError(ErrUnsupportedProvider, types.ErrorCodeInvalidApiType)
	}
}

// buildCompletionsProvider 构造 OpenAI Completions provider，附加自定义 header。
func buildCompletionsProvider(apiKey, baseURL string, headers map[string]string, legacyCompat bool) provider.Provider {
	opts := []bamboocompletions.Option{
		bamboocompletions.WithAPIKey(apiKey),
		bamboocompletions.WithBaseURL(baseURL),
	}
	if legacyCompat {
		opts = append(opts, bamboocompletions.WithLegacyCompat())
	}
	for k, v := range headers {
		opts = append(opts, bamboocompletions.WithHeader(k, v))
	}
	return bamboocompletions.NewCompletionsProviderWithOptions(opts...)
}
