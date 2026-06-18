package bamboo

import (
	"context"
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	// 空白 import 触发各 codec 子包的 init() 注册。
	// codec.Get 依赖包级变量（registry.go:9-21）由子包 init() 赋值，
	// 不显式 import 子包会导致 codec.Get 返回 nil。
	_ "github.com/bamboo-services/bamboo-messages/bamboo/codec/anthropic"
	_ "github.com/bamboo-services/bamboo-messages/bamboo/codec/gemini"
	_ "github.com/bamboo-services/bamboo-messages/bamboo/codec/openai"
	_ "github.com/bamboo-services/bamboo-messages/bamboo/codec/responses"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
)

// errStreamErrorNoDetail 用于 event.Error 为 nil 但事件类型为 EventError 的兜底。
var errStreamErrorNoDetail = errors.New("bamboo stream error event without detail")

// ChatRelay 对话中继统一内核。
//
// 替代 TextHelper/ClaudeHelper/GeminiHelper/ResponsesHelper 内部的
// Convert→DoRequest→DoResponse 三段式，用 bamboo 中间表示做协议归一化：
//
//   - 入口侧：codec.ParseRequest 把入口协议请求体解析为协议无关的 RelayRequest
//     （替代 adaptor.Convert*Request 的入口→上游格式转换）
//   - 上游侧：provider.Chat/Complete 发起上游请求（替代 adaptor.DoRequest）
//   - 出口侧：codec.NewSerializer/SerializeResponse 把上游事件转回入口协议格式
//     （替代 adaptor.DoResponse 的上游→入口格式转换 + reply 生成）
//
// 调用方只需传 new-api 侧的 types.RelayFormat，格式映射在 bridge 内部完成。
// info.ApiType（经 ChannelMeta 嵌入）决定上游用哪个 bamboo provider。
//
// 返回 (usage, nil) 成功；(nil, err) 失败。
// 当 errors.Is(err, ErrUnsupportedProvider) 时，调用方应 fallback 原生链路。
func ChatRelay(c *gin.Context, info *relaycommon.RelayInfo,
	entryFormat types.RelayFormat, requestBody []byte) (*dto.Usage, *types.NewAPIError) {

	// ① 入口格式映射：RelayFormat → codec FormatType
	codecFmt, ok := relayFormatToCodec(entryFormat)
	if !ok {
		// 非对话格式（Audio/Image/Task/Realtime/Rerank/Embedding）不应进入 bridge
		return nil, types.NewError(ErrUnsupportedProvider, types.ErrorCodeInvalidApiType)
	}

	entryCodec, gerr := bamboocodec.Get(codecFmt)
	if gerr != nil || entryCodec == nil {
		return nil, types.NewError(fmt.Errorf("bamboo codec not registered: %s", codecFmt), types.ErrorCodeInvalidRequest)
	}
	relayReq, parseErr := entryCodec.ParseRequest(requestBody)
	if parseErr != nil {
		return nil, translateCodecError(parseErr) // 内部 errors.As 断言 *CodecError
	}

	// ② 上游侧：根据 ApiType 构造 bamboo provider
	p, provErr := newProvider(info)
	if provErr != nil {
		return nil, provErr // 含 ErrUnsupportedProvider，调用方判 errors.Is 做 fallback
	}
	client := bamboosdk.NewClient(p)

	// ③ 出口侧：按入口 codec 序列化响应
	if relayReq.IsStream {
		return doStreamRelay(c, info, client, entryCodec, relayReq)
	}
	return doCompleteRelay(c, client, entryCodec, relayReq)
}

// doStreamRelay 消费 bamboo StreamEvent，按入口 codec 序列化为出口 SSE。
func doStreamRelay(c *gin.Context, info *relaycommon.RelayInfo, client bamboosdk.BambooClient,
	entryCodec bamboocodec.Codec, req *bamboocodec.RelayRequest) (*dto.Usage, *types.NewAPIError) {

	// goroutine 泄漏防护：派生可取消 context。
	// 当客户端断开（c.Request.Context().Done()）或函数返回时取消，
	// 通知 bamboo provider 的 Chat channel 消费循环及时退出，避免泄漏。
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// 监听客户端断开，主动取消上游 context
	go func() {
		<-c.Request.Context().Done()
		cancel()
	}()

	eventCh, err := client.Chat(ctx, req.Messages, req.System, req.Config)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeDoRequestFailed)
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Flush()

	serializer := entryCodec.NewSerializer()
	var usage dto.Usage

	for event := range eventCh {
		if event.Type == bamboosdk.EventError {
			cause := errStreamErrorNoDetail
			if event.Error != nil {
				cause = event.Error // *BambooError 已实现 error 接口（bamboo/errors.go:46）
			}
			return nil, types.NewError(cause, types.ErrorCodeBadResponseBody)
		}

		// 累计 thinking delta 的 reasoning token
		accumulateReasoningFromEvent(info.OriginModelName, &usage, &event)

		data, serr := serializer.Serialize(event)
		if serr != nil {
			return nil, translateCodecError(serr)
		}
		if _, werr := c.Writer.Write(data); werr != nil {
			break // 客户端断开
		}
		c.Writer.Flush()

		// 从 message_delta 提取 usage
		if event.Type == bamboosdk.EventMessageDelta && event.Usage != nil {
			usage.PromptTokens = int(event.Usage.InputTokens)
			usage.CompletionTokens = int(event.Usage.OutputTokens)
		}
	}

	// flush 剩余缓冲（如 OpenAI codec 的 [DONE] 终止符）
	tail, _ := serializer.Flush()
	if len(tail) > 0 {
		c.Writer.Write(tail)
		c.Writer.Flush()
	}

	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	return &usage, nil
}

// accumulateReasoningFromEvent 从 StreamEvent 的 thinking delta 提取 reasoning token。
//
// bamboo 的 StreamEvent.Delta 是 any，content_block_delta 事件携带 *StreamDelta，
// 其中 Type==DeltaThinkingDelta 的增量代表思考过程。用 new-api 的 CountTextToken
// 精确计数（按 modelName 选 tiktoken/分词器），累计到 CompletionTokenDetails。
//
// 注意：thinking 文本分多次 delta 增量到达，这里逐段计数后求和，
// 与 tiktoken 对完整文本计数可能有微小差异（分段边界），可接受。
func accumulateReasoningFromEvent(modelName string, usage *dto.Usage, event *bamboosdk.StreamEvent) {
	if event.Type != bamboosdk.EventContentBlockDelta || event.Delta == nil {
		return
	}
	delta, ok := event.Delta.(*bamboosdk.StreamDelta)
	if !ok || delta == nil {
		return
	}
	if delta.Type == bamboosdk.DeltaThinkingDelta && delta.Thinking != "" {
		tokens := service.CountTextToken(delta.Thinking, modelName)
		accumulateReasoning(usage, tokens)
	}
}

// doCompleteRelay 非流式中继。
func doCompleteRelay(c *gin.Context, client bamboosdk.BambooClient,
	entryCodec bamboocodec.Codec, req *bamboocodec.RelayRequest) (*dto.Usage, *types.NewAPIError) {

	resp, err := client.Complete(c.Request.Context(), req.Messages, req.System, req.Config)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeDoRequestFailed)
	}
	body, serr := entryCodec.SerializeResponse(resp)
	if serr != nil {
		return nil, translateCodecError(serr)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Write(body)

	return &dto.Usage{
		PromptTokens:     int(resp.Usage.InputTokens),
		CompletionTokens: int(resp.Usage.OutputTokens),
		TotalTokens:      int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
	}, nil
}
