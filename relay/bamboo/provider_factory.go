package bamboo

import (
	"context"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	bambooanthropic "github.com/bamboo-services/bamboo-messages/provider/anthropic"
	bamboogemini "github.com/bamboo-services/bamboo-messages/provider/gemini"
	bamboocompletions "github.com/bamboo-services/bamboo-messages/provider/openai/completions"
	bambooresponses "github.com/bamboo-services/bamboo-messages/provider/openai/responses"
	"github.com/bamboo-services/bamboo-messages/provider"

	"github.com/QuantumNous/new-api/constant"
	channelconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

// versionRegex 匹配 URL path 以 /v + 数字 结尾的模式（如 /v1, /v4）。
var versionRegex = regexp.MustCompile(`/v\d+$`)

// resolveBaseURL 将渠道配置的 BaseURL 解析为实际请求 URL。
//
// 对 coding-plan 快捷键（glm-coding-plan / kimi-coding-plan / doubao-coding-plan 等）
// 查 ChannelSpecialBases 映射表，按上游格式（resolveUpstreamRelayFormat）选择对应的
// Claude 或 OpenAI 端点。auto 模式（upstreamRelayFormat 为空）时回退到入口 RelayFormat。
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
	// 按上游格式选 BaseURL（手动覆盖 > ApiType 推断 > 入口格式回退）
	upstreamFmt := resolveUpstreamRelayFormat(info)
	if upstreamFmt == "" {
		upstreamFmt = info.RelayFormat // auto 模式回退到入口格式（旧行为）
	}
	if upstreamFmt == types.RelayFormatClaude && specialPlan.ClaudeBaseURL != "" {
		return specialPlan.ClaudeBaseURL
	}
	if upstreamFmt == types.RelayFormatOpenAIResponses && specialPlan.OpenAIBaseURL != "" {
		return specialPlan.OpenAIBaseURL
	}
	if upstreamFmt == types.RelayFormatGemini && specialPlan.ClaudeBaseURL != "" {
		return specialPlan.ClaudeBaseURL // Gemini 无 special plan，走 Claude 端点
	}
	// 默认 OpenAI（含 OpenAI/OpenAIResponses）
	if specialPlan.OpenAIBaseURL != "" {
		return specialPlan.OpenAIBaseURL
	}
	return baseURL
}

// ensureOpenAIBaseURL 确保 OpenAI Provider 的 BaseURL 包含 /v1 版本路径。
//
// openai-go SDK 会在 BaseURL 后自动拼接 /chat/completions，
// 如果用户只配了纯域名（如 https://ai.akass.cn），SDK 拼出 https://ai.akass.cn/chat/completions
// → 缺少 /v1 前缀 → 上游返回 text/html 错误页面。
//
// 规则：
//   - 空字符串 → 原样返回
//   - path 以 /v + 数字 结尾（如 /v1, /v4, /v3）→ 原样返回
//   - 其他 → 去除尾部斜杠后追加 /v1
func ensureOpenAIBaseURL(baseURL string) string {
	if baseURL == "" {
		return baseURL
	}
	trimmed := strings.TrimRight(baseURL, "/")
	if versionRegex.MatchString(trimmed) {
		return baseURL
	}
	return trimmed + "/v1"
}

// resolveUpstreamFormat 决定 bamboo 实际使用的上游 provider 类型。
//
// 优先级：渠道 ChannelOtherSettings.BambooUpstreamFormat 手动覆盖 > ApiType 自动推断。
// 当用户在渠道设置中手动指定了 "openai"/"anthropic"/"gemini"/"responses" 时，
// 强制使用对应协议与上游通信（覆盖自动推断），适用于"OpenAI 兼容渠道实发 Anthropic 格式"等场景。
// 为空或 "auto" 时走 ApiType 自动推断逻辑（原行为不变）。
func resolveUpstreamFormat(info *relaycommon.RelayInfo) dto.BambooUpstreamFormatType {
	manual := dto.BambooUpstreamFormatType(info.ChannelOtherSettings.BambooUpstreamFormat)
	switch manual {
	case dto.BambooUpstreamFormatOpenAI, dto.BambooUpstreamFormatAnthropic,
		dto.BambooUpstreamFormatGemini, dto.BambooUpstreamFormatResponses:
		return manual
	default:
		return dto.BambooUpstreamFormatAuto
	}
}

// bambooUpstreamFormatToRelayFormat 将渠道配置的 BambooUpstreamFormat 转为 RelayFormat。
// BambooUpstreamFormatAuto / 空字符串 返回 ""，语义为"未覆盖，由调用方回退到入口格式"。
func bambooUpstreamFormatToRelayFormat(fmt dto.BambooUpstreamFormatType) types.RelayFormat {
	switch fmt {
	case dto.BambooUpstreamFormatOpenAI:
		return types.RelayFormatOpenAI
	case dto.BambooUpstreamFormatAnthropic:
		return types.RelayFormatClaude
	case dto.BambooUpstreamFormatGemini:
		return types.RelayFormatGemini
	case dto.BambooUpstreamFormatResponses:
		return types.RelayFormatOpenAIResponses
	default:
		return ""
	}
}

// apiTypeToRelayFormat 从渠道 ApiType 推断上游 RelayFormat。
// 用于 BambooUpstreamFormatAuto 时，保持与 buildProviderByApiType 一致的格式推断。
// 未知 ApiType 返回 ""。
func apiTypeToRelayFormat(apiType int) types.RelayFormat {
	switch apiType {
	case constant.APITypeAnthropic:
		return types.RelayFormatClaude
	case constant.APITypeGemini:
		return types.RelayFormatGemini
	case constant.APITypeCodex:
		return types.RelayFormatOpenAIResponses
	case constant.APITypeOpenAI, constant.APITypeXai:
		return types.RelayFormatOpenAI
	case constant.APITypeDeepSeek, constant.APITypeMoonshot,
		constant.APITypeSiliconFlow, constant.APITypeMistral,
		constant.APITypeZhipuV4,
		constant.APITypePerplexity, constant.APITypeCohere,
		constant.APITypeMiniMax, constant.APITypeBaiduV2,
		constant.APITypeOpenRouter, constant.APITypeXinference:
		return types.RelayFormatOpenAI
	default:
		return ""
	}
}

// resolveUpstreamRelayFormat 解析实际上游 RelayFormat，供 bridge.go 更新格式链路。
// 优先级：手动指定 BambooUpstreamFormat > ApiType 自动推断。
// 返回 "" 表示无法确定（调用方回退到入口格式）。
func resolveUpstreamRelayFormat(info *relaycommon.RelayInfo) types.RelayFormat {
	upstreamFmt := resolveUpstreamFormat(info)
	if upstreamFmt != dto.BambooUpstreamFormatAuto {
		return bambooUpstreamFormatToRelayFormat(upstreamFmt)
	}
	return apiTypeToRelayFormat(info.ApiType)
}

// newProvider 根据 RelayInfo（含渠道级上游格式覆盖）构造对应的 bamboo provider。
//
// 返回 (provider, upstreamRelayFormat, error)。
// upstreamRelayFormat 为解析出的实际上游协议格式（openai/claude/gemini/openai_responses），
// 空字符串表示无法推断（调用方回退到入口格式）。
//
// 流程：
//  1. 读取渠道 ChannelOtherSettings.BambooUpstreamFormat，若手动指定了 openai/anthropic/gemini/responses，
//     则强制使用对应协议（覆盖 ApiType 自动推断）
//  2. 否则按 info.ApiType 自动分发到 native provider
//  3. 未覆盖的 ApiType（AWS/讯飞/腾讯/智谱v3/Coze/Dify/百度v1/阿里等）返回包裹
//     ErrUnsupportedProvider 的错误，调用方（ChatRelay）据此 fallback 到原生链路
//
// c 用于解析 header passthrough/placeholder 规则（与原生 DoApiRequest 一致），
// 传入 nil 时仅应用 ChannelsOverride 中的显式 header。
//
// 当 info.ChannelMeta.ParamOverride 非空时，构造一个 provider.RequestInterceptor
// 注入到 provider 中，复用 relaycommon.ApplyParamOverrideWithRelayInfo 的全部 23 种
// 参数覆盖能力（含 header 操作通过 info 自动 sync）。ParamOverride 为空时不注入，
// 行为与升级前完全一致（零开销）。
func newProvider(c *gin.Context, info *relaycommon.RelayInfo) (provider.Provider, types.RelayFormat, *types.NewAPIError) {
	apiKey := info.ApiKey
	baseURL := resolveBaseURL(info)

	var headers map[string]string
	if c != nil {
		resolved, err := channel.ResolveHeaderOverride(info, c)
		if err != nil {
			return nil, "", types.NewError(err, types.ErrorCodeChannelHeaderOverrideInvalid)
		}
		headers = resolved
	}

	// 构造参数覆盖拦截器（若配置了 ParamOverride）
	paramOverrideInterceptor := buildParamOverrideInterceptor(info)

	// 解析上游格式（手动覆盖 > ApiType 推断），与 provider 构造逻辑共享
	upstreamRelayFormat := resolveUpstreamRelayFormat(info)

	legacyCompat := info.ChannelOtherSettings.IsBambooLegacyCompat()

	upstreamFmt := resolveUpstreamFormat(info)
	if upstreamFmt != dto.BambooUpstreamFormatAuto {
		p, apiErr := buildProviderByFormat(upstreamFmt, apiKey, baseURL, headers, legacyCompat, paramOverrideInterceptor)
		if apiErr != nil {
			return nil, "", apiErr
		}
		return p, upstreamRelayFormat, nil
	}

	p, apiErr := buildProviderByApiType(info.ApiType, apiKey, baseURL, headers, legacyCompat, paramOverrideInterceptor)
	if apiErr != nil {
		return nil, "", apiErr
	}
	return p, upstreamRelayFormat, nil
}

// buildParamOverrideInterceptor 根据 RelayInfo.ChannelMeta.ParamOverride 构造一个
// provider.RequestInterceptor，在 SDK 发起上游 HTTP 请求前应用参数覆盖。
//
// 复用 relaycommon.ApplyParamOverrideWithRelayInfo 的全部 23 种操作能力
// （set/delete/regex/conditions/header ops 等）。Header 类操作（set_header 等）
// 通过 ApplyParamOverrideWithRelayInfo 内部 sync 到 info.RuntimeHeadersOverride，
// 后续在 newProvider 的 channel.ResolveHeaderOverride 中已被合并到 headers 参数
// （注：当前实现下 header 操作走 info sync 路径，body 操作走 interceptor 路径，
// 两者正交无冲突）。
//
// 当 ParamOverride 为空时返回 nil，调用方据此跳过注入（保持零开销）。
func buildParamOverrideInterceptor(info *relaycommon.RelayInfo) provider.RequestInterceptor {
	if info == nil || info.ChannelMeta == nil || len(info.ChannelMeta.ParamOverride) == 0 {
		return nil
	}
	return func(ctx context.Context, body []byte) ([]byte, error) {
		return relaycommon.ApplyParamOverrideWithRelayInfo(body, info)
	}
}

// buildProviderByFormat 按用户手动指定的上游协议格式构造 provider。
//
// legacyCompat 由 UI 的 bamboo_legacy_compat 字段控制（传统模式开关），
// 当用户启用传统模式时，OpenAI Completions provider 使用旧字段名 max_tokens
// 而非 max_completion_tokens，且不发送 reasoning_effort / parallel_tool_calls。
//
// interceptor 可以为 nil（无参数覆盖时），此时 4 个 Provider 的 WithInterceptor
// option 不会被触发，构造行为与升级前一致。
func buildProviderByFormat(fmt dto.BambooUpstreamFormatType, apiKey, baseURL string,
	headers map[string]string, legacyCompat bool,
	interceptor provider.RequestInterceptor) (provider.Provider, *types.NewAPIError) {

	switch fmt {
	case dto.BambooUpstreamFormatAnthropic:
		return newAnthropicProvider(apiKey, baseURL, headers, legacyCompat, interceptor), nil
	case dto.BambooUpstreamFormatGemini:
		return newGeminiProvider(apiKey, baseURL, headers, interceptor), nil
	case dto.BambooUpstreamFormatResponses:
		return newResponsesProvider(apiKey, baseURL, headers, interceptor), nil
	case dto.BambooUpstreamFormatOpenAI:
		return buildCompletionsProvider(apiKey, baseURL, headers, legacyCompat, interceptor), nil
	default:
		return nil, types.NewError(ErrUnsupportedProvider, types.ErrorCodeInvalidApiType)
	}
}

// buildProviderByApiType 按渠道 ApiType 自动推断上游协议（原 newProvider switch 逻辑）。
// auto 模式下 legacyCompat 由调用方从 ChannelOtherSettings.BambooLegacyCompat 读取。
func buildProviderByApiType(apiType int, apiKey, baseURL string, headers map[string]string,
	legacyCompat bool, interceptor provider.RequestInterceptor) (provider.Provider, *types.NewAPIError) {
	switch apiType {
	case constant.APITypeAnthropic:
		return newAnthropicProvider(apiKey, baseURL, headers, legacyCompat, interceptor), nil

	case constant.APITypeGemini:
		return newGeminiProvider(apiKey, baseURL, headers, interceptor), nil

	case constant.APITypeCodex:
		return newResponsesProvider(apiKey, baseURL, headers, interceptor), nil

	case constant.APITypeOpenAI, constant.APITypeXai,
		constant.APITypeDeepSeek, constant.APITypeMoonshot,
		constant.APITypeSiliconFlow, constant.APITypeMistral,
		constant.APITypeZhipuV4,
		constant.APITypePerplexity, constant.APITypeCohere,
		constant.APITypeMiniMax, constant.APITypeBaiduV2,
		constant.APITypeOpenRouter, constant.APITypeXinference:
		return buildCompletionsProvider(apiKey, baseURL, headers, legacyCompat, interceptor), nil

	default:
		return nil, types.NewError(ErrUnsupportedProvider, types.ErrorCodeInvalidApiType)
	}
}

// --- 以下为各 provider 的工厂方法 ---
// debug 信息由 bridge.go 通过 FormatRelayInput/FormatRelayParsed/FormatDebugRequest
// 收集到 RelayInfo.BambooDebug，不再调用 provider.SetDebug()。

func newAnthropicProvider(apiKey, baseURL string, headers map[string]string, legacyCompat bool, interceptor provider.RequestInterceptor) provider.Provider {
	opts := []bambooanthropic.Option{
		bambooanthropic.WithAPIKey(apiKey),
		bambooanthropic.WithBaseURL(baseURL),
	}
	for k, v := range headers {
		opts = append(opts, bambooanthropic.WithHeader(k, v))
	}
	if interceptor != nil {
		opts = append(opts, bambooanthropic.WithInterceptor(interceptor))
	}
	if legacyCompat {
		opts = append(opts, bambooanthropic.WithLegacyCompat())
	}
	return bambooanthropic.NewProviderWithOptions(opts...)
}

func newGeminiProvider(apiKey, baseURL string, headers map[string]string, interceptor provider.RequestInterceptor) provider.Provider {
	opts := []bamboogemini.Option{
		bamboogemini.WithAPIKey(apiKey),
		bamboogemini.WithBaseURL(baseURL),
	}
	for k, v := range headers {
		opts = append(opts, bamboogemini.WithHeader(k, v))
	}
	if interceptor != nil {
		opts = append(opts, bamboogemini.WithInterceptor(interceptor))
	}
	return bamboogemini.NewProviderWithOptions(opts...)
}

func newResponsesProvider(apiKey, baseURL string, headers map[string]string, interceptor provider.RequestInterceptor) provider.Provider {
	baseURL = ensureOpenAIBaseURL(baseURL)
	opts := []bambooresponses.Option{
		bambooresponses.WithAPIKey(apiKey),
		bambooresponses.WithBaseURL(baseURL),
	}
	for k, v := range headers {
		opts = append(opts, bambooresponses.WithHeader(k, v))
	}
	if interceptor != nil {
		opts = append(opts, bambooresponses.WithInterceptor(interceptor))
	}
	return bambooresponses.NewResponsesProviderWithOptions(opts...)
}

// buildCompletionsProvider 构造 OpenAI Completions provider，附加自定义 header。
func buildCompletionsProvider(apiKey, baseURL string, headers map[string]string, legacyCompat bool, interceptor provider.RequestInterceptor) provider.Provider {
	baseURL = ensureOpenAIBaseURL(baseURL)
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
	if interceptor != nil {
		opts = append(opts, bamboocompletions.WithInterceptor(interceptor))
	}
	return bamboocompletions.NewCompletionsProviderWithOptions(opts...)
}
