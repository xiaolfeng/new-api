package bamboo

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
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
