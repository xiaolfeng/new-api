package bamboo

import (
	"errors"
	"strings"

	pkgErrors "github.com/bamboo-services/bamboo-messages/pkg/errors"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
)

// ErrUnsupportedProvider 表示该上游 ApiType 未被 bamboo 覆盖，
// 调用方应 fallback 到 new-api 原生三段式。
//
// 判定方式：errors.Is(err, ErrUnsupportedProvider)。
// *types.NewAPIError 已实现 Unwrap()（types/error.go:101-107），
// NewError(ErrUnsupportedProvider, ...) 会把它包进 Err 字段，故 errors.Is 链可达。
var ErrUnsupportedProvider = errors.New("bamboo: unsupported provider for this api type")

// translateCodecError 把 bamboo BambooError 翻译为 new-api 错误。
//
// SDK v0.8.9 起统一使用 pkgErrors.BambooError（Category + Message + StatusCode），
// 旧的 bamboocodec.CodecError / ErrorType 枚举已移除。
// 错误分类通过 StatusCode 映射（与 codec/anthropic/error.go 的映射逻辑对齐）：
//
//	401/403 → ErrorCodeAccessDenied（认证失败）
//	429     → ErrorCodeBadResponse（限流，new-api 无专用码）
//	4xx     → ErrorCodeInvalidRequest（请求格式错误）
//	5xx/0   → ErrorCodeConvertRequestFailed（内部/上游错误）
func translateCodecError(err error) *types.NewAPIError {
	if err == nil {
		return nil
	}
	var be *pkgErrors.BambooError
	if !errors.As(err, &be) {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed)
	}
	switch {
	case be.StatusCode == 401 || be.StatusCode == 403:
		return types.NewError(be, types.ErrorCodeAccessDenied)
	case be.StatusCode == 429:
		return types.NewError(be, types.ErrorCodeBadResponse)
	case be.StatusCode >= 400 && be.StatusCode < 500:
		return types.NewError(be, types.ErrorCodeInvalidRequest)
	default:
		return types.NewError(be, types.ErrorCodeConvertRequestFailed)
	}
}

// translateSDKError 把 SDK Provider.Chat/Complete 返回的 error 翻译为 new-api 错误。
//
// 与 translateCodecError 不同，本函数处理的 error 来源是 SDK 内部 HTTP 调用链：
//   - 参数覆盖拦截器失败（ApplyParamOverrideWithRelayInfo 返回 error，
//     含 return_error 操作、无效 JSON path、条件断言失败等）
//   - 上游 HTTP 请求失败（网络错误、4xx/5xx 等）
//
// 区分逻辑：先用 relaycommon.AsParamOverrideReturnError 断言是否为 param override
// 的 return_error 操作（用户主动配置的拦截），若是则用 relaycommon.NewAPIErrorFromParamOverride
// 走与原生链路完全一致的转换路径；否则用通用 ErrorCodeDoRequestFailed 兜底。
// 逻辑与 relay/param_override_error.go 的 newAPIErrorFromParamOverride 等价，
// 这里内联是为了避免跨包循环依赖（relay 包不可被 relay/bamboo 反向 import）。
//
// 调用位置：bridge.go 的 doStreamRelay/doCompleteRelay 中 client.Chat/Complete 失败时。
func translateSDKError(err error) *types.NewAPIError {
	if err == nil {
		return nil
	}
	// 优先识别 param override 错误（与原生 claude_handler.go 走同一转换路径）
	if fixedErr, ok := relaycommon.AsParamOverrideReturnError(err); ok {
		return relaycommon.NewAPIErrorFromParamOverride(fixedErr)
	}
	// 参数覆盖失败但非 return_error（如无效 JSON path）也归为 ParamOverrideInvalid
	if isParamOverrideFailure(err) {
		return types.NewError(err, types.ErrorCodeChannelParamOverrideInvalid, types.ErrOptionWithSkipRetry())
	}
	// 通用兜底（保持与改造前一致的错误码）
	return types.NewError(err, types.ErrorCodeDoRequestFailed)
}

// isParamOverrideFailure 启发式判断 error 是否来自 param override 拦截器。
//
// 拦截器在 interceptorTransport.RoundTrip 中通过 fmt.Errorf("interceptorTransport(%s): ...", ...)
// 包装 error，所以可以检查 error 链中是否含 "interceptorTransport" 字样作为兜底识别。
// 这是 best-effort，不会误判上游 HTTP 错误（上游错误来自 http.Client.Do 而非 transport 内部）。
func isParamOverrideFailure(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "interceptorTransport")
}
