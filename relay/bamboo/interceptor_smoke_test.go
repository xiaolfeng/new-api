package bamboo

import (
	"context"
	"testing"

	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
	"github.com/bamboo-services/bamboo-messages/provider"
	"github.com/bamboo-services/bamboo-messages/provider/anthropic"
)

// TestSDKInterceptorAvailable 验证 SDK v0.7.0 的 RequestInterceptor 类型
// 与 WithInterceptor option 在 new-api 编译路径中可用。
//
// 这是从 SDK Task 1-6 过渡到 new-api Task 9 的 smoke test：
// 不测真实行为，只测类型符号可被识别（编译期检查）。
func TestSDKInterceptorAvailable(t *testing.T) {
	// 验证 RequestInterceptor 函数签名契约
	var _ provider.RequestInterceptor = func(ctx context.Context, body []byte) ([]byte, error) {
		return body, nil
	}

	// 验证 ApplyInterceptors helper 可调用
	_, _ = provider.ApplyInterceptors(context.Background(), []byte("{}"), nil)

	// 验证 WithInterceptor option 可用于 anthropic Provider 构造
	_ = anthropic.NewProviderWithOptions(
		anthropic.WithAPIKey("test"),
		anthropic.WithInterceptor(provider.RequestInterceptor(
			func(ctx context.Context, body []byte) ([]byte, error) { return body, nil },
		)),
	)

	// 验证 NewInterceptorHTTPClient 工厂存在
	if cli := provider.NewInterceptorHTTPClient(nil, nil); cli != nil {
		t.Error("无拦截器时应返回 nil")
	}

	// 引用 codec/sdk 主类型，确认 v0.7.0 兼容 v0.6.x 已有 API
	_ = bamboocodec.FormatOpenAI
	_ = bamboosdk.NewClient
}
