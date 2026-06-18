package bamboo

import (
	"errors"

	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
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
