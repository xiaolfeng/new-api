package imagerec

import (
	"net/http/httptest"
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
	err := MaybeRewrite(testGinContext(), info, req)
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
	err := MaybeRewrite(testGinContext(), info, req)
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
	hopFunc = func(c *gin.Context, parent *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (string, *dto.Usage, *kittypes.NewAPIError) {
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
	err := MaybeRewrite(testGinContext(), info, req)
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
	hopFunc = func(c *gin.Context, parent *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (string, *dto.Usage, *kittypes.NewAPIError) {
		called = true
		return "should not run", nil, nil
	}

	info := &relaycommon.RelayInfo{OriginModelName: "deepseek-chat"}
	req := imageReq([]bamboosdk.BambooMessage{
		bamboosdk.NewUserMessageBlocks(pngBlock("oldimg")),
		bamboosdk.NewAssistantMessage("<<<image_recognition>>>\nold\n<<<end_image_recognition>>>"),
		bamboosdk.NewUserMessage("just text now"),
	})
	err := MaybeRewrite(testGinContext(), info, req)
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
	err := MaybeRewrite(testGinContext(), info, req)
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

func TestSplitCaptionsPrefersMarkers(t *testing.T) {
	got := splitCaptions("[Image 1]\ncat\n[Image 2]\ndog", 2, 4096)
	require.Len(t, got, 2)
	assert.Equal(t, "cat", got[0])
	assert.Equal(t, "dog", got[1])
}
