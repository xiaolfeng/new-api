package bamboo

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponsesBaseURLUsesWebSocket(t *testing.T) {
	assert.True(t, responsesBaseURLUsesWebSocket("wss://api.openai.com/v1"))
	assert.True(t, responsesBaseURLUsesWebSocket("  WS://gateway.example/v1 "))
	assert.False(t, responsesBaseURLUsesWebSocket("https://api.openai.com/v1"))
	assert.False(t, responsesBaseURLUsesWebSocket(""))
}

func TestStreamResponsesWebSocketConvertsCompletionsUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var sawModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var req map[string]any
		require.NoError(t, common.Unmarshal(body, &req))
		sawModel, _ = req["model"].(string)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: {\"id\":\"1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello-from-upstream\"}}]}\n\n")
		_, _ = fmt.Fprint(w, "data: {\"id\":\"2\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":4,\"completion_tokens\":2,\"total_tokens\":6}}\n\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(server.Close)

	info := makeInfoWithBaseURL(constant.APITypeOpenAI, server.URL)
	info.OriginModelName = "gpt-test"
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	var payloads []string
	result, apiErr := StreamResponsesWebSocket(c.Request.Context(), c, info, []byte(`{"model":"gpt-test","input":"hi"}`), "stream-a", false, NewResponsesWSMemory(), func(payload []byte) error {
		payloads = append(payloads, string(payload))
		return nil
	})
	require.Nil(t, apiErr)
	require.True(t, result.Delivered)
	assert.Equal(t, "gpt-test", sawModel)
	joined := strings.Join(payloads, "\n")
	assert.Contains(t, joined, "hello-from-upstream")
	assert.Contains(t, joined, `"stream_id":"stream-a"`)
	assert.NotContains(t, joined, "data:")
}

func TestStreamResponsesWebSocketMissingPreviousResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	info := makeInfoWithBaseURL(constant.APITypeAnthropic, "http://127.0.0.1:1")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	called := false
	result, apiErr := StreamResponsesWebSocket(c.Request.Context(), c, info, []byte(`{"model":"claude","input":"hi","previous_response_id":"resp_missing"}`), "", false, NewResponsesWSMemory(), func([]byte) error {
		called = true
		return nil
	})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.False(t, result.Delivered)
	assert.False(t, called)
	assert.ErrorContains(t, apiErr, "resp_missing")
}

func TestStreamResponsesWebSocketGenerateFalseSkipsUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	t.Cleanup(server.Close)
	info := makeInfoWithBaseURL(constant.APITypeOpenAI, server.URL)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	var payloads []string
	result, apiErr := StreamResponsesWebSocket(c.Request.Context(), c, info, []byte(`{"model":"gpt-test","input":"remember this"}`), "warm", true, NewResponsesWSMemory(), func(payload []byte) error {
		payloads = append(payloads, string(payload))
		return nil
	})
	require.Nil(t, apiErr)
	require.True(t, result.Delivered)
	assert.False(t, called)
	joined := strings.Join(payloads, "\n")
	assert.Contains(t, joined, "response.created")
	assert.Contains(t, joined, "response.completed")
	assert.Contains(t, joined, `"stream_id":"warm"`)
}
