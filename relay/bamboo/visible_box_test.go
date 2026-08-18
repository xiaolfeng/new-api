package bamboo

import (
	"testing"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestPrependVisibleBox(t *testing.T) {
	resp := &bamboosdk.Response{
		Content: []bamboosdk.ContentBlock{bamboosdk.NewTextBlock("hello")},
	}
	info := &relaycommon.RelayInfo{
		ImageRecognizePlan: &relaycommon.ImageRecognizePlan{
			VisibleBox: "<<<image_recognition>>>\n[Image 1]\ncat\n<<<end_image_recognition>>>\n",
		},
	}
	prependVisibleBox(resp, info)
	require.Len(t, resp.Content, 2)
	first, ok := resp.Content[0].(*bamboosdk.ThinkingBlock)
	require.True(t, ok)
	assert.Contains(t, first.Thinking, relaycommon.ImageRecognizeFenceStart)
	second, ok := resp.Content[1].(*bamboosdk.TextBlock)
	require.True(t, ok)
	assert.Equal(t, "hello", second.Text)
}

func TestPrependVisibleBoxSkipWhenEmpty(t *testing.T) {
	resp := &bamboosdk.Response{
		Content: []bamboosdk.ContentBlock{bamboosdk.NewTextBlock("hello")},
	}
	prependVisibleBox(resp, &relaycommon.RelayInfo{})
	require.Len(t, resp.Content, 1)
}
