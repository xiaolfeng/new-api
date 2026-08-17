package hosttool

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
)

type mcpCall struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	} `json:"params"`
}

type mcpEnvelope struct {
	Result *struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"result"`
}

func callMCP(ctx context.Context, endpoint, tool string, args map[string]any, headers map[string]string, timeout time.Duration) (string, int, error) {
	if err := validateOperatorURL(endpoint); err != nil {
		return "", 0, err
	}
	reqBody := mcpCall{JSONRPC: "2.0", ID: 1, Method: "tools/call"}
	reqBody.Params.Name = tool
	reqBody.Params.Arguments = args
	payload, err := common.Marshal(reqBody)
	if err != nil {
		return "", 0, err
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "new-api-hosttool")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := service.GetHttpClient().Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return "", resp.StatusCode, err
	}
	if resp.StatusCode >= 400 {
		return "", resp.StatusCode, fmt.Errorf("mcp status %d", resp.StatusCode)
	}
	text, err := parseMCPResponse(body)
	return text, resp.StatusCode, err
}

func parseMCPResponse(body []byte) (string, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return "", fmt.Errorf("empty mcp body")
	}
	if text, ok := decodeMCPJSON(trimmed); ok {
		return text, nil
	}
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if text, ok := decodeMCPJSON([]byte(payload)); ok {
			return text, nil
		}
	}
	return "", fmt.Errorf("no mcp text")
}

func decodeMCPJSON(raw []byte) (string, bool) {
	var env mcpEnvelope
	if err := common.Unmarshal(raw, &env); err != nil || env.Result == nil {
		return "", false
	}
	for _, item := range env.Result.Content {
		if strings.TrimSpace(item.Text) != "" {
			return item.Text, true
		}
	}
	return "", false
}

func validateOperatorURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("operator url must be http(s)")
	}
	return nil
}

func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}
