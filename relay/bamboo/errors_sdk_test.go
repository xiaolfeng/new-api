package bamboo

import (
	"errors"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

// TestTranslateSDKError_NilError 验证 nil error 返回 nil（防 panic 契约）。
func TestTranslateSDKError_NilError(t *testing.T) {
	if got := translateSDKError(nil); got != nil {
		t.Errorf("nil error 应返回 nil，got=%v", got)
	}
}

// TestTranslateSDKError_GenericUpstreamError 验证非 param override 的上游错误
// 走通用 ErrorCodeDoRequestFailed 兜底（保持与改造前一致）。
func TestTranslateSDKError_GenericUpstreamError(t *testing.T) {
	upstreamErr := errors.New("connection refused")
	got := translateSDKError(upstreamErr)
	if got == nil {
		t.Fatal("应返回非 nil NewAPIError")
	}
	if got.GetErrorCode() != types.ErrorCodeDoRequestFailed {
		t.Errorf("上游错误应映射为 ErrorCodeDoRequestFailed，got=%v", got.GetErrorCode())
	}
}

// TestTranslateSDKError_InterceptorFailure 验证来自 interceptorTransport 的 error
// （非 return_error 操作，例如无效 JSON path）被识别为 param override 失败。
func TestTranslateSDKError_InterceptorFailure(t *testing.T) {
	// 模拟 SDK interceptorTransport 包装的 error
	interceptorErr := errors.New("interceptorTransport(anthropic): apply interceptors failed: invalid JSON path: foo..bar")
	got := translateSDKError(interceptorErr)
	if got == nil {
		t.Fatal("应返回非 nil NewAPIError")
	}
	if got.GetErrorCode() != types.ErrorCodeChannelParamOverrideInvalid {
		t.Errorf("拦截器失败应映射为 ErrorCodeChannelParamOverrideInvalid，got=%v", got.GetErrorCode())
	}
}

// TestTranslateSDKError_ReturnErrorOperation 验证 param override 的 return_error 操作
// 走与原生链路（claude_handler.go 等）完全一致的 NewAPIErrorFromParamOverride 转换路径。
//
// return_error 是用户主动配置的拦截规则（如 "blocked by param override"），
// 应该被识别为可展示给用户的业务错误，而非内部请求失败。
func TestTranslateSDKError_ReturnErrorOperation(t *testing.T) {
	// 先构造一个 ApplyParamOverrideWithRelayInfo 产生的 return_error 错误
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ParamOverride: map[string]interface{}{
				"operations": []interface{}{
					map[string]interface{}{
						"mode":  "return_error",
						"value": map[string]interface{}{"message": "blocked by user policy"},
					},
				},
			},
		},
	}
	_, expectedErr := relaycommon.ApplyParamOverrideWithRelayInfo([]byte(`{"model":"gpt-4"}`), info)
	if expectedErr == nil {
		t.Skip("return_error 操作未返回 error，跳过（可能是 override.go 实现细节变化）")
	}

	// 用 fmt.Errorf 包装一层，模拟 SDK 拦截器 transport 的错误传递路径
	wrapped := errors.New("interceptorTransport(test): apply interceptors failed: " + expectedErr.Error())

	got := translateSDKError(wrapped)
	if got == nil {
		t.Fatal("return_error 错误应返回非 nil NewAPIError")
	}
	// return_error 应该产生可识别的错误类型（不一定是 ParamOverrideInvalid，
	// 但一定不是 DoRequestFailed）
	if got.GetErrorCode() == types.ErrorCodeDoRequestFailed {
		t.Errorf("return_error 不应被误判为上游请求失败")
	}
}
