package relay

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/bamboo/imagerec"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	relaykittypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

func init() {
	imagerec.SetHopFunc(executeImageRecognizeHop)
}

func executeImageRecognizeHop(parentCtx *gin.Context, parent *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest, live imagerec.CaptionLive) (caption string, usage *dto.Usage, hopErr *relaykittypes.NewAPIError) {
	started := time.Now()
	defer func() {
		recordImageRecognizeToolLog(parent, request, started, caption, hopErr)
	}()
	if parent == nil || request == nil {
		return "", nil, relaykittypes.NewError(fmt.Errorf("image recognition hop missing request"), relaykittypes.ErrorCodeInvalidRequest, relaykittypes.ErrOptionWithSkipRetry())
	}
	st := model_setting.GetBambooSettings()
	if st == nil {
		return "", nil, relaykittypes.NewError(fmt.Errorf("bamboo settings unavailable"), relaykittypes.ErrorCodeInvalidRequest, relaykittypes.ErrOptionWithSkipRetry())
	}

	channel, err := model.GetChannelById(st.ImageRecognizeChannelId, true)
	if err != nil {
		return "", nil, relaykittypes.NewError(fmt.Errorf("image recognition channel %d not found", st.ImageRecognizeChannelId), relaykittypes.ErrorCodeGetChannelFailed, relaykittypes.ErrOptionWithSkipRetry())
	}
	if channel.Status != rootcommon.ChannelStatusEnabled {
		return "", nil, relaykittypes.NewError(fmt.Errorf("image recognition channel %d is disabled", channel.Id), relaykittypes.ErrorCodeGetChannelFailed, relaykittypes.ErrOptionWithSkipRetry())
	}
	modelName := strings.TrimSpace(st.ImageRecognizeModel)
	if !channelAllowsModel(channel, modelName) {
		return "", nil, relaykittypes.NewError(fmt.Errorf("image recognition model %s is not on channel %d", modelName, channel.Id), relaykittypes.ErrorCodeInvalidRequest, relaykittypes.ErrOptionWithSkipRetry())
	}

	apiType, ok := rootcommon.ChannelType2APIType(channel.Type)
	if !ok {
		return "", nil, relaykittypes.NewError(fmt.Errorf("image recognition channel type %d is not supported", channel.Type), relaykittypes.ErrorCodeInvalidApiType, relaykittypes.ErrOptionWithSkipRetry())
	}
	adaptor := GetAdaptor(apiType)
	if adaptor == nil {
		return "", nil, relaykittypes.NewError(fmt.Errorf("image recognition adaptor missing for api type %d", apiType), relaykittypes.ErrorCodeInvalidApiType, relaykittypes.ErrOptionWithSkipRetry())
	}

	key, keyIndex, keyErr := channel.GetNextEnabledKey()
	if keyErr != nil {
		// 识别子渠道无可用 key：包成带 skipRetry 的渠道错误，避免被外层
		// 当成主对话渠道错误处理而误禁用主渠道。
		return "", nil, relaykittypes.NewError(
			fmt.Errorf("image recognition channel %d has no available key", channel.Id),
			relaykittypes.ErrorCodeChannelNoAvailableKey, relaykittypes.ErrOptionWithSkipRetry())
	}

	stream := live != nil && request.IsStream(nil)
	inner, rec := newImageRecognizeGinContext(parentCtx, stream)
	bindImageRecognizeChannel(inner, channel, modelName, key, keyIndex)
	timeout := time.Duration(st.ClampImageRecognizeTimeout()) * time.Millisecond
	ctx, cancel := context.WithTimeout(inner.Request.Context(), timeout)
	defer cancel()
	inner.Request = inner.Request.WithContext(ctx)

	child := buildImageRecognizeRelayInfo(parent, request, modelName, stream)
	child.InitChannelMeta(inner)
	child.ImageRecognizeInner = true
	child.OriginModelName = modelName
	child.UpstreamModelName = modelName
	if child.ChannelMeta != nil {
		child.ChannelMeta.UpstreamModelName = modelName
	}

	meta := request.GetTokenCountMeta()
	tokens, tokenErr := service.EstimateRequestToken(inner, meta, child)
	if tokenErr != nil {
		return "", nil, relaykittypes.NewError(tokenErr, relaykittypes.ErrorCodeCountTokenFailed, relaykittypes.ErrOptionWithSkipRetry())
	}
	child.SetEstimatePromptTokens(tokens)
	priceData, priceErr := helper.ModelPriceHelper(inner, child, tokens, meta)
	if priceErr != nil {
		return "", nil, relaykittypes.NewError(priceErr, relaykittypes.ErrorCodeModelPriceError, relaykittypes.ErrOptionWithStatusCode(http.StatusBadRequest))
	}
	if !priceData.FreeModel {
		if preErr := service.PreConsumeBilling(inner, priceData.QuotaToPreConsume, child); preErr != nil {
			return "", nil, preErr
		}
		defer func() {
			if child.Billing != nil && child.Billing.NeedsRefund() {
				child.Billing.Refund(inner)
			}
		}()
	}

	adaptor.Init(child)
	converted, convErr := adaptor.ConvertOpenAIRequest(inner, child, request)
	if convErr != nil {
		return "", nil, relaykittypes.NewError(convErr, relaykittypes.ErrorCodeConvertRequestFailed, relaykittypes.ErrOptionWithSkipRetry())
	}
	jsonData, marshalErr := rootcommon.Marshal(converted)
	if marshalErr != nil {
		return "", nil, relaykittypes.NewError(marshalErr, relaykittypes.ErrorCodeJsonMarshalFailed, relaykittypes.ErrOptionWithSkipRetry())
	}
	jsonData, fieldErr := relaycommon.RemoveDisabledFields(jsonData, child.ChannelOtherSettings, child.ChannelSetting.PassThroughBodyEnabled)
	if fieldErr != nil {
		return "", nil, relaykittypes.NewError(fieldErr, relaykittypes.ErrorCodeConvertRequestFailed, relaykittypes.ErrOptionWithSkipRetry())
	}
	if len(child.ParamOverride) > 0 {
		var poErr error
		jsonData, poErr = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, child)
		if poErr != nil {
			return "", nil, relaykittypes.NewError(poErr, relaykittypes.ErrorCodeChannelParamOverrideInvalid, relaykittypes.ErrOptionWithSkipRetry())
		}
	}

	respAny, doErr := adaptor.DoRequest(inner, child, bytes.NewReader(jsonData))
	if doErr != nil {
		// 上游瞬时错误（5xx/网络），可重试：不标 skipRetry。
		return "", nil, relaykittypes.NewError(doErr, relaykittypes.ErrorCodeDoRequestFailed)
	}
	httpResp, _ := respAny.(*http.Response)
	if stream {
		var scanErr error
		caption, usage, scanErr = consumeVisionStream(httpResp, live)
		if scanErr != nil {
			// 响应体解析失败，可重试：不标 skipRetry。
			return caption, usage, relaykittypes.NewError(scanErr, relaykittypes.ErrorCodeBadResponseBody)
		}
		if strings.TrimSpace(caption) == "" {
			// 空内容，可重试换 key 再试：不标 skipRetry。
			return caption, usage, relaykittypes.NewError(fmt.Errorf("image recognition returned empty content"), relaykittypes.ErrorCodeBadResponseBody)
		}
		service.PostTextConsumeQuota(inner, child, usage, []string{"image_recognize"})
		return caption, usage, nil
	}
	usageAny, respErr := adaptor.DoResponse(inner, httpResp, child)
	if respErr != nil {
		return "", nil, respErr
	}

	usage, _ = usageAny.(*dto.Usage)
	caption = extractCaptionFromRecorder(rec)
	if strings.TrimSpace(caption) == "" {
		// 空内容，可重试换 key 再试：不标 skipRetry。
		return caption, usage, relaykittypes.NewError(fmt.Errorf("image recognition returned empty content"), relaykittypes.ErrorCodeBadResponseBody)
	}

	service.PostTextConsumeQuota(inner, child, usage, []string{"image_recognize"})
	return caption, usage, nil
}

func channelAllowsModel(channel *model.Channel, name string) bool {
	if channel == nil || name == "" {
		return false
	}
	for _, item := range channel.GetModels() {
		item = strings.TrimSpace(item)
		if item == "*" || item == name {
			return true
		}
	}
	return false
}

func newImageRecognizeGinContext(parent *gin.Context, stream bool) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	inner, _ := gin.CreateTestContext(rec)
	if parent != nil && parent.Request != nil {
		inner.Request = parent.Request.Clone(parent.Request.Context())
	} else {
		inner.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	}
	if inner.Request.Header == nil {
		inner.Request.Header = make(http.Header)
	}
	inner.Request.Header.Set("Content-Type", "application/json")
	if stream {
		inner.Request.Header.Set("Accept", "text/event-stream")
	} else {
		inner.Request.Header.Set("Accept", "application/json")
	}
	if parent != nil {
		copyGinKeysForImageRecognize(parent, inner)
	}
	return inner, rec
}

func bindImageRecognizeChannel(c *gin.Context, channel *model.Channel, modelName, key string, keyIndex int) {
	if c == nil || channel == nil {
		return
	}
	rootcommon.SetContextKey(c, constant.ContextKeyChannelId, channel.Id)
	rootcommon.SetContextKey(c, constant.ContextKeyChannelName, channel.Name)
	rootcommon.SetContextKey(c, constant.ContextKeyChannelType, channel.Type)
	rootcommon.SetContextKey(c, constant.ContextKeyChannelCreateTime, channel.CreatedTime)
	rootcommon.SetContextKey(c, constant.ContextKeyChannelSetting, channel.GetSetting())
	rootcommon.SetContextKey(c, constant.ContextKeyChannelOtherSetting, channel.GetOtherSettings())
	rootcommon.SetContextKey(c, constant.ContextKeyChannelParamOverride, channel.GetParamOverride())
	rootcommon.SetContextKey(c, constant.ContextKeyChannelHeaderOverride, channel.GetHeaderOverride())
	rootcommon.SetContextKey(c, constant.ContextKeyChannelKey, key)
	rootcommon.SetContextKey(c, constant.ContextKeyChannelBaseUrl, channel.GetBaseURL())
	rootcommon.SetContextKey(c, constant.ContextKeyChannelModelMapping, channel.GetModelMapping())
	rootcommon.SetContextKey(c, constant.ContextKeyChannelStatusCodeMapping, channel.GetStatusCodeMapping())
	rootcommon.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, channel.ChannelInfo.IsMultiKey)
	rootcommon.SetContextKey(c, constant.ContextKeyChannelMultiKeyIndex, keyIndex)
	rootcommon.SetContextKey(c, constant.ContextKeyOriginalModel, modelName)
	if channel.OpenAIOrganization != nil && *channel.OpenAIOrganization != "" {
		rootcommon.SetContextKey(c, constant.ContextKeyChannelOrganization, *channel.OpenAIOrganization)
	}
}

func copyGinKeysForImageRecognize(parent, inner *gin.Context) {
	keep := []constant.ContextKey{
		constant.ContextKeyUserId,
		constant.ContextKeyUserGroup,
		constant.ContextKeyUsingGroup,
		constant.ContextKeyUserQuota,
		constant.ContextKeyUserEmail,
		constant.ContextKeyUserSetting,
		constant.ContextKeyTokenId,
		constant.ContextKeyTokenKey,
		constant.ContextKeyTokenUnlimited,
		constant.ContextKeyTokenGroup,
		constant.ContextKeyRequestStartTime,
	}
	for _, key := range keep {
		if val, ok := parent.Get(string(key)); ok {
			inner.Set(string(key), val)
		}
	}
	if val, ok := parent.Get(rootcommon.RequestIdKey); ok {
		inner.Set(rootcommon.RequestIdKey, val)
	}
}

func buildImageRecognizeRelayInfo(parent *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest, modelName string, stream bool) *relaycommon.RelayInfo {
	info := &relaycommon.RelayInfo{
		Request:             request,
		UserId:              parent.UserId,
		UsingGroup:          parent.UsingGroup,
		UserGroup:           parent.UserGroup,
		UserQuota:           parent.UserQuota,
		UserEmail:           parent.UserEmail,
		UserSetting:         parent.UserSetting,
		TokenId:             parent.TokenId,
		TokenKey:            parent.TokenKey,
		TokenUnlimited:      parent.TokenUnlimited,
		TokenGroup:          parent.TokenGroup,
		RequestId:           parent.RequestId + "-imagerec",
		OriginModelName:     modelName,
		RelayMode:           relayconstant.RelayModeChatCompletions,
		RelayFormat:         relaykittypes.RelayFormatOpenAI,
		RequestURLPath:      "/v1/chat/completions",
		IsStream:            stream,
		StartTime:           time.Now(),
		ImageRecognizeInner: true,
	}
	if parent.RequestId == "" {
		info.RequestId = rootcommon.NewRequestId() + "-imagerec"
	}
	return info
}

func extractCaptionFromRecorder(rec *httptest.ResponseRecorder) string {
	if rec == nil {
		return ""
	}
	body := rec.Body.Bytes()
	if len(body) == 0 {
		return ""
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content any `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := rootcommon.Unmarshal(body, &parsed); err != nil {
		return strings.TrimSpace(string(body))
	}
	if len(parsed.Choices) == 0 {
		return ""
	}
	return mediaContentToString(parsed.Choices[0].Message.Content)
}

func recordImageRecognizeToolLog(
	parent *relaycommon.RelayInfo,
	request *dto.GeneralOpenAIRequest,
	started time.Time,
	caption string,
	hopErr *relaykittypes.NewAPIError,
) {
	log := buildImageRecognizeToolLog(parent, request, started, caption, hopErr)
	if log == nil {
		return
	}
	model.RecordToolLogs([]*model.ToolLog{log})
}

func buildImageRecognizeToolLog(
	parent *relaycommon.RelayInfo,
	request *dto.GeneralOpenAIRequest,
	started time.Time,
	caption string,
	hopErr *relaykittypes.NewAPIError,
) *model.ToolLog {
	if parent == nil {
		return nil
	}
	st := model_setting.GetBambooSettings()
	channelID := 0
	backend := ""
	if st != nil {
		channelID = st.ImageRecognizeChannelId
		backend = strings.TrimSpace(st.ImageRecognizeModel)
	}
	username := ""
	if parent.UserId > 0 {
		username, _ = model.GetUsernameById(parent.UserId, false)
	}
	tokenName := ""
	if parent.TokenId > 0 {
		if token, err := model.GetTokenById(parent.TokenId); err == nil && token != nil {
			tokenName = token.Name
		}
	}
	imageCount := countVisionImages(request)
	query := ""
	if imageCount > 0 {
		query = fmt.Sprintf("%d images", imageCount)
	}
	errorCode := ""
	if hopErr != nil {
		errorCode = string(hopErr.GetErrorCode())
	}
	result := strings.TrimSpace(caption)
	if result == "" && hopErr != nil {
		result = hopErr.Error()
	}
	maxRunes := 4096
	if st != nil {
		maxRunes = st.ClampImageRecognizeMaxOutputRunes()
	}
	truncated := false
	result, truncated = truncateRunesWithMark(result, maxRunes)
	durationMs := time.Since(started).Milliseconds()
	if durationMs < 0 {
		durationMs = 0
	}
	return &model.ToolLog{
		UserId:       parent.UserId,
		Username:     username,
		TokenId:      parent.TokenId,
		TokenName:    tokenName,
		ChannelId:    channelID,
		Group:        parent.UsingGroup,
		ModelName:    parent.OriginModelName,
		RequestId:    parent.RequestId,
		Ip:           clientIPFromHeaders(parent.RequestHeaders),
		OriginalName: relaycommon.ImageRecognizeOriginal,
		Canonical:    relaycommon.ImageRecognizeCanonical,
		Kind:         relaycommon.ImageRecognizeToolKind,
		Mode:         "hop",
		Backend:      backend,
		Query:        query,
		ErrorCode:    errorCode,
		DurationMs:   durationMs,
		Truncated:    truncated,
		Result:       result,
	}
}

func countVisionImages(req *dto.GeneralOpenAIRequest) int {
	if req == nil {
		return 0
	}
	n := 0
	for i := range req.Messages {
		for _, part := range req.Messages[i].ParseContent() {
			if part.Type == dto.ContentTypeImageURL {
				n++
			}
		}
	}
	return n
}

func clientIPFromHeaders(headers map[string]string) string {
	if headers == nil {
		return ""
	}
	for _, key := range []string{"X-Real-Ip", "X-Real-IP", "X-Forwarded-For"} {
		value := strings.TrimSpace(headers[key])
		if value == "" {
			continue
		}
		if i := strings.IndexByte(value, ','); i >= 0 {
			value = value[:i]
		}
		return strings.TrimSpace(value)
	}
	return ""
}

func truncateRunesWithMark(s string, max int) (string, bool) {
	if max <= 0 || s == "" {
		return s, false
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s, false
	}
	return string(runes[:max]) + "...[truncated]", true
}

func mediaContentToString(content any) string {
	switch v := content.(type) {
	case string:
		return strings.TrimSpace(v)
	case []any:
		var b strings.Builder
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := m["text"].(string); ok {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(text)
			}
		}
		return strings.TrimSpace(b.String())
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", content))
	}
}
