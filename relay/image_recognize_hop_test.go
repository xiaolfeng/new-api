package relay

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	relaykittypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

func TestBuildImageRecognizeToolLog(t *testing.T) {
	st := model_setting.GetBambooSettings()
	prev := *st
	t.Cleanup(func() { *st = prev })
	st.ImageRecognizeChannelId = 9
	st.ImageRecognizeModel = "vision-model"

	msg := dto.Message{Role: "user"}
	msg.SetMediaContent([]dto.MediaContent{
		{Type: dto.ContentTypeText, Text: "see this"},
		{
			Type: dto.ContentTypeImageURL,
			ImageUrl: &dto.MessageImageUrl{Url: "data:image/png;base64,aaa"},
		},
		{
			Type: dto.ContentTypeImageURL,
			ImageUrl: &dto.MessageImageUrl{Url: "data:image/png;base64,bbb"},
		},
	})
	req := &dto.GeneralOpenAIRequest{Messages: []dto.Message{msg}}
	parent := &relaycommon.RelayInfo{
		UserId:          0,
		UsingGroup:      "default",
		OriginModelName: "deepseek-chat",
		RequestId:       "req-parent",
		RequestHeaders:  map[string]string{"X-Real-IP": "1.2.3.4"},
	}

	log := buildImageRecognizeToolLog(parent, req, time.Now().Add(-240*time.Millisecond), "[Image 1]\nA cat", nil)
	require.NotNil(t, log)
	assert.Equal(t, "req-parent", log.RequestId)
	assert.Equal(t, "deepseek-chat", log.ModelName)
	assert.Equal(t, 9, log.ChannelId)
	assert.Equal(t, "vision-model", log.Backend)
	assert.Equal(t, relaycommon.ImageRecognizeOriginal, log.OriginalName)
	assert.Equal(t, relaycommon.ImageRecognizeCanonical, log.Canonical)
	assert.Equal(t, relaycommon.ImageRecognizeToolKind, log.Kind)
	assert.Equal(t, "hop", log.Mode)
	assert.Equal(t, "2 images", log.Query)
	assert.Equal(t, "1.2.3.4", log.Ip)
	assert.Equal(t, "[Image 1]\nA cat", log.Result)
	assert.Empty(t, log.ErrorCode)
	assert.GreaterOrEqual(t, log.DurationMs, int64(200))
}

func TestBuildImageRecognizeToolLogRecordsError(t *testing.T) {
	parent := &relaycommon.RelayInfo{RequestId: "req-err", OriginModelName: "m"}
	hopErr := relaykittypes.NewError(assert.AnError, relaykittypes.ErrorCodeBadResponseBody)
	log := buildImageRecognizeToolLog(parent, nil, time.Now(), "", hopErr)
	require.NotNil(t, log)
	assert.Equal(t, string(relaykittypes.ErrorCodeBadResponseBody), log.ErrorCode)
	assert.NotEmpty(t, log.Result)
}

func TestBuildImageRecognizeToolLogSkipsNilParent(t *testing.T) {
	assert.Nil(t, buildImageRecognizeToolLog(nil, nil, time.Now(), "x", nil))
}

func TestCountVisionImages(t *testing.T) {
	assert.Equal(t, 0, countVisionImages(nil))
	msg := dto.Message{Role: "user"}
	msg.SetMediaContent([]dto.MediaContent{
		{Type: dto.ContentTypeImageURL, ImageUrl: &dto.MessageImageUrl{Url: "data:image/png;base64,a"}},
	})
	assert.Equal(t, 1, countVisionImages(&dto.GeneralOpenAIRequest{Messages: []dto.Message{msg}}))
}
