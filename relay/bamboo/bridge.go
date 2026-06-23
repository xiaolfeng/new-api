package bamboo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	bamboosdk "github.com/bamboo-services/bamboo-messages/bamboo"
	bamboocodec "github.com/bamboo-services/bamboo-messages/bamboo/codec"
	bamboorelay "github.com/bamboo-services/bamboo-messages/bamboo/relay"
	"github.com/bamboo-services/bamboo-messages/provider"
	// 空白 import 触发各 codec 子包的 init() 注册。
	// codec.Get 依赖包级变量（registry.go:9-21）由子包 init() 赋值，
	// 不显式 import 子包会导致 codec.Get 返回 nil。
	_ "github.com/bamboo-services/bamboo-messages/bamboo/codec/anthropic"
	_ "github.com/bamboo-services/bamboo-messages/bamboo/codec/gemini"
	_ "github.com/bamboo-services/bamboo-messages/bamboo/codec/openai"
	_ "github.com/bamboo-services/bamboo-messages/bamboo/codec/responses"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
)

// errStreamErrorNoDetail 用于 event.Error 为 nil 但事件类型为 EventError 的兜底。
var errStreamErrorNoDetail = errors.New("bamboo stream error event without detail")

// maxResponseBodyLen 限制 info.ResponseBody 的最大长度，防止超大日志撑爆数据库。
const maxResponseBodyLen = 50000

// truncateResponseBody 按字节截断超长响应体，回退到最后一个合法 UTF-8 边界并追加截断标记。
func truncateResponseBody(body string) string {
	if len(body) <= maxResponseBodyLen {
		return body
	}
	cut := maxResponseBodyLen
	// 回退到最后一个合法的 UTF-8 起始字节，避免截断多字节字符中间
	for cut > 0 && !utf8.RuneStart(body[cut]) {
		cut--
	}
	return body[:cut] + "\n...[truncated]"
}

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

	// debug 收集：当 EnableBambooDebugLog 开启时，用 FormatRelayInput/FormatRelayParsed
	// 收集格式化 debug 字符串，写入 context key 供日志详情展示。
	// 不再调用 provider.SetDebug(true)，避免 log.Printf 刷屏。
	debugEnabled := model_setting.GetBambooSettings().EnableBambooDebugLog
	var debugBuf strings.Builder

	relayReq, parseErr := entryCodec.ParseRequest(requestBody)
	if parseErr != nil {
		flushBambooDebug(info, &debugBuf, debugEnabled)
		return nil, translateCodecError(parseErr) // 内部 errors.As 断言 *CodecError
	}

	info.BambooRelayData = extractBambooRelayData(relayReq)

	if debugEnabled {
		debugBuf.WriteString(bamboorelay.FormatRelayParsed("ChatRelay", codecFmt, relayReq))
		debugBuf.WriteByte('\n')
	}

	// ② 上游侧：根据 ApiType 构造 bamboo provider
	p, upstreamRelayFormat, provErr := newProvider(c, info)
	if provErr != nil {
		flushBambooDebug(info, &debugBuf, debugEnabled)
		return nil, provErr // 含 ErrUnsupportedProvider，调用方判 errors.Is 做 fallback
	}

	// 确定有效的上游格式：provider 解析结果优先，为空时回退到入口格式
	effectiveUpstreamFormat := upstreamRelayFormat
	if effectiveUpstreamFormat == "" {
		effectiveUpstreamFormat = entryFormat
	}

	// 更新 RelayInfo 格式链路，供 billing/log/relay-output 下游消费
	info.AppendRequestConversion(effectiveUpstreamFormat)
	info.FinalRequestRelayFormat = effectiveUpstreamFormat

	if debugEnabled {
		// 使用真实上游格式作为 out 参数（而非入口格式）
		outCodecFmt, _ := relayFormatToCodec(effectiveUpstreamFormat)
		debugBuf.WriteString(bamboorelay.FormatRelayInput("ChatRelay", codecFmt, outCodecFmt, requestBody))
		debugBuf.WriteByte('\n')

		debugBuf.WriteString(provider.FormatDebugRequest(
			"bamboo-bridge",
			fmt.Sprintf("upstream provider=%T model=%s relayFormat=%s", p, relayReq.Config.Model, effectiveUpstreamFormat),
			nil, relayReq.Config,
		))
	}
	flushBambooDebug(info, &debugBuf, debugEnabled)

	client := bamboosdk.NewClient(p)

	// ③ 出口侧：按入口 codec 序列化响应
	if relayReq.IsStream {
		return doStreamRelay(c, info, client, entryCodec, codecFmt, relayReq)
	}
	return doCompleteRelay(c, info, client, entryCodec, relayReq)
}

func flushBambooDebug(info *relaycommon.RelayInfo, buf *strings.Builder, enabled bool) {
	if enabled && buf.Len() > 0 {
		info.BambooDebug = buf.String()
	}
}

// doStreamRelay 消费 bamboo StreamEvent，按入口 codec 序列化为出口 SSE。
//
// 当渠道配置了平滑缓冲（BambooSmoothLevel != off）时，序列化后的 SSE 帧
// 经 SmoothPacer 切分为微帧并按自适应间隔匀速释放，再写入 HTTP Response；
// 否则直接透传上游事件。
func doStreamRelay(c *gin.Context, info *relaycommon.RelayInfo, client bamboosdk.BambooClient,
	entryCodec bamboocodec.Codec, outFmt bamboocodec.FormatType, req *bamboocodec.RelayRequest) (*dto.Usage, *types.NewAPIError) {

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

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
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	serializer := entryCodec.NewSerializer()
	var usage dto.Usage

	collector := newBambooTimingCollector()

	var streamItems []string

	// 平滑缓冲：当渠道配置了有效档位时，序列化输出经 SmoothPacer 匀速释放
	smoothLevel := resolveSmoothLevel()
	var smooth *smoothBufferWriter
	if smoothLevel != dto.BambooSmoothLevelOff {
		writeFn := func(data []byte) bool {
			if _, werr := c.Writer.Write(data); werr != nil {
				return false
			}
			c.Writer.Flush()
			return true
		}
		smooth = startSmoothBuffer(ctx, outFmt, smoothLevel, writeFn)
	}

	// writeSSE 写入一帧 SSE 数据（平滑缓冲时推入 pacer，否则直接写）
	writeSSE := func(data []byte) bool {
		if smooth != nil {
			smooth.push(data)
			return true
		}
		if _, werr := c.Writer.Write(data); werr != nil {
			return false
		}
		c.Writer.Flush()
		return true
	}

	for event := range eventCh {
		if event.Type == bamboosdk.EventError {
			cause := errStreamErrorNoDetail
			if event.Error != nil {
				cause = event.Error
			}
			if smooth != nil {
				smooth.wait()
			}
			return nil, types.NewError(cause, types.ErrorCodeBadResponseBody)
		}

		collector.observe(event)

		accumulateReasoningFromEvent(info.OriginModelName, &usage, &event)

		data, serr := serializer.Serialize(event)
		if serr != nil {
			if smooth != nil {
				smooth.wait()
			}
			return nil, translateCodecError(serr)
		}

		streamItems = append(streamItems, string(data))

		// TTFT：首个有效事件序列化成功后记录首次响应时间（幂等，仅首次生效）。
		// 与原生路径 stream_scanner.go 行为一致：在上游数据到达时标记，而非客户端实际收到时。
		// 即使 SmoothPacer 开启（输出被缓冲延迟），TTFT 仍反映上游真实首字延迟。
		info.SetFirstResponseTime()

		if !writeSSE(data) {
			break
		}

		if event.Type == bamboosdk.EventMessageDelta && event.Usage != nil {
			usage.PromptTokens = int(event.Usage.InputTokens)
			usage.CompletionTokens = int(event.Usage.OutputTokens)
			usage.PromptTokensDetails.CachedTokens = int(event.Usage.CacheReadInputTokens)
			usage.PromptTokensDetails.CachedCreationTokens = int(event.Usage.CacheCreationInputTokens)
		}
	}

	// flush 剩余缓冲（如 OpenAI codec 的 [DONE] 终止符）
	tail, _ := serializer.Flush()
	if len(tail) > 0 {
		writeSSE(tail)
		streamItems = append(streamItems, string(tail))
	}

	// 通知 pacer 上游结束，等待所有微帧排空
	if smooth != nil {
		smooth.signalEnd()
		smooth.wait()
	}

	if len(streamItems) > 0 {
		info.ResponseBody = truncateResponseBody(strings.Join(streamItems, "\n"))
	}

	if timingResult := collector.result(); !timingResult.IsZero() {
		result := timingResult
		info.BambooTiming = &result
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
func doCompleteRelay(c *gin.Context, info *relaycommon.RelayInfo, client bamboosdk.BambooClient,
	entryCodec bamboocodec.Codec, req *bamboocodec.RelayRequest) (*dto.Usage, *types.NewAPIError) {

	resp, err := client.Complete(c.Request.Context(), req.Messages, req.System, req.Config)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeDoRequestFailed)
	}

	info.SetFirstResponseTime()

	// 空响应检测：content 为空且 output_tokens 为 0 时标记，供 controller 触发重试。
	// 参考 service/text_quota.go 的 CompletionTokens==0 && PromptTokens>0 判断，
	// 这里在 bamboo 非流式结果上通过 Response.Content 长度 + Usage.OutputTokens 检测。
	if len(resp.Content) == 0 && resp.Usage.OutputTokens == 0 {
		common.SetContextKey(c, constant.ContextKeyEmptyResponse, true)
	}

	body, serr := entryCodec.SerializeResponse(resp)
	if serr != nil {
		return nil, translateCodecError(serr)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Write(body)

	info.ResponseBody = truncateResponseBody(string(body))

	return &dto.Usage{
		PromptTokens:     int(resp.Usage.InputTokens),
		CompletionTokens: int(resp.Usage.OutputTokens),
		TotalTokens:      int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         int(resp.Usage.CacheReadInputTokens),
			CachedCreationTokens: int(resp.Usage.CacheCreationInputTokens),
		},
	}, nil
}

func extractBambooRelayData(req *bamboocodec.RelayRequest) *relaycommon.BambooRelayExtract {
	if req == nil {
		return nil
	}
	extract := &relaycommon.BambooRelayExtract{
		System: req.System,
	}
	for _, msg := range req.Messages {
		msgExt := relaycommon.BambooMessageExtract{
			Role: string(msg.Role),
		}
		for _, block := range msg.Content {
			ext := relaycommon.BambooBlockExtract{Type: string(block.BlockType())}
			switch b := block.(type) {
			case *bamboosdk.TextBlock:
				ext.Text = b.Text
			case *bamboosdk.ThinkingBlock:
				ext.Thinking = b.Thinking
			case *bamboosdk.ToolUseBlock:
				ext.ToolID = b.ID
				ext.ToolName = b.Name
				ext.ToolInput = b.Input
			case *bamboosdk.ToolResultBlock:
				ext.ToolID = b.ToolUseID
				ext.ToolName = b.ToolName
				ext.ToolResult = b.Content
				ext.IsError = b.IsError
			}
			msgExt.Blocks = append(msgExt.Blocks, ext)
		}
		extract.Messages = append(extract.Messages, msgExt)
	}
	return extract
}
