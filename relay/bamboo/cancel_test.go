package bamboo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	// 空白 import 注册 OpenAI codec，供 entryCodec.NewSerializer 使用。
	_ "github.com/bamboo-services/bamboo-messages/bamboo/codec/openai"
	pkgErrors "github.com/bamboo-services/bamboo-messages/pkg/errors"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// mockBridgeClient 可编程的 BambooClient 模拟实现。
type mockBridgeClient struct {
	chatFn     func(ctx context.Context) (<-chan bamboosdk.StreamEvent, error)
	completeFn func(ctx context.Context) (*bamboosdk.Response, error)
}

func (m *mockBridgeClient) Chat(_ context.Context, _ []bamboosdk.BambooMessage, _ string, _ *bamboosdk.RequestConfig) (<-chan bamboosdk.StreamEvent, error) {
	return m.chatFn(context.Background())
}

func (m *mockBridgeClient) Complete(_ context.Context, _ []bamboosdk.BambooMessage, _ string, _ *bamboosdk.RequestConfig) (*bamboosdk.Response, error) {
	return m.completeFn(context.Background())
}

func newBridgeTestGinContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	return c
}

func newBridgeTestRelay() (*relaycommon.RelayInfo, bamboocodec.Codec, *bamboocodec.RelayRequest) {
	info := &relaycommon.RelayInfo{
		OriginModelName: "test-model",
		IsStream:        true,
	}
	entryCodec, err := bamboocodec.Get(bamboocodec.FormatOpenAI)
	if err != nil {
		panic("openai codec not registered: " + err.Error())
	}
	req := &bamboocodec.RelayRequest{
		Config: &bamboosdk.RequestConfig{Model: "test-model"},
	}
	return info, entryCodec, req
}

// TestDoStreamRelay_SyncCancel 验证首个事件前客户端取消时，
// doStreamRelay 正常返回且 StreamStatus 标记为 client_gone（不产生错误）。
func TestDoStreamRelay_SyncCancel(t *testing.T) {
	c := newBridgeTestGinContext()
	info, entryCodec, req := newBridgeTestRelay()

	cancelErr := pkgErrors.NewBambooErrorWithCause("SDK", "对话已取消: context canceled", 0, context.Canceled)
	client := &mockBridgeClient{
		chatFn: func(_ context.Context) (<-chan bamboosdk.StreamEvent, error) {
			return nil, cancelErr
		},
	}

	usage, relayErr := doStreamRelay(c, info, client, entryCodec, bamboocodec.FormatOpenAI, req)
	if relayErr != nil {
		t.Fatalf("客户端取消不应返回错误，got=%v", relayErr)
	}
	if usage != nil {
		t.Errorf("同步取消无内容，usage 应为 nil，got=%v", usage)
	}
	if info.StreamStatus == nil {
		t.Fatal("应初始化 StreamStatus")
	}
	if info.StreamStatus.EndReason != relaycommon.StreamEndReasonClientGone {
		t.Errorf("EndReason 应为 client_gone，got=%q", info.StreamStatus.EndReason)
	}
}

// TestDoStreamRelay_StreamCancelEvent 验证流式过程中客户端取消（EventError 事件）
// 时正常结束并标记 client_gone。
func TestDoStreamRelay_StreamCancelEvent(t *testing.T) {
	c := newBridgeTestGinContext()
	info, entryCodec, req := newBridgeTestRelay()

	cancelErr := pkgErrors.NewBambooErrorWithCause("SDK", "对话已取消: context canceled", 0, context.Canceled)
	client := &mockBridgeClient{
		chatFn: func(_ context.Context) (<-chan bamboosdk.StreamEvent, error) {
			ch := make(chan bamboosdk.StreamEvent, 1)
			ch <- bamboosdk.StreamEvent{Type: bamboosdk.EventError, Error: cancelErr}
			close(ch)
			return ch, nil
		},
	}

	usage, relayErr := doStreamRelay(c, info, client, entryCodec, bamboocodec.FormatOpenAI, req)
	if relayErr != nil {
		t.Fatalf("流式取消不应返回错误，got=%v", relayErr)
	}
	if usage == nil {
		t.Fatal("流式取消应返回已结算 usage")
	}
	if info.StreamStatus == nil {
		t.Fatal("应初始化 StreamStatus")
	}
	if info.StreamStatus.EndReason != relaycommon.StreamEndReasonClientGone {
		t.Errorf("EndReason 应为 client_gone，got=%q", info.StreamStatus.EndReason)
	}
}

// TestDoStreamRelay_NormalEndMarksDone 验证流自然结束时 StreamStatus 标记为 done。
func TestDoStreamRelay_NormalEndMarksDone(t *testing.T) {
	c := newBridgeTestGinContext()
	info, entryCodec, req := newBridgeTestRelay()

	client := &mockBridgeClient{
		chatFn: func(_ context.Context) (<-chan bamboosdk.StreamEvent, error) {
			ch := make(chan bamboosdk.StreamEvent)
			close(ch)
			return ch, nil
		},
	}

	usage, relayErr := doStreamRelay(c, info, client, entryCodec, bamboocodec.FormatOpenAI, req)
	if relayErr != nil {
		t.Fatalf("正常结束不应返回错误，got=%v", relayErr)
	}
	if usage == nil {
		t.Fatal("正常结束应返回 usage")
	}
	if info.StreamStatus == nil {
		t.Fatal("应初始化 StreamStatus")
	}
	if info.StreamStatus.EndReason != relaycommon.StreamEndReasonDone {
		t.Errorf("EndReason 应为 done，got=%q", info.StreamStatus.EndReason)
	}
}

// TestDoCompleteRelay_Cancel 验证非流式客户端取消时正常返回（不记错误日志）。
func TestDoCompleteRelay_Cancel(t *testing.T) {
	c := newBridgeTestGinContext()
	info, entryCodec, req := newBridgeTestRelay()

	cancelErr := pkgErrors.NewBambooErrorWithCause("SDK", "对话已取消: context canceled", 0, context.Canceled)
	client := &mockBridgeClient{
		completeFn: func(_ context.Context) (*bamboosdk.Response, error) {
			return nil, cancelErr
		},
	}

	usage, relayErr := doCompleteRelay(c, info, client, entryCodec, bamboocodec.FormatOpenAI, req)
	if relayErr != nil {
		t.Fatalf("客户端取消不应返回错误，got=%v", relayErr)
	}
	if usage != nil {
		t.Errorf("取消无结果，usage 应为 nil，got=%v", usage)
	}
}
