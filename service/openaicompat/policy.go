package openaicompat

import (
	"slices"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
)

func ShouldChatCompletionsUseResponsesPolicy(policy model_setting.ChatCompletionsToResponsesPolicy, channelID int, channelType int, model string) bool {
	if !policy.IsChannelEnabled(channelID, channelType) {
		return false
	}
	return matchAnyRegex(policy.ModelPatterns, model)
}

func ShouldChatCompletionsUseResponsesGlobal(channelID int, channelType int, model string) bool {
	return ShouldChatCompletionsUseResponsesPolicy(
		model_setting.GetGlobalSettings().ChatCompletionsToResponsesPolicy,
		channelID,
		channelType,
		model,
	)
}

// ShouldResponsesUseChatCompletions 判断是否应将 Responses 格式请求转换为 Chat Completions 格式。
// 当双向转换同时启用时，通过 RequestConversionChain 检测循环以防止无限递归。
func ShouldResponsesUseChatCompletions(info *relaycommon.RelayInfo) bool {
	if !model_setting.IsResponsesToChatCompletionsEnabled() {
		return false
	}
	// 循环守卫：如果转换链中已包含 openai 格式，说明已经历过 Responses→ChatCompletions 转换，
	// 此时若 ChatCompletions→Responses 也启用，会导致无限循环，必须阻止。
	if slices.Contains(info.RequestConversionChain, types.RelayFormatOpenAI) {
		return false
	}
	return true
}
