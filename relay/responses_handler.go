package relay

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	appconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/openaicompat"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func ResponsesHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)
	if info.RelayMode == relayconstant.RelayModeResponsesCompact {
		switch info.ApiType {
		case appconstant.APITypeOpenAI, appconstant.APITypeCodex:
		default:
			return types.NewErrorWithStatusCode(
				fmt.Errorf("unsupported endpoint %q for api type %d", "/v1/responses/compact", info.ApiType),
				types.ErrorCodeInvalidRequest,
				http.StatusBadRequest,
				types.ErrOptionWithSkipRetry(),
			)
		}
	}

	var responsesReq *dto.OpenAIResponsesRequest
	switch req := info.Request.(type) {
	case *dto.OpenAIResponsesRequest:
		responsesReq = req
	case *dto.OpenAIResponsesCompactionRequest:
		responsesReq = &dto.OpenAIResponsesRequest{
			Model:              req.Model,
			Input:              req.Input,
			Instructions:       req.Instructions,
			PreviousResponseID: req.PreviousResponseID,
		}
	default:
		return types.NewErrorWithStatusCode(
			fmt.Errorf("invalid request type, expected dto.OpenAIResponsesRequest or dto.OpenAIResponsesCompactionRequest, got %T", info.Request),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	request, err := common.DeepCopy(responsesReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to GeneralOpenAIRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	err = helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	// Responses→ChatCompletions conversion: prefer native Responses, and only
	// route directly through Chat Completions when a recent unsupported probe
	// has already established that this channel/model needs the compatibility path.
	passThroughGlobal := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	responsesToChatFallbackEnabled := info.RelayMode != relayconstant.RelayModeResponsesCompact &&
		!passThroughGlobal &&
		!info.ChannelSetting.PassThroughBodyEnabled &&
		openaicompat.ShouldResponsesUseChatCompletions(info)
	if info.RelayMode != relayconstant.RelayModeResponsesCompact &&
		!passThroughGlobal &&
		!info.ChannelSetting.PassThroughBodyEnabled &&
		openaicompat.ShouldResponsesUseChatCompletionsCached(info) {
		usage, newApiErr := responsesViaChatCompletions(c, info, adaptor, responsesReq)
		if newApiErr != nil {
			return newApiErr
		}

		postResponsesUsageQuota(c, info, usage)
		return nil
	}

	var requestBody io.Reader
	if model_setting.GetGlobalSettings().PassThroughRequestEnabled || info.ChannelSetting.PassThroughBodyEnabled {
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			return types.NewError(err, types.ErrorCodeReadRequestBodyFailed, types.ErrOptionWithSkipRetry())
		}
		requestBody = common.ReaderOnly(storage)
	} else {
		convertedRequest, err := adaptor.ConvertOpenAIResponsesRequest(c, info, *request)
		if err != nil {
			newAPIError = types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
			if responsesToChatFallbackEnabled && isResponsesToChatFallbackCandidate(newAPIError) {
				usage, fallbackErr := responsesViaChatCompletions(c, info, adaptor, responsesReq)
				if fallbackErr != nil {
					return fallbackErr
				}
				openaicompat.MarkResponsesToChatCompletionsFallback(info)
				postResponsesUsageQuota(c, info, usage)
				return nil
			}
			return newAPIError
		}
		relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)
		jsonData, err := common.Marshal(convertedRequest)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		// remove disabled fields for OpenAI Responses API
		jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		// apply param override
		if len(info.ParamOverride) > 0 {
			jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
			if err != nil {
				return newAPIErrorFromParamOverride(err)
			}
		}

		if common.DebugEnabled {
			println("requestBody: ", string(jsonData))
		}
		requestBody = bytes.NewBuffer(jsonData)
	}

	var httpResp *http.Response
	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		newAPIError = types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
		if responsesToChatFallbackEnabled && isResponsesToChatFallbackCandidate(newAPIError) {
			usage, fallbackErr := responsesViaChatCompletions(c, info, adaptor, responsesReq)
			if fallbackErr != nil {
				return fallbackErr
			}
			openaicompat.MarkResponsesToChatCompletionsFallback(info)
			postResponsesUsageQuota(c, info, usage)
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
				openaicompat.MarkResponsesToChatCompletionsFallback(info)
				postResponsesUsageQuota(c, info, usage)
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

	postResponsesUsageQuota(c, info, usageDto)
	return nil
}

func postResponsesUsageQuota(c *gin.Context, info *relaycommon.RelayInfo, usageDto *dto.Usage) {
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
