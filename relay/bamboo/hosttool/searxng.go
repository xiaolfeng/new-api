package hosttool

import (
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

type searxngResponse struct {
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
	} `json:"results"`
}

func searchSearXNG(ctx context.Context, base, query string, num int, timeout time.Duration) ([]SearchHit, error) {
	base = strings.TrimRight(base, "/")
	if base == "" {
		return nil, fmt.Errorf("searxng_base_url empty")
	}
	endpoint := base + "/search?q=" + url.QueryEscape(query) + "&format=json"
	if err := validateOperatorURL(endpoint); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "new-api-hosttool")
	resp, err := service.GetHttpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, statusError{code: resp.StatusCode, msg: fmt.Sprintf("searxng status %d", resp.StatusCode)}
	}
	var parsed searxngResponse
	if err := common.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	hits := make([]SearchHit, 0, num)
	for _, r := range parsed.Results {
		if len(hits) >= num {
			break
		}
		hits = append(hits, SearchHit{Title: r.Title, URL: r.URL, Snippet: r.Content})
	}
	return hits, nil
}

type statusError struct {
	code int
	msg  string
}

func (e statusError) Error() string { return e.msg }
