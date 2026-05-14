package openai

import (
	"bufio"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func parseChatStreamChunks(body string) []dto.ChatCompletionsStreamResponse {
	chunks := make([]dto.ChatCompletionsStreamResponse, 0)
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "" || data == "[DONE]" {
			continue
		}
		var chunk dto.ChatCompletionsStreamResponse
		if err := common.UnmarshalJsonStr(data, &chunk); err == nil {
			chunks = append(chunks, chunk)
		}
	}
	return chunks
}

func TestOaiResponsesToChatStreamKeepsToolDeltaAfterText(t *testing.T) {
	c, w := setupGinContext("test-resp-chat-stream")
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-4o",
		},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: buildSSEBody([]string{
			`{"type":"response.created","response":{"id":"resp_1","model":"gpt-4o","created_at":1700000000}}`,
			`{"type":"response.output_text.delta","delta":"先查一下。"}`,
			`{"type":"response.output_item.added","item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"exec_command"}}`,
			`{"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":"{\"cmd\":\"pwd\"}"}`,
			`{"type":"response.completed","response":{"id":"resp_1","model":"gpt-4o","created_at":1700000000,"usage":{"input_tokens":10,"output_tokens":6,"total_tokens":16}}}`,
		}),
	}

	usage, apiErr := OaiResponsesToChatStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)

	chunks := parseChatStreamChunks(w.Body.String())
	require.NotEmpty(t, chunks)

	var sawText bool
	var sawToolName bool
	var sawToolArgs bool
	var finishReason string
	for _, chunk := range chunks {
		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]
		if choice.Delta.Content != nil && *choice.Delta.Content == "先查一下。" {
			sawText = true
		}
		for _, toolCall := range choice.Delta.ToolCalls {
			if toolCall.Function.Name == "exec_command" {
				sawToolName = true
			}
			if toolCall.Function.Arguments == `{"cmd":"pwd"}` {
				sawToolArgs = true
			}
		}
		if choice.FinishReason != nil {
			finishReason = *choice.FinishReason
		}
	}

	require.True(t, sawText)
	require.True(t, sawToolName)
	require.True(t, sawToolArgs)
	require.Equal(t, "tool_calls", finishReason)
}
