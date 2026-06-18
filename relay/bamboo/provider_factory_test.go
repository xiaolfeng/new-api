package bamboo

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// makeInfo 构造一个最小可用的 RelayInfo（补 apiKey/baseURL 避免 provider 空值 panic）。
// 注意：RelayInfo 通过指针嵌入 *ChannelMeta（relay_info.go:198），
// 必须显式初始化 ChannelMeta，否则访问 ApiType/ApiKey 等提升字段会 nil 解引用 panic。
func makeInfo(apiType int) *relaycommon.RelayInfo {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiType:         apiType,
			ApiKey:          "test-key",
			ChannelBaseUrl:  "https://api.example.com",
		},
	}
	return info
}

func TestNewProvider_SupportedOpenAICompatible(t *testing.T) {
	// OpenAI 兼容渠道应返回非 nil provider 且无错误
	supportedTypes := []int{
		constant.APITypeOpenAI,
		constant.APITypeDeepSeek,
		constant.APITypeMoonshot,
		constant.APITypeSiliconFlow,
		constant.APITypeMistral,
		constant.APITypeXai,
		constant.APITypeZhipuV4,
		constant.APITypePerplexity,
		constant.APITypeCohere,
		constant.APITypeMiniMax,
		constant.APITypeBaiduV2,
		constant.APITypeOpenRouter,
		constant.APITypeXinference,
	}
	for _, apiType := range supportedTypes {
		info := makeInfo(apiType)
		p, err := newProvider(info)
		if err != nil {
			t.Errorf("APIType %d: expected nil err, got %v", apiType, err)
			continue
		}
		if p == nil {
			t.Errorf("APIType %d: expected non-nil provider", apiType)
		}
	}
}

func TestNewProvider_SupportedNativeProtocols(t *testing.T) {
	nativeTypes := []int{
		constant.APITypeAnthropic,
		constant.APITypeGemini,
		constant.APITypeCodex,
	}
	for _, apiType := range nativeTypes {
		info := makeInfo(apiType)
		p, err := newProvider(info)
		if err != nil {
			t.Errorf("APIType %d: expected nil err, got %v", apiType, err)
			continue
		}
		if p == nil {
			t.Errorf("APIType %d: expected non-nil provider", apiType)
		}
	}
}

func TestNewProvider_UnsupportedReturnsFallback(t *testing.T) {
	// AWS/讯飞/腾讯等未覆盖渠道应返回包裹 ErrUnsupportedProvider 的错误
	unsupportedTypes := []int{
		constant.APITypeAws,
		constant.APITypeXunfei,
		constant.APITypeTencent,
		constant.APITypeZhipu, // 智谱 v3 JWT
		constant.APITypeCoze,
		constant.APITypeDify,
		constant.APITypeBaidu, // 千帆 v1
		constant.APITypeAli,   // DashScope
	}
	for _, apiType := range unsupportedTypes {
		info := makeInfo(apiType)
		p, err := newProvider(info)
		if p != nil {
			t.Errorf("APIType %d: expected nil provider for unsupported", apiType)
			continue
		}
		if err == nil {
			t.Errorf("APIType %d: expected non-nil err for unsupported", apiType)
			continue
		}
		// *types.NewAPIError 实现了 Unwrap，errors.Is 链可达 ErrUnsupportedProvider
		if !errors.Is(err, ErrUnsupportedProvider) {
			t.Errorf("APIType %d: expected ErrUnsupportedProvider in chain, got %v", apiType, err)
		}
	}
}
