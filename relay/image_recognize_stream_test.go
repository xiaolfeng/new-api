package relay

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/QuantumNous/new-api/relay/bamboo/imagerec"
)

type captureLive struct {
	deltas []string
}

func (c *captureLive) Begin() bool         { return true }
func (c *captureLive) OnDelta(text string) { c.deltas = append(c.deltas, text) }
func (c *captureLive) End()                {}

func TestParseVisionStreamDataOpenAI(t *testing.T) {
	text, usage := parseVisionStreamData(`{"choices":[{"delta":{"content":"hello "}}]}`)
	assert.Equal(t, "hello ", text)
	assert.Nil(t, usage)

	text, _ = parseVisionStreamData(`{"choices":[{"delta":{"reasoning_content":"think"}}]}`)
	assert.Equal(t, "think", text)

	text, usage = parseVisionStreamData(`{"choices":[{"delta":{}}],"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}`)
	assert.Equal(t, "", text)
	require.NotNil(t, usage)
	assert.Equal(t, 3, usage.PromptTokens)
	assert.Equal(t, 4, usage.CompletionTokens)
}

func TestParseVisionStreamDataAnthropic(t *testing.T) {
	text, _ := parseVisionStreamData(`{"type":"content_block_delta","delta":{"type":"text_delta","text":"cat"}}`)
	assert.Equal(t, "cat", text)
}

func TestCaptureLiveMatchesCaptionSink(t *testing.T) {
	var live imagerec.CaptionLive = &captureLive{}
	live.OnDelta("a")
	live.OnDelta("b")
	got := live.(*captureLive)
	assert.Equal(t, []string{"a", "b"}, got.deltas)
}
