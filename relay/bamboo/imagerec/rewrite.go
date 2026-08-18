package imagerec

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	kittypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

type HopFunc func(c *gin.Context, parent *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (caption string, usage *dto.Usage, err *kittypes.NewAPIError)

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

func latestUserIndex(messages []bamboosdk.BambooMessage) int {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == bamboosdk.RoleUser {
			return i
		}
	}
	return -1
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
func MaybeRewrite(c *gin.Context, info *relaycommon.RelayInfo, req *bamboocodec.RelayRequest) *kittypes.NewAPIError {
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

	latestIdx := latestUserIndex(req.Messages)
	var latestImages []extractedImage
	var latestText string
	if latestIdx >= 0 {
		latestImages = collectImages(req.Messages[latestIdx])
		latestText = collectUserText(req.Messages[latestIdx])
	}

	if len(latestImages) == 0 {
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

	visionReq, err := buildVisionRequest(c, st, latestText, latestImages)
	if err != nil {
		return kittypes.NewError(err, kittypes.ErrorCodeInvalidRequest, kittypes.ErrOptionWithSkipRetry())
	}

	started := time.Now()
	rawCaption, usage, hopErr := hopFunc(c, info, visionReq)
	duration := time.Since(started).Milliseconds()
	if hopErr != nil {
		info.ImageRecognizePlan = &relaycommon.ImageRecognizePlan{
			Enabled:    true,
			ChannelId:  st.ImageRecognizeChannelId,
			Model:      strings.TrimSpace(st.ImageRecognizeModel),
			ImageCount: len(latestImages),
			DurationMs: duration,
			Error:      hopErr.Error(),
		}
		return hopErr
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
		ChannelId:  st.ImageRecognizeChannelId,
		Model:      strings.TrimSpace(st.ImageRecognizeModel),
		ImageCount: len(latestImages),
		VisibleBox: BuildVisibleBox(captions),
		DurationMs: duration,
	}
	if usage != nil {
		plan.PromptTokens = usage.PromptTokens
		plan.CompletionTok = usage.CompletionTokens
	}
	info.ImageRecognizePlan = plan
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

func buildVisionRequest(c *gin.Context, st *model_setting.BambooSettings, userText string, images []extractedImage) (*dto.GeneralOpenAIRequest, error) {
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
		Stream:   commonBoolPtr(false),
	}, nil
}

func commonBoolPtr(v bool) *bool { return &v }

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
