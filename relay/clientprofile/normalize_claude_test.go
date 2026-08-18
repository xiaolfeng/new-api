package clientprofile

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeClaudeDropsInputJSONDeltaOnTextBlock(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ClientProfile: common.ClientProfileClaudeCode,
		RelayFormat:   types.RelayFormatClaude,
		RequestId:     "req-drop",
	}

	start := []byte(`event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

`)
	require.NotEmpty(t, NormalizeClaudeSSEFrame(info, start))

	delta := []byte(`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"query\":\"x\"}"}}

`)
	assert.Nil(t, NormalizeClaudeSSEFrame(info, delta))
}

func TestNormalizeClaudeResponsesHasNoServerToolUse(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ClientProfile: common.ClientProfileClaudeCode,
		RelayFormat:   types.RelayFormatOpenAIResponses,
	}
	raw := []byte(`{"type":"response","output":[{"type":"function_call","name":"read_file"}]}`)
	got := NormalizeClaudeSSEData(info, raw)
	assert.Equal(t, raw, got)
	assert.NotContains(t, string(got), "server_tool_use")
}

func TestNormalizeClaudeNoOpForGrokOnMessages(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ClientProfile: common.ClientProfileGrokBuild,
		RelayFormat:   types.RelayFormatClaude,
	}
	raw := []byte(`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{}"}}

`)
	got := NormalizeClaudeSSEFrame(info, raw)
	assert.Equal(t, raw, got)
}

func TestNormalizeClaudeKeepsHelperStreamingShape(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ClientProfile: common.ClientProfileClaudeCode,
		RelayFormat:   types.RelayFormatClaude,
	}
	start := []byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","content":[]}}

`)
	require.Equal(t, start, NormalizeClaudeSSEFrame(info, start))

	block := []byte(`event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"server_tool_use","id":"srvtoolu_1","name":"web_search","input":""}}

`)
	require.Equal(t, block, NormalizeClaudeSSEFrame(info, block))

	delta := []byte(`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"query\":\"q\"}"}}

`)
	assert.Equal(t, delta, NormalizeClaudeSSEFrame(info, delta))
}
