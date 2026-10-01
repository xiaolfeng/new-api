package bamboo

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	_ "github.com/bamboo-services/bamboo-messages/bamboo/codec/openai"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
)

func TestChatRelay_UnsupportedFormatFallsBack(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	// 用 OpenAI ApiType 但传非对话 RelayFormat（Audio）
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiType:        0, // APITypeOpenAI
			ApiKey:         "test-key",
			ChannelBaseUrl: "https://api.example.com",
		},
	}

	_, err := ChatRelay(c, info, types.RelayFormatOpenAIAudio, []byte("{}"))
	if err == nil {
		t.Fatal("expected error for unsupported format, got nil")
	}
	if !errors.Is(err, ErrUnsupportedProvider) {
		t.Fatalf("expected ErrUnsupportedProvider for audio format, got %v", err)
	}
}

func TestChatRelay_UnsupportedProviderFallsBack(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	// 用不存在的 ApiType 触发 provider_factory 的 default fallback
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiType:        9999, // 不存在的 ApiType
			ApiKey:         "test-key",
			ChannelBaseUrl: "https://api.example.com",
		},
	}

	_, err := ChatRelay(c, info, types.RelayFormatOpenAI, []byte("{}"))
	if err == nil {
		t.Fatal("expected error for unsupported provider, got nil")
	}
	if !errors.Is(err, ErrUnsupportedProvider) {
		t.Fatalf("expected ErrUnsupportedProvider, got %v", err)
	}
}

func TestChatRelay_InjectsClaudeDefaultMaxTokensWhenUnspecified(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	info := &relaycommon.RelayInfo{
		OriginModelName: "claude-3-5-sonnet-20241022",
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiType:        1, // APITypeAnthropic
			ApiKey:         "test-key",
			ChannelBaseUrl: "https://api.anthropic.com",
		},
	}

	// 请求体内未包含 max_tokens（OpenAI 入口）
	reqBody := []byte(`{"model":"claude-3-5-sonnet-20241022","messages":[{"role":"user","content":"hi"}]}`)
	codecFmt, _ := relayFormatToCodec(types.RelayFormatOpenAI)
	codec, _ := bamboocodec.Get(codecFmt)
	relayReq, err := codec.ParseRequest(reqBody)
	if err != nil {
		t.Fatalf("ParseRequest failed: %v", err)
	}

	if relayReq.Config == nil {
		relayReq.Config = &bamboosdk.RequestConfig{}
	}
	relayReq.Config.Model = info.GetOriginModelName()

	if relayReq.Config.MaxTokens <= 0 && resolveUpstreamRelayFormat(info) == types.RelayFormatClaude {
		if def := model_setting.GetClaudeSettings().GetDefaultMaxTokens(relayReq.Config.Model); def > 0 {
			relayReq.Config.MaxTokens = int64(def)
		}
	}

	expectedMax := int64(model_setting.GetClaudeSettings().GetDefaultMaxTokens("claude-3-5-sonnet-20241022"))
	if relayReq.Config.MaxTokens != expectedMax {
		t.Fatalf("expected MaxTokens=%d, got %d", expectedMax, relayReq.Config.MaxTokens)
	}
}
func TestChatRelay_ModelMappingPrecedence(t *testing.T) {
	info := &relaycommon.RelayInfo{
		OriginModelName: "gemini-3.8-flash",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-2.5-flash",
			IsModelMapped:     true,
		},
	}

	reqBody := []byte(`{"model":"gemini-3.8-flash","messages":[{"role":"user","content":"hi"}]}`)
	codecFmt, _ := relayFormatToCodec(types.RelayFormatOpenAI)
	codec, _ := bamboocodec.Get(codecFmt)
	relayReq, err := codec.ParseRequest(reqBody)
	if err != nil {
		t.Fatalf("ParseRequest failed: %v", err)
	}

	if info.IsModelMapped && info.GetUpstreamModelName() != "" {
		relayReq.Config.Model = info.GetUpstreamModelName()
	} else if relayReq.Config.Model == "" {
		if model := info.GetUpstreamModelName(); model != "" {
			relayReq.Config.Model = model
		} else {
			relayReq.Config.Model = info.GetOriginModelName()
		}
	}

	if relayReq.Config.Model != "gemini-2.5-flash" {
		t.Fatalf("expected mapped model %q, got %q", "gemini-2.5-flash", relayReq.Config.Model)
	}
}

