package clientprofile

import (
	"bytes"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/bamboo/hosttool"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func grokResponsesInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ClientProfile: common.ClientProfileGrokBuild,
		RelayFormat:   types.RelayFormatOpenAIResponses,
		RequestId:     "req1",
		StartTime:     time.Unix(1700000000, 0),
	}
}

func TestShouldNormalizeResponses(t *testing.T) {
	info := grokResponsesInfo()
	assert.True(t, ShouldNormalizeResponses(info))

	info.RelayFormat = types.RelayFormatClaude
	assert.False(t, ShouldNormalizeResponses(info))

	info = grokResponsesInfo()
	info.ClientProfile = common.ClientProfileCodex
	assert.False(t, ShouldNormalizeResponses(info))
}

func TestNormalizeFunctionCallMissingAndObjectArgs(t *testing.T) {
	info := grokResponsesInfo()
	raw := []byte(`{"object":"response","created_at":1,"output":[{"type":"function_call","name":"read_file"}]}`)
	out := NormalizeResponsesPayload(info, raw)
	var root map[string]any
	require.NoError(t, common.Unmarshal(out, &root))
	assert.Contains(t, root, "created")
	assert.Contains(t, root, "created_at")
	item := root["output"].([]any)[0].(map[string]any)
	assert.Equal(t, "{}", item["arguments"])
	assert.Equal(t, "completed", item["status"])
	assert.NotEmpty(t, item["id"])
	assert.NotEmpty(t, item["call_id"])
	assert.True(t, info.EgressFilledArgs)
	assert.True(t, info.EgressFilledCreated)

	info = grokResponsesInfo()
	raw = []byte(`{"object":"response","created_at":1,"created":1,"output":[{"type":"function_call","id":"fc_1","call_id":"call_1","name":"read_file","status":"completed","arguments":{"target_file":"foo.go"}}]}`)
	out = NormalizeResponsesPayload(info, raw)
	require.NoError(t, common.Unmarshal(out, &root))
	item = root["output"].([]any)[0].(map[string]any)
	args, ok := item["arguments"].(string)
	require.True(t, ok)
	assert.Contains(t, args, "target_file")
	assert.NotContains(t, args, `[WebSearch]`)
}

func TestNormalizeFunctionCallInputAndEmptyString(t *testing.T) {
	info := grokResponsesInfo()
	raw := []byte(`{"object":"response","created_at":1,"created":1,"output":[{"type":"function_call","id":"fc_1","call_id":"call_1","name":"read_file","status":"completed","input":{"target_file":"a.go"}}]}`)
	out := NormalizeResponsesPayload(info, raw)
	var root map[string]any
	require.NoError(t, common.Unmarshal(out, &root))
	item := root["output"].([]any)[0].(map[string]any)
	_, hasInput := item["input"]
	assert.False(t, hasInput)
	assert.Equal(t, `{"target_file":"a.go"}`, item["arguments"])

	info = grokResponsesInfo()
	raw = []byte(`{"object":"response","created_at":1,"created":1,"output":[{"type":"function_call","id":"fc_1","call_id":"call_1","name":"read_file","status":"completed","arguments":""}]}`)
	out = NormalizeResponsesPayload(info, raw)
	require.NoError(t, common.Unmarshal(out, &root))
	item = root["output"].([]any)[0].(map[string]any)
	assert.Equal(t, "{}", item["arguments"])
}

func TestNormalizeFirstFunctionCallFrameHasArguments(t *testing.T) {
	info := grokResponsesInfo()
	raw := []byte(`{"type":"response.output_item.added","item":{"type":"function_call","name":"read_file"}}`)
	out := NormalizeResponsesSSEData(info, raw)
	var root map[string]any
	require.NoError(t, common.Unmarshal(out, &root))
	item := root["item"].(map[string]any)
	assert.Equal(t, "{}", item["arguments"])
	assert.Equal(t, "in_progress", item["status"])
	_, hasCreated := item["created"]
	assert.False(t, hasCreated)
}

func TestNormalizeGrokMessagesIsByteStable(t *testing.T) {
	info := grokResponsesInfo()
	info.RelayFormat = types.RelayFormatClaude
	raw := []byte(`{"type":"message","content":[{"type":"text","text":"hi"}]}`)
	got := NormalizeResponsesPayload(info, raw)
	assert.Equal(t, raw, got)
	assert.True(t, bytes.Equal(raw, got))
}

func TestNormalizeAlreadyLegalGrokDoesNotRemarshal(t *testing.T) {
	info := grokResponsesInfo()
	raw := []byte(`{"object":"response","created":1,"created_at":1,"output":[{"type":"function_call","id":"fc_1","call_id":"call_1","name":"read_file","status":"completed","arguments":"{}"}]}`)
	got := NormalizeResponsesPayload(info, raw)
	assert.Equal(t, raw, got)
}

func TestNormalizeResponseCreatedEventRootGetsCreated(t *testing.T) {
	info := grokResponsesInfo()
	raw := []byte(`{"type":"response.created","sequence_number":1,"response_id":"resp_1","response":{"id":"resp_1","object":"response","created_at":1700000000,"status":"in_progress","model":"grok-4.6","output":[],"usage":{"input_tokens":0,"output_tokens":0,"total_tokens":0}}}`)
	out := NormalizeResponsesSSEData(info, raw)
	var root map[string]any
	require.NoError(t, common.Unmarshal(out, &root))
	assert.Contains(t, root, "created")
	_, rootCreatedAt := root["created_at"]
	assert.False(t, rootCreatedAt)
	resp := root["response"].(map[string]any)
	assert.Contains(t, resp, "created")
	assert.Contains(t, resp, "created_at")
	_, itemCreated := root["item"]
	assert.False(t, itemCreated)
}

func TestNormalizeOutputItemEventDoesNotGetCreated(t *testing.T) {
	info := grokResponsesInfo()
	raw := []byte(`{"type":"response.output_item.added","output_index":0,"item":{"type":"web_search_call","id":"ws_1","status":"in_progress","action":{"type":"search","query":"q"}}}`)
	out := NormalizeResponsesSSEData(info, raw)
	var root map[string]any
	require.NoError(t, common.Unmarshal(out, &root))
	_, hasCreated := root["created"]
	assert.False(t, hasCreated)
	item := root["item"].(map[string]any)
	_, itemCreated := item["created"]
	assert.False(t, itemCreated)
}

func TestNormalizeWebSearchCallDoesNotGainArguments(t *testing.T) {
	info := grokResponsesInfo()
	raw := []byte(`{"object":"response","created_at":9,"output":[{"type":"web_search_call","id":"ws_1","status":"completed","action":{"type":"search","query":"q"}}]}`)
	out := NormalizeResponsesPayload(info, raw)
	var root map[string]any
	require.NoError(t, common.Unmarshal(out, &root))
	item := root["output"].([]any)[0].(map[string]any)
	_, hasArgs := item["arguments"]
	assert.False(t, hasArgs)
	assert.Equal(t, "web_search_call", item["type"])
	assert.Contains(t, root, "created")
}

func TestNormalizeCodexDoesNotGetCreated(t *testing.T) {
	body, err := hosttool.MarshalBuiltinComplete("grok-4.6", "req1", 9, hosttool.ExecResult{
		Kind:  "search",
		OK:    true,
		Query: "q",
		Hits:  []hosttool.SearchHit{{Title: "T", URL: "https://example.com"}},
	})
	require.NoError(t, err)

	codex := &relaycommon.RelayInfo{
		ClientProfile: common.ClientProfileCodex,
		RelayFormat:   types.RelayFormatOpenAIResponses,
		RequestId:     "req1",
		StartTime:     time.Unix(1700000000, 0),
	}
	codexOut := NormalizeResponsesPayload(codex, body)
	assert.Equal(t, body, codexOut)
	assert.NotContains(t, string(codexOut), `"created":`)

	grok := grokResponsesInfo()
	grokOut := NormalizeResponsesPayload(grok, append([]byte(nil), body...))
	assert.Contains(t, string(grokOut), `"created":`)
	assert.Contains(t, string(grokOut), `"created_at":`)
	assert.Contains(t, string(grokOut), `"type":"web_search_call"`)
}

func TestNormalizeBuiltinStreamFramesHelper3(t *testing.T) {
	frames, err := hosttool.BuiltinStreamFrames("grok-4.6", "req1", 9, hosttool.ExecResult{
		Kind:  "search",
		OK:    true,
		Query: "q",
		Hits:  []hosttool.SearchHit{{Title: "T", URL: "https://example.com"}},
	})
	require.NoError(t, err)
	info := grokResponsesInfo()
	var joined bytes.Buffer
	for _, frame := range frames {
		out := NormalizeResponsesSSEFrame(info, frame)
		joined.Write(out)
		if bytes.Contains(frame, []byte("response.created")) || bytes.Contains(frame, []byte("response.completed")) {
			assert.Contains(t, string(out), `"created":`)
			assert.Contains(t, string(out), `"created_at":`)
			var ev map[string]any
			for _, line := range bytes.Split(out, []byte("\n")) {
				trim := bytes.TrimSpace(line)
				if bytes.HasPrefix(trim, []byte("data:")) {
					require.NoError(t, common.Unmarshal(bytes.TrimSpace(trim[5:]), &ev))
					break
				}
			}
			require.NotEmpty(t, ev)
			assert.Contains(t, ev, "created")
		}
	}
	assert.Contains(t, joined.String(), `"type":"web_search_call"`)

	ping := []byte(": ping\n\n")
	assert.Equal(t, ping, NormalizeResponsesSSEFrame(info, ping))
}

func TestNormalizeResponsesMissedUAWarn(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ClientProfile: common.ClientProfileGeneric,
		RelayFormat:   types.RelayFormatOpenAIResponses,
		RequestId:     "miss1",
	}
	raw := []byte(`{"object":"response","output":[{"type":"function_call","name":"read_file"}]}`)
	// 调用方禁止预判 ShouldNormalizeResponses；generic 必须进 helper 才能观测。
	out := NormalizeResponsesPayload(info, raw)
	assert.Equal(t, raw, out)
	assert.True(t, info.EgressMissedUAWarned)

	out2 := NormalizeResponsesPayload(info, raw)
	assert.Equal(t, raw, out2)
	assert.True(t, info.EgressMissedUAWarned)
}

func TestProductionGrokUAMatchesProfile(t *testing.T) {
	id := common.MatchClientProfile(common.ProductionGrokBuildUserAgent, nil)
	require.Equal(t, common.ClientProfileGrokBuild, id.Profile)
	assert.Equal(t, common.ClientSourceGrokBuild, id.Source)
	assert.Equal(t, "grok-pager", id.Hit)
}

func TestOfficialCodexUAKeepsWebSearchCallWithoutCreated(t *testing.T) {
	id := common.MatchClientProfile(common.OfficialCodexCLIUserAgent, map[string]string{
		"Originator": "codex_cli_rs",
	})
	require.Equal(t, common.ClientProfileCodex, id.Profile)
	assert.Equal(t, "codex_cli_rs", id.Hit)

	body, err := hosttool.MarshalBuiltinComplete("gpt-5.4", "req-codex", 9, hosttool.ExecResult{
		Kind:  "search",
		OK:    true,
		Query: "codex search",
		Hits:  []hosttool.SearchHit{{Title: "Docs", URL: "https://example.com/codex"}},
	})
	require.NoError(t, err)

	info := &relaycommon.RelayInfo{
		ClientProfile: id.Profile,
		RelayFormat:   types.RelayFormatOpenAIResponses,
		RequestId:     "req-codex",
		StartTime:     time.Unix(1700000000, 0),
	}
	out := NormalizeResponsesPayload(info, body)
	assert.Equal(t, body, out)
	assert.Contains(t, string(out), `"type":"web_search_call"`)
	assert.Contains(t, string(out), `"query":"codex search"`)
	assert.NotContains(t, string(out), `"created":`)
	assert.NotContains(t, string(out), "server_tool_use")
	assert.NotContains(t, string(out), `[WebSearch]`)
}
