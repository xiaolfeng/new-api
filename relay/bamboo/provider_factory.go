package bamboo

import (
	bambooanthropic "github.com/bamboo-services/bamboo-messages/provider/anthropic"
	bamboogemini "github.com/bamboo-services/bamboo-messages/provider/gemini"
	bamboocompletions "github.com/bamboo-services/bamboo-messages/provider/openai/completions"
	bambooresponses "github.com/bamboo-services/bamboo-messages/provider/openai/responses"
	"github.com/bamboo-services/bamboo-messages/provider"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

// newProvider 根据 RelayInfo.ApiType 构造对应的 bamboo provider。
//
// ApiKey/ChannelBaseUrl 经 *ChannelMeta 嵌入提升访问（relay_info.go:74-92,198）。
// 未覆盖的 ApiType（AWS/讯飞/腾讯/智谱v3/Coze/Dify/百度v1/阿里等）返回包裹
// ErrUnsupportedProvider 的错误，调用方（ChatRelay）据此 fallback 到原生链路。
func newProvider(info *relaycommon.RelayInfo) (provider.Provider, *types.NewAPIError) {
	apiKey := info.ApiKey
	baseURL := info.ChannelBaseUrl

	switch info.ApiType {
	case constant.APITypeAnthropic:
		return bambooanthropic.NewProviderWithOptions(
			bambooanthropic.WithAPIKey(apiKey),
			bambooanthropic.WithBaseURL(baseURL),
		), nil

	case constant.APITypeGemini:
		return bamboogemini.NewProviderWithOptions(
			bamboogemini.WithAPIKey(apiKey),
			bamboogemini.WithBaseURL(baseURL),
		), nil

	case constant.APITypeCodex:
		return bambooresponses.NewResponsesProviderWithOptions(
			bambooresponses.WithAPIKey(apiKey),
			bambooresponses.WithBaseURL(baseURL),
		), nil

	case constant.APITypeOpenAI, constant.APITypeXai:
		// OpenAI 官方 + xAI grok-3-mini 支持 max_completion_tokens + reasoning_effort，
		// 行为对齐最新 OpenAI 标准（openai/adaptor.go:317-320 / xai/adaptor.go:77-79 均做
		// MaxTokens→MaxCompletionTokens 转换），不需要 legacy 兼容模式。
		return bamboocompletions.NewCompletionsProviderWithOptions(
			bamboocompletions.WithAPIKey(apiKey),
			bamboocompletions.WithBaseURL(baseURL),
		), nil

	case constant.APITypeDeepSeek, constant.APITypeMoonshot,
		constant.APITypeSiliconFlow, constant.APITypeMistral,
		constant.APITypeZhipuV4,
		constant.APITypePerplexity, constant.APITypeCohere,
		constant.APITypeMiniMax, constant.APITypeBaiduV2,
		constant.APITypeOpenRouter, constant.APITypeXinference:
		// 其余 OpenAI 兼容渠道统一使用 max_tokens（旧字段名），需要 Legacy 兼容模式：
		//   - 使用 max_tokens 而非 max_completion_tokens（各适配器均用 MaxTokens 字段）
		//   - parallel_tool_calls 仅在有工具时发送（避免不支持该参数的端点报错）
		//   - 跳过 reasoning_effort 自动映射（这些服务商均不支持该字段）
		//   - 保留 thinking 透传（DeepSeek-V4 / SiliconFlow 等需要）
		return bamboocompletions.NewCompletionsProviderWithOptions(
			bamboocompletions.WithAPIKey(apiKey),
			bamboocompletions.WithBaseURL(baseURL),
			bamboocompletions.WithLegacyCompat(),
		), nil

	default:
		// AWS/讯飞/腾讯/智谱v3/Coze/Dify 等特殊协议，bamboo 不覆盖
		return nil, types.NewError(ErrUnsupportedProvider, types.ErrorCodeInvalidApiType)
	}
}
