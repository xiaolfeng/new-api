package imagerec

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	kittypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

func testGinContext() *gin.Context {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	return c
}

func imageReq(messages []bamboosdk.BambooMessage) *bamboocodec.RelayRequest {
	return &bamboocodec.RelayRequest{Messages: messages}
}

func pngBlock(data string) bamboosdk.ContentBlock {
	return bamboosdk.NewImageBlock(bamboosdk.ContentSource{
		Type:      "base64",
		MediaType: "image/png",
		Data:      data,
	})
}

func TestMaybeRewrite_Disabled(t *testing.T) {
	st := model_setting.GetBambooSettings()
	prev := *st
	t.Cleanup(func() { *st = prev })
	st.EnableImageRecognize = false

	info := &relaycommon.RelayInfo{OriginModelName: "deepseek-chat"}
	req := imageReq([]bamboosdk.BambooMessage{
		bamboosdk.NewUserMessageBlocks(bamboosdk.NewTextBlock("hi"), pngBlock("aaa")),
	})
	err := MaybeRewrite(testGinContext(), info, req, nil, nil)
	require.Nil(t, err)
	require.NotNil(t, info.ImageRecognizePlan)
	assert.Equal(t, relaycommon.ImageRecognizeSkipDisabled, info.ImageRecognizePlan.SkippedReason)
	_, ok := req.Messages[0].Content[1].(*bamboosdk.ImageBlock)
	assert.True(t, ok, "disabled path must keep images")
}

func TestMaybeRewrite_InnerHopSkipped(t *testing.T) {
	info := &relaycommon.RelayInfo{ImageRecognizeInner: true, OriginModelName: "deepseek-chat"}
	req := imageReq([]bamboosdk.BambooMessage{
		bamboosdk.NewUserMessageBlocks(pngBlock("aaa")),
	})
	err := MaybeRewrite(testGinContext(), info, req, nil, nil)
	require.Nil(t, err)
	assert.Equal(t, relaycommon.ImageRecognizeSkipInnerHop, info.ImageRecognizePlan.SkippedReason)
}

func TestMaybeRewrite_LatestUserImageRecognizedAndHistoryStripped(t *testing.T) {
	st := model_setting.GetBambooSettings()
	prev := *st
	t.Cleanup(func() { *st = prev })
	st.EnableImageRecognize = true
	st.ImageRecognizeChannelId = 9
	st.ImageRecognizeModel = "vision-model"

	origHop := hopFunc
	t.Cleanup(func() { hopFunc = origHop })
	hopFunc = func(c *gin.Context, parent *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest, live CaptionLive) (string, *dto.Usage, *kittypes.NewAPIError) {
		require.Equal(t, "vision-model", request.Model)
		require.False(t, parent.ImageRecognizeInner)
		return "[Image 1]\nA red cat", &dto.Usage{PromptTokens: 10, CompletionTokens: 4}, nil
	}

	info := &relaycommon.RelayInfo{OriginModelName: "deepseek-chat"}
	req := imageReq([]bamboosdk.BambooMessage{
		bamboosdk.NewUserMessageBlocks(bamboosdk.NewTextBlock("old"), pngBlock("oldimg")),
		bamboosdk.NewAssistantMessage("previous answer"),
		bamboosdk.NewUserMessageBlocks(bamboosdk.NewTextBlock("what is this"), pngBlock("newimg")),
	})
	err := MaybeRewrite(testGinContext(), info, req, nil, nil)
	require.Nil(t, err)
	require.NotNil(t, info.ImageRecognizePlan)
	assert.True(t, info.ImageRecognizePlan.Enabled)
	assert.Equal(t, 1, info.ImageRecognizePlan.ImageCount)
	assert.Contains(t, info.ImageRecognizePlan.VisibleBox, relaycommon.ImageRecognizeFenceStart)
	assert.Contains(t, info.ImageRecognizePlan.VisibleBox, "A red cat")

	for _, msg := range req.Messages {
		for _, block := range msg.Content {
			_, isImg := block.(*bamboosdk.ImageBlock)
			assert.False(t, isImg, "all images must be stripped")
		}
	}
	histText := req.Messages[0].Content[1].(*bamboosdk.TextBlock)
	assert.Equal(t, relaycommon.ImageRecognizeHistoryMark, histText.Text)
	latestText := req.Messages[2].Content[1].(*bamboosdk.TextBlock)
	assert.Contains(t, latestText.Text, "A red cat")
}

func TestMaybeRewrite_HistoryOnlyDoesNotHop(t *testing.T) {
	st := model_setting.GetBambooSettings()
	prev := *st
	t.Cleanup(func() { *st = prev })
	st.EnableImageRecognize = true
	st.ImageRecognizeChannelId = 9
	st.ImageRecognizeModel = "vision-model"

	called := false
	origHop := hopFunc
	t.Cleanup(func() { hopFunc = origHop })
	hopFunc = func(c *gin.Context, parent *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest, live CaptionLive) (string, *dto.Usage, *kittypes.NewAPIError) {
		called = true
		return "should not run", nil, nil
	}

	info := &relaycommon.RelayInfo{OriginModelName: "deepseek-chat"}
	req := imageReq([]bamboosdk.BambooMessage{
		bamboosdk.NewUserMessageBlocks(pngBlock("oldimg")),
		bamboosdk.NewAssistantMessage("<<<image_recognition>>>\nold\n<<<end_image_recognition>>>"),
		bamboosdk.NewUserMessage("just text now"),
	})
	err := MaybeRewrite(testGinContext(), info, req, nil, nil)
	require.Nil(t, err)
	assert.False(t, called)
	assert.Equal(t, relaycommon.ImageRecognizeSkipNoLatestUserImage, info.ImageRecognizePlan.SkippedReason)
	assert.Empty(t, info.ImageRecognizePlan.VisibleBox)
	hist := req.Messages[0].Content[0].(*bamboosdk.TextBlock)
	assert.Equal(t, relaycommon.ImageRecognizeHistoryMark, hist.Text)
}

func TestMaybeRewrite_NotConfigured(t *testing.T) {
	st := model_setting.GetBambooSettings()
	prev := *st
	t.Cleanup(func() { *st = prev })
	st.EnableImageRecognize = true
	st.ImageRecognizeChannelId = 0
	st.ImageRecognizeModel = ""

	info := &relaycommon.RelayInfo{OriginModelName: "deepseek-chat"}
	req := imageReq([]bamboosdk.BambooMessage{
		bamboosdk.NewUserMessageBlocks(pngBlock("newimg")),
	})
	err := MaybeRewrite(testGinContext(), info, req, nil, nil)
	require.NotNil(t, err)
	assert.Equal(t, relaycommon.ImageRecognizeSkipNotConfigured, info.ImageRecognizePlan.SkippedReason)
}

func TestBuildVisibleBox(t *testing.T) {
	box := BuildVisibleBox([]string{"cat", "dog"})
	assert.Contains(t, box, relaycommon.ImageRecognizeFenceStart)
	assert.Contains(t, box, "[Image 1]\ncat")
	assert.Contains(t, box, "[Image 2]\ndog")
	assert.Contains(t, box, relaycommon.ImageRecognizeFenceEnd)
}

func TestBuildVisionRequestIsParserOnly(t *testing.T) {
	st := model_setting.GetBambooSettings()
	prev := *st
	t.Cleanup(func() { *st = prev })
	st.EnableImageRecognize = true
	st.ImageRecognizeChannelId = 9
	st.ImageRecognizeModel = "vision-model"
	st.ImageRecognizePrompt = ""

	var got *dto.GeneralOpenAIRequest
	origHop := hopFunc
	t.Cleanup(func() { hopFunc = origHop })
	hopFunc = func(c *gin.Context, parent *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest, live CaptionLive) (string, *dto.Usage, *kittypes.NewAPIError) {
		got = request
		return "[Image 1]\nA sign", &dto.Usage{PromptTokens: 1, CompletionTokens: 1}, nil
	}

	info := &relaycommon.RelayInfo{OriginModelName: "deepseek-chat"}
	req := imageReq([]bamboosdk.BambooMessage{
		bamboosdk.NewUserMessageBlocks(bamboosdk.NewTextBlock("这是什么？帮我写邮件"), pngBlock("img")),
	})
	err := MaybeRewrite(testGinContext(), info, req, nil, nil)
	require.Nil(t, err)
	require.NotNil(t, got)
	require.GreaterOrEqual(t, len(got.Messages), 2)
	assert.Equal(t, "system", got.Messages[0].Role)
	sys, _ := got.Messages[0].Content.(string)
	assert.Contains(t, sys, "你只是图片解析模块")
	assert.Contains(t, sys, "禁止")
	assert.Contains(t, sys, "回答用户问题")

	parts := got.Messages[1].ParseContent()
	var texts []string
	for _, p := range parts {
		if p.Type == dto.ContentTypeText {
			texts = append(texts, p.Text)
		}
	}
	joined := strings.Join(texts, "\n")
	assert.Contains(t, joined, "仅作理解图片场景的参考")
	assert.Contains(t, joined, "不要回答它")
	assert.Contains(t, joined, "只解释图片本身")
}

func TestSplitCaptionsPrefersMarkers(t *testing.T) {
	got := splitCaptions("[Image 1]\ncat\n[Image 2]\ndog", 2, 4096)
	require.Len(t, got, 2)
	assert.Equal(t, "cat", got[0])
	assert.Equal(t, "dog", got[1])
}

func enableImageRecognize(t *testing.T) {
	t.Helper()
	st := model_setting.GetBambooSettings()
	prev := *st
	t.Cleanup(func() { *st = prev })
	st.EnableImageRecognize = true
	st.ImageRecognizeChannelId = 9
	st.ImageRecognizeModel = "vision-model"
}

func TestMaybeRewrite_TotalTokensTrailerDoesNotStripBeforeHop(t *testing.T) {
	enableImageRecognize(t)

	hopped := false
	origHop := hopFunc
	t.Cleanup(func() { hopFunc = origHop })
	hopFunc = func(c *gin.Context, parent *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest, live CaptionLive) (string, *dto.Usage, *kittypes.NewAPIError) {
		hopped = true
		parts := request.Messages[1].ParseContent()
		hasImage := false
		for _, p := range parts {
			if p.Type == dto.ContentTypeImageURL {
				hasImage = true
			}
		}
		require.True(t, hasImage, "vision hop must receive the image before any [image] rewrite")
		return "[Image 1]\nA screenshot", &dto.Usage{PromptTokens: 3, CompletionTokens: 2}, nil
	}

	info := &relaycommon.RelayInfo{OriginModelName: "deepseek-chat"}
	req := imageReq([]bamboosdk.BambooMessage{
		bamboosdk.NewUserMessageBlocks(bamboosdk.NewTextBlock("这是啥"), pngBlock("pasted")),
		bamboosdk.NewUserMessage("<total_tokens>15000000 tokens left</total_tokens>"),
	})
	err := MaybeRewrite(testGinContext(), info, req, nil, nil)
	require.Nil(t, err)
	require.True(t, hopped)
	require.NotNil(t, info.ImageRecognizePlan)
	assert.True(t, info.ImageRecognizePlan.Enabled)
	assert.Equal(t, 1, info.ImageRecognizePlan.ImageCount)
	assert.Contains(t, info.ImageRecognizePlan.VisibleBox, "A screenshot")

	var texts []string
	for _, block := range req.Messages[0].Content {
		tb, ok := block.(*bamboosdk.TextBlock)
		if !ok {
			continue
		}
		texts = append(texts, tb.Text)
	}
	joined := strings.Join(texts, "\n")
	assert.Contains(t, joined, "A screenshot")
	assert.NotContains(t, joined, relaycommon.ImageRecognizeHistoryMark)
	_, isImg := req.Messages[0].Content[1].(*bamboosdk.ImageBlock)
	assert.False(t, isImg)
}

func TestMaybeRewrite_TotalTokensTrailerDoesNotPromoteHistory(t *testing.T) {
	enableImageRecognize(t)

	called := false
	origHop := hopFunc
	t.Cleanup(func() { hopFunc = origHop })
	hopFunc = func(c *gin.Context, parent *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest, live CaptionLive) (string, *dto.Usage, *kittypes.NewAPIError) {
		called = true
		return "should not run", nil, nil
	}

	info := &relaycommon.RelayInfo{OriginModelName: "deepseek-chat"}
	req := imageReq([]bamboosdk.BambooMessage{
		bamboosdk.NewUserMessageBlocks(bamboosdk.NewTextBlock("old"), pngBlock("oldimg")),
		bamboosdk.NewAssistantMessage("previous"),
		bamboosdk.NewUserMessage("just text now"),
		bamboosdk.NewUserMessage("<total_tokens>15000000 tokens left</total_tokens>"),
	})
	err := MaybeRewrite(testGinContext(), info, req, nil, nil)
	require.Nil(t, err)
	assert.False(t, called)
	assert.Equal(t, relaycommon.ImageRecognizeSkipNoLatestUserImage, info.ImageRecognizePlan.SkippedReason)
	hist := req.Messages[0].Content[1].(*bamboosdk.TextBlock)
	assert.Equal(t, relaycommon.ImageRecognizeHistoryMark, hist.Text)
}

func TestMaybeRewrite_LiftsToolResultImageThenRecognizes(t *testing.T) {
	enableImageRecognize(t)

	hopped := false
	origHop := hopFunc
	t.Cleanup(func() { hopFunc = origHop })
	hopFunc = func(c *gin.Context, parent *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest, live CaptionLive) (string, *dto.Usage, *kittypes.NewAPIError) {
		hopped = true
		parts := request.Messages[1].ParseContent()
		hasImage := false
		for _, p := range parts {
			if p.Type != dto.ContentTypeImageURL {
				continue
			}
			if img := p.GetImageMedia(); img != nil && strings.Contains(img.Url, "abc123") {
				hasImage = true
			}
		}
		require.True(t, hasImage, "tool_result image must be lifted before the vision hop")
		return "[Image 1]\nA photo", &dto.Usage{PromptTokens: 4, CompletionTokens: 2}, nil
	}

	info := &relaycommon.RelayInfo{OriginModelName: "deepseek-chat"}
	req := imageReq([]bamboosdk.BambooMessage{
		bamboosdk.NewAssistantMessageBlocks(bamboosdk.NewToolUseBlock("call_1", "Read", map[string]string{"file_path": "4.png"})),
		bamboosdk.NewUserMessageBlocks(bamboosdk.NewToolResultBlock("call_1", "", false)),
		bamboosdk.NewUserMessage("<total_tokens>14999596 tokens left</total_tokens>"),
	})
	entry := []byte(`{"messages":[
		{"role":"assistant","content":[{"type":"tool_use","id":"call_1","name":"Read","input":{}}]},
		{"role":"user","content":[{"type":"tool_result","tool_use_id":"call_1","content":[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"abc123"}}]}]},
		{"role":"user","content":"<total_tokens>14999596 tokens left</total_tokens>"}
	]}`)
	err := MaybeRewrite(testGinContext(), info, req, entry, nil)
	require.Nil(t, err)
	require.True(t, hopped)
	require.NotNil(t, info.ImageRecognizePlan)
	assert.True(t, info.ImageRecognizePlan.Enabled)
	assert.Contains(t, info.ImageRecognizePlan.VisibleBox, "A photo")

	foundCaption := false
	for _, block := range req.Messages[1].Content {
		tb, ok := block.(*bamboosdk.TextBlock)
		if !ok {
			continue
		}
		if strings.Contains(tb.Text, "A photo") {
			foundCaption = true
		}
	}
	assert.True(t, foundCaption)
}

func TestMaybeRewrite_HopFailureRetriesThenSucceeds(t *testing.T) {
	enableImageRecognize(t)
	st := model_setting.GetBambooSettings()
	st.ImageRecognizeRetryTimes = 1

	calls := 0
	origHop := hopFunc
	t.Cleanup(func() { hopFunc = origHop })
	hopFunc = func(c *gin.Context, parent *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest, live CaptionLive) (string, *dto.Usage, *kittypes.NewAPIError) {
		calls++
		if calls == 1 {
			return "", nil, kittypes.NewError(errors.New("transient 5xx"), kittypes.ErrorCodeDoRequestFailed)
		}
		return "[Image 1]\nA red cat", &dto.Usage{PromptTokens: 10, CompletionTokens: 4}, nil
	}

	info := &relaycommon.RelayInfo{OriginModelName: "deepseek-chat"}
	req := imageReq([]bamboosdk.BambooMessage{
		bamboosdk.NewUserMessageBlocks(pngBlock("aaa")),
	})
	err := MaybeRewrite(testGinContext(), info, req, nil, nil)
	require.Nil(t, err)
	require.Equal(t, 2, calls)
	require.NotNil(t, info.ImageRecognizePlan)
	assert.False(t, info.ImageRecognizePlan.Failed)
	assert.Equal(t, 1, info.ImageRecognizePlan.RetryCount)
	assert.Contains(t, info.ImageRecognizePlan.VisibleBox, "A red cat")
}

func TestMaybeRewrite_HopFailureFailOpenContinues(t *testing.T) {
	enableImageRecognize(t)
	st := model_setting.GetBambooSettings()
	st.ImageRecognizeRetryTimes = 1
	st.ImageRecognizeFailOpen = true

	calls := 0
	origHop := hopFunc
	t.Cleanup(func() { hopFunc = origHop })
	hopFunc = func(c *gin.Context, parent *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest, live CaptionLive) (string, *dto.Usage, *kittypes.NewAPIError) {
		calls++
		return "", nil, kittypes.NewError(errors.New("upstream down"), kittypes.ErrorCodeDoRequestFailed)
	}

	info := &relaycommon.RelayInfo{OriginModelName: "deepseek-chat"}
	req := imageReq([]bamboosdk.BambooMessage{
		bamboosdk.NewUserMessageBlocks(pngBlock("aaa")),
	})
	err := MaybeRewrite(testGinContext(), info, req, nil, nil)
	require.Nil(t, err)
	require.Equal(t, 2, calls)
	require.NotNil(t, info.ImageRecognizePlan)
	assert.True(t, info.ImageRecognizePlan.Failed)
	assert.Equal(t, 1, info.ImageRecognizePlan.RetryCount)
	assert.Contains(t, info.ImageRecognizePlan.VisibleBox, relaycommon.ImageRecognizeFailOpenNote)
	for _, msg := range req.Messages {
		for _, block := range msg.Content {
			_, isImg := block.(*bamboosdk.ImageBlock)
			assert.False(t, isImg, "fail-open must strip all images")
		}
	}
}

func TestMaybeRewrite_HopFailureFailClosedReturnsError(t *testing.T) {
	enableImageRecognize(t)
	st := model_setting.GetBambooSettings()
	st.ImageRecognizeRetryTimes = 1
	st.ImageRecognizeFailOpen = false

	origHop := hopFunc
	t.Cleanup(func() { hopFunc = origHop })
	hopFunc = func(c *gin.Context, parent *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest, live CaptionLive) (string, *dto.Usage, *kittypes.NewAPIError) {
		return "", nil, kittypes.NewError(errors.New("upstream down"), kittypes.ErrorCodeDoRequestFailed)
	}

	info := &relaycommon.RelayInfo{OriginModelName: "deepseek-chat"}
	req := imageReq([]bamboosdk.BambooMessage{
		bamboosdk.NewUserMessageBlocks(pngBlock("aaa")),
	})
	err := MaybeRewrite(testGinContext(), info, req, nil, nil)
	require.NotNil(t, err)
	assert.True(t, kittypes.IsSkipRetryError(err), "final error must suppress outer retry")
	require.NotNil(t, info.ImageRecognizePlan)
	assert.True(t, info.ImageRecognizePlan.Failed)
	assert.Equal(t, 1, info.ImageRecognizePlan.RetryCount)
}

func TestMaybeRewrite_HopSkipRetryErrorNoRetry(t *testing.T) {
	enableImageRecognize(t)
	st := model_setting.GetBambooSettings()
	st.ImageRecognizeRetryTimes = 3
	st.ImageRecognizeFailOpen = true

	calls := 0
	origHop := hopFunc
	t.Cleanup(func() { hopFunc = origHop })
	hopFunc = func(c *gin.Context, parent *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest, live CaptionLive) (string, *dto.Usage, *kittypes.NewAPIError) {
		calls++
		return "", nil, kittypes.NewError(errors.New("channel disabled"), kittypes.ErrorCodeGetChannelFailed, kittypes.ErrOptionWithSkipRetry())
	}

	info := &relaycommon.RelayInfo{OriginModelName: "deepseek-chat"}
	req := imageReq([]bamboosdk.BambooMessage{
		bamboosdk.NewUserMessageBlocks(pngBlock("aaa")),
	})
	err := MaybeRewrite(testGinContext(), info, req, nil, nil)
	require.Nil(t, err)
	require.Equal(t, 1, calls, "skip-retry errors must not be retried")
	require.NotNil(t, info.ImageRecognizePlan)
	assert.True(t, info.ImageRecognizePlan.Failed)
	assert.Zero(t, info.ImageRecognizePlan.RetryCount)
}

type testCaptionLive struct {
	begins int
	ended  bool
}

func (l *testCaptionLive) Begin() bool {
	l.begins++
	return true
}

func (l *testCaptionLive) OnDelta(text string) {}

func (l *testCaptionLive) End() {
	l.ended = true
}

func TestMaybeRewrite_RetryDoesNotStreamInterimAttempts(t *testing.T) {
	enableImageRecognize(t)
	st := model_setting.GetBambooSettings()
	st.ImageRecognizeRetryTimes = 1

	var gotLive []CaptionLive
	origHop := hopFunc
	t.Cleanup(func() { hopFunc = origHop })
	hopFunc = func(c *gin.Context, parent *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest, live CaptionLive) (string, *dto.Usage, *kittypes.NewAPIError) {
		gotLive = append(gotLive, live)
		if len(gotLive) == 1 {
			return "", nil, kittypes.NewError(errors.New("transient"), kittypes.ErrorCodeDoRequestFailed)
		}
		return "[Image 1]\nok", &dto.Usage{PromptTokens: 1, CompletionTokens: 1}, nil
	}

	live := &testCaptionLive{}
	info := &relaycommon.RelayInfo{OriginModelName: "deepseek-chat"}
	req := imageReq([]bamboosdk.BambooMessage{
		bamboosdk.NewUserMessageBlocks(pngBlock("aaa")),
	})
	err := MaybeRewrite(testGinContext(), info, req, nil, live)
	require.Nil(t, err)
	require.Len(t, gotLive, 2)
	assert.Nil(t, gotLive[0], "interim attempts must not live-stream")
	assert.Same(t, live, gotLive[1], "final attempt uses the live caption stream")
	assert.Equal(t, 1, live.begins)
	assert.True(t, live.ended)
}

func TestMaybeRewrite_BuildVisionRequestFailureFailOpen(t *testing.T) {
	enableImageRecognize(t)
	st := model_setting.GetBambooSettings()
	st.ImageRecognizeFailOpen = true

	called := false
	origHop := hopFunc
	t.Cleanup(func() { hopFunc = origHop })
	hopFunc = func(c *gin.Context, parent *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest, live CaptionLive) (string, *dto.Usage, *kittypes.NewAPIError) {
		called = true
		return "", nil, nil
	}

	// base64 空数据会让 resolveImageDataURL 在 hop 之前失败。
	bad := bamboosdk.NewImageBlock(bamboosdk.ContentSource{Type: "base64", MediaType: "image/png", Data: ""})
	info := &relaycommon.RelayInfo{OriginModelName: "deepseek-chat"}
	req := imageReq([]bamboosdk.BambooMessage{
		bamboosdk.NewUserMessageBlocks(bad),
	})
	err := MaybeRewrite(testGinContext(), info, req, nil, nil)
	require.Nil(t, err)
	assert.False(t, called, "hop must not run when the vision request cannot be built")
	require.NotNil(t, info.ImageRecognizePlan)
	assert.True(t, info.ImageRecognizePlan.Failed)
	assert.Contains(t, info.ImageRecognizePlan.VisibleBox, relaycommon.ImageRecognizeFailOpenNote)
	for _, msg := range req.Messages {
		for _, block := range msg.Content {
			_, isImg := block.(*bamboosdk.ImageBlock)
			assert.False(t, isImg)
		}
	}
}
