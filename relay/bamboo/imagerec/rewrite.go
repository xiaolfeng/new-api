package imagerec

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	kittypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

// CaptionLive 把识图 token 推给父客户端。nil 表示不直播（非流父请求）。
type CaptionLive interface {
	Begin() bool
	OnDelta(text string)
	End()
}

type HopFunc func(c *gin.Context, parent *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest, live CaptionLive) (caption string, usage *dto.Usage, err *kittypes.NewAPIError)

var hopFunc HopFunc

func SetHopFunc(fn HopFunc) {
	hopFunc = fn
}

func modelHasVision(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	if model.ModelHasTag(info.OriginModelName, "vision") {
		return true
	}
	if upstream := info.GetUpstreamModelName(); upstream != "" && upstream != info.OriginModelName {
		return model.ModelHasTag(upstream, "vision")
	}
	return false
}

// Claude Code 会在真实用户消息后再塞一条预算提示，不能当成“最近一次用户输入”。
var totalTokensTrailerRe = regexp.MustCompile(`(?s)^\s*<total_tokens>.*</total_tokens>\s*$`)

func isSyntheticUserMessage(msg bamboosdk.BambooMessage) bool {
	for _, block := range msg.Content {
		switch b := block.(type) {
		case *bamboosdk.ImageBlock:
			if b != nil && b.Source != nil {
				return false
			}
		case *bamboosdk.DocumentBlock:
			if b != nil && b.Source != nil {
				return false
			}
		case *bamboosdk.ToolResultBlock:
			if b != nil {
				return false
			}
		}
	}
	text := collectUserText(msg)
	if text == "" {
		return true
	}
	return totalTokensTrailerRe.MatchString(text)
}

// latestUserTurnStart 返回最近一轮用户消息的起点（不含助手）。
// 会跳过 <total_tokens> 这类合成尾巴，避免把本轮图片误判成历史。
func latestUserTurnStart(messages []bamboosdk.BambooMessage) int {
	real := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != bamboosdk.RoleUser {
			continue
		}
		if isSyntheticUserMessage(messages[i]) {
			continue
		}
		real = i
		break
	}
	if real < 0 {
		return -1
	}
	for i := real - 1; i >= 0; i-- {
		if messages[i].Role == bamboosdk.RoleAssistant {
			return i + 1
		}
	}
	return 0
}

func collectTurnImages(messages []bamboosdk.BambooMessage, start int) []extractedImage {
	if start < 0 {
		return nil
	}
	seen := make(map[string]struct{})
	out := make([]extractedImage, 0)
	for i := start; i < len(messages); i++ {
		if messages[i].Role != bamboosdk.RoleUser {
			continue
		}
		for _, img := range collectImages(messages[i]) {
			if _, dup := seen[img.Fingerprint]; dup {
				continue
			}
			seen[img.Fingerprint] = struct{}{}
			out = append(out, img)
		}
	}
	return out
}

func collectTurnText(messages []bamboosdk.BambooMessage, start int) string {
	if start < 0 {
		return ""
	}
	var b strings.Builder
	for i := start; i < len(messages); i++ {
		if messages[i].Role != bamboosdk.RoleUser {
			continue
		}
		if isSyntheticUserMessage(messages[i]) {
			continue
		}
		text := collectUserText(messages[i])
		if text == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(text)
	}
	return b.String()
}

type extractedImage struct {
	Fingerprint string
	Source      *bamboosdk.ContentSource
}

func collectImages(msg bamboosdk.BambooMessage) []extractedImage {
	seen := make(map[string]struct{})
	out := make([]extractedImage, 0)
	for _, block := range msg.Content {
		ib, ok := block.(*bamboosdk.ImageBlock)
		if !ok || ib == nil || ib.Source == nil {
			continue
		}
		fp := fingerprint(ib.Source)
		if _, dup := seen[fp]; dup {
			continue
		}
		seen[fp] = struct{}{}
		out = append(out, extractedImage{Fingerprint: fp, Source: ib.Source})
	}
	return out
}

func fingerprint(src *bamboosdk.ContentSource) string {
	if src == nil {
		return ""
	}
	sum := sha256.Sum256([]byte(src.Type + "\n" + src.MediaType + "\n" + src.Data + "\n" + src.URL))
	return fmt.Sprintf("%x", sum)
}

func collectUserText(msg bamboosdk.BambooMessage) string {
	var b strings.Builder
	for _, block := range msg.Content {
		tb, ok := block.(*bamboosdk.TextBlock)
		if !ok || tb == nil {
			continue
		}
		if text := strings.TrimSpace(tb.Text); text != "" {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(text)
		}
	}
	return b.String()
}

func stripAllImages(req *bamboocodec.RelayRequest, captions map[string]string, captionOrder []string) {
	orderIndex := make(map[string]int, len(captionOrder))
	for i, fp := range captionOrder {
		orderIndex[fp] = i + 1
	}
	for i := range req.Messages {
		next := make([]bamboosdk.ContentBlock, 0, len(req.Messages[i].Content))
		for _, block := range req.Messages[i].Content {
			ib, ok := block.(*bamboosdk.ImageBlock)
			if !ok || ib == nil || ib.Source == nil {
				next = append(next, block)
				continue
			}
			fp := fingerprint(ib.Source)
			if cap, ok := captions[fp]; ok {
				n := orderIndex[fp]
				next = append(next, bamboosdk.NewTextBlock(formatLatestCaption(n, cap)))
				continue
			}
			next = append(next, bamboosdk.NewTextBlock(relaycommon.ImageRecognizeHistoryMark))
		}
		req.Messages[i].Content = next
	}
}

func formatLatestCaption(n int, caption string) string {
	caption = strings.TrimSpace(caption)
	if n <= 0 {
		return caption
	}
	return fmt.Sprintf("[Image %d]\n%s", n, caption)
}

func BuildVisibleBox(captions []string) string {
	if len(captions) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(relaycommon.ImageRecognizeFenceStart)
	b.WriteByte('\n')
	for i, cap := range captions {
		b.WriteString(formatLatestCaption(i+1, cap))
		if i != len(captions)-1 {
			b.WriteByte('\n')
		}
	}
	b.WriteByte('\n')
	b.WriteString(relaycommon.ImageRecognizeFenceEnd)
	b.WriteByte('\n')
	return b.String()
}

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "\n...[truncated]"
}

// MaybeRewrite inspects the parsed bamboo request and, when needed, runs
// image recognition then strips every ImageBlock from the IR.
// 失败语义：hop 执行/识别请求构造失败先按 ImageRecognizeRetryTimes 重试瞬时错误；
// 重试耗尽后按 ImageRecognizeFailOpen 决定降级继续（剥图 + 说明）还是返回错误。
// entryBytes 是入口协议原文，用来把 codec 丢掉的 tool_result 图片捞回 IR。
func MaybeRewrite(c *gin.Context, info *relaycommon.RelayInfo, req *bamboocodec.RelayRequest, entryBytes []byte, live CaptionLive) *kittypes.NewAPIError {
	if info == nil || req == nil {
		return nil
	}
	if info.ImageRecognizeInner {
		info.ImageRecognizePlan = &relaycommon.ImageRecognizePlan{
			SkippedReason: relaycommon.ImageRecognizeSkipInnerHop,
		}
		return nil
	}

	st := model_setting.GetBambooSettings()
	if st == nil || !st.EnableImageRecognize {
		info.ImageRecognizePlan = &relaycommon.ImageRecognizePlan{
			SkippedReason: relaycommon.ImageRecognizeSkipDisabled,
		}
		return nil
	}

	if modelHasVision(info) {
		info.ImageRecognizePlan = &relaycommon.ImageRecognizePlan{
			SkippedReason: relaycommon.ImageRecognizeSkipHasVision,
		}
		return nil
	}

	start := latestUserTurnStart(req.Messages)
	liftToolResultImages(req, start, entryBytes)
	var latestImages []extractedImage
	var latestText string
	if start >= 0 {
		latestImages = collectTurnImages(req.Messages, start)
		latestText = collectTurnText(req.Messages, start)
	}

	if len(latestImages) == 0 {
		// 本轮确认没图之后，才把历史 ImageBlock 改成 [image]，避免挡在识图前面。
		stripAllImages(req, nil, nil)
		info.ImageRecognizePlan = &relaycommon.ImageRecognizePlan{
			Enabled:       false,
			SkippedReason: relaycommon.ImageRecognizeSkipNoLatestUserImage,
		}
		return nil
	}

	if st.ImageRecognizeChannelId <= 0 || strings.TrimSpace(st.ImageRecognizeModel) == "" {
		info.ImageRecognizePlan = &relaycommon.ImageRecognizePlan{
			SkippedReason: relaycommon.ImageRecognizeSkipNotConfigured,
			ChannelId:     st.ImageRecognizeChannelId,
			Model:         st.ImageRecognizeModel,
			ImageCount:    len(latestImages),
		}
		return kittypes.NewError(
			fmt.Errorf("image recognition is enabled but channel/model is not configured"),
			kittypes.ErrorCodeInvalidRequest,
			kittypes.ErrOptionWithSkipRetry(),
		)
	}

	maxImages := st.ClampImageRecognizeMaxImages()
	if len(latestImages) > maxImages {
		return kittypes.NewError(
			fmt.Errorf("too many images in the latest message: %d (max %d)", len(latestImages), maxImages),
			kittypes.ErrorCodeInvalidRequest,
			kittypes.ErrOptionWithSkipRetry(),
		)
	}

	if hopFunc == nil {
		return kittypes.NewError(
			fmt.Errorf("image recognition hop is not registered"),
			kittypes.ErrorCodeInvalidRequest,
			kittypes.ErrOptionWithSkipRetry(),
		)
	}

	visionReq, err := buildVisionRequest(c, st, latestText, latestImages, live != nil)
	if err != nil {
		if st.ImageRecognizeFailOpen {
			return failOpenImageRecognize(info, st, req, latestImages, err)
		}
		return kittypes.NewError(err, kittypes.ErrorCodeInvalidRequest, kittypes.ErrOptionWithSkipRetry())
	}

	// 重试循环：仅瞬时/上游类错误（非 skipRetry）触发重试；中间尝试不直播
	// caption（live=nil），避免客户端看到重试产生的重复/残缺内容。
	retries := st.ClampImageRecognizeRetryTimes()
	started := time.Now()
	var (
		rawCaption string
		usage      *dto.Usage
		hopErr     *kittypes.NewAPIError
	)
	executed := 0
	for attempt := 0; attempt <= retries; attempt++ {
		executed++
		last := attempt == retries
		var tryLive CaptionLive
		if last {
			tryLive = live
		}
		if tryLive != nil {
			tryLive.Begin()
		}
		rawCaption, usage, hopErr = hopFunc(c, info, visionReq, tryLive)
		if tryLive != nil {
			tryLive.End()
		}
		if hopErr != nil && !last && !kittypes.IsSkipRetryError(hopErr) {
			continue
		}
		break
	}
	duration := time.Since(started).Milliseconds()
	if hopErr != nil {
		info.ImageRecognizePlan = &relaycommon.ImageRecognizePlan{
			Enabled:    true,
			Failed:     true,
			RetryCount: executed - 1,
			ChannelId:  st.ImageRecognizeChannelId,
			Model:      strings.TrimSpace(st.ImageRecognizeModel),
			ImageCount: len(latestImages),
			DurationMs: duration,
			Error:      hopErr.Error(),
		}
		if st.ImageRecognizeFailOpen {
			return failOpenImageRecognize(info, st, req, latestImages, hopErr)
		}
		if kittypes.IsSkipRetryError(hopErr) {
			return hopErr
		}
		// 重试已耗尽：补包 skipRetry，抑制外层主链路对同一请求再整单重试。
		return kittypes.NewError(hopErr.Err, hopErr.GetErrorCode(),
			kittypes.ErrOptionWithSkipRetry(), kittypes.ErrOptionWithStatusCode(hopErr.StatusCode))
	}

	maxRunes := st.ClampImageRecognizeMaxOutputRunes()
	captions := splitCaptions(rawCaption, len(latestImages), maxRunes)
	byFP := make(map[string]string, len(latestImages))
	order := make([]string, 0, len(latestImages))
	for i, img := range latestImages {
		cap := ""
		if i < len(captions) {
			cap = captions[i]
		}
		byFP[img.Fingerprint] = cap
		order = append(order, img.Fingerprint)
	}
	stripAllImages(req, byFP, order)

	plan := &relaycommon.ImageRecognizePlan{
		Enabled:    true,
		RetryCount: executed - 1,
		ChannelId:  st.ImageRecognizeChannelId,
		Model:      strings.TrimSpace(st.ImageRecognizeModel),
		ImageCount: len(latestImages),
		DurationMs: duration,
	}
	if live == nil {
		plan.VisibleBox = BuildVisibleBox(captions)
	}
	if usage != nil {
		plan.PromptTokens = usage.PromptTokens
		plan.CompletionTok = usage.CompletionTokens
	}
	info.ImageRecognizePlan = plan
	return nil
}

// failOpenImageRecognize 是识图最终失败（重试耗尽或请求构造失败）时的降级出口：
// 把所有图片替换成占位符，在 visible box 说明失败，让主流程继续而不是失败整个请求。
// plan 已存在（hop 失败路径）时复用并补 VisibleBox；否则新建。
func failOpenImageRecognize(info *relaycommon.RelayInfo, st *model_setting.BambooSettings, req *bamboocodec.RelayRequest, latestImages []extractedImage, failErr error) *kittypes.NewAPIError {
	if info == nil || req == nil {
		return nil
	}
	plan := info.ImageRecognizePlan
	if plan == nil {
		plan = &relaycommon.ImageRecognizePlan{
			Enabled:    true,
			Failed:     true,
			ChannelId:  st.ImageRecognizeChannelId,
			Model:      strings.TrimSpace(st.ImageRecognizeModel),
			ImageCount: len(latestImages),
		}
		info.ImageRecognizePlan = plan
	}
	plan.Failed = true
	plan.VisibleBox = relaycommon.ImageRecognizeFailOpenNote + "\n"
	if plan.Error == "" && failErr != nil {
		plan.Error = failErr.Error()
	}
	stripAllImages(req, nil, nil)
	return nil
}

func splitCaptions(raw string, n, maxRunes int) []string {
	raw = strings.TrimSpace(raw)
	if n <= 0 {
		return nil
	}
	if n == 1 {
		return []string{truncateRunes(raw, maxRunes)}
	}
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = ""
	}
	// Prefer [Image N] markers from the vision model; otherwise put the whole reply on image 1.
	assigned := false
	lines := strings.Split(raw, "\n")
	current := -1
	var buf strings.Builder
	flush := func() {
		if current >= 0 && current < n {
			out[current] = truncateRunes(strings.TrimSpace(buf.String()), maxRunes)
			assigned = true
		}
		buf.Reset()
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		var idx int
		if _, err := fmt.Sscanf(trimmed, "[Image %d]", &idx); err == nil && idx >= 1 && idx <= n {
			flush()
			current = idx - 1
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, fmt.Sprintf("[Image %d]", idx)))
			if rest != "" {
				buf.WriteString(rest)
			}
			continue
		}
		if current >= 0 {
			if buf.Len() > 0 {
				buf.WriteByte('\n')
			}
			buf.WriteString(line)
		}
	}
	flush()
	if !assigned {
		out[0] = truncateRunes(raw, maxRunes)
	}
	return out
}

func buildVisionRequest(c *gin.Context, st *model_setting.BambooSettings, userText string, images []extractedImage, stream bool) (*dto.GeneralOpenAIRequest, error) {
	parts := make([]dto.MediaContent, 0, len(images)+2)
	if strings.TrimSpace(userText) != "" {
		parts = append(parts, dto.MediaContent{
			Type: dto.ContentTypeText,
			Text: "以下文字仅作理解图片场景的参考，不要回答它，也不要据此给建议或下一步：\n" + strings.TrimSpace(userText),
		})
	}
	parts = append(parts, dto.MediaContent{
		Type: dto.ContentTypeText,
		Text: fmt.Sprintf("请解析这 %d 张图片。只解释图片本身，按 [Image 1]…[Image %d] 分段输出客观描述。", len(images), len(images)),
	})
	for i, img := range images {
		dataURL, mime, err := resolveImageDataURL(c, img.Source)
		if err != nil {
			return nil, fmt.Errorf("image %d: %w", i+1, err)
		}
		parts = append(parts, dto.MediaContent{
			Type: dto.ContentTypeImageURL,
			ImageUrl: &dto.MessageImageUrl{
				Url:      dataURL,
				Detail:   "high",
				MimeType: mime,
			},
		})
	}

	userMsg := dto.Message{Role: "user"}
	userMsg.SetMediaContent(parts)

	modelName := strings.TrimSpace(st.ImageRecognizeModel)
	messages := []dto.Message{
		{Role: "system", Content: st.ResolvedImageRecognizePrompt()},
		userMsg,
	}
	return &dto.GeneralOpenAIRequest{
		Model:    modelName,
		Messages: messages,
		Stream:   &stream,
	}, nil
}

type wireMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type wireContentPart struct {
	Type    string                   `json:"type"`
	Source  *bamboosdk.ContentSource `json:"source"`
	Content json.RawMessage          `json:"content"`
}

func liftToolResultImages(req *bamboocodec.RelayRequest, start int, entryBytes []byte) {
	if req == nil || start < 0 || len(entryBytes) == 0 {
		return
	}
	var wire struct {
		Messages []wireMessage `json:"messages"`
	}
	if err := common.Unmarshal(entryBytes, &wire); err != nil {
		return
	}
	if len(wire.Messages) != len(req.Messages) {
		return
	}
	for i := start; i < len(req.Messages); i++ {
		if req.Messages[i].Role != bamboosdk.RoleUser {
			continue
		}
		existing := collectImages(req.Messages[i])
		seen := make(map[string]struct{}, len(existing))
		for _, img := range existing {
			seen[img.Fingerprint] = struct{}{}
		}
		for _, src := range imagesFromWireContent(wire.Messages[i].Content) {
			srcCopy := src
			fp := fingerprint(&srcCopy)
			if _, ok := seen[fp]; ok {
				continue
			}
			seen[fp] = struct{}{}
			req.Messages[i].Content = append(req.Messages[i].Content, bamboosdk.NewImageBlock(srcCopy))
		}
	}
}

func imagesFromWireContent(raw json.RawMessage) []bamboosdk.ContentSource {
	if len(raw) == 0 {
		return nil
	}
	var asString string
	if err := common.Unmarshal(raw, &asString); err == nil {
		return nil
	}
	var parts []wireContentPart
	if err := common.Unmarshal(raw, &parts); err != nil {
		return nil
	}
	out := make([]bamboosdk.ContentSource, 0)
	for _, part := range parts {
		switch part.Type {
		case "image":
			if part.Source == nil {
				continue
			}
			if strings.TrimSpace(part.Source.Data) == "" && strings.TrimSpace(part.Source.URL) == "" {
				continue
			}
			out = append(out, *part.Source)
		case "tool_result":
			out = append(out, imagesFromWireContent(part.Content)...)
		}
	}
	return out
}

func resolveImageDataURL(c *gin.Context, src *bamboosdk.ContentSource) (string, string, error) {
	if src == nil {
		return "", "", fmt.Errorf("empty image source")
	}
	switch src.Type {
	case "base64":
		mime := strings.TrimSpace(src.MediaType)
		if mime == "" {
			mime = "image/png"
		}
		if !strings.HasPrefix(mime, "image/") {
			return "", "", fmt.Errorf("unsupported media type %s", mime)
		}
		data := strings.TrimSpace(src.Data)
		if data == "" {
			return "", "", fmt.Errorf("empty base64 data")
		}
		if strings.HasPrefix(data, "data:") {
			return data, mime, nil
		}
		return "data:" + mime + ";base64," + data, mime, nil
	case "url":
		url := strings.TrimSpace(src.URL)
		if url == "" {
			return "", "", fmt.Errorf("empty image url")
		}
		if strings.HasPrefix(url, "data:") {
			return url, src.MediaType, nil
		}
		if err := service.ValidateSSRFProtectedFetchURL(url); err != nil {
			return "", "", err
		}
		source := kittypes.NewFileSourceFromData(url, src.MediaType)
		b64, mime, err := service.GetBase64Data(c, source, "image_recognize")
		if err != nil {
			return "", "", err
		}
		if mime == "" {
			mime = src.MediaType
		}
		if mime == "" {
			mime = "image/png"
		}
		return "data:" + mime + ";base64," + b64, mime, nil
	default:
		return "", "", fmt.Errorf("unsupported image source type %s", src.Type)
	}
}
