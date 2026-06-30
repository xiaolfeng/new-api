package bamboo

import (
	"errors"
	"strings"

	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

// ErrUnsupportedProvider 表示该上游 ApiType 未被 bamboo 覆盖，
// 调用方应 fallback 到 new-api 原生三段式。
//
// 判定方式：errors.Is(err, ErrUnsupportedProvider)。
// *types.NewAPIError 已实现 Unwrap()（types/error.go:101-107），
// NewError(ErrUnsupportedProvider, ...) 会把它包进 Err 字段，故 errors.Is 链可达。
var ErrUnsupportedProvider = errors.New("bamboo: unsupported provider for this api type")

// translateCodecError 把 bamboo CodecError 翻译为 new-api 错误。
//
// 入参为 error 接口（ParseRequest/Serialize 返回裸 error），
// 内部用 errors.As 做 *CodecError 类型断言；非 CodecError 走默认分支。
//
// CodecError.Type 实际枚举（bamboo/codec/errors.go:9-22）：
//   ErrInvalidRequest / ErrProviderError / ErrAuthError / ErrRateLimit / ErrInternal
//
// ErrorCode 映射（new-api types/error.go 真实存在的常量，复审已核对全 31 个）：
//   new-api 无 auth/rateLimit/upstream 专用码，复用语义最近的现有常量。
func translateCodecError(err error) *types.NewAPIError {
	if err == nil {
		return nil
	}
	var ce *bamboocodec.CodecError
	if !errors.As(err, &ce) {
		// 非 CodecError（如 provider 内部错误），用通用转换失败码
		return types.NewError(err, types.ErrorCodeConvertRequestFailed)
	}
	switch ce.Type {
	case bamboocodec.ErrInvalidRequest:
		return types.NewError(ce, types.ErrorCodeInvalidRequest)
	case bamboocodec.ErrAuthError:
		return types.NewError(ce, types.ErrorCodeAccessDenied)
	case bamboocodec.ErrRateLimit:
		return types.NewError(ce, types.ErrorCodeBadResponse)
	case bamboocodec.ErrProviderError:
		return types.NewError(ce, types.ErrorCodeBadResponseStatusCode)
	default: // ErrInternal 等
		return types.NewError(ce, types.ErrorCodeConvertRequestFailed)
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
