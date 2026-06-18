package bamboo

import (
	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	"github.com/QuantumNous/new-api/types"
)

// relayFormatToCodec 把 new-api 的 RelayFormat 映射为 bamboo codec 的 FormatType。
//
// 包内私有，由 ChatRelay 内部使用。
// 非对话格式（Audio/Image/Task/Realtime/Rerank/Embedding 等）返回 ("", false)，
// 调用方据此 fallback 到 new-api 原生链路，避免误入 bamboo 路径。
func relayFormatToCodec(f types.RelayFormat) (bamboocodec.FormatType, bool) {
	switch f {
	case types.RelayFormatOpenAI:
		return bamboocodec.FormatOpenAI, true
	case types.RelayFormatClaude:
		return bamboocodec.FormatAnthropic, true
	case types.RelayFormatOpenAIResponses:
		return bamboocodec.FormatResponses, true
	case types.RelayFormatGemini:
		return bamboocodec.FormatGemini, true
	default:
		return "", false
	}
}
