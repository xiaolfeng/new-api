package hosttool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMCPResponseJSON(t *testing.T) {
	body := []byte(`{"result":{"content":[{"type":"text","text":"hello world"}]}}`)
	text, err := parseMCPResponse(body)
	require.NoError(t, err)
	assert.Equal(t, "hello world", text)
}

func TestParseMCPResponseSSE(t *testing.T) {
	body := []byte("event: message\ndata: {\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"from sse\"}]}}\n\n")
	text, err := parseMCPResponse(body)
	require.NoError(t, err)
	assert.Equal(t, "from sse", text)
}

func TestRedactURL(t *testing.T) {
	got := redactURL("https://mcp.exa.ai/mcp?exaApiKey=secret")
	assert.Equal(t, "https://mcp.exa.ai/mcp", got)
}
