package openaicompat

import (
	"strconv"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/stretchr/testify/require"
)

func TestResponsesToChatCompletionsFallbackCacheKeySeparatesChannelAndModel(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         10,
			ChannelType:       1,
			UpstreamModelName: "gpt-4o",
			ApiVersion:        "2024-10-21",
		},
		RelayMode: relayconstant.RelayModeResponses,
	}

	require.Equal(t, "10|1|gpt-4o|2024-10-21|"+strconv.Itoa(relayconstant.RelayModeResponses), ResponsesToChatCompletionsFallbackCacheKey(info))

	otherModel := *info
	otherModel.ChannelMeta = &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4.1"}
	require.NotEqual(t, ResponsesToChatCompletionsFallbackCacheKey(info), ResponsesToChatCompletionsFallbackCacheKey(&otherModel))

	otherChannel := *info
	otherChannel.ChannelMeta = &relaycommon.ChannelMeta{
		ChannelId:         11,
		ChannelType:       1,
		UpstreamModelName: "gpt-4o",
		ApiVersion:        "2024-10-21",
	}
	require.NotEqual(t, ResponsesToChatCompletionsFallbackCacheKey(info), ResponsesToChatCompletionsFallbackCacheKey(&otherChannel))
}

func TestResponsesToChatCompletionsFallbackCacheHit(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	originalEnabled := settings.ResponsesToChatCompletionsEnabled
	settings.ResponsesToChatCompletionsEnabled = true
	defer func() {
		settings.ResponsesToChatCompletionsEnabled = originalEnabled
		ClearResponsesToChatCompletionsFallbackCache()
	}()

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         10,
			ChannelType:       1,
			UpstreamModelName: "gpt-4o",
		},
		RelayMode: relayconstant.RelayModeResponses,
	}

	ClearResponsesToChatCompletionsFallbackCache()
	require.False(t, ShouldResponsesUseChatCompletionsCached(info))

	MarkResponsesToChatCompletionsFallback(info)
	require.True(t, ShouldResponsesUseChatCompletionsCached(info))

	info.ChannelId = 11
	require.False(t, ShouldResponsesUseChatCompletionsCached(info))
}
