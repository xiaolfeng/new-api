package relay

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/bamboo"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/reasoning"

	"github.com/gin-gonic/gin"
)

const responsesToChatFallbackTTL = 5 * time.Minute

type responsesToChatFallbackEntry struct {
	ExpiresAt time.Time
}

var responsesToChatFallbackCache sync.Map

func shouldResponsesUseChatCompletions(info *relaycommon.RelayInfo) bool {
	if info == nil || !model_setting.IsResponsesToChatCompletionsEnabled() {
		return false
	}
	for _, format := range info.RequestConversionChain {
		if format == types.RelayFormatOpenAI {
			return false
		}
	}
	return true
}

func responsesToChatCompletionsFallbackCacheKey(info *relaycommon.RelayInfo) string {
	if info == nil || info.ChannelMeta == nil {
		return ""
	}
	parts := []string{
		strconv.Itoa(info.ChannelId),
		strconv.Itoa(info.ChannelType),
		strings.TrimSpace(info.UpstreamModelName),
		strings.TrimSpace(info.ApiVersion),
		strconv.Itoa(info.RelayMode),
	}
	return strings.Join(parts, "|")
}

func shouldResponsesUseChatCompletionsCached(info *relaycommon.RelayInfo) bool {
	if !shouldResponsesUseChatCompletions(info) {
		return false
	}
	key := responsesToChatCompletionsFallbackCacheKey(info)
	if key == "" {
		return false
	}
	raw, ok := responsesToChatFallbackCache.Load(key)
	if !ok {
		return false
	}
	entry, ok := raw.(responsesToChatFallbackEntry)
	if !ok || time.Now().After(entry.ExpiresAt) {
		responsesToChatFallbackCache.Delete(key)
		return false
	}
	return true
}

func markResponsesToChatCompletionsFallback(info *relaycommon.RelayInfo) {
	if info == nil || !model_setting.IsResponsesToChatCompletionsEnabled() {
		return
	}
	key := responsesToChatCompletionsFallbackCacheKey(info)
	if key == "" {
		return
	}
	responsesToChatFallbackCache.Store(key, responsesToChatFallbackEntry{
		ExpiresAt: time.Now().Add(responsesToChatFallbackTTL),
	})
}

func ResponsesHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)
	if info.RelayMode == relayconstant.RelayModeResponsesCompact &&
		!common.SupportsResponsesCompact(info.ChannelType, info.ApiType) {
		return types.NewErrorWithStatusCode(
			fmt.Errorf("unsupported endpoint %q for api type %d", "/v1/responses/compact", info.ApiType),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	var responsesReq *dto.OpenAIResponsesRequest
	switch req := info.Request.(type) {
	case *dto.OpenAIResponsesRequest:
		responsesReq = req
	case *dto.OpenAIResponsesCompactionRequest:
		// Only fields documented for POST /v1/responses/compact are forwarded:
		// model, input, instructions, previous_response_id, prompt_cache_key,
		// prompt_cache_options, prompt_cache_retention, service_tier.
		// Undocumented Codex-parity fields (tools, reasoning, text) are parsed
		// for client compatibility but intentionally not sent upstream.
		responsesReq = &dto.OpenAIResponsesRequest{
			Model:                req.Model,
			Input:                req.Input,
			Instructions:         req.Instructions,
			PreviousResponseID:   req.PreviousResponseID,
			ParallelToolCalls:    req.ParallelToolCalls,
			ServiceTier:          req.ServiceTier,
			PromptCacheKey:       req.PromptCacheKey,
			PromptCacheOptions:   req.PromptCacheOptions,
			PromptCacheRetention: req.PromptCacheRetention,
		}
	default:
		return types.NewErrorWithStatusCode(
			fmt.Errorf("invalid request type, expected dto.OpenAIResponsesRequest or dto.OpenAIResponsesCompactionRequest, got %T", info.Request),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	// bamboo 中继桥：灰度开启时由 bamboo 替代协议转换三段式内核
	if model_setting.GetBambooSettings().EnableBambooRelay && info.RelayMode != relayconstant.RelayModeResponsesCompact {
		// 归一化第三方扩展 effort（max→xhigh）：qwen 等上游只接受
		// none/minimal/low/medium/high/xhigh，直接透传 "max" 会被拒绝。
		if responsesReq.Reasoning != nil && responsesReq.Reasoning.Effort != "" {
			responsesReq.Reasoning.Effort = reasoning.NormalizeEffort(responsesReq.Reasoning.Effort)
		}
		bodyBytes, mErr := common.Marshal(responsesReq)
		if mErr != nil {
			return types.NewError(mErr, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		usage, relayErr := bamboo.ChatRelay(c, info, types.RelayFormatOpenAIResponses, bodyBytes)
		if relayErr != nil {
			if errors.Is(relayErr, bamboo.ErrUnsupportedProvider) {
				return originalResponsesRelay(c, info, responsesReq)
			}
			if usage != nil {
				// 失败但已收到部分交付：按实际用量结算（脱敏 + relay_error 标记），
				// 并标记不可重试，避免换渠道后因计费会话幂等漏计实际消耗。
				service.PostFailedRelayTextQuota(c, info, usage, relayErr)
				relayErr.MarkSkipRetry()
			}
			return relayErr
		}
		ConsumeResponsesQuota(c, info, usage)
		return nil
	}

	return originalResponsesRelay(c, info, responsesReq)
}

// originalResponsesRelay 是 new-api 原生三段式中继，作为 bamboo 未覆盖渠道的 fallback。
func originalResponsesRelay(c *gin.Context, info *relaycommon.RelayInfo, responsesReq *dto.OpenAIResponsesRequest) (newAPIError *types.NewAPIError) {
	// Responses→ChatCompletions conversion: prefer native Responses, and only
	// route directly through Chat Completions when a recent unsupported probe
	// has already established that this channel/model needs the compatibility path.
	passThroughGlobal := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	responsesToChatFallbackEnabled := info.RelayMode != relayconstant.RelayModeResponsesCompact &&
		!passThroughGlobal &&
		!info.ChannelSetting.PassThroughBodyEnabled &&
		shouldResponsesUseChatCompletions(info)
	if info.RelayMode != relayconstant.RelayModeResponsesCompact &&
		!passThroughGlobal &&
		!info.ChannelSetting.PassThroughBodyEnabled &&
		shouldResponsesUseChatCompletionsCached(info) {
		adaptor := GetAdaptor(info.ApiType)
		if adaptor == nil {
			return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
		}
		adaptor.Init(info)
		usage, newApiErr := responsesViaChatCompletions(c, info, adaptor, responsesReq)
		if newApiErr != nil {
			return newApiErr
		}

		ConsumeResponsesQuota(c, info, usage)
		return nil
	}

	adaptor, requestBody, closer, apiErr := PrepareResponsesRequest(c, info, responsesReq)
	if apiErr != nil {
		if responsesToChatFallbackEnabled && isResponsesToChatFallbackCandidate(apiErr) {
			usage, fallbackErr := responsesViaChatCompletions(c, info, adaptor, responsesReq)
			if fallbackErr != nil {
				return fallbackErr
			}
			markResponsesToChatCompletionsFallback(info)
			ConsumeResponsesQuota(c, info, usage)
			return nil
		}
		return apiErr
	}
	defer closer.Close()

	var httpResp *http.Response
	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		newAPIError = types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
		if responsesToChatFallbackEnabled && isResponsesToChatFallbackCandidate(newAPIError) {
			usage, fallbackErr := responsesViaChatCompletions(c, info, adaptor, responsesReq)
			if fallbackErr != nil {
				return fallbackErr
			}
			markResponsesToChatCompletionsFallback(info)
			ConsumeResponsesQuota(c, info, usage)
			return nil
		}
		return newAPIError
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")

	if resp != nil {
		httpResp = resp.(*http.Response)

		if httpResp.StatusCode != http.StatusOK {
			newAPIError = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
			// reset status code 重置状态码
			service.ResetStatusCode(newAPIError, statusCodeMappingStr)
			if responsesToChatFallbackEnabled && info.SendResponseCount == 0 && isResponsesToChatFallbackCandidate(newAPIError) {
				usage, fallbackErr := responsesViaChatCompletions(c, info, adaptor, responsesReq)
				if fallbackErr != nil {
					return fallbackErr
				}
				markResponsesToChatCompletionsFallback(info)
				ConsumeResponsesQuota(c, info, usage)
				return nil
			}
			return newAPIError
		}
	}

	usage, newAPIError := adaptor.DoResponse(c, httpResp, info)
	if newAPIError != nil {
		// reset status code 重置状态码
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return newAPIError
	}

	usageDto := usage.(*dto.Usage)
	if info.RelayMode == relayconstant.RelayModeResponsesCompact {
		originModelName := info.OriginModelName
		originPriceData := info.PriceData

		_, err := helper.ModelPriceHelper(c, info, info.GetEstimatePromptTokens(), &types.TokenCountMeta{})
		if err != nil {
			info.OriginModelName = originModelName
			info.PriceData = originPriceData
			return types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithSkipRetry(), types.ErrOptionWithStatusCode(http.StatusBadRequest))
		}
		service.PostTextConsumeQuota(c, info, usageDto, nil)

		info.OriginModelName = originModelName
		info.PriceData = originPriceData
		return nil
	}

	ConsumeResponsesQuota(c, info, usageDto)
	return nil
}

// ConsumeResponsesQuota applies the same settlement dispatch to HTTP and
// WebSocket Responses usage. Compact requests keep their separate repricing.
func ConsumeResponsesQuota(c *gin.Context, info *relaycommon.RelayInfo, usageDto *dto.Usage) {
	if usageDto == nil {
		usageDto = &dto.Usage{}
	}
	containAudioTokens := usageDto.CompletionTokenDetails.AudioTokens > 0 || usageDto.PromptTokensDetails.AudioTokens > 0
	containsAudioRatios := ratio_setting.ContainsAudioRatio(info.OriginModelName) || ratio_setting.ContainsAudioCompletionRatio(info.OriginModelName)

	if strings.HasPrefix(info.OriginModelName, "gpt-4o-audio") || (containAudioTokens && containsAudioRatios) {
		service.PostAudioConsumeQuota(c, info, usageDto, "")
		return
	}
	service.PostTextConsumeQuota(c, info, usageDto, nil)
}

func isResponsesToChatFallbackCandidate(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	switch err.StatusCode {
	case http.StatusBadRequest, http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusNotImplemented:
	default:
		if err.GetErrorCode() != types.ErrorCodeConvertRequestFailed && err.GetErrorCode() != types.ErrorCodeDoRequestFailed {
			return false
		}
	}

	message := strings.ToLower(err.Error() + " " + err.ToOpenAIError().Message)
	if strings.Contains(message, "responses") ||
		strings.Contains(message, "/v1/responses") ||
		strings.Contains(message, "endpoint") {
		return strings.Contains(message, "not support") ||
			strings.Contains(message, "unsupported") ||
			strings.Contains(message, "unknown url") ||
			strings.Contains(message, "not found") ||
			strings.Contains(message, "no such endpoint") ||
			strings.Contains(message, "invalid endpoint") ||
			strings.Contains(message, "endpoint not found")
	}
	return strings.Contains(message, "responses api is not supported") ||
		strings.Contains(message, "responses is not supported")
}
