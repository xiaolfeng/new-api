package bamboo

import (
	"testing"

	"github.com/QuantumNous/new-api/types"
)

func TestRelayFormatToCodec_SupportedFormats(t *testing.T) {
	cases := []struct {
		name   string
		format types.RelayFormat
		want   string // codec.FormatType 的 string 值
	}{
		{"OpenAI", types.RelayFormatOpenAI, "openai"},
		{"Claude", types.RelayFormatClaude, "anthropic"},
		{"Responses", types.RelayFormatOpenAIResponses, "responses"},
		{"Gemini", types.RelayFormatGemini, "gemini"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := relayFormatToCodec(c.format)
			if !ok {
				t.Fatalf("expected ok=true for %s, got false", c.name)
			}
			if string(got) != c.want {
				t.Fatalf("expected %q, got %q", c.want, string(got))
			}
		})
	}
}

func TestRelayFormatToCodec_UnsupportedFormats(t *testing.T) {
	// 非对话格式不应映射到 codec（bridge 应据此 fallback）
	unsupported := []types.RelayFormat{
		types.RelayFormatOpenAIAudio,
		types.RelayFormatOpenAIImage,
		types.RelayFormatEmbedding,
		types.RelayFormatRerank,
		types.RelayFormatTask,
		types.RelayFormatOpenAIRealtime,
	}
	for _, f := range unsupported {
		_, ok := relayFormatToCodec(f)
		if ok {
			t.Fatalf("expected ok=false for format %q, got true", f)
		}
	}
}
