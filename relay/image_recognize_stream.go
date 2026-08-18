package relay

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/bamboo/imagerec"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
)

func consumeVisionStream(resp *http.Response, live imagerec.CaptionLive) (string, *dto.Usage, error) {
	if resp == nil || resp.Body == nil {
		return "", nil, fmt.Errorf("image recognition stream is empty")
	}
	defer service.CloseResponseBodyGracefully(resp)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", nil, fmt.Errorf("image recognition upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var caption strings.Builder
	usage := &dto.Usage{UsageSemantic: "openai"}
	scanner := helper.NewStreamScanner(resp.Body)
	scanner.Split(bufio.ScanLines)
	var leftover strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			if strings.TrimSpace(line) != "" && leftover.Len() < 1<<20 {
				leftover.WriteString(line)
			}
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		text, chunkUsage := parseVisionStreamData(data)
		if text != "" {
			caption.WriteString(text)
			if live != nil {
				live.OnDelta(text)
			}
		}
		mergeVisionUsage(usage, chunkUsage)
	}
	if err := scanner.Err(); err != nil {
		return caption.String(), usage, err
	}
	if strings.TrimSpace(caption.String()) == "" && leftover.Len() > 0 {
		fallback := extractCaptionFromJSON([]byte(leftover.String()))
		if fallback != "" {
			caption.WriteString(fallback)
			if live != nil {
				live.OnDelta(fallback)
			}
		}
	}
	if usage.TotalTokens == 0 && (usage.PromptTokens > 0 || usage.CompletionTokens > 0) {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	return caption.String(), usage, nil
}

func parseVisionStreamData(data string) (string, *dto.Usage) {
	var raw map[string]any
	if err := rootcommon.UnmarshalJsonStr(data, &raw); err != nil {
		return "", nil
	}
	text := visionTextFromOpenAI(raw)
	if text == "" {
		text = visionTextFromAnthropic(raw)
	}
	return text, visionUsageFromMap(raw)
}

func visionTextFromOpenAI(raw map[string]any) string {
	choices, ok := raw["choices"].([]any)
	if !ok || len(choices) == 0 {
		return ""
	}
	choice, ok := choices[0].(map[string]any)
	if !ok {
		return ""
	}
	delta, _ := choice["delta"].(map[string]any)
	if delta == nil {
		if msg, ok := choice["message"].(map[string]any); ok {
			return stringifyVisionContent(msg["content"])
		}
		return ""
	}
	if content := stringifyVisionContent(delta["content"]); content != "" {
		return content
	}
	if reasoning, ok := delta["reasoning_content"].(string); ok {
		return reasoning
	}
	return ""
}

func visionTextFromAnthropic(raw map[string]any) string {
	typ, _ := raw["type"].(string)
	if typ != "content_block_delta" {
		return ""
	}
	delta, ok := raw["delta"].(map[string]any)
	if !ok {
		return ""
	}
	if text, ok := delta["text"].(string); ok {
		return text
	}
	return ""
}

func stringifyVisionContent(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var b strings.Builder
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := m["text"].(string); ok {
				b.WriteString(text)
			}
		}
		return b.String()
	default:
		return ""
	}
}

func visionUsageFromMap(raw map[string]any) *dto.Usage {
	u, ok := raw["usage"].(map[string]any)
	if !ok {
		return nil
	}
	out := &dto.Usage{UsageSemantic: "openai"}
	out.PromptTokens = intFromAny(u["prompt_tokens"])
	if out.PromptTokens == 0 {
		out.PromptTokens = intFromAny(u["input_tokens"])
	}
	out.CompletionTokens = intFromAny(u["completion_tokens"])
	if out.CompletionTokens == 0 {
		out.CompletionTokens = intFromAny(u["output_tokens"])
	}
	out.TotalTokens = intFromAny(u["total_tokens"])
	if out.PromptTokens == 0 && out.CompletionTokens == 0 && out.TotalTokens == 0 {
		return nil
	}
	return out
}

func mergeVisionUsage(dst, src *dto.Usage) {
	if dst == nil || src == nil {
		return
	}
	if src.PromptTokens > 0 {
		dst.PromptTokens = src.PromptTokens
	}
	if src.CompletionTokens > 0 {
		dst.CompletionTokens = src.CompletionTokens
	}
	if src.TotalTokens > 0 {
		dst.TotalTokens = src.TotalTokens
	}
}

func intFromAny(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	default:
		return 0
	}
}

func extractCaptionFromJSON(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content any `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := rootcommon.Unmarshal(body, &parsed); err != nil {
		return ""
	}
	if len(parsed.Choices) == 0 {
		return ""
	}
	return mediaContentToString(parsed.Choices[0].Message.Content)
}
