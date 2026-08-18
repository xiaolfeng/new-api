package hosttool

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

type fetchRequest struct {
	URL          string
	Format       string
	Timeout      time.Duration
	Prompt       string
	Pattern      string
	StartLine    int
	MaxMatches   int
	ContextLines int
}

func runFetch(ctx context.Context, st *model_setting.BambooSettings, in fetchRequest) ExecResult {
	result := ExecResult{Kind: "fetch", URL: in.URL, Prompt: in.Prompt}
	if !strings.HasPrefix(in.URL, "http://") && !strings.HasPrefix(in.URL, "https://") {
		result.ErrorCode = ErrSSRFBlocked
		return result
	}
	if err := service.ValidateSSRFProtectedFetchURL(in.URL); err != nil {
		result.ErrorCode = ErrSSRFBlocked
		return result
	}

	timeout := in.Timeout
	if timeout <= 0 {
		timeout = time.Duration(st.ClampTimeout()) * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body, contentType, status, err := doFetch(ctx, in.URL, chromeUA)
	if err != nil {
		result.ErrorCode = classifyFetchErr(ctx, err, status)
		return result
	}
	if status == http.StatusForbidden {
		retryBody, retryType, retryStatus, retryErr := doFetch(ctx, in.URL, "new-api-hosttool")
		if retryErr == nil {
			body, contentType, status = retryBody, retryType, retryStatus
		}
	}
	if status >= 400 {
		result.ErrorCode = ErrUpstream4xx
		return result
	}
	mime := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if strings.HasPrefix(mime, "image/") {
		result.ErrorCode = ErrInvalidInput
		return result
	}

	format := strings.ToLower(in.Format)
	if format == "" {
		format = "markdown"
	}
	switch {
	case format == "html":
		result.Body = body
	case strings.Contains(mime, "html"):
		result.Body = composeFetchedHTML(body, format)
	default:
		result.Body = body
	}
	result.Body = applyFetchView(result.Body, in.StartLine, in.Pattern, in.MaxMatches, in.ContextLines)
	result.Pattern = in.Pattern
	result.OK = true
	return result
}

const chromeUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36"

func doFetch(ctx context.Context, rawURL, ua string) (string, string, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", "", 0, err
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain,text/markdown,*/*;q=0.1")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := service.GetSSRFProtectedHTTPClient().Do(req)
	if err != nil {
		return "", "", 0, err
	}
	defer resp.Body.Close()

	limit := model_setting.GetBambooSettings().ClampMaxFetchBytes()
	data, err := io.ReadAll(io.LimitReader(resp.Body, int64(limit)+1))
	if err != nil {
		return "", "", resp.StatusCode, err
	}
	if len(data) > limit {
		data = data[:limit]
	}
	return string(data), resp.Header.Get("Content-Type"), resp.StatusCode, nil
}

func classifyFetchErr(ctx context.Context, err error, status int) string {
	if ctx.Err() == context.Canceled || errors.Is(err, context.Canceled) {
		return ErrCanceled
	}
	if ctx.Err() == context.DeadlineExceeded || errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "blocked") {
		return ErrSSRFBlocked
	}
	if status >= 400 && status < 500 {
		return ErrUpstream4xx
	}
	return ErrUpstream4xx
}
