package openaicompat

import (
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
)

const responsesToChatFallbackTTL = 5 * time.Minute

type responsesToChatFallbackEntry struct {
	ExpiresAt time.Time
}

var responsesToChatFallbackCache sync.Map

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

func ShouldResponsesUseChatCompletionsCached(info *relaycommon.RelayInfo) bool {
	if !ShouldResponsesUseChatCompletions(info) {
		return false
	}
	key := ResponsesToChatCompletionsFallbackCacheKey(info)
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

func MarkResponsesToChatCompletionsFallback(info *relaycommon.RelayInfo) {
	if info == nil || !model_setting.IsResponsesToChatCompletionsEnabled() {
		return
	}
	key := ResponsesToChatCompletionsFallbackCacheKey(info)
	if key == "" {
		return
	}
	responsesToChatFallbackCache.Store(key, responsesToChatFallbackEntry{
		ExpiresAt: time.Now().Add(responsesToChatFallbackTTL),
	})
}

func ClearResponsesToChatCompletionsFallbackCache() {
	responsesToChatFallbackCache.Range(func(key, _ any) bool {
		responsesToChatFallbackCache.Delete(key)
		return true
	})
}

func ResponsesToChatCompletionsFallbackCacheKey(info *relaycommon.RelayInfo) string {
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
