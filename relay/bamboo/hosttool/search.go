package hosttool

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

type searchRequest struct {
	Query                string
	NumResults           int
	Livecrawl            string
	Type                 string
	ContextMaxCharacters int
	AllowedDomains       []string
	BlockedDomains       []string
}

func runSearch(ctx context.Context, info *relaycommon.RelayInfo, st *model_setting.BambooSettings, in searchRequest) (ExecResult, error) {
	result := ExecResult{Kind: "search", Query: in.Query}
	backend := st.ResolvedSearchBackend()
	if backend == "off" {
		result.ErrorCode = ErrBackendDisabled
		return result, nil
	}

	timeout := time.Duration(st.ClampTimeout()) * time.Millisecond
	chain := []string{backend}
	if st.AllowThirdPartySearchEgress {
		for _, fb := range st.SearchFallback {
			fb = strings.ToLower(strings.TrimSpace(fb))
			if fb == "" || fb == backend {
				continue
			}
			if fb == "exa" || fb == "parallel" || fb == "searxng" {
				chain = append(chain, fb)
			}
		}
	}

	var lastErr error
	for i, name := range chain {
		text, hits, err := invokeSearchBackend(ctx, info, st, name, in, timeout)
		if err == nil {
			result.OK = true
			result.Backend = name
			result.OpaqueText = text
			result.Hits = filterHits(hits, in.AllowedDomains, in.BlockedDomains)
			return result, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
		// 4xx 不 failover；传输失败 / 5xx / 超时才切。
		var se statusError
		if errors.As(err, &se) && se.code >= 400 && se.code < 500 {
			result.ErrorCode = ErrUpstream4xx
			result.Backend = name
			return result, nil
		}
		if i == len(chain)-1 {
			break
		}
	}
	if lastErr != nil && errors.Is(lastErr, context.DeadlineExceeded) {
		result.ErrorCode = ErrTimeout
		return result, nil
	}
	if lastErr != nil && errors.Is(lastErr, context.Canceled) {
		result.ErrorCode = ErrCanceled
		return result, nil
	}
	var se statusError
	if errors.As(lastErr, &se) && se.code >= 400 && se.code < 500 {
		result.ErrorCode = ErrUpstream4xx
		return result, nil
	}
	result.ErrorCode = ErrUpstream4xx
	if lastErr != nil {
		// 5xx / 传输失败：对客户端仍用稳定码。
		result.ErrorCode = ErrUpstream4xx
	}
	return result, nil
}

func invokeSearchBackend(ctx context.Context, info *relaycommon.RelayInfo, st *model_setting.BambooSettings, name string, in searchRequest, timeout time.Duration) (string, []SearchHit, error) {
	switch name {
	case "exa":
		args := map[string]any{
			"query":      in.Query,
			"type":       firstNonEmpty(in.Type, "auto"),
			"numResults": in.NumResults,
			"livecrawl":  firstNonEmpty(in.Livecrawl, "fallback"),
		}
		if in.ContextMaxCharacters > 0 {
			args["contextMaxCharacters"] = in.ContextMaxCharacters
		}
		endpoint := st.ResolvedExaMCPURL()
		if st.ExaAPIKey != "" {
			sep := "?"
			if strings.Contains(endpoint, "?") {
				sep = "&"
			}
			endpoint = endpoint + sep + "exaApiKey=" + url.QueryEscape(st.ExaAPIKey)
		}
		text, status, err := callMCP(ctx, endpoint, "web_search_exa", args, nil, timeout)
		if err != nil {
			if status >= 400 {
				return "", nil, statusError{code: status, msg: err.Error()}
			}
			return "", nil, err
		}
		return text, limitHits(parseSearchHits(text), in.NumResults), nil
	case "parallel":
		sessionID := parallelSessionID(info)
		modelName := info.OriginModelName
		if len(modelName) > 100 {
			modelName = modelName[:100]
		}
		args := map[string]any{
			"objective":      in.Query,
			"search_queries": []string{in.Query},
			"session_id":     sessionID,
			"model_name":     modelName,
		}
		headers := map[string]string{}
		if st.ParallelAPIKey != "" {
			headers["Authorization"] = "Bearer " + st.ParallelAPIKey
		}
		text, status, err := callMCP(ctx, st.ResolvedParallelMCPURL(), "web_search", args, headers, timeout)
		if err != nil {
			if status >= 400 {
				return "", nil, statusError{code: status, msg: err.Error()}
			}
			return "", nil, err
		}
		return text, limitHits(parseSearchHits(text), in.NumResults), nil
	case "searxng":
		hits, err := searchSearXNG(ctx, st.SearxngBaseURL, in.Query, in.NumResults, timeout)
		return "", hits, err
	default:
		return "", nil, fmt.Errorf("unknown backend %s", name)
	}
}

func parallelSessionID(info *relaycommon.RelayInfo) string {
	if info == nil {
		return "hosttool"
	}
	if info.TokenKey != "" {
		sum := sha256.Sum256([]byte(info.TokenKey))
		return hex.EncodeToString(sum[:8])
	}
	if info.RequestId != "" {
		return info.RequestId
	}
	return "hosttool"
}

func filterHits(hits []SearchHit, allowed, blocked []string) []SearchHit {
	if len(hits) == 0 {
		return hits
	}
	allow := toLowerSet(allowed)
	block := toLowerSet(blocked)
	if len(allow) == 0 && len(block) == 0 {
		return hits
	}
	out := make([]SearchHit, 0, len(hits))
	for _, h := range hits {
		host := hostOf(h.URL)
		if len(block) > 0 && block[host] {
			continue
		}
		if len(allow) > 0 && !allow[host] {
			continue
		}
		out = append(out, h)
	}
	return out
}

func toLowerSet(items []string) map[string]bool {
	if len(items) == 0 {
		return nil
	}
	out := make(map[string]bool, len(items))
	for _, item := range items {
		item = strings.ToLower(strings.TrimSpace(item))
		if item != "" {
			out[item] = true
		}
	}
	return out
}

func hostOf(raw string) string {
	raw = strings.ToLower(raw)
	raw = strings.TrimPrefix(raw, "https://")
	raw = strings.TrimPrefix(raw, "http://")
	if i := strings.IndexByte(raw, '/'); i >= 0 {
		raw = raw[:i]
	}
	return raw
}
