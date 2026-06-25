package bamboo

import (
	"context"
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/relay/common"
)

// TestBuildParamOverrideInterceptor_NilCases 验证 buildParamOverrideInterceptor
// 在 ParamOverride 为空时返回 nil（保证零开销 fast path）。
func TestBuildParamOverrideInterceptor_NilCases(t *testing.T) {
	t.Run("info 为 nil", func(t *testing.T) {
		got := buildParamOverrideInterceptor(nil)
		if got != nil {
			t.Errorf("info=nil 时应返回 nil，got=%v", got)
		}
	})

	t.Run("ChannelMeta 为 nil", func(t *testing.T) {
		info := &common.RelayInfo{}
		got := buildParamOverrideInterceptor(info)
		if got != nil {
			t.Errorf("ChannelMeta=nil 时应返回 nil，got=%v", got)
		}
	})

	t.Run("ParamOverride 为空 map", func(t *testing.T) {
		info := &common.RelayInfo{
			ChannelMeta: &common.ChannelMeta{
				ParamOverride: map[string]interface{}{},
			},
		}
		got := buildParamOverrideInterceptor(info)
		if got != nil {
			t.Errorf("空 ParamOverride 时应返回 nil，got=%v", got)
		}
	})
}

// TestBuildParamOverrideInterceptor_Registered 验证 ParamOverride 非空时
// 返回非 nil 拦截器，且该拦截器内部确实调用 ApplyParamOverrideWithRelayInfo。
func TestBuildParamOverrideInterceptor_Registered(t *testing.T) {
	info := &common.RelayInfo{
		ChannelMeta: &common.ChannelMeta{
			ParamOverride: map[string]interface{}{
				"temperature": 0.9,
			},
		},
	}

	interceptor := buildParamOverrideInterceptor(info)
	if interceptor == nil {
		t.Fatal("ParamOverride 非空时应返回非 nil 拦截器")
	}

	// 实际调用拦截器，验证它不 panic 且返回非 nil body
	body := []byte(`{"model":"gpt-4","temperature":0.5}`)
	got, err := interceptor(context.Background(), body)
	if err != nil {
		t.Fatalf("拦截器调用失败: %v", err)
	}
	if got == nil {
		t.Error("拦截器返回 nil body")
	}
	// 注：具体覆盖行为由 relay/common/override_test.go 覆盖，
	// 这里只验证集成路径通畅（不 panic + 不返回 error）
}

// TestBuildParamOverrideInterceptor_PropagatesError 验证当 ApplyParamOverrideWithRelayInfo
// 返回 error 时（如无效 param override 配置），拦截器原样向上冒泡。
//
// 此契约保证后续 Task 10 的 bridge.go 错误适配能拿到原始 error。
func TestBuildParamOverrideInterceptor_PropagatesError(t *testing.T) {
	// 构造一个会导致 ApplyParamOverrideWithRelayInfo 失败的场景：
	// 使用 return_error 操作模式，它会主动返回 error
	info := &common.RelayInfo{
		ChannelMeta: &common.ChannelMeta{
			ParamOverride: map[string]interface{}{
				"operations": []interface{}{
					map[string]interface{}{
						"mode":  "return_error",
						"value": map[string]interface{}{"message": "blocked by param override"},
					},
				},
			},
		},
	}

	interceptor := buildParamOverrideInterceptor(info)
	if interceptor == nil {
		t.Fatal("应返回非 nil 拦截器")
	}

	_, err := interceptor(context.Background(), []byte(`{"model":"gpt-4"}`))
	if err == nil {
		t.Error("return_error 操作应导致拦截器返回 error")
	}

	// 验证 error 是非 sentinel 的真实 error（防止误判）
	if errors.Is(err, context.Canceled) {
		t.Errorf("error 不应是 context.Canceled，got=%v", err)
	}
}
